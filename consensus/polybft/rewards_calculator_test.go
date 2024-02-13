package polybft

import (
	"math/big"
	"testing"

	"github.com/0xPolygon/polygon-edge/types"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/umbracle/ethgo"
)

func TestRewardsCalculator_getStakedBalance(t *testing.T) {
	block := &types.Header{}

	t.Run("returns error when systemState.GetStakedBalance fails", func(t *testing.T) {
		systemStateMock := new(systemStateMock)
		systemStateMock.On("GetStakedBalance").Return(nil, assert.AnError)

		calculator := &rewardsCalculator{
			logger:     hclog.Default(),
			blockchain: nil,
		}

		_, err := calculator.getStakedBalance(block, systemStateMock)
		assert.EqualError(t, err, assert.AnError.Error())
	})

	t.Run("returns correct balance", func(t *testing.T) {
		expectedReward := big.NewInt(100)
		systemStateMock := new(systemStateMock)
		systemStateMock.On("GetStakedBalance").Return(expectedReward, nil)

		calculator := &rewardsCalculator{
			logger:     hclog.Default(),
			blockchain: nil,
		}

		reward, err := calculator.getStakedBalance(block, systemStateMock)
		assert.NoError(t, err)
		assert.Equal(t, expectedReward, reward)
	})
}

func TestRewardsCalculator_GetMaxReward(t *testing.T) {
	block := &types.Header{}

	mockSetup := func() (*blockchainMock, *stateProviderMock, *systemStateMock) {
		blockchainMock := new(blockchainMock)
		stateProviderMock := new(stateProviderMock)
		systemStateMock := new(systemStateMock)

		blockchainMock.On("GetStateProviderForBlock", block).Return(stateProviderMock, nil)
		blockchainMock.On("GetSystemState", stateProviderMock).Return(systemStateMock)

		return blockchainMock, stateProviderMock, systemStateMock
	}

	t.Run("returns error when getStakedBalance fails", func(t *testing.T) {
		blockchainMock, _, systemStateMock := mockSetup()
		systemStateMock.On("GetStakedBalance").Return(nil, assert.AnError)

		calculator := NewRewardsCalculator(hclog.Default(), blockchainMock)

		_, err := calculator.GetMaxReward(block)
		assert.EqualError(t, err, assert.AnError.Error())
	})

	t.Run("returns error when getMaxBaseReward fails", func(t *testing.T) {
		blockchainMock, _, systemStateMock := mockSetup()
		systemStateMock.On("GetStakedBalance").Return(big.NewInt(100), nil)
		systemStateMock.On("GetBaseReward").Return(nil, assert.AnError)

		calculator := NewRewardsCalculator(hclog.Default(), blockchainMock)

		_, err := calculator.GetMaxReward(block)
		assert.EqualError(t, err, assert.AnError.Error())
	})

	t.Run("returns max reward", func(t *testing.T) {
		blockchainMock, _, systemStateMock := mockSetup()
		systemStateMock.On("GetStakedBalance").Return(ethgo.Ether(1), nil)
		systemStateMock.On("GetBaseReward").Return(&BigNumDecimal{Numerator: big.NewInt(500), Denominator: big.NewInt(10000)}, nil)

		calculator := NewRewardsCalculator(hclog.Default(), blockchainMock)

		reward, err := calculator.GetMaxReward(block)
		assert.NoError(t, err)

		expectedReward := big.NewInt(9564285714285)

		assert.Equal(t, expectedReward, reward)
	})
}

func TestRewardsCalculator_calcMaxReward(t *testing.T) {
	tests := []struct {
		name           string
		staked         *big.Int
		base           *big.Int
		vesting        *big.Int
		rsi            *big.Int
		macro          *big.Int
		expectedReward *big.Int
	}{
		{
			name:           "base case",
			staked:         big.NewInt(10000000000000000),
			base:           big.NewInt(500),
			vesting:        big.NewInt(52000),
			rsi:            big.NewInt(15000),
			macro:          big.NewInt(10000),
			expectedReward: big.NewInt(2500000000000),
		},
		{
			name:           "base case 2",
			staked:         bigIntFromString("275000000000000000000"),
			base:           big.NewInt(500),
			vesting:        big.NewInt(2178),
			rsi:            big.NewInt(15000),
			macro:          big.NewInt(7500),
			expectedReward: big.NewInt(2630178571428571),
		},
		{
			name:           "Too small staked amount",
			staked:         big.NewInt(1000),
			base:           big.NewInt(500),
			vesting:        big.NewInt(52000),
			rsi:            big.NewInt(15000),
			macro:          big.NewInt(10000),
			expectedReward: big.NewInt(0),
		},
		{
			name:           "Zero staked",
			staked:         big.NewInt(0),
			base:           big.NewInt(500),
			vesting:        big.NewInt(1000),
			rsi:            big.NewInt(15000),
			macro:          big.NewInt(10000),
			expectedReward: big.NewInt(0),
		},
		{
			name:           "Zero base",
			staked:         big.NewInt(1000000000000000000),
			base:           big.NewInt(0),
			vesting:        big.NewInt(1000),
			rsi:            big.NewInt(15000),
			macro:          big.NewInt(10000),
			expectedReward: big.NewInt(4761904761904),
		},
		{
			name:           "Zero vesting",
			staked:         big.NewInt(1000000000000000000),
			base:           big.NewInt(500),
			vesting:        big.NewInt(0),
			rsi:            big.NewInt(15000),
			macro:          big.NewInt(10000),
			expectedReward: big.NewInt(2380952380952),
		},
		{
			name:           "Zero RSI",
			staked:         big.NewInt(1000000000000000000),
			base:           big.NewInt(500),
			vesting:        big.NewInt(1000),
			rsi:            big.NewInt(0),
			macro:          big.NewInt(10000),
			expectedReward: big.NewInt(0),
		},
		{
			name:           "Zero macro factor",
			staked:         big.NewInt(1000000000000000000),
			base:           big.NewInt(500),
			vesting:        big.NewInt(1000),
			rsi:            big.NewInt(15000),
			macro:          big.NewInt(0),
			expectedReward: big.NewInt(0),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reward := calcMaxReward(tt.staked, tt.base, tt.vesting, tt.rsi, tt.macro)
			require.True(t, tt.expectedReward.Cmp(reward) == 0, "expected %s, got %s", tt.expectedReward, reward)
		})
	}
}

func bigIntFromString(s string) *big.Int {
	i, ok := new(big.Int).SetString(s, 10)
	if !ok {
		panic("failed to parse big int") //nolint:gocritic
	}

	return i
}
