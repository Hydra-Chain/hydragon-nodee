package polybft

import (
	"math/big"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/0xPolygon/polygon-edge/chain"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	bls "github.com/0xPolygon/polygon-edge/consensus/polybft/signer"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/validator"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/wallet"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/helper/hex"
	"github.com/0xPolygon/polygon-edge/state"
	"github.com/0xPolygon/polygon-edge/types"
)

var (
	oneCoin = big.NewInt(1e18)
)

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
		valid2deleg := make(map[types.Address][]*wallet.Key, accSet.Len()) // delegators assigned to validators

		// add contracts to genesis data
		alloc := map[types.Address]*chain.GenesisAccount{
			contracts.ValidatorSetContract: {
				Code: contractsapi.ValidatorSet.DeployedBytecode,
			},
			contracts.RewardPoolContract: {
				Code: contractsapi.RewardPool.DeployedBytecode,
			},
			contracts.BLSContract: {
				Code: contractsapi.BLS.DeployedBytecode,
			},
			contracts.LiquidityTokenContract: {
				Code: contractsapi.LiquidityToken.DeployedBytecode,
			},
		}

		// validator data for polybft config
		initValidators := make([]*validator.GenesisValidator, accSet.Len())

		for i, val := range accSet {
			// add validator to genesis data
			alloc[val.Address] = &chain.GenesisAccount{
				Balance: oneCoin,
			}

			signature, err := bls.MakeKOSKSignature(accSetPrivateKeys[i].Bls, val.Address, 0, bls.DomainValidatorSet)
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

		// init LiquidityToken
		err := initLiquidityToken(polyBFTConfig, transition)
		require.NoError(t, err)

		// init RewardPool
		err = initRewardPool(polyBFTConfig, transition)
		require.NoError(t, err)

		// init ValidatorSet
		err = initValidatorSet(polyBFTConfig, transition)
		require.NoError(t, err)

		// delegate amounts to validators
		for valAddress, delegators := range valid2deleg {
			for _, delegator := range delegators {
				encoded, err := contractsapi.ValidatorSet.Abi.Methods["delegate"].Encode(
					[]interface{}{valAddress, false})
				require.NoError(t, err)

				result := transition.Call2(types.Address(delegator.Address()), contracts.ValidatorSetContract, encoded, delegateAmount, 1000000000000)
				require.False(t, result.Failed())
			}
		}

		commitEpochInput := createTestCommitEpochInput(t, 1, polyBFTConfig.EpochSize)
		input, err := commitEpochInput.EncodeAbi()
		require.NoError(t, err)

		// call commit epoch
		result := transition.Call2(contracts.SystemCaller, contracts.ValidatorSetContract, input, big.NewInt(0), 10000000000)
		require.NoError(t, result.Err)
		t.Logf("Number of validators %d on commit epoch when we add %d of delegators, Gas used %+v\n", accSet.Len(), accSet.Len()*delegatorsPerValidator, result.GasUsed)

		// create input for distribute rewards
		maxRewardToDistribute := createTestRewardToDistributeValue(t, transition)
		distributeRewards := createTestDistributeRewardsInput(t, 1, accSet, polyBFTConfig.EpochSize)
		distributeRewardsInput, err := distributeRewards.EncodeAbi()
		require.NoError(t, err)

		// Normally injecting balance to the system caller is handled by a higher order method in the executor.go
		// but here we use call2 directly so we need to do it manually
		transition.Txn().AddBalance(contracts.SystemCaller, maxRewardToDistribute)

		// call reward distributor
		result = transition.Call2(contracts.SystemCaller, contracts.RewardPoolContract, distributeRewardsInput, maxRewardToDistribute, 10000000000)
		require.NoError(t, result.Err)
		t.Logf("Number of validators %d on reward distribution when we add %d of delegators, Gas used %+v\n", accSet.Len(), accSet.Len()*delegatorsPerValidator, result.GasUsed)

		commitEpochInput = createTestCommitEpochInput(t, 2, polyBFTConfig.EpochSize)
		input, err = commitEpochInput.EncodeAbi()
		require.NoError(t, err)

		// call commit epoch
		result = transition.Call2(contracts.SystemCaller, contracts.ValidatorSetContract, input, big.NewInt(0), 10000000000)
		require.NoError(t, result.Err)
		t.Logf("Number of validators %d, Number of delegator %d, Gas used %+v\n", accSet.Len(), accSet.Len()*delegatorsPerValidator, result.GasUsed)

		distributeRewards = createTestDistributeRewardsInput(t, 2, accSet, polyBFTConfig.EpochSize)
		distributeRewardsInput, err = distributeRewards.EncodeAbi()
		require.NoError(t, err)

		transition.Txn().AddBalance(contracts.SystemCaller, maxRewardToDistribute)

		// call reward distributor
		result = transition.Call2(contracts.SystemCaller, contracts.RewardPoolContract, distributeRewardsInput, maxRewardToDistribute, 10000000000)
		require.NoError(t, result.Err)
		t.Logf("Number of validators %d on reward distribution when we add %d of delegators, Gas used %+v\n", accSet.Len(), accSet.Len()*delegatorsPerValidator, result.GasUsed)
	}
}

// Test Transaction fees distribution to FeeHandler
func TestIntegration_DistributeFee(t *testing.T) {
	fromAddr := types.Address{0x1}
	toAddr := &types.Address{0x2}
	value := big.NewInt(1)
	gasPrice := big.NewInt(10)
	txFees := new(big.Int).Mul(gasPrice, big.NewInt(21000))
	fromBalance := new(big.Int).Add(value, txFees)

	alloc := map[types.Address]*chain.GenesisAccount{
		contracts.FeeHandlerContract: {
			Code: contractsapi.FeeHandler.DeployedBytecode,
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

	// init ValidatorSet
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
	require.Equal(t, transition.GetBalance(contracts.FeeHandlerContract), new(big.Int).Div(txFees, big.NewInt(2)))
}

func deployAndInitContract(t *testing.T, transition *state.Transition, bytecode []byte, sender types.Address,
	initCallback func() ([]byte, error)) types.Address {
	t.Helper()

	deployResult := transition.Create2(sender, bytecode, big.NewInt(0), 1e9)
	assert.NoError(t, deployResult.Err)

	if initCallback != nil {
		initInput, err := initCallback()
		require.NoError(t, err)

		result := transition.Call2(sender, deployResult.Address, initInput, big.NewInt(0), 1e9)
		require.NoError(t, result.Err)
	}

	return deployResult.Address
}

func leftPadBytes(slice []byte, length int) []byte {
	if len(slice) >= length {
		return slice
	}
	padding := make([]byte, length-len(slice))
	return append(padding, slice...)
}
