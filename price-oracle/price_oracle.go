package priceoracle

import (
	"errors"
	"fmt"
	"math/big"
	"net"
	"time"

	"github.com/0xPolygon/polygon-edge/blockchain"
	"github.com/0xPolygon/polygon-edge/consensus"
	"github.com/0xPolygon/polygon-edge/consensus/polybft"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/validator"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/wallet"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/secrets"
	"github.com/0xPolygon/polygon-edge/state"
	"github.com/0xPolygon/polygon-edge/txrelayer"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/hashicorp/go-hclog"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/contract"
)

var (
	alreadyVotedMapping = make(map[uint64]bool)
	txRelayer           txrelayer.TxRelayer
	priceVotedEventABI  = contractsapi.PriceOracle.Abi.Events["PriceVoted"]
)

// emit PriceVoted(_price, msg.sender, day);
type voteResult struct {
	price            string
	validatorAddress string
	day              uint64
}

func (vr voteResult) PrintOutput() {
	fmt.Printf("\n[VOTE]\n")
	fmt.Println("Validator Address: ", vr.validatorAddress)
	fmt.Println("Voted Price: ", vr.price)
	fmt.Println("Day: ", vr.day)
	fmt.Printf("\n")
}

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
	priceFeed, err := NewPriceFeed()
	if err != nil {
		return nil, err
	}

	polybftConsensus, ok := consensus.(polybftBackend)
	if !ok {
		return nil, fmt.Errorf("consensus must be hydragon")
	}

	// read account
	account, err := wallet.NewAccountFromSecret(secretsManager)
	if err != nil {
		return nil, fmt.Errorf("failed to read account data. Error: %w", err)
	}

	formattedURL, err := formatJSONRPCURL(jsonRPC)
	if err != nil {
		return nil, err
	}

	return &PriceOracle{
		logger:         logger.Named("price-oracle"),
		blockchain:     polybft.NewBlockchainBackend(executor, blockchain),
		priceFeed:      priceFeed,
		polybftBackend: polybftConsensus,
		jsonRPC:        formattedURL,
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
			block := ev.NewChain[0]
			p.logger.Debug("received new block notification", "block", block.Number)

			isValidator, err := p.isValidator(block)
			if err != nil {
				p.logger.Error("failed to check if node is validator", "err", err)

				continue
			}

			if !isValidator {
				continue
			}

			if !p.blockMustBeProcessed(ev) {
				continue
			}

			should, err := p.shouldExecuteVote(block)
			if err != nil {
				p.logger.Error("failed to check if vote must be executed:", "error", err)

				continue
			}

			if should {
				if err := p.executeVote(block); err != nil {
					p.logger.Error("failed to execute vote", "err", err)

					return
				}

				p.logger.Info("vote executed successfully")
			}
		}
	}
}

func (p *PriceOracle) Close() error {
	close(p.closeCh)
	p.logger.Info("price oracle stopped")

	return nil
}

// shouldExecuteVote verifies that the validator should vote
func (p *PriceOracle) shouldExecuteVote(header *types.Header) (bool, error) {
	// check if the current time is in the voting window
	if !isVotingTime(header.Timestamp) {
		p.logger.Debug("Not currently in voting time window")

		return false, nil
	}

	// check is voting already made for the current day
	if p.alreadyVoted(header) { // vito
		return false, nil
	}

	// initialize the system state for the given header
	state, err := p.getState(header)
	if err != nil {
		return false, fmt.Errorf("get system state: %w", err)
	}

	// then check if the contract is in a proper state to vote
	shouldVote, falseReason, err := state.shouldVote(
		p.account,
		p.jsonRPC,
	)
	if err != nil {
		return false, err
	}

	if !shouldVote {
		p.logger.Debug("should not vote", "reason", falseReason)

		return false, nil
	}

	return true, nil
}

