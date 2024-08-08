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
