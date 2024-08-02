package polybft

import (
	"math/big"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/0xPolygon/polygon-edge/chain"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/signer"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/validator"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/wallet"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/helper/common"
	"github.com/0xPolygon/polygon-edge/helper/hex"
	"github.com/0xPolygon/polygon-edge/state"
	"github.com/0xPolygon/polygon-edge/state/runtime"
	"github.com/0xPolygon/polygon-edge/types"
)

var (
	oneCoin      = big.NewInt(1e18)
	epochsInYear = big.NewInt(31500)
	denominator  = big.NewInt(10000)
)

type TotalBalanceHydraStakingFn struct{}

type VaultDistributionHydraChainFn struct{}

// Test Transaction fees distribution to FeeHandler
func TestIntegration_DistributeFee(t *testing.T) {
	t.Parallel()

	fromAddr := types.Address{0x1}
	toAddr := &types.Address{0x2}
	value := big.NewInt(1)
	gasPrice := big.NewInt(10)
	txFees := new(big.Int).Mul(gasPrice, big.NewInt(21000))
	fromBalance := new(big.Int).Add(value, txFees)

	alloc := map[types.Address]*chain.GenesisAccount{
		contracts.FeeHandlerContract: {
			Code: contractsapi.HydraVault.DeployedBytecode,
		},
		fromAddr: {
			Balance: fromBalance,
		},
	}

	transition := newTestTransition(t, alloc)

	polyBFTConfig := PolyBFTConfig{
		// just an address for governance
		Governance: *toAddr,
	}

	// init FeeHandler
	err := initFeeHandler(polyBFTConfig, transition)
	require.NoError(t, err)

	tx := &types.Transaction{
		Nonce:    0,
		From:     fromAddr,
		To:       toAddr,
		Value:    value,
		Type:     types.LegacyTx,
		GasPrice: gasPrice,
		Gas:      21000,
	}

	err = transition.Write(tx)
	require.NoError(t, err)

	// Balance of FeeHandler must increase with 50% of the reward
	require.Equal(
		t,
		transition.GetBalance(contracts.FeeHandlerContract),
		new(big.Int).Div(txFees, big.NewInt(2)),
	)
}

