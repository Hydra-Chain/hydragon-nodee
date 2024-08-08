package priceoracle

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/0xPolygon/polygon-edge/blockchain"
	"github.com/0xPolygon/polygon-edge/consensus"
	"github.com/0xPolygon/polygon-edge/consensus/polybft"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/validator"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/wallet"
	"github.com/0xPolygon/polygon-edge/secrets"
	"github.com/0xPolygon/polygon-edge/state"
	"github.com/0xPolygon/polygon-edge/txrelayer"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/contract"
)

var alreadyVotedMapping = make(map[uint64]bool)

type blockchainBackend interface {
	// CurrentHeader returns the header of blockchain block head
	CurrentHeader() *types.Header
	// GetStateProviderForBlock returns a reference to make queries to the state at 'block'.
	GetStateProviderForBlock(block *types.Header) (contract.Provider, error)
	// GetSystemState creates a new instance of SystemState interface
	GetSystemState(provider contract.Provider) polybft.SystemState
	// SubscribeEvents subscribes to blockchain events
	SubscribeEvents() blockchain.Subscription
	// UnubscribeEvents unsubscribes from blockchain events
	UnubscribeEvents(subscription blockchain.Subscription)
}

// polybftBackend is an interface defining polybft methods needed by fsm and sync tracker
type polybftBackend interface {
	// GetValidators retrieves validator set for the given block
	GetValidators(blockNumber uint64, parents []*types.Header) (validator.AccountSet, error)
}

type PriceOracle struct {
	logger hclog.Logger
	// closeCh is used to signal that the price oracle is stopped
	closeCh        chan struct{}
	blockchain     blockchainBackend
	polybftBackend polybftBackend
	// key encapsulates ECDSA signing account
	account   *wallet.Account
	jsonRPC   string
	priceFeed PriceFeed
}

func NewPriceOracle(
	logger hclog.Logger,
	blockchain *blockchain.Blockchain,
	executor *state.Executor,
	consensus consensus.Consensus,
	jsonRPC string,
	secretsManager secrets.SecretsManager,
) (*PriceOracle, error) {
	priceFeed := NewDummyPriceFeed()
	polybftConsensus, ok := consensus.(polybftBackend)
	if !ok {
		return nil, fmt.Errorf("consensus must be hydragon")
	}

	// read account
	account, err := wallet.NewAccountFromSecret(secretsManager)
	if err != nil {
		return nil, fmt.Errorf("failed to read account data. Error: %w", err)
	}

	return &PriceOracle{
		logger:         logger.Named("price-oracle"),
		blockchain:     polybft.NewBlockchainBackend(executor, blockchain),
		priceFeed:      priceFeed,
		polybftBackend: polybftConsensus,
		jsonRPC:        jsonRPC,
		account:        account,
	}, nil
}

func (p *PriceOracle) Start() error {
	p.logger.Info("starting price oracle")

	go p.StartOracleProcess()

	return nil
}

func (p *PriceOracle) StartOracleProcess() {
	newBlockSub := p.blockchain.SubscribeEvents()
	defer p.blockchain.UnubscribeEvents(newBlockSub)

	eventCh := newBlockSub.GetEventCh()

	for {
		select {
		case <-p.closeCh:
			return
		case ev := <-eventCh:
			if !p.blockMustBeProcessed(ev) {
				continue
			}

			block := ev.NewChain[0]
			currentValidators, err := p.polybftBackend.GetValidators(block.Number, nil)
			if err != nil {
				p.logger.Error(
					"failed to query current validator set",
					"block number",
					block.Number,
					"error",
					err,
				)

				continue
			}

			isValidator := currentValidators.ContainsNodeID(p.account.Address().String())
			if !isValidator {
				continue
			}

			p.logger.Debug("received new block notification", "block", block.Number)

			if err = p.handleNewBlock(block); err != nil {
				p.logger.Error("failed to handle new block", "err", err)
			}
		}
	}
}

func (p *PriceOracle) Close() error {
	close(p.closeCh)
	p.logger.Info("price oracle stopped")

	return nil
}

func (p *PriceOracle) handleNewBlock(header *types.Header) error {
	if !isVotingTime(header.Timestamp) {
		p.logger.Debug("Not currently in voting time window")

		return nil
	}

	should, err := p.shouldExecuteVote(header)
	if err != nil {
		return fmt.Errorf("failed to check if vote must be executed:  error %w", err)
	}

	if should {
		return p.executeVote(header)
	}

	// Continue: Make a contract check to ensure it is a proper decision to vote,
	// keep info if already voted or consensus already made about specific date
	// so you don't have to continue trying to process in such case

	return nil
}

