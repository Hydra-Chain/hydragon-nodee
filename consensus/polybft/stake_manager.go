package polybft

import (
	"bytes"
	"encoding/json"
	"errors"
	"math"
	"math/big"
	"sort"
	"strings"

	"fmt"

	"github.com/0xPolygon/polygon-edge/consensus/polybft/bitmap"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	bls "github.com/0xPolygon/polygon-edge/consensus/polybft/signer"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/validator"
	"github.com/0xPolygon/polygon-edge/helper/hex"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/abi"
)

var (
	bigZero          = big.NewInt(0)
	validatorTypeABI = abi.MustNewType("tuple(uint256[4] blsKey, uint256 stake, bool isWhitelisted, bool isActive)")
)

// StakeManager interface provides functions for handling stake change of validators
// and updating validator set based on changed stake
type StakeManager interface {
	PostBlock(req *PostBlockRequest) error
	UpdateValidatorSet(epoch uint64, currentValidatorSet validator.AccountSet) (*validator.ValidatorSetDelta, error)
}

// dummyStakeManager is a dummy implementation of StakeManager interface
// used only for unit testing
type dummyStakeManager struct{}

func (d *dummyStakeManager) PostBlock(req *PostBlockRequest) error { return nil }
func (d *dummyStakeManager) UpdateValidatorSet(epoch uint64,
	currentValidatorSet validator.AccountSet) (*validator.ValidatorSetDelta, error) {
	return &validator.ValidatorSetDelta{}, nil
}

var _ StakeManager = (*stakeManager)(nil)

// stakeManager saves StakeChanged events that happened in each block
// and calculates updated validator set based on changed stake
type stakeManager struct {
	logger hclog.Logger
	state  *State
	// Hydra modify: Root chain is unused so remove rootchain relayer
	// rootChainRelayer        txrelayer.TxRelayer
	key ethgo.Key
	// Hydra modify: Root chain is unused so remove supernetManagerContract
	// supernetManagerContract types.Address
	maxValidatorSetSize int
	// Hydra modify: Gives access to ValidatorSet contract state at specific block
	blockchain     blockchainBackend
	eventsGetter   *eventsGetter[*contractsapi.StakeChangedEvent]
	polybftBackend polybftBackend
}

// newStakeManager returns a new instance of stake manager
func newStakeManager(
	logger hclog.Logger,
	state *State,
	key ethgo.Key,
	validatorSetAddr types.Address,
	maxValidatorSetSize int,
	blockchain blockchainBackend,
	polybftBackend polybftBackend,
) (*stakeManager, error) {
	eventsGetter := &eventsGetter[*contractsapi.StakeChangedEvent]{
		blockchain: blockchain,
		isValidLogFn: func(l *types.Log) bool {
			return l.Address == validatorSetAddr
		},
		parseEventFn: func(h *types.Header, l *ethgo.Log) (*contractsapi.StakeChangedEvent, bool, error) {
			var stakeChangedEvent contractsapi.StakeChangedEvent
			doesMatch, err := stakeChangedEvent.ParseLog(l)

			return &stakeChangedEvent, doesMatch, err
		},
	}

	sm := &stakeManager{
		logger:              logger,
		state:               state,
		key:                 key,
		maxValidatorSetSize: maxValidatorSetSize,
		blockchain:          blockchain,
		eventsGetter:        eventsGetter,
		polybftBackend:      polybftBackend,
	}

	if err := sm.init(blockchain); err != nil {
		return nil, err
	}

	return sm, nil
}

