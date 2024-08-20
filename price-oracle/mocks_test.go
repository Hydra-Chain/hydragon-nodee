package priceoracle

import (
	"math/big"

	"github.com/0xPolygon/polygon-edge/blockchain"
	"github.com/0xPolygon/polygon-edge/consensus/polybft"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/validator"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/wallet"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/stretchr/testify/mock"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/contract"
)

type MockBlockchainBackend struct {
	mock.Mock
}

var _ blockchainBackend = (*MockBlockchainBackend)(nil)

func (m *MockBlockchainBackend) CurrentHeader() *types.Header {
	args := m.Called()
	return args.Get(0).(*types.Header)
}

func (m *MockBlockchainBackend) GetStateProviderForBlock(block *types.Header) (contract.Provider, error) {
	args := m.Called(block)
	return args.Get(0).(contract.Provider), args.Error(1)
}

func (m *MockBlockchainBackend) GetSystemState(provider contract.Provider) polybft.SystemState {
	args := m.Called(provider)
	return args.Get(0).(polybft.SystemState)
}

func (m *MockBlockchainBackend) SubscribeEvents() blockchain.Subscription {
	args := m.Called()
	return args.Get(0).(blockchain.Subscription)
}

func (m *MockBlockchainBackend) UnubscribeEvents(subscription blockchain.Subscription) {
	m.Called(subscription)
}

type MockPolybftBackend struct {
	mock.Mock
}

func (m *MockPolybftBackend) GetValidators(blockNumber uint64, parents []*types.Header) (validator.AccountSet, error) {
	args := m.Called(blockNumber, parents)
	return args.Get(0).(validator.AccountSet), args.Error(1)
}

var _ PriceOracleState = (*MockState)(nil)

type MockPriceFeed struct {
	mock.Mock
}

func (m *MockPriceFeed) GetPrice(timestamp uint64) (*big.Int, error) {
	args := m.Called(timestamp)
	return args.Get(0).(*big.Int), args.Error(1)
}

type MockTxRelayer struct {
	mock.Mock
}

func (m *MockTxRelayer) SendTransaction(tx *ethgo.Transaction, key *wallet.Account) (*types.Receipt, error) {
	args := m.Called(tx, key)
	return args.Get(0).(*types.Receipt), args.Error(1)
}

// MockState is a mock implementation of the PriceOracleState interface
type MockState struct {
	mock.Mock
}

func (m *MockState) shouldVote(account *wallet.Account) (bool, string, error) {
	args := m.Called(account)
	return args.Bool(0), args.String(1), args.Error(2)
}

// MockStateProvider is a mock implementation of the PriceOracleStateProvider interface
type MockStateProvider struct {
	mock.Mock
}

func (m *MockStateProvider) GetPriceOracleState(header *types.Header) (PriceOracleState, error) {
	args := m.Called(header)
	return args.Get(0).(PriceOracleState), args.Error(1)
}