func (p *PriceOracle) isValidator(block *types.Header) (bool, error) {
	currentValidators, err := p.polybftBackend.GetValidators(block.Number, nil)
	if err != nil {
		return false, fmt.Errorf(
			"failed to query current validator set, block number %d, error %w",
			block.Number,
			err,
		)
	}

	return currentValidators.ContainsNodeID(p.account.Address().String()), nil
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

// executeVote get the price from the price feed and votes
func (p *PriceOracle) executeVote(header *types.Header) error {
	price, err := p.priceFeed.GetPrice(header)
	if err != nil {
		return fmt.Errorf("get price: %w", err)
	}

	err = p.vote(price)
	if err != nil {
		return fmt.Errorf("vote: failed %w", err)
	}

	alreadyVotedMapping[calcDayNumber(header.Timestamp)] = true

	return nil
}

// getState builds SystemState instance for the given header
func (p *PriceOracle) getState(header *types.Header) (PriceOracleState, error) {
	provider, err := p.blockchain.GetStateProviderForBlock(header)
	if err != nil {
		return nil, err
	}

	return newPriceOracleState(p.blockchain.GetSystemState(provider), contracts.PriceOracleContract, provider), nil
}

func (p *PriceOracle) vote(price *big.Int) error {
	txRelayer, err := NewTxRelayer(p.jsonRPC)
	if err != nil {
		return err
	}

	voteFn := &contractsapi.VotePriceOracleFn{
		Price: price,
	}
	input, err := voteFn.EncodeAbi()
	if err != nil {
		return err
	}

	txn := &ethgo.Transaction{
		From:  p.account.Ecdsa.Address(),
		Input: input,
		To:    (*ethgo.Address)(&contracts.PriceOracleContract),
	}

	receipt, err := txRelayer.SendTransaction(txn, p.account.Ecdsa)
	if err != nil {
		return err
	}

	if receipt.Status != uint64(types.ReceiptSuccess) {
		return errors.New("vote transaction failed")
	}

	result := &voteResult{}
	foundVoteLog := false

	for _, log := range receipt.Logs {
		if priceVotedEventABI.Match(log) {
			event, err := priceVotedEventABI.ParseLog(log)
			if err != nil {
				return err
			}

			result.price = event["price"].(*big.Int).String()                     //nolint:forcetypeassert
			result.validatorAddress = event["validator"].(ethgo.Address).String() //nolint:forcetypeassert
			result.day = event["day"].(*big.Int).Uint64()                         //nolint:forcetypeassert

			foundVoteLog = true
		}
	}

	if !foundVoteLog {
		return fmt.Errorf("could not find an appropriate log in the receipt that validates the vote has happened")
	}

	result.PrintOutput()

	return nil
}

func NewTxRelayer(jsoNRPC string) (txrelayer.TxRelayer, error) {
	txRelayer, err := txrelayer.NewTxRelayer(
		txrelayer.WithIPAddress(jsoNRPC), txrelayer.WithReceiptTimeout(150*time.Millisecond))
	if err != nil {
		return nil, err
	}

	return txRelayer, nil
}

func formatJSONRPCURL(jsonRPC string) (string, error) {
	_, port, err := net.SplitHostPort(jsonRPC)
	if err != nil {
		return "", err
	}

	formattedURL := fmt.Sprintf("http://127.0.0.1:%s", port)

	return formattedURL, nil
}

const (
	// APIs will give the price for the previous day 35 mins after midnight.
	// So, we configure the vote to start 36 mins after midnight
	dailyVotingStartTime = uint64(36 * 60)                       // 36 minutes in seconds
	dailyVotingEndTime   = dailyVotingStartTime + uint64(3*3600) // 3 hours in seconds
)

func isVotingTime(timestamp uint64) bool {
	// Seconds since the start of the day
	secondsInDay := timestamp % 86400

	// Check if the seconds in the day falls between startTimeSeconds and endTimeSeconds
	return secondsInDay >= dailyVotingStartTime && secondsInDay < dailyVotingEndTime
}

// isBlockOlderThan checks if the block is older than the given number of minutes
func isBlockOlderThan(header *types.Header, minutes int64) bool {
	return time.Now().UTC().Unix()-int64(header.Timestamp) > minutes*60
}

func calcDayNumber(timestamp uint64) uint64 {
	// Number of seconds in a day
	const secondsInADay uint64 = 86400

	// Calculate the current day number
	return timestamp / secondsInADay
}