// PostBlock is called on every insert of finalized block (either from consensus or syncer)
// It will read any StakeChanged event that happened in block and update full validator set in db
func (s *stakeManager) PostBlock(req *PostBlockRequest) error {
	fullValidatorSet, err := s.getOrInitValidatorSet()
	if err != nil {
		return err
	}

	blockHeader := req.FullBlock.Block.Header
	blockNumber := blockHeader.Number

	s.logger.Debug("Stake manager on post block",
		"block", blockNumber,
		"last saved", fullValidatorSet.BlockNumber,
		"last updated", fullValidatorSet.UpdatedAtBlockNumber)

	// get StakeChanged currentBlockEvents from current block
	stakeChangedEvents, err := s.eventsGetter.getFromBlocks(fullValidatorSet.BlockNumber, req.FullBlock)
	if err != nil {
		return fmt.Errorf("could not get StakeChanged events from current block. Error: %w", err)
	}

	if err = s.updateWithReceipts(&fullValidatorSet, stakeChangedEvents, blockHeader); err != nil {
		return err
	}

	// we should save new state even if number of events is zero
	// because otherwise next time we will process more blocks
	fullValidatorSet.EpochID = req.Epoch
	fullValidatorSet.BlockNumber = blockNumber

	return s.state.StakeStore.insertFullValidatorSet(fullValidatorSet)
}

func (s *stakeManager) init(blockchain blockchainBackend) error {
	currentHeader := blockchain.CurrentHeader()
	currentBlockNumber := currentHeader.Number

	validatorSet, err := s.getOrInitValidatorSet()
	if err != nil {
		return err
	}

	// early return if current block is already processed
	if validatorSet.BlockNumber == currentBlockNumber {
		return nil
	}

	// retrieve epoch needed for state
	epochID, err := getEpochID(blockchain, currentHeader)
	if err != nil {
		return err
	}

	s.logger.Debug("Stake manager on post block",
		"block", currentBlockNumber,
		"last saved", validatorSet.BlockNumber,
		"last updated", validatorSet.UpdatedAtBlockNumber)

	stakeChangedEvents, err := s.eventsGetter.getEventsFromAllBlocks(validatorSet.BlockNumber+1, currentBlockNumber)
	if err != nil {
		return err
	}

	if err := s.updateWithReceipts(&validatorSet, stakeChangedEvents, currentHeader); err != nil {
		return err
	}

	// we should save new state even if number of events is zero
	// because otherwise next time we will process more blocks
	validatorSet.EpochID = epochID
	validatorSet.BlockNumber = currentBlockNumber

	return s.state.StakeStore.insertFullValidatorSet(validatorSet)
}

func (s *stakeManager) getOrInitValidatorSet() (validatorSetState, error) {
	validatorSet, err := s.state.StakeStore.getFullValidatorSet()
	if err != nil {
		if !errors.Is(err, errNoFullValidatorSet) {
			return validatorSetState{}, err
		}

		validators, err := s.polybftBackend.GetValidators(0, nil)
		if err != nil {
			return validatorSetState{}, err
		}

		validatorSet = validatorSetState{
			BlockNumber:          0,
			EpochID:              0,
			UpdatedAtBlockNumber: 0,
			Validators:           newValidatorStakeMap(validators),
		}

		if err = s.state.StakeStore.insertFullValidatorSet(validatorSet); err != nil {
			return validatorSetState{}, err
		}
	}

	return validatorSet, nil
}

func (s *stakeManager) updateWithReceipts(
	fullValidatorSet *validatorSetState,
	stakeChangedEvents []*contractsapi.StakeChangedEvent,
	blockHeader *types.Header) error {
	if len(stakeChangedEvents) == 0 {
		return nil
	}

	systemState, err := s.getSystemStateForBlock(blockHeader)
	if err != nil {
		return err
	}

	exponent, err := systemState.GetVotingPowerExponent()
	if err != nil {
		return err
	}

	for _, event := range stakeChangedEvents {
		s.logger.Debug(
			"StakeChanged event",
			"validator:", event.Validator,
			"new stake balance:", event.NewStake,
			"exponent:", exponent,
		)

		// update the stake
		fullValidatorSet.Validators.setStake(event.Validator, event.NewStake, exponent)
	}

	blockNumber := blockHeader.Number

	for addr, data := range fullValidatorSet.Validators {
		if data.BlsKey == nil {
			blsKey, err := s.getBlsKey(data.Address)
			if err != nil {
				s.logger.Warn("Could not get info for new validator",
					"block", blockNumber, "address", addr)
			}

			data.BlsKey = blsKey
		}

		data.IsActive = data.VotingPower.Cmp(bigZero) > 0
	}

	// mark on which block validator set has been updated
	fullValidatorSet.UpdatedAtBlockNumber = blockNumber

	s.logger.Debug("Full validator set after", "block", blockNumber, "data", fullValidatorSet.Validators)

	return nil
}

