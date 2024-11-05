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
	"github.com/umbracle/ethgo/jsonrpc"
)

type MockPriceOracle struct {
	PriceOracle
}

var _ contract.Provider = (*stateProviderMock)(nil)

type stateProviderMock struct {
	mock.Mock
}

func (s *stateProviderMock) Call(ethgo.Address, []byte, *contract.CallOpts) ([]byte, error) {
	return nil, nil
}

func (s *stateProviderMock) Txn(ethgo.Address, ethgo.Key, []byte) (contract.Txn, error) {
	return nil, nil
}

type MockBlockchainBackend struct {
	mock.Mock
}

var _ blockchainBackend = (*MockBlockchainBackend)(nil)

func (m *MockBlockchainBackend) CurrentHeader() *types.Header {
	args := m.Called()

	header, ok := args.Get(0).(*types.Header)
	if !ok {
		panic("Expected *types.Header but got a different type")
	}

	return header
}

func (m *MockBlockchainBackend) GetStateProviderForBlock(
	block *types.Header,
) (contract.Provider, error) {
	args := m.Called(block)

	provider, ok := args.Get(0).(contract.Provider)
	if !ok {
		panic("Expected contract.Provider but got a different type")
	}

	return provider, args.Error(1)
}

func (m *MockBlockchainBackend) GetSystemState(provider contract.Provider) polybft.SystemState {
	args := m.Called(provider)

	state, ok := args.Get(0).(polybft.SystemState)
	if !ok {
		panic("Expected polybft.SystemState but got a different type")
	}

	return state
}

func (m *MockBlockchainBackend) SubscribeEvents() blockchain.Subscription {
	args := m.Called()

	subsciption, ok := args.Get(0).(blockchain.Subscription)
	if !ok {
		panic("Expected blockchain.Subscription but got a different type")
	}

	return subsciption
}

func (m *MockBlockchainBackend) UnubscribeEvents(subscription blockchain.Subscription) {
	m.Called(subscription)
}

type MockPolybftBackend struct {
	mock.Mock
}

func (m *MockPolybftBackend) GetValidators(
	blockNumber uint64,
	parents []*types.Header,
) (validator.AccountSet, error) {
	args := m.Called(blockNumber, parents)

	accSet, ok := args.Get(0).(validator.AccountSet)
	if !ok {
		panic("Expected validator.AccountSet but got a different type")
	}

	return accSet, args.Error(1)
}

// MockTxRelayer is a mock implementation of the TxRelayer interface
type MockTxRelayer struct {
	mock.Mock
}

func (m *MockTxRelayer) Call(from ethgo.Address, to ethgo.Address, input []byte) (string, error) {
	args := m.Called(from, to, input)

	return args.String(0), args.Error(1)
}

func (m *MockTxRelayer) SendTransaction(
	txn *ethgo.Transaction,
	key ethgo.Key,
) (*ethgo.Receipt, error) {
	args := m.Called(txn, key)

	receipt, ok := args.Get(0).(*ethgo.Receipt)
	if !ok {
		panic("Expected *ethgo.Receipt but got a different type")
	}

	return receipt, args.Error(1)
}

func (m *MockTxRelayer) SendTransactionLocal(txn *ethgo.Transaction) (*ethgo.Receipt, error) {
	args := m.Called(txn)

	receipt, ok := args.Get(0).(*ethgo.Receipt)
	if !ok {
		panic("Expected *ethgo.Receipt but got a different type")
	}

	return receipt, args.Error(1)
}

func (m *MockTxRelayer) Client() *jsonrpc.Client {
	args := m.Called()

	client, ok := args.Get(0).(*jsonrpc.Client)
	if !ok {
		panic("Expected *jsonrpc.Client but got a different type")
	}

	return client
}

var _ PriceOracleState = (*MockState)(nil)

// MockState is a mock implementation of the PriceOracleState interface
type MockState struct {
	mock.Mock
}

func (m *MockState) shouldVote(dayNumber uint64) (bool, string, error) {
	args := m.Called(dayNumber)

	return args.Bool(0), args.String(1), args.Error(2)
}

// MockStateProvider is a mock implementation of the PriceOracleStateProvider interface
type MockStateProvider struct {
	mock.Mock
}

func (m *MockStateProvider) GetPriceOracleState(
	header *types.Header,
	validatorAccount *wallet.Account,
) (PriceOracleState, error) {
	args := m.Called(header, validatorAccount)

	state, ok := args.Get(0).(PriceOracleState)
	if !ok {
		panic("Expected PriceOracleState but got a different type")
	}

	return state, args.Error(1)
}

// MockPriceFeed is a mock implementation of the PriceFeed interface
type MockPriceFeed struct {
	mock.Mock
}

func (m *MockPriceFeed) GetPrice(header *types.Header) (*big.Int, error) {
	args := m.Called(header)

	price, ok := args.Get(0).(*big.Int)
	if !ok {
		panic("Expected *big.Int but got a different type")
	}

	return price, args.Error(1)
}