// Test Transaction fees distribution to DAOIncentiveVault
func TestIntegration_DistributeDAOIncentive(t *testing.T) {
	t.Parallel()

	// init validator set
	validatorSet := validator.NewTestValidators(t, 5)

	reward := oneCoin

	accSet := validatorSet.GetPublicIdentities()
	accSetPrivateKeys := validatorSet.GetPrivateIdentities()

	// add contracts to genesis data
	alloc := getGenesisContractsMappings(t)

	// validator data for polybft config
	initValidators := make([]*validator.GenesisValidator, accSet.Len())

	for i, val := range accSet {
		// add validator to genesis data
		alloc[val.Address] = &chain.GenesisAccount{
			Balance: oneCoin,
		}

		signature, err := signer.MakeKOSKSignature(
			accSetPrivateKeys[i].Bls,
			val.Address,
			0,
			signer.DomainHydraChain,
		)
		require.NoError(t, err)

		signatureBytes, err := signature.Marshal()
		require.NoError(t, err)

		// create validator data for polybft config
		initValidators[i] = &validator.GenesisValidator{
			Address:      val.Address,
			BlsKey:       hex.EncodeToString(val.BlsKey.Marshal()),
			BlsSignature: hex.EncodeToString(signatureBytes),
			Stake:        initialMinStake,
		}
	}

	transition := newTestTransition(t, alloc)

	polyBFTConfig := PolyBFTConfig{
		InitialValidatorSet: initValidators,
		EpochSize:           24 * 60 * 60 / 2,
		SprintSize:          5,
		EpochReward:         reward.Uint64(),
		// use 1st account as governance address
		Governance: validatorSet.ToValidatorSet().Accounts().GetAddresses()[0],
	}

	// init all genesis contracts
	initGenesisContracts(t, transition, polyBFTConfig)

	commitEpochInput := createTestCommitEpochInput(t, 1, accSet, polyBFTConfig.EpochSize)
	input, err := commitEpochInput.EncodeAbi()
	require.NoError(t, err)

	// call commit epoch
	result := systemCallResult(t, transition, contracts.HydraChainContract, input)
	t.Logf(
		"Number of validators %d on commit epoch, Gas used %+v\n",
		accSet.Len(),
		result.GasUsed,
	)

	// create input for the vault distribution
	vaultDistributionInput, err := contractsapi.HydraChain.Abi.Methods["vaultDistribution"].Encode(&VaultDistributionHydraChainFn{})
	require.NoError(t, err)

	// call the vault distribution to get the value before distribution
	vaultDistributionRes := systemCallResult(t, transition, contracts.HydraChainContract, vaultDistributionInput)
	valueDistributionAmount := new(big.Int).SetBytes(vaultDistributionRes.ReturnValue)
	require.Equal(t, new(big.Int).Cmp(valueDistributionAmount), 0)

	// create input for distribute DAO incentive
	distributeDAOIncentiveInput, err := createTestDistributeDAOIncentiveInput(t).EncodeAbi()
	require.NoError(t, err)

	// call reward DAO incentive distributor
	result = systemCallResult(t, transition, contracts.HydraChainContract, distributeDAOIncentiveInput)

	// create input to get the total balance
	totalBalanceInput, err := contractsapi.HydraStaking.Abi.Methods["totalBalance"].Encode(&TotalBalanceHydraStakingFn{})
	require.NoError(t, err)

	// call the total balance method
	totalBalanceRes := systemCallResult(t, transition, contracts.HydraStakingContract, totalBalanceInput)
	totalBalance := new(big.Int).SetBytes(totalBalanceRes.ReturnValue)
	daoIncentiveRewards := calcDAORewards(totalBalance)

	// call the vault distribution to get the value before distribution
	vaultDistributionRes = systemCallResult(t, transition, contracts.HydraChainContract, vaultDistributionInput)
	valueDistributionAmount = new(big.Int).SetBytes(vaultDistributionRes.ReturnValue)

	// Vault distribution amount must increase 2% of the total staked amount
	require.Equal(
		t,
		valueDistributionAmount,
		daoIncentiveRewards,
	)
}

