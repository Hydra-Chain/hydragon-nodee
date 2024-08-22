package polybft

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"sort"
	"strings"

	"github.com/0xPolygon/polygon-edge/bls"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/bitmap"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/validator"
	"github.com/0xPolygon/polygon-edge/helper/hex"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/abi"
	bolt "go.etcd.io/bbolt"
)

var (
	bigZero          = big.NewInt(0)
	validatorTypeABI = abi.MustNewType(
		"tuple(uint256[4] blsKey, uint256 stake, bool isWhitelisted, bool isActive)",
	)
)

// StakeManager interface provides functions for handling stake change of validators
// and updating validator set based on changed stake
type StakeManager interface {
	EventSubscriber
	PostBlock(req *PostBlockRequest) error
	UpdateValidatorSet(
		epoch uint64,
		currentValidators validator.AccountSet,
	) (*validator.ValidatorSetDelta, error)
}

var _ StakeManager = (*dummyStakeManager)(nil)

// dummyStakeManager is a dummy implementation of StakeManager interface
// used only for unit testing
type dummyStakeManager struct{}

func (d *dummyStakeManager) PostBlock(req *PostBlockRequest) error { return nil }
func (d *dummyStakeManager) UpdateValidatorSet(epoch uint64,
	currentValidators validator.AccountSet) (*validator.ValidatorSetDelta, error) {
	return &validator.ValidatorSetDelta{}, nil
}

// EventSubscriber implementation
func (d *dummyStakeManager) GetLogFilters() map[types.Address][]types.Hash {
	return make(map[types.Address][]types.Hash)
}
func (d *dummyStakeManager) ProcessLog(header *types.Header, log *ethgo.Log, dbTx *bolt.Tx) error {
	return nil
}

var _ StakeManager = (*stakeManager)(nil)

// stakeManager saves StakeChanged events that happened in each block
// and calculates updated validator set based on changed stake
type stakeManager struct {
	logger               hclog.Logger
	state                *State
	key                  ethgo.Key
	hydraStakingContract types.Address
	maxValidatorSetSize  int
	polybftBackend       polybftBackend
	// Hydra modify: Gives access to HydraChain contract state at specific block
	blockchain BlockchainBackend
}

// newStakeManager returns a new instance of stake manager
func newStakeManager(
	logger hclog.Logger,
	state *State,
	key ethgo.Key,
	hydraStakingAddr types.Address,
	maxValidatorSetSize int,
	polybftBackend polybftBackend,
	dbTx *bolt.Tx,
	blockchain BlockchainBackend,
) (*stakeManager, error) {
	sm := &stakeManager{
		logger:               logger,
		state:                state,
		key:                  key,
		hydraStakingContract: hydraStakingAddr,
		maxValidatorSetSize:  maxValidatorSetSize,
		polybftBackend:       polybftBackend,
		blockchain:           blockchain,
	}

	if err := sm.init(dbTx); err != nil {
		return nil, err
	}

	return sm, nil
}

// PostBlock is called on every insert of finalized block (either from consensus or syncer)
// It will read any StakeChanged event that happened in block and update full validator set in db
// Note that EventSubscriber - AddLog will get all the stakeChanged events that happened in block
func (s *stakeManager) PostBlock(req *PostBlockRequest) error {
	fullValidatorSet, err := s.getOrInitValidatorSet(req.DBTx)
	if err != nil {
		return err
	}

	blockHeader := req.FullBlock.Block.Header
	blockNumber := blockHeader.Number

	s.logger.Debug("Stake manager on post block",
		"block", blockNumber,
		"last saved", fullValidatorSet.BlockNumber,
		"last updated", fullValidatorSet.UpdatedAtBlockNumber)

	// we should save new state even if number of events is zero
	// because otherwise next time we will process more blocks
	fullValidatorSet.EpochID = req.Epoch
	fullValidatorSet.BlockNumber = blockNumber

	return s.state.StakeStore.insertFullValidatorSet(fullValidatorSet, req.DBTx)
}

func (s *stakeManager) init(dbTx *bolt.Tx) error {
	currentHeader := s.blockchain.CurrentHeader()
	currentBlockNumber := currentHeader.Number

	fullValidatorSet, err := s.getOrInitValidatorSet(dbTx)
	if err != nil {
		return err
	}

	// early return if current block is already processed
	if fullValidatorSet.BlockNumber == currentBlockNumber {
		return nil
	}

	// retrieve epoch needed for state
	epochID, err := getEpochID(s.blockchain, currentHeader)
	if err != nil {
		return err
	}

	s.logger.Debug("Stake manager on post block",
		"block", currentBlockNumber,
		"last saved", fullValidatorSet.BlockNumber,
		"last updated", fullValidatorSet.UpdatedAtBlockNumber)

	// we will use eventsGetter to update the fullValidatorSet if
	// for any reason, we don't have the correct state
	eventsGetter := &eventsGetter[*contractsapi.BalanceChangedEvent]{
		receiptsGetter: receiptsGetter{
			blockchain: s.blockchain,
		},
		isValidLogFn: func(l *types.Log) bool {
			return l.Address == s.hydraStakingContract
		},
		parseEventFn: func(h *types.Header, l *ethgo.Log) (*contractsapi.BalanceChangedEvent, bool, error) {
			var balanceChangedEvent contractsapi.BalanceChangedEvent
			doesMatch, err := balanceChangedEvent.ParseLog(l)

			return &balanceChangedEvent, doesMatch, err
		},
	}

	stakeChangedEvents, err := eventsGetter.getEventsFromBlocksRange(
		fullValidatorSet.BlockNumber+1,
		currentBlockNumber,
	)
	if err != nil {
		return err
	}

	if err := s.updateWithReceipts(&fullValidatorSet, stakeChangedEvents, currentHeader); err != nil {
		return err
	}

	// we should save new state even if number of events is zero
	// because otherwise next time we will process more blocks
	fullValidatorSet.EpochID = epochID
	fullValidatorSet.BlockNumber = currentBlockNumber

	return s.state.StakeStore.insertFullValidatorSet(fullValidatorSet, dbTx)
}

