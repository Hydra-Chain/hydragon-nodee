package polybft

import (
	"math/big"
	"testing"

	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/helper/common"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/assert"
)

func TestRewardWalletCalculator_GetRewardWalletFundAmount(t *testing.T) {
	block := &types.Header{}

	mockSetup := func() *blockchainMock {
		blockchainMock := new(blockchainMock)

		return blockchainMock
	}

	t.Run("returns error when GetAccountBalance fails", func(t *testing.T) {
		blockchainMock := mockSetup()
		blockchainMock.On("GetAccountBalance", block, contracts.RewardWalletContract).Return(
			big.NewInt(0),
			assert.AnError,
		)

		calculator := NewRewardWalletCalculator(hclog.Default(), blockchainMock)

		_, err := calculator.GetRewardWalletFundAmount(block)
		assert.EqualError(t, err, "cannot get account balance")

		// Assert that the expected calls were made
		blockchainMock.AssertExpectations(t)
	})

	t.Run("returns full required amount to fund the reward wallet", func(t *testing.T) {
		blockchainMock := mockSetup()
		blockchainMock.On("GetAccountBalance", block, contracts.RewardWalletContract).Return(big.NewInt(0), nil)

		calculator := NewRewardWalletCalculator(hclog.Default(), blockchainMock)

		amount, err := calculator.GetRewardWalletFundAmount(block)
		assert.NoError(t, err)

		requiredAmount := common.GetTwoThirdOfMaxUint256()
		assert.Equal(t, requiredAmount, amount)
	})

	t.Run("returns partial amount to fund the reward wallet", func(t *testing.T) {
		blockchainMock := mockSetup()

		requiredAmount := common.GetTwoThirdOfMaxUint256()
		currentBalance := new(big.Int).Div(requiredAmount, big.NewInt(3))
		blockchainMock.On("GetAccountBalance", block, contracts.RewardWalletContract).Return(currentBalance, nil)

		calculator := NewRewardWalletCalculator(hclog.Default(), blockchainMock)

		amount, err := calculator.GetRewardWalletFundAmount(block)
		assert.NoError(t, err)

		remainingAmountToFund := new(big.Int).Sub(requiredAmount, currentBalance)
		assert.Equal(t, remainingAmountToFund, amount)
	})

	t.Run("returns 0 amount to fund the amount because there are sufficient HYDRA", func(t *testing.T) {
		requiredAmount := common.GetTwoThirdOfMaxUint256()

		blockchainMock := mockSetup()
		blockchainMock.On("GetAccountBalance", block, contracts.RewardWalletContract).Return(requiredAmount, nil)

		calculator := NewRewardWalletCalculator(hclog.Default(), blockchainMock)

		amount, err := calculator.GetRewardWalletFundAmount(block)
		assert.NoError(t, err)

		assert.Equal(t, big.NewInt(0), amount)
	})
}
