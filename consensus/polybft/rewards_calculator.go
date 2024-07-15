package polybft

import (
	"fmt"
	"math/big"

	"github.com/0xPolygon/polygon-edge/types"
	"github.com/hashicorp/go-hclog"
)

var (
	epochsInYear = big.NewInt(31500)
	denominator  = big.NewInt(10000)
	vestingBonus = [52]*big.Int{
		big.NewInt(6),
		big.NewInt(16),
		big.NewInt(30),
		big.NewInt(46),
		big.NewInt(65),
		big.NewInt(85),
		big.NewInt(108),
		big.NewInt(131),
		big.NewInt(157),
		big.NewInt(184),
		big.NewInt(212),
		big.NewInt(241),
		big.NewInt(272),
		big.NewInt(304),
		big.NewInt(338),
		big.NewInt(372),
		big.NewInt(407),
		big.NewInt(444),
		big.NewInt(481),
		big.NewInt(520),
		big.NewInt(559),
		big.NewInt(599),
		big.NewInt(641),
		big.NewInt(683),
		big.NewInt(726),
		big.NewInt(770),
		big.NewInt(815),
		big.NewInt(861),
		big.NewInt(907),
		big.NewInt(955),
		big.NewInt(1003),
		big.NewInt(1052),
		big.NewInt(1101),
		big.NewInt(1152),
		big.NewInt(1203),
		big.NewInt(1255),
		big.NewInt(1307),
		big.NewInt(1361),
		big.NewInt(1415),
		big.NewInt(1470),
		big.NewInt(1525),
		big.NewInt(1581),
		big.NewInt(1638),
		big.NewInt(1696),
		big.NewInt(1754),
		big.NewInt(1812),
		big.NewInt(1872),
		big.NewInt(1932),
		big.NewInt(1993),
		big.NewInt(2054),
		big.NewInt(2116),
		big.NewInt(2178),
	}

	ErrCannotGetSystemState = fmt.Errorf("cannot get system state")
)

type RewardsCalculator interface {
	GetMaxReward(block *types.Header) (*big.Int, error)
}

type rewardsCalculator struct {
	logger     hclog.Logger
	blockchain blockchainBackend
}

func NewRewardsCalculator(logger hclog.Logger, blockchain blockchainBackend) RewardsCalculator {
	return &rewardsCalculator{
		logger:     logger,
		blockchain: blockchain,
	}
}

func (r *rewardsCalculator) GetMaxReward(block *types.Header) (*big.Int, error) {
	systemState, err := r.getSystemState(block)
	if err != nil {
		return nil, err
	}

	stakedBalance, err := r.getStakedBalance(block, systemState)
	if err != nil {
		return nil, err
	}

	baseReward, err := r.getMaxBaseReward(block, systemState)
	if err != nil {
		return nil, err
	}

	vestingBonus, err := r.getVestingBonus(52)
	if err != nil {
		return nil, err
	}

	rsiBonus, err := r.getMaxRSIBonus(block, systemState)
	if err != nil {
		return nil, err
	}

	macroFactor, err := r.getMacroFactor(block, systemState)
	if err != nil {
		return nil, err
	}

	reward := calcMaxReward(stakedBalance, baseReward.Numerator, vestingBonus, rsiBonus, macroFactor)

	return reward, nil
}

func (r *rewardsCalculator) getStakedBalance(block *types.Header, systemState SystemState) (*big.Int, error) {
	stakedBalance, err := systemState.GetStakedBalance()
	if err != nil {
		return nil, err
	}

	return stakedBalance, nil
}

func (r *rewardsCalculator) getMaxBaseReward(block *types.Header, systemState SystemState) (*BigNumDecimal, error) {
	reward, err := systemState.GetBaseReward()
	if err != nil {
		return nil, err
	}

	return reward, nil
}

func (r *rewardsCalculator) getVestingBonus(vestingWeeks uint64) (*big.Int, error) {
	bonus := vestingBonus[vestingWeeks-1]

	return bonus, nil
}

func (r *rewardsCalculator) getMaxRSIBonus(block *types.Header, systemState SystemState) (*big.Int, error) {
	rsi, err := systemState.GetMaxRSI()
	if err != nil {
		return nil, err
	}

	return rsi, nil
}

func (r *rewardsCalculator) getMacroFactor(block *types.Header, systemState SystemState) (*big.Int, error) {
	reward, err := systemState.GetMacroFactor()
	if err != nil {
		return nil, err
	}

	return reward, nil
}

func (r *rewardsCalculator) getSystemState(block *types.Header) (SystemState, error) {
	provider, err := r.blockchain.GetStateProviderForBlock(block)
	if err != nil {
		return nil, ErrCannotGetSystemState
	}

	return r.blockchain.GetSystemState(provider), nil
}

func calcMaxReward(staked *big.Int, base *big.Int, vesting *big.Int, rsi *big.Int, macro *big.Int) *big.Int {
	res := big.NewInt(0)
	denSum := new(big.Int)
	denSum = denSum.Mul(denominator, denominator).Mul(denSum, denominator)

	baseVestingSum := big.NewInt(0).Add(base, vesting)
	macroRsiProd := big.NewInt(0).Mul(macro, rsi)
	nominator := big.NewInt(0).Mul(baseVestingSum, macroRsiProd)

	return res.Mul(staked, nominator).Div(res, denSum).Div(res, epochsInYear)
}