func (s *stakeManager) getOrInitValidatorSet(dbTx *bolt.Tx) (validatorSetState, error) {
	fullValidatorSet, err := s.state.StakeStore.getFullValidatorSet(dbTx)
	if err != nil {
		if !errors.Is(err, errNoFullValidatorSet) {
			return validatorSetState{}, err
		}

		validators, err := s.polybftBackend.GetValidatorsWithTx(0, nil, dbTx)
		if err != nil {
			return validatorSetState{}, err
		}

		fullValidatorSet = validatorSetState{
			BlockNumber:          0,
			EpochID:              0,
			UpdatedAtBlockNumber: 0,
			Validators:           newValidatorStakeMap(validators),
		}

		if err = s.state.StakeStore.insertFullValidatorSet(fullValidatorSet, dbTx); err != nil {
			return validatorSetState{}, err
		}
	}

	return fullValidatorSet, nil
}

func (s *stakeManager) updateWithReceipts(
	fullValidatorSet *validatorSetState,
	balanceChangedEvents []*contractsapi.BalanceChangedEvent,
	blockHeader *types.Header) error {
	if len(balanceChangedEvents) == 0 {
		return nil
	}

	systemState, err := s.getSystemStateForBlock(blockHeader)
	if err != nil {
		return err
	}

	exponent, err := systemState.GetVotingPowerExponent()
	for _, event := range balanceChangedEvents {
		s.logger.Debug(
			"BalanceChanged event",
			"validator:", event.Account,
			"new stake balance:", event.NewBalance,
			"exponent:", exponent,
		)

		// update the stake
		fullValidatorSet.Validators.setStake(event.Account, event.NewBalance, exponent)
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

	s.logger.Debug(
		"HydraChain state after",
		"block",
		blockNumber,
		"data",
		fullValidatorSet.Validators,
	)

	return nil
}

// UpdateValidatorSet returns an updated list of validators
// based on balance change (transfer) events from HydraChain contract
func (s *stakeManager) UpdateValidatorSet(
	epoch uint64, oldValidators validator.AccountSet) (*validator.ValidatorSetDelta, error) {
	s.logger.Info("Calculating validators set update...", "epoch", epoch)

	fullValidatorSet, err := s.state.StakeStore.getFullValidatorSet(nil)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to get full validators set. Epoch: %d. Error: %w",
			epoch,
			err,
		)
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

	for i, validator := range oldValidators {
		oldActiveMap[validator.Address] = validator
		// remove existing validators from the validators list if they did not make it to the list
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
		newValidatorSet, err := oldValidators.Copy().ApplyDelta(delta)
		if err != nil {
			return nil, err
		}

		s.logger.Debug("New validator set", "validatorSet", newValidatorSet)
	}

	return delta, nil
}

// Hydra modification: getBlsKey returns bls key for validator from the HydraChain contract
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

// EventSubscriber implementation

// GetLogFilters returns a map of log filters for getting desired events,
// where the key is the address of contract that emits desired events,
// and the value is a slice of signatures of events we want to get.
// This function is the implementation of EventSubscriber interface
func (s *stakeManager) GetLogFilters() map[types.Address][]types.Hash {
	var balanceChangedEvent contractsapi.BalanceChangedEvent

	return map[types.Address][]types.Hash{
		s.hydraStakingContract: {types.Hash(balanceChangedEvent.Sig())},
	}
}

// ProcessLog is the implementation of EventSubscriber interface,
// used to handle a log defined in GetLogFilters, provided by event provider
func (s *stakeManager) ProcessLog(header *types.Header, log *ethgo.Log, dbTx *bolt.Tx) error {
	var balanceChangedEvent contractsapi.BalanceChangedEvent

	doesMatch, err := balanceChangedEvent.ParseLog(log)
	if err != nil {
		return err
	}

	if !doesMatch {
		return nil
	}

	fullValidatorSet, err := s.getOrInitValidatorSet(dbTx)
	if err != nil {
		return err
	}

	if err := s.updateWithReceipts(&fullValidatorSet,
		[]*contractsapi.BalanceChangedEvent{&balanceChangedEvent}, header); err != nil {
		return err
	}

	return s.state.StakeStore.insertFullValidatorSet(fullValidatorSet, dbTx)
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
func (sc *validatorStakeMap) setStake(
	address types.Address,
	stakedBalance *big.Int,
	exponent *BigNumDecimal,
) {
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
	vpower := math.Pow(
		float64(stakedH.Uint64()),
		float64(exp.Numerator.Uint64())/float64(exp.Denominator.Uint64()),
	)
	res := big.NewInt(int64(vpower))

	return res
}

func getEpochID(blockchain BlockchainBackend, header *types.Header) (uint64, error) {
	provider, err := blockchain.GetStateProviderForBlock(header)
	if err != nil {
		return 0, err
	}

	return blockchain.GetSystemState(provider).GetEpoch()
}
