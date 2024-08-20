package priceoracle

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/0xPolygon/polygon-edge/blockchain"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/validator"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/wallet"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/contract"
)

func TestIsValidator(t *testing.T) {
	mockPolybftBackend := new(MockPolybftBackend)
	validators := validator.NewTestValidatorsWithAliases(
		t,
		[]string{"A", "B", "C", "D", "E", "F"},
	)

	validatorSet := validators.GetPublicIdentities()

	block := &types.Header{
		Number: 7,
	}

	tests := []struct {
		name                string
		block               *types.Header
		validators          validator.AccountSet
		account             *wallet.Account
		getValidatorsError  error
		expectedIsValidator bool
		expectedError       error
	}{
		{
			name:                "valid validator",
			block:               block,
			validators:          validatorSet,
			account:             validators.GetValidator("B").Account,
			getValidatorsError:  nil,
			expectedIsValidator: true,
			expectedError:       nil,
		},
		{
			name:                "not a validator",
			block:               block,
			validators:          validatorSet,
			account:             validator.NewTestValidator(t, "X", 1000).Account,
			getValidatorsError:  nil,
			expectedIsValidator: false,
			expectedError:       nil,
		},
		{
			name:                "error querying validators",
			block:               block,
			validators:          nil,
			getValidatorsError:  errors.New("failed to get validators"),
			expectedIsValidator: false,
			expectedError:       fmt.Errorf("failed to query current validator set, block number %d, error %w", block.Number, errors.New("failed to get validators")),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPolybftBackend.On("GetValidators", tt.block.Number, mock.Anything).Return(tt.validators, tt.getValidatorsError).Once()

			priceOracle := &PriceOracle{
				polybftBackend: mockPolybftBackend,
				account:        tt.account,
			}

			isValidator, err := priceOracle.isValidator(tt.block)

			require.Equal(t, tt.expectedIsValidator, isValidator)
			if tt.expectedError != nil {
				require.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestIsBlockOlderThan(t *testing.T) {
	now := time.Now().UTC().Unix()

	tests := []struct {
		name     string
		header   *types.Header
		minutes  int64
		expected bool
	}{
		{
			name: "Block is older than 2 minutes",
			header: &types.Header{
				Timestamp: uint64(now - (2*60 + 1)), // more than 2 minutes ago
			},
			minutes:  2,
			expected: true,
		},
		{
			name: "Block is exactly 2 minutes old",
			header: &types.Header{
				Timestamp: uint64(now - 2*60), // 2 minutes ago
			},
			minutes:  2,
			expected: false,
		},
		{
			name: "Block is less than 2 minutes old",
			header: &types.Header{
				Timestamp: uint64(now - 1*60), // 1 minute ago
			},
			minutes:  2,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isBlockOlderThan(tt.header, tt.minutes)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestBlockMustBeProcessed(t *testing.T) {
	// Mock structures
	mockBlockchainBackend := new(MockBlockchainBackend)
	priceOracle := &PriceOracle{
		blockchain: mockBlockchainBackend,
	}

	tests := []struct {
		name          string
		event         *blockchain.Event
		currentHeader *types.Header
		expected      bool
	}{
		{
			name: "block timestamp is older than 2 minutes",
			event: &blockchain.Event{
				NewChain: []*types.Header{
					{
						Timestamp: uint64(time.Now().UTC().Add(-3 * time.Minute).Unix()),
						Number:    5,
					},
				},
			},
			currentHeader: &types.Header{Number: 6},
			expected:      false,
		},
		{
			name: "block number is smaller than current header",
			event: &blockchain.Event{
				NewChain: []*types.Header{
					{
						Timestamp: uint64(time.Now().UTC().Unix()),
						Number:    5,
					},
				},
			},
			currentHeader: &types.Header{Number: 6},
			expected:      false,
		},
		{
			name: "event type is fork",
			event: &blockchain.Event{
				NewChain: []*types.Header{
					{
						Timestamp: uint64(time.Now().UTC().Unix()),
						Number:    7,
					},
				},
				Type: blockchain.EventFork,
			},
			currentHeader: &types.Header{Number: 6},
			expected:      false,
		},
		{
			name: "successful case",
			event: &blockchain.Event{
				NewChain: []*types.Header{
					{
						Timestamp: uint64(time.Now().UTC().Unix()),
						Number:    7,
					},
				},
				Type: blockchain.EventHead,
			},
			currentHeader: &types.Header{Number: 6},
			expected:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up the expected behavior for the mock
			mockBlockchainBackend.On("CurrentHeader").Return(tt.currentHeader)

			// Call the method under test
			result := priceOracle.blockMustBeProcessed(tt.event)

			// Assert the result
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestIsVotingTime(t *testing.T) {
	tests := []struct {
		name      string
		timestamp uint64
		expected  bool
	}{
		{
			name:      "Before voting time",
			timestamp: uint64(time.Date(2024, 10, 21, 0, 35, 59, 0, time.UTC).Unix()),
			expected:  false,
		},
		{
			name:      "At the start of voting time",
			timestamp: uint64(time.Date(2024, 10, 21, 0, 36, 0, 0, time.UTC).Unix()),
			expected:  true,
		},
		{
			name:      "During voting time",
			timestamp: uint64(time.Date(2024, 10, 21, 1, 30, 0, 0, time.UTC).Unix()),
			expected:  true,
		},
		{
			name:      "At the end of voting time",
			timestamp: uint64(time.Date(2024, 10, 21, 3, 35, 59, 0, time.UTC).Unix()),
			expected:  true,
		},
		{
			name:      "After voting time",
			timestamp: uint64(time.Date(2024, 10, 21, 3, 36, 0, 0, time.UTC).Unix()),
			expected:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function under test
			result := isVotingTime(tt.timestamp)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestCalcDayNumber(t *testing.T) {
	tests := []struct {
		name      string
		timestamp uint64
		expected  uint64
	}{
		{
			name:      "Start of the first day",
			timestamp: 0,
			expected:  0,
		},
		{
			name:      "Middle of the first day",
			timestamp: 43200, // 12 hours
			expected:  0,
		},
		{
			name:      "Start of the second day",
			timestamp: 86400, // 24 hours
			expected:  1,
		},
		{
			name:      "Middle of the second day",
			timestamp: 86400 + 43200, // 36 hours
			expected:  1,
		},
		{
			name:      "Start of the third day",
			timestamp: 2 * 86400, // 48 hours
			expected:  2,
		},
		{
			name:      "End of the third day",
			timestamp: 3*86400 - 1, // Just before 72 hours
			expected:  2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := calcDayNumber(tt.timestamp)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestAlreadyVoted(t *testing.T) {
	validators := validator.NewTestValidatorsWithAliases(
		t,
		[]string{"A", "B", "C", "D", "E", "F"},
	)

	// Set up the mocked price oracle
	day1Ts := uint64(time.Date(2024, 8, 20, 1, 30, 0, 0, time.UTC).Unix())
	day2Ts := uint64(time.Date(2024, 8, 21, 1, 30, 0, 0, time.UTC).Unix())
	mockPriceOracle := &MockPriceOracle{
		MockAlreadyVotedMapping: map[uint64]bool{
			calcDayNumber(day1Ts): true,
			calcDayNumber(day2Ts): false,
		},
	}

	tests := []struct {
		name          string
		block         *types.Header
		currentHeader *types.Header
		event         *blockchain.Event
		validators    *validator.AccountSet
		account       *wallet.Account
		expected      bool
	}{
		{
			name: "Already voted",
			block: &types.Header{
				Number:    7,
				Timestamp: day1Ts,
			},
			account:  validators.GetValidator("A").Account,
			expected: true,
		},
		{
			name: "Not voted",
			block: &types.Header{
				Number:    10,
				Timestamp: day2Ts,
			},
			account:  validators.GetValidator("A").Account,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPriceOracle.account = tt.account

			// Call the function under test
			result := mockPriceOracle.alreadyVoted(tt.block)
			require.Equal(t, tt.expected, result)
		})
	}
}

func TestGetState(t *testing.T) {
	mockBlockchainBackend := new(MockBlockchainBackend)
	priceOracle := &PriceOracle{
		blockchain: mockBlockchainBackend,
	}

	validators := validator.NewTestValidatorsWithAliases(
		t,
		[]string{"A", "B", "C", "D", "E", "F"},
	)

	block := &types.Header{
		Number:    7,
		Timestamp: uint64(time.Date(2024, 8, 20, 1, 30, 0, 0, time.UTC).Unix()),
	}

	// Set up the expected behavior for the mock
	mockPriceOracle := &MockPriceOracle{
		PriceOracle: *priceOracle,
		MockAlreadyVotedMapping: map[uint64]bool{
			block.Timestamp: true,
		},
	}

	getStateProviderForBlockErr := errors.New("failed to GetStateProviderForBlock")

	tests := []struct {
		name          string
		block         *types.Header
		currentHeader *types.Header
		event         *blockchain.Event
		validators    *validator.AccountSet
		account       *wallet.Account
		getStateError error
		expectedError error
		expectedState *priceOracleState
	}{
		{
			name:          "Unable to get system state",
			block:         block,
			account:       validators.GetValidator("A").Account,
			getStateError: getStateProviderForBlockErr,
			expectedError: getStateProviderForBlockErr,
			expectedState: nil,
		},
		{
			name:          "Get system state",
			block:         block,
			account:       validators.GetValidator("A").Account,
			getStateError: nil,
			expectedError: nil,
			expectedState: &priceOracleState{new(systemStateMock), contract.NewContract(
				ethgo.Address(contracts.PriceOracleContract),
				contractsapi.PriceOracle.Abi,
			)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// mockPolybftBackend.On("GetValidators", tt.block.Number, mock.Anything).Return(tt.validators, tt.getValidatorsError).Once()

			// priceOracle := &PriceOracle{
			// 	polybftBackend: mockPolybftBackend,
			// 	account:        tt.account,
			// }

			// isValidator, err := priceOracle.isValidator(tt.block)

			// require.Equal(t, tt.expectedIsValidator, isValidator)
			// if tt.expectedError != nil {
			// 	require.EqualError(t, err, tt.expectedError.Error())
			// } else {
			// 	require.NoError(t, err)
			// }

			mockBlockchainBackend.On("GetStateProviderForBlock", mock.Anything).Return(new(stateProviderMock), tt.getStateError).Once()
			mockBlockchainBackend.On("GetSystemState", mock.Anything).Return(new(systemStateMock)).Twice()

			mockPriceOracle.account = tt.account

			// call the function under test
			state, err := mockPriceOracle.PriceOracle.getState(tt.block)

			require.Equal(t, tt.expectedState, state)
			if tt.expectedError != nil {
				require.EqualError(t, err, tt.expectedError.Error())
			}
			// mockBlockchainBackend.AssertExpectations(t)
		})
	}
}

func (m *MockPriceOracle) alreadyVoted(header *types.Header) bool {
	dayNumber := calcDayNumber(header.Timestamp)

	return m.MockAlreadyVotedMapping[dayNumber]
}
