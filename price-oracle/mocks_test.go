package priceoracle

import (
	"fmt"
	"math/big"

	"github.com/0xPolygon/polygon-edge/blockchain"
	"github.com/0xPolygon/polygon-edge/bls"
	"github.com/0xPolygon/polygon-edge/consensus/polybft"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/validator"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/wallet"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/stretchr/testify/mock"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/contract"
)

var _ polybft.SystemState = (*systemStateMock)(nil)

type systemStateMock struct {
	mock.Mock
}

func (m *systemStateMock) GetNextCommittedIndex() (uint64, error) {
	args := m.Called()

	if len(args) == 1 {
		index, _ := args.Get(0).(uint64)

		return index, nil
	} else if len(args) == 2 {
		index, _ := args.Get(0).(uint64)

		return index, args.Error(1)
	}

	return 0, nil
}

func (m *systemStateMock) GetEpoch() (uint64, error) {
	args := m.Called()
	if len(args) == 1 {
		epochNumber, _ := args.Get(0).(uint64)

		return epochNumber, nil
	} else if len(args) == 2 {
		epochNumber, _ := args.Get(0).(uint64)
		err, ok := args.Get(1).(error)
		if ok {
			return epochNumber, err
		}

		return epochNumber, nil
	}

	return 0, nil
}

func (s *systemStateMock) GetValidatorBlsKey(addr types.Address) (*bls.PublicKey, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *systemStateMock) GetVotingPowerExponent() (*polybft.BigNumDecimal, error) {
	args := s.Called()
	exp, _ := args.Get(0).(*polybft.BigNumDecimal)
	var err error
	if args.Get(1) != nil {
		err, _ = args.Get(1).(error)
	}

	return exp, err
}

type MockPriceOracle struct {
	PriceOracle
	MockAlreadyVotedMapping map[uint64]bool
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
