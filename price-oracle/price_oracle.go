package priceoracle

import (
	"errors"
	"fmt"
	"math/big"
	"net"
	"strings"
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
	alreadyVotedMapping        = make(map[uint64]bool)
	priceUpdatedAlreadyMapping = make(map[uint64]bool)
	calculatedDayNumber        = make(map[uint64]uint64)
	priceVotedEventABI         = contractsapi.PriceOracle.Abi.Events["PriceVoted"]
)

// emit PriceVoted(_price, msg.sender, day);
type voteResult struct {
	price            string
	validatorAddress string
	day              uint64
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
	stateProvider  PriceOracleStateProvider
	// key encapsulates ECDSA signing account
	account   *wallet.Account
	priceFeed PriceFeed
	txRelayer txrelayer.TxRelayer
}

func NewPriceOracle(
	logger hclog.Logger,
	blockchain *blockchain.Blockchain,
	executor *state.Executor,
	consensus consensus.Consensus,
	jsonRPC string,
	secretsManager secrets.SecretsManager,
	secretsManagerConfig *secrets.SecretsManagerConfig,
) (*PriceOracle, error) {
	priceFeed, err := NewPriceFeed(secretsManagerConfig)
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

	blockchainBackend := polybft.NewBlockchainBackend(executor, blockchain)

	txRelayer, err := getVoteTxRelayer(jsonRPC)
	if err != nil {
		return nil, err
	}

	return &PriceOracle{
		logger:         logger.Named("price-oracle"),
		blockchain:     blockchainBackend,
		stateProvider:  NewPriceOracleStateProvider(blockchainBackend),
		priceFeed:      priceFeed,
		polybftBackend: polybftConsensus,
		txRelayer:      txRelayer,
		account:        account,
		closeCh:        make(chan struct{}),
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
			p.handleEvent(ev)
		}
	}
}

func (p *PriceOracle) Close() error {
	close(p.closeCh)
	p.logger.Info("price oracle stopped")

	return nil
}

func (p *PriceOracle) handleEvent(ev *blockchain.Event) {
	block := ev.NewChain[0]
	p.logger.Debug("received new block notification", "block", block.Number)

	isValidator, err := p.isValidator(block)
	if err != nil {
		p.logger.Error("failed to check if node is validator", "err", err)

		return
	}

	if !isValidator || !p.blockMustBeProcessed(ev) {
		return
	}

	should, err := p.shouldExecuteVote(block)
	if err != nil {
		p.logger.Error("failed to check if vote must be executed:", "error", err)

		return
	}

	if should {
		if err := p.executeVote(block); err != nil {
			p.logger.Error("failed to execute vote", "err", err)

			return
		}

		p.logger.Info("vote executed successfully")
	}
}

// shouldExecuteVote verifies that the validator should vote
func (p *PriceOracle) shouldExecuteVote(header *types.Header) (bool, error) {
	// check if the current time is in the voting window
	if !isVotingTime(header.Timestamp) {
		p.logger.Debug("Not currently in voting time window")

		return false, nil
	}

	// check is voting made or price updated already for the current day
	if p.alreadyVoted(header) || p.priceUpdatedAlready(header) {
		return false, nil
	}

	// initialize the system state for the given header
	state, err := p.stateProvider.GetPriceOracleState(header, p.account)
	if err != nil {
		return false, fmt.Errorf("get system state: %w", err)
	}

	// then check if the contract is in a proper state to vote
	dayNumber := calcDayNumber(header.Timestamp)
	shouldVote, falseReason, err := state.shouldVote(dayNumber)
	if err != nil {
		return false, err
	}

	if !shouldVote {
		p.logger.Debug("should not vote", "reason", falseReason)

		if falseReason == "PRICE_ALREADY_SET" {
			priceUpdatedAlreadyMapping[dayNumber] = true
		}

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

func (p *PriceOracle) priceUpdatedAlready(header *types.Header) bool {
	return priceUpdatedAlreadyMapping[calcDayNumber(header.Timestamp)]
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

func (p *PriceOracle) vote(price *big.Int) error {
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
		Gas:   1000000,
	}

	receipt, err := p.txRelayer.SendTransaction(txn, p.account.Ecdsa)
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
				return fmt.Errorf("failed to parse log: %w", err)
			}

			result.price = event["price"].(*big.Int).String()                     //nolint:forcetypeassert
			result.validatorAddress = event["validator"].(ethgo.Address).String() //nolint:forcetypeassert
			result.day = event["day"].(*big.Int).Uint64()                         //nolint:forcetypeassert

			foundVoteLog = true
		}
	}

	if !foundVoteLog {
		return fmt.Errorf(
			"could not find an appropriate log in the receipt that validates the vote has happened",
		)
	}

	p.logger.Info(
		"[VOTE INFO] Validator address: %s, voted price: %s, day: %d",
		result.validatorAddress,
		result.price,
		result.day,
	)

	return nil
}

const (
	// APIs will give the price for the previous day 35 mins after midnight.
	// So, we configure the vote to start 36 mins after midnight to give it 1 min margin
	dailyVotingStartTime = uint64(36 * 60)                       // 36 minutes in seconds
	dailyVotingEndTime   = dailyVotingStartTime + uint64(3*3600) // 3 hours in seconds
	secondsInADay        = uint64(86400)
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
	if calculatedDayNumber[timestamp] != 0 {
		return calculatedDayNumber[timestamp]
	}

	// Calculate the current day number
	dayNumber := timestamp / secondsInADay
	calculatedDayNumber[timestamp] = dayNumber

	return dayNumber
}

func getVoteTxRelayer(rpcEndpoint string) (txrelayer.TxRelayer, error) {
	if rpcEndpoint == "" || strings.Contains(rpcEndpoint, "0.0.0.0") {
		_, port, err := net.SplitHostPort(rpcEndpoint)
		if err == nil {
			rpcEndpoint = fmt.Sprintf("http://%s:%s", "127.0.0.1", port)
		} else {
			rpcEndpoint = txrelayer.DefaultRPCAddress
		}
	}

	return txrelayer.NewTxRelayer(
		txrelayer.WithIPAddress(rpcEndpoint), txrelayer.WithReceiptTimeout(150*time.Millisecond))
}