func TestIntegration_CommitEpoch(t *testing.T) {
	t.Parallel()

	// init validator sets
	// (cannot run test case with more than 100 validators at the moment,
	// because active validator set is capped to 100 on smart contract side)
	validatorSetSize := []int{5, 10, 50, 100}
	// number of delegators per validator
	delegatorsPerValidator := 100

	initialBalance := uint64(5e18) // 5 tokens
	reward := oneCoin
	delegateAmount := oneCoin

	validatorSets := make([]*validator.TestValidators, len(validatorSetSize), len(validatorSetSize))

	// create all validator sets which will be used in test
	for i, size := range validatorSetSize {
		aliases := make([]string, size, size)
		vps := make([]uint64, size, size)

		for j := 0; j < size; j++ {
			aliases[j] = "v" + strconv.Itoa(j)
			vps[j] = initialBalance
		}

		validatorSets[i] = validator.NewTestValidatorsWithAliases(t, aliases, vps)
	}

	// iterate through the validator set and do the test for each of them
	for _, currentValidators := range validatorSets {
		accSet := currentValidators.GetPublicIdentities()
		accSetPrivateKeys := currentValidators.GetPrivateIdentities()
		valid2deleg := make(
			map[types.Address][]*wallet.Key,
			accSet.Len(),
		) // delegators assigned to validators

		// add contracts to genesis data
		alloc := getGenesisContractsMappings(t)

		// validator data for polybft config
		initValidators := make([]*validator.GenesisValidator, accSet.Len())

		for i, val := range accSet {
			// add validator to genesis data
			alloc[val.Address] = &chain.GenesisAccount{
				Balance: oneCoin,
			}

			signature, err := signer.MakeKOSKSignature(
				accSetPrivateKeys[i].Bls,
				val.Address,
				0,
				signer.DomainHydraChain,
			)
			require.NoError(t, err)

			signatureBytes, err := signature.Marshal()
			require.NoError(t, err)

			// create validator data for polybft config
			initValidators[i] = &validator.GenesisValidator{
				Address:      val.Address,
				BlsKey:       hex.EncodeToString(val.BlsKey.Marshal()),
				BlsSignature: hex.EncodeToString(signatureBytes),
				Stake:        initialMinStake,
			}

			// create delegators
			delegatorAccs := createRandomTestKeys(t, delegatorsPerValidator)

			// add delegators to genesis data
			for j := 0; j < delegatorsPerValidator; j++ {
				delegator := delegatorAccs[j]
				alloc[types.Address(delegator.Address())] = &chain.GenesisAccount{
					Balance: new(big.Int).SetUint64(initialBalance),
				}
			}

			valid2deleg[val.Address] = delegatorAccs
		}

		transition := newTestTransition(t, alloc)

		polyBFTConfig := PolyBFTConfig{
			InitialValidatorSet: initValidators,
			EpochSize:           24 * 60 * 60 / 2,
			SprintSize:          5,
			EpochReward:         reward.Uint64(),
			// use 1st account as governance address
			Governance: currentValidators.ToValidatorSet().Accounts().GetAddresses()[0],
		}

		// init all genesis contracts
		initGenesisContracts(t, transition, polyBFTConfig)

		// delegate amounts to validators
		for valAddress, delegators := range valid2deleg {
			for _, delegator := range delegators {
				encoded, err := contractsapi.HydraDelegation.Abi.Methods["delegate"].Encode(
					[]interface{}{valAddress, false})
				require.NoError(t, err)

				result := transition.Call2(
					types.Address(delegator.Address()),
					contracts.HydraDelegationContract,
					encoded,
					delegateAmount,
					1000000000000,
				)
				require.False(t, result.Failed())
			}
		}

		commitEpochInput := createTestCommitEpochInput(t, 1, accSet, polyBFTConfig.EpochSize)
		input, err := commitEpochInput.EncodeAbi()
		require.NoError(t, err)

		// call commit epoch
		result := transition.Call2(
			contracts.SystemCaller,
			contracts.HydraChainContract,
			input,
			big.NewInt(0),
			10000000000,
		)
		require.NoError(t, result.Err)
		t.Logf(
			"Number of validators %d on commit epoch when we add %d of delegators, Gas used %+v\n",
			accSet.Len(),
			accSet.Len()*delegatorsPerValidator,
			result.GasUsed,
		)

		// create input for reward wallet fund
		rewardWalletFundAmount := createTestRewardWalletFundAmount(t)
		fundRewardWalletInput, err := createTestFundRewardWalletInput(t).EncodeAbi()
		require.NoError(t, err)

		// call reward distributor
		result = transition.Call2(
			contracts.SystemCaller,
			contracts.RewardWalletContract,
			fundRewardWalletInput,
			rewardWalletFundAmount,
			10000000000,
		)

		// create input for distribute rewards
		distributeRewards := createTestDistributeRewardsInput(t, 1, accSet, polyBFTConfig.EpochSize)
		distributeRewardsInput, err := distributeRewards.EncodeAbi()
		require.NoError(t, err)

		// call reward distributor
		result = transition.Call2(
			contracts.SystemCaller,
			contracts.HydraStakingContract,
			distributeRewardsInput,
			big.NewInt(0),
			10000000000,
		)
		require.NoError(t, result.Err)

		// create input for distribute DAO incentive
		distributeDAOIncentive := createTestDistributeDAOIncentiveInput(t)
		distributeDAOIncentiveInput, err := distributeDAOIncentive.EncodeAbi()
		require.NoError(t, err)

		// call reward DAO incentive distributor
		result = transition.Call2(
			contracts.SystemCaller,
			contracts.HydraChainContract,
			distributeDAOIncentiveInput,
			big.NewInt(0),
			10000000000,
		)
		require.NoError(t, result.Err)

		commitEpochInput = createTestCommitEpochInput(t, 2, accSet, polyBFTConfig.EpochSize)
		input, err = commitEpochInput.EncodeAbi()
		require.NoError(t, err)

		// call commit epoch
		result = transition.Call2(
			contracts.SystemCaller,
			contracts.HydraChainContract,
			input,
			big.NewInt(0),
			10000000000,
		)
		require.NoError(t, result.Err)
		t.Logf(
			"Number of validators %d, Number of delegator %d, Gas used %+v\n",
			accSet.Len(),
			accSet.Len()*delegatorsPerValidator,
			result.GasUsed,
		)

		distributeRewards = createTestDistributeRewardsInput(t, 2, accSet, polyBFTConfig.EpochSize)
		distributeRewardsInput, err = distributeRewards.EncodeAbi()
		require.NoError(t, err)

		// call reward distributor
		result = transition.Call2(
			contracts.SystemCaller,
			contracts.HydraStakingContract,
			distributeRewardsInput,
			big.NewInt(0),
			10000000000,
		)
		require.NoError(t, result.Err)
	}
}