// getSystemState builds SystemState instance for the given header
func (p *PriceOracle) shouldExecuteVote(header *types.Header) (bool, error) {
	// first check is voting already made for the current day
	if p.alreadyVoted(header) {
		return false, nil
	}

	// then check if the contract is in a proper state to vote
	state, err := p.getState(header)
	if err != nil {
		return false, fmt.Errorf("get system state: %w", err)
	}

	shouldVote, falseReason, err := state.shouldVote(p.account.Address().String(), calcDayNumber(header.Timestamp))
	if err != nil {
		return false, err
	}

	if !shouldVote {
		p.logger.Debug("should not vote", "reason", falseReason)

		return false, nil
	}

	return true, nil
}

// 1. Skip checking older blocks to ensure bulk synchronization remains fast.
// 2. The blockchain notification system can eventually deliver
// stale block notifications or fork blocks. These should be ignored
// 3. Ignore blocks from forks
func (p *PriceOracle) blockMustBeProcessed(ev *blockchain.Event) bool {
	block := ev.NewChain[0]

	return !isBlockOlderThan(block, 2) &&
		block.Number >= p.blockchain.CurrentHeader().Number && (ev.Type != blockchain.EventFork)
}

func (p *PriceOracle) alreadyVoted(header *types.Header) bool {
	return alreadyVotedMapping[calcDayNumber(header.Timestamp)]
}

// getSystemState builds SystemState instance for the given header
func (p *PriceOracle) executeVote(header *types.Header) error {
	price, err := p.priceFeed.GetPrice(header.Timestamp)
	if err != nil {
		return fmt.Errorf("get price: %w", err)
	}

	err = p.vote(price)
	if err != nil {
		return fmt.Errorf("vote: failed %w", err)
	}

	return nil
}

// getSystemState builds SystemState instance for the given header
func (p *PriceOracle) getState(header *types.Header) (PriceOracleState, error) {
	provider, err := p.blockchain.GetStateProviderForBlock(header)
	if err != nil {
		return nil, err
	}

	return newPriceOracleState(p.blockchain.GetSystemState(provider)), nil
}

func (p *PriceOracle) vote(price *big.Int) error {
	txRelayer, err := txrelayer.NewTxRelayer(
		txrelayer.WithIPAddress(p.jsonRPC), txrelayer.WithReceiptTimeout(150*time.Millisecond))
	if err != nil {
		return err
	}

	// registerFn := &contractsapi.RegisterHydraChainFn{
	// 	Signature: sigMarshal,
	// 	Pubkey:    account.Bls.PublicKey().ToBigInt(),
	// }

	// input, err := registerFn.EncodeAbi()
	// if err != nil {
	// 	return nil, fmt.Errorf("register validator failed: %w", err)
	// }

	txn := &ethgo.Transaction{
		Input: []byte{},
		To:    nil,
	}

	receipt, err := txRelayer.SendTransaction(txn, p.account.Ecdsa)
	if err != nil {
		return err
	}

	if receipt.Status != uint64(types.ReceiptSuccess) {
		return errors.New("register validator transaction failed")
	}

	// result := &registerResult{}
	// foundNewValidatorLog := false

	// for _, log := range receipt.Logs {
	// 	if newValidatorEventABI.Match(log) {
	// 		event, err := newValidatorEventABI.ParseLog(log)
	// 		if err != nil {
	// 			return err
	// 		}

	// 		result.validatorAddress = event["validator"].(ethgo.Address).String() //nolint:forcetypeassert
	// 		result.stakeResult = "No stake parameters have been submitted"
	// 		result.amount = "0"
	// 		foundNewValidatorLog = true
	// 	}
	// }

	// if !foundNewValidatorLog {
	// 	return fmt.Errorf("could not find an appropriate log in the receipt that validates the registration has happened")
	// }

	return nil
}

const (
	dailyVotingStartTime = uint64(0)        // in seconds
	dailyVotingEndTime   = uint64(3 * 3600) // in seconds
)

func isVotingTime(timestamp uint64) bool {
	// Seconds since the start of the day
	secondsInDay := timestamp % 86400

	// Check if the seconds in the day falls between startTimeSeconds and endTimeSeconds
	return secondsInDay >= dailyVotingStartTime && secondsInDay < dailyVotingEndTime
}

func isBlockOlderThan(header *types.Header, minutes int64) bool {
	return time.Now().UTC().Unix()-int64(header.Timestamp) > minutes*60
}

func calcDayNumber(timestamp uint64) uint64 {
	// Number of seconds in a day
	const secondsInADay uint64 = 86400

	// Calculate the current day number
	return timestamp / secondsInADay
}
