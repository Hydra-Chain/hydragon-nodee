package polybft

import (
	"math/big"

	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/helper/common"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/hashicorp/go-hclog"
)

type RewardWalletCalculator interface {
	GetRewardWalletFundAmount(block *types.Header) (*big.Int, error)
}

type rewardWalletCalculator struct {
	logger     hclog.Logger
	blockchain blockchainBackend
}

func NewRewardWalletCalculator(logger hclog.Logger, blockchain blockchainBackend) RewardWalletCalculator {
	return &rewardWalletCalculator{
		logger:     logger,
		blockchain: blockchain,
	}
}

func (r *rewardWalletCalculator) GetRewardWalletFundAmount(block *types.Header) (*big.Int, error) {
	requiredAmount := common.GetTwoThirdOfMaxUint256()

	// Get the current RewardWallet balance
	currentBalance, err := r.blockchain.GetAccountBalance(block, contracts.RewardWalletContract)
	if err != nil {
		return nil, err
	}

	// Check if the current balance is less than the required amount
	// If so, then return the remaining funds to fulfill
	if currentBalance.Cmp(requiredAmount) == -1 {
		return new(big.Int).Sub(requiredAmount, currentBalance), nil
	}

	return big.NewInt(0), nil
}