// Internal function to get the genesis contracts mappings
func getGenesisContractsMappings(t *testing.T) map[types.Address]*chain.GenesisAccount {
	t.Helper()

	// add contracts to genesis data
	return map[types.Address]*chain.GenesisAccount{
		contracts.HydraChainContract: {
			Code: contractsapi.HydraChain.DeployedBytecode,
		},
		contracts.HydraStakingContract: {
			Code: contractsapi.HydraStaking.DeployedBytecode,
		},
		contracts.HydraDelegationContract: {
			Code: contractsapi.HydraDelegation.DeployedBytecode,
		},
		contracts.VestingManagerFactoryContract: {
			Code: contractsapi.VestingManagerFactory.DeployedBytecode,
		},
		contracts.APRCalculatorContract: {
			Code: contractsapi.APRCalculator.DeployedBytecode,
		},
		contracts.BLSContract: {
			Code: contractsapi.BLS.DeployedBytecode,
		},
		contracts.LiquidityTokenContract: {
			Code: contractsapi.LiquidityToken.DeployedBytecode,
		},
		contracts.RewardWalletContract: {
			Code:    contractsapi.RewardWallet.DeployedBytecode,
			Balance: common.GetTwoThirdOfMaxUint256(),
		},
		contracts.DAOIncentiveVaultContract: {
			Code: contractsapi.HydraVault.DeployedBytecode,
		},
	}
}

// Internal function to init the genesis contracts
func initGenesisContracts(t *testing.T, transition *state.Transition, polyBFTConfig PolyBFTConfig) {
	t.Helper()

	// init LiquidityToken
	err := initLiquidityToken(polyBFTConfig, transition)
	require.NoError(t, err)

	// init HydraChain
	err = initHydraChain(polyBFTConfig, transition)
	require.NoError(t, err)

	// init HydraStaking
	err = initHydraStaking(polyBFTConfig, transition)
	require.NoError(t, err)

	// init HydraDelegation
	err = initHydraDelegation(polyBFTConfig, transition)
	require.NoError(t, err)

	// init VestingManagerFactory
	err = initVestingManagerFactory(polyBFTConfig, transition)
	require.NoError(t, err)

	// init APRCalculator
	err = initAPRCalculator(polyBFTConfig, transition)
	require.NoError(t, err)

	// initialize RewardWallet SC
	err = initRewardWallet(polyBFTConfig, transition)
	require.NoError(t, err)

	// initialize DAOIncentiveVault SC
	err = initDAOIncentiveVault(polyBFTConfig, transition)
	require.NoError(t, err)
}

// Function to create a system call and return the result
func systemCallResult(t *testing.T, transition *state.Transition, to types.Address, input []byte) *runtime.ExecutionResult {
	t.Helper()

	result := transition.Call2(
		contracts.SystemCaller,
		to,
		input,
		big.NewInt(0),
		10000000000,
	)
	require.NoError(t, result.Err)

	return result
}

// Function to calculate the DAO rewards, same as in distributeVaultFunds method from the contracts
func calcDAORewards(staked *big.Int) *big.Int {
	res := big.NewInt(0)

	return res.Mul(staked, big.NewInt(200)).Div(res, denominator).Div(res, epochsInYear)
}
