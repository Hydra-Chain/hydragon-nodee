package priceoracle

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/0xPolygon/polygon-edge/blockchain"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/validator"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/wallet"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

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

func TestShouldExecuteVote(t *testing.T) {
	mockState := new(MockState)
	mockStateProvider := new(MockStateProvider)
	mockAccount := &wallet.Account{}

	txRelayer, _ := getVoteTxRelayer("0.0.0.0:8545")

	priceOracle := &PriceOracle{
		account:       mockAccount,
		txRelayer:     txRelayer,
		logger:        hclog.NewNullLogger(),
		stateProvider: mockStateProvider, // Inject the mock state provider
	}

	tests := []struct {
		name               string
		header             *types.Header
		alreadyVoted       bool
		shouldMockState    bool
		stateShouldVote    bool
		stateShouldVoteErr error
		expectedResult     bool
		expectedError      error
	}{
		{
			name:            "Not in voting time",
			header:          &types.Header{Timestamp: uint64(time.Date(2024, 10, 21, 0, 30, 0, 0, time.UTC).Unix())},
			alreadyVoted:    false,
			shouldMockState: false,
			stateShouldVote: false,
			expectedResult:  false,
			expectedError:   nil,
		},
		{
			name:            "Already voted",
			header:          &types.Header{Timestamp: uint64(time.Date(2024, 10, 21, 1, 0, 0, 0, time.UTC).Unix())},
			alreadyVoted:    true,
			shouldMockState: false,
			stateShouldVote: false,
			expectedResult:  false,
			expectedError:   nil,
		},
		{
			name:            "Should not vote based on state",
			header:          &types.Header{Timestamp: uint64(time.Date(2024, 10, 21, 1, 0, 0, 0, time.UTC).Unix())},
			alreadyVoted:    false,
			shouldMockState: true,
			stateShouldVote: false,
			expectedResult:  false,
			expectedError:   nil,
		},
		{
			name:               "Error in shouldVote",
			header:             &types.Header{Timestamp: uint64(time.Date(2024, 10, 21, 1, 0, 0, 0, time.UTC).Unix())},
			alreadyVoted:       false,
			shouldMockState:    true,
			stateShouldVote:    false,
			stateShouldVoteErr: errors.New("should vote error"),
			expectedResult:     false,
			expectedError:      errors.New("should vote error"),
		},
		{
			name:            "Should vote",
			header:          &types.Header{Timestamp: uint64(time.Date(2024, 10, 21, 1, 0, 0, 0, time.UTC).Unix())},
			alreadyVoted:    false,
			shouldMockState: true,
			stateShouldVote: true,
			expectedResult:  true,
			expectedError:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set the alreadyVotedMapping state
			dayNumber := calcDayNumber(tt.header.Timestamp)
			alreadyVotedMapping[dayNumber] = tt.alreadyVoted

			if tt.shouldMockState {
				// Mock the GetPriceOracleState and shouldVote methods
				mockStateProvider.On("GetPriceOracleState", tt.header).Return(mockState, nil).Once()
				mockState.On("shouldVote", mockAccount).Return(tt.stateShouldVote, "", tt.stateShouldVoteErr).Once()
			}

			// Call the function under test
			result, err := priceOracle.shouldExecuteVote(tt.header)

			// Assert the results
			require.Equal(t, tt.expectedResult, result)
			if tt.expectedError != nil {
				require.EqualError(t, err, tt.expectedError.Error())
			} else {
				require.NoError(t, err)
			}

			// Assert that the mock expectations were met
			mockStateProvider.AssertExpectations(t)
			mockState.AssertExpectations(t)
		})
	}
}