// UpdateValidatorSet returns an updated validator set
// based on stake change (transfer) events from ValidatorSet contract
func (s *stakeManager) UpdateValidatorSet(
	epoch uint64, oldValidatorSet validator.AccountSet) (*validator.ValidatorSetDelta, error) {
	s.logger.Info("Calculating validators set update...", "epoch", epoch)

	fullValidatorSet, err := s.state.StakeStore.getFullValidatorSet()
	if err != nil {
		return nil, fmt.Errorf("failed to get full validators set. Epoch: %d. Error: %w", epoch, err)
	}

	// stake map that holds stakes for all validators
	stakeMap := fullValidatorSet.Validators

	// slice of all validator set
	newValidatorSet := stakeMap.getSorted(s.maxValidatorSetSize)
	// set of all addresses that will be in next validator set
	addressesSet := make(map[types.Address]struct{}, len(newValidatorSet))

	for _, si := range newValidatorSet {
		addressesSet[si.Address] = struct{}{}
	}

	removedBitmap := bitmap.Bitmap{}
	updatedValidators := validator.AccountSet{}
	addedValidators := validator.AccountSet{}
	oldActiveMap := make(map[types.Address]*validator.ValidatorMetadata)

	for i, validator := range oldValidatorSet {
		oldActiveMap[validator.Address] = validator
		// remove existing validators from validator set if they did not make it to the set
		if _, exists := addressesSet[validator.Address]; !exists {
			removedBitmap.Set(uint64(i))
		}
	}

	for _, newValidator := range newValidatorSet {
		// check if its already in existing validator set
		if oldValidator, exists := oldActiveMap[newValidator.Address]; exists {
			if oldValidator.VotingPower.Cmp(newValidator.VotingPower) != 0 {
				updatedValidators = append(updatedValidators, newValidator)
			}
		} else {
			if newValidator.BlsKey == nil {
				newValidator.BlsKey, err = s.getBlsKey(newValidator.Address)
				if err != nil {
					return nil, fmt.Errorf("could not retrieve validator data. Address: %v. Error: %w",
						newValidator.Address, err)
				}
			}

			addedValidators = append(addedValidators, newValidator)
		}
	}

	s.logger.Info("Calculating validators set update finished.", "epoch", epoch)

	delta := &validator.ValidatorSetDelta{
		Added:   addedValidators,
		Updated: updatedValidators,
		Removed: removedBitmap,
	}

	if s.logger.IsDebug() {
		newValidatorSet, err := oldValidatorSet.Copy().ApplyDelta(delta)
		if err != nil {
			return nil, err
		}

		s.logger.Debug("New validator set", "validatorSet", newValidatorSet)
	}

	return delta, nil
}

// Hydra modification: getBlsKey returns bls key for validator from the childValidatorSet contract
// getBlsKey returns bls key for validator from the supernet contract
func (s *stakeManager) getBlsKey(address types.Address) (*bls.PublicKey, error) {
	header := s.blockchain.CurrentHeader()

	systemState, err := s.getSystemStateForBlock(header)
	if err != nil {
		return nil, err
	}

	blsKey, err := systemState.GetValidatorBlsKey(address)
	if err != nil {
		return nil, err
	}

	return blsKey, nil
}

func (s *stakeManager) getSystemStateForBlock(block *types.Header) (SystemState, error) {
	provider, err := s.blockchain.GetStateProviderForBlock(block)
	if err != nil {
		return nil, err
	}

	systemState := s.blockchain.GetSystemState(provider)

	return systemState, nil
}

type validatorSetState struct {
	BlockNumber          uint64            `json:"block"`
	EpochID              uint64            `json:"epoch"`
	UpdatedAtBlockNumber uint64            `json:"updated_at_block"`
	Validators           validatorStakeMap `json:"validators"`
}

func (vs validatorSetState) Marshal() ([]byte, error) {
	return json.Marshal(vs)
}

func (vs *validatorSetState) Unmarshal(b []byte) error {
	return json.Unmarshal(b, vs)
}

// validatorStakeMap holds ValidatorMetadata for each validator address
type validatorStakeMap map[types.Address]*validator.ValidatorMetadata

// newValidatorStakeMap returns a new instance of validatorStakeMap
func newValidatorStakeMap(validatorSet validator.AccountSet) validatorStakeMap {
	stakeMap := make(validatorStakeMap, len(validatorSet))

	for _, v := range validatorSet {
		stakeMap[v.Address] = v.Copy()
	}

	return stakeMap
}

// Hydra modification: Calculate voting power with our own formula
// Set is active flag based on voting power and not on staked amount
// setStake sets given amount of stake to a validator defined by address
func (sc *validatorStakeMap) setStake(address types.Address, stakedBalance *big.Int, exponent *BigNumDecimal) {
	votingPower := sc.calcVotingPower(stakedBalance, exponent)

	isActive := votingPower.Cmp(bigZero) > 0

	if metadata, exists := (*sc)[address]; exists {
		metadata.VotingPower = votingPower
		metadata.IsActive = isActive
	} else {
		(*sc)[address] = &validator.ValidatorMetadata{
			VotingPower: votingPower,
			Address:     address,
			IsActive:    isActive,
		}
	}
}

// getSorted returns validators (*ValidatorMetadata) in sorted order
func (sc validatorStakeMap) getSorted(maxValidatorSetSize int) validator.AccountSet {
	activeValidators := make(validator.AccountSet, 0, len(sc))

	for _, v := range sc {
		if v.VotingPower.Cmp(bigZero) > 0 {
			activeValidators = append(activeValidators, v)
		}
	}

	sort.Slice(activeValidators, func(i, j int) bool {
		v1, v2 := activeValidators[i], activeValidators[j]

		switch v1.VotingPower.Cmp(v2.VotingPower) {
		case 1:
			return true
		case 0:
			return bytes.Compare(v1.Address[:], v2.Address[:]) < 0
		default:
			return false
		}
	})

	if len(activeValidators) <= maxValidatorSetSize {
		return activeValidators
	}

	return activeValidators[:maxValidatorSetSize]
}

func (sc validatorStakeMap) String() string {
	var sb strings.Builder

	for _, x := range sc.getSorted(len(sc)) {
		bls := ""
		if x.BlsKey != nil {
			bls = hex.EncodeToString(x.BlsKey.Marshal())
		}

		sb.WriteString(fmt.Sprintf("%s:%s:%s:%t\n",
			x.Address, x.VotingPower, bls, x.IsActive))
	}

	return sb.String()
}

// calcVotingPower calculates voting power for a given staked balance
func (sc *validatorStakeMap) calcVotingPower(stakedBalance *big.Int, exp *BigNumDecimal) *big.Int {
	// in case validator unstaked its full balance
	if stakedBalance.Cmp(bigZero) == 0 {
		return bigZero
	}

	stakedH := big.NewInt(0).Div(stakedBalance, big.NewInt(1e18))
	vpower := math.Pow(float64(stakedH.Uint64()), float64(exp.Numerator.Uint64())/float64(exp.Denominator.Uint64()))
	res := big.NewInt(int64(vpower))

	return res
}

func getEpochID(blockchain blockchainBackend, header *types.Header) (uint64, error) {
	provider, err := blockchain.GetStateProviderForBlock(header)
	if err != nil {
		return 0, err
	}

	return blockchain.GetSystemState(provider).GetEpoch()
}
