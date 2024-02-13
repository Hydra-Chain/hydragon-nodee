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

// H_MODIFY: Unused functionality
// func TestIntegratoin_PerformExit(t *testing.T) {
// 	t.Parallel()

// 	const gasLimit = 1000000000000

// 	// create validator set and checkpoint mngr
// 	currentValidators := newTestValidatorsWithAliases(t, []string{"A", "B", "C", "D"}, []uint64{100, 100, 100, 100})
// 	accSet := currentValidators.getPublicIdentities()
// 	cm := checkpointManager{blockchain: &blockchainMock{}}

// 	deployerAddress := types.Address{76, 76, 1} // account that will deploy contracts
// 	senderAddress := types.Address{1}           // account that sends exit/withdraw transactions
// 	receiverAddr := types.Address{6}            // account that receive tokens
// 	amount1 := big.NewInt(3)                    // amount of the first widrawal
// 	amount2 := big.NewInt(2)                    // amount of the second widrawal
// 	bn256Addr := types.Address{2}               // bls contract
// 	stateSenderAddr := types.Address{5}         // generic bridge contract on rootchain

// 	alloc := map[types.Address]*chain.GenesisAccount{
// 		senderAddress:         {Balance: new(big.Int).Add(amount1, amount2)}, // give some ethers to sender
// 		deployerAddress:       {Balance: ethgo.Ether(100)},                   // give 100 ethers to deployer
// 		contracts.BLSContract: {Code: contractsapi.BLS.DeployedBytecode},
// 		bn256Addr:             {Code: contractsapi.BLS256.DeployedBytecode},
// 		stateSenderAddr:       {Code: contractsapi.StateSender.DeployedBytecode},
// 	}
// 	transition := newTestTransition(t, alloc)

// 	getField := func(addr types.Address, abi *abi.ABI, function string, args ...interface{}) []byte {
// 		input, err := abi.GetMethod(function).Encode(args)
// 		require.NoError(t, err)

// 		result := transition.Call2(deployerAddress, addr, input, big.NewInt(0), gasLimit)
// 		require.True(t, result.Succeeded())

// deploy MockERC20 as root chain ERC 20 token
// rootERC20Addr := deployAndInitContract(t, transition, contractsapi.RootERC20.Bytecode, deployerAddress, nil)

// deploy CheckpointManager
// checkpointManagerInit := func() ([]byte, error) {
// 	return (&contractsapi.InitializeCheckpointManagerFn{
// 		NewBls:          contracts.BLSContract,
// 		NewBn256G2:      bn256Addr,
// 		NewValidatorSet: accSet.ToAPIBinding(),
// 		ChainID_:        big.NewInt(0),
// 	}).EncodeAbi()
// }

// checkpointMgrConstructor := &contractsapi.CheckpointManagerConstructorFn{Initiator: deployerAddress}
// constructorInput, err := checkpointMgrConstructor.EncodeAbi()
// require.NoError(t, err)

// checkpointManagerAddr := deployAndInitContract(t, transition, append(contractsapi.CheckpointManager.Bytecode, constructorInput...), deployerAddress, checkpointManagerInit)

// // deploy ExitHelper
// exitHelperInit := func() ([]byte, error) {
// 	return (&contractsapi.InitializeExitHelperFn{NewCheckpointManager: checkpointManagerAddr}).EncodeAbi()
// }
// exitHelperContractAddress := deployAndInitContract(t, transition, contractsapi.ExitHelper.Bytecode, deployerAddress, exitHelperInit)

// // deploy RootERC20Predicate
// rootERC20PredicateInit := func() ([]byte, error) {
// 	return (&contractsapi.InitializeRootERC20PredicateFn{
// 		NewStateSender:         stateSenderAddr,
// 		NewExitHelper:          exitHelperContractAddress,
// 		NewChildERC20Predicate: contracts.ChildERC20PredicateContract,
// 		NewChildTokenTemplate:  contracts.ChildERC20Contract,
// 		NativeTokenRootAddress: contracts.NativeERC20TokenContract,
// 	}).EncodeAbi()
// }
// rootERC20PredicateAddr := deployAndInitContract(t, transition, contractsapi.RootERC20Predicate.Bytecode, deployerAddress, rootERC20PredicateInit)

// 	// deploy RootERC20Predicate
// 	rootERC20PredicateInit := func() ([]byte, error) {
// 		return (&contractsapi.InitializeRootERC20PredicateFn{
// 			NewStateSender:         stateSenderAddr,
// 			NewExitHelper:          exitHelperContractAddress,
// 			NewChildERC20Predicate: contracts.ChildERC20PredicateContract,
// 			NewChildTokenTemplate:  contracts.ChildERC20Contract,
// 			NativeTokenRootAddress: contracts.NativeERC20TokenContract,
// 		}).EncodeAbi()
// 	}
// 	rootERC20PredicateAddr := deployAndInitContract(t, transition, contractsapi.RootERC20Predicate, deployerAddress, rootERC20PredicateInit)

// 	// validate initialization of CheckpointManager
// 	require.Equal(t, getField(checkpointManagerAddr, contractsapi.CheckpointManager.Abi, "currentCheckpointBlockNumber")[31], uint8(0))

// 	accSetHash, err := accSet.Hash()
// 	require.NoError(t, err)

// 	blockHash := types.Hash{5}
// 	blockNumber := uint64(1)
// 	epochNumber := uint64(1)
// 	blockRound := uint64(1)

// 	// mint
// 	mintInput, err := (&contractsapi.MintRootERC20Fn{
// 		To:     senderAddress,
// 		Amount: alloc[senderAddress].Balance,
// 	}).EncodeAbi()
// 	require.NoError(t, err)

// 	result := transition.Call2(deployerAddress, rootERC20Addr, mintInput, nil, gasLimit)
// 	require.NoError(t, result.Err)

// 	// approve
// 	approveInput, err := (&contractsapi.ApproveRootERC20Fn{
// 		Spender: rootERC20PredicateAddr,
// 		Amount:  alloc[senderAddress].Balance,
// 	}).EncodeAbi()
// 	require.NoError(t, err)

// 	result = transition.Call2(senderAddress, rootERC20Addr, approveInput, big.NewInt(0), gasLimit)
// 	require.NoError(t, result.Err)

// 	// deposit
// 	depositInput, err := (&contractsapi.DepositToRootERC20PredicateFn{
// 		RootToken: rootERC20Addr,
// 		Receiver:  receiverAddr,
// 		Amount:    new(big.Int).Add(amount1, amount2),
// 	}).EncodeAbi()
// 	require.NoError(t, err)

// 	// send sync events to childchain so that receiver can obtain tokens
// 	result = transition.Call2(senderAddress, rootERC20PredicateAddr, depositInput, big.NewInt(0), gasLimit)
// 	require.NoError(t, result.Err)

// 	// simulate withdrawal from childchain to rootchain
// 	widthdrawSig := crypto.Keccak256([]byte("WITHDRAW"))
// 	erc20DataType := abi.MustNewType(
// 		"tuple(bytes32 withdrawSignature, address rootToken, address withdrawer, address receiver, uint256 amount)")

// 	exitData1, err := erc20DataType.Encode(map[string]interface{}{
// 		"withdrawSignature": widthdrawSig,
// 		"rootToken":         ethgo.Address(rootERC20Addr),
// 		"withdrawer":        ethgo.Address(senderAddress),
// 		"receiver":          ethgo.Address(receiverAddr),
// 		"amount":            amount1,
// 	})
// 	require.NoError(t, err)

// exits := []*ExitEvent{
// 	{
// 		L2StateSyncedEvent: &contractsapi.L2StateSyncedEvent{
// 			ID:       big.NewInt(1),
// 			Sender:   contracts.ChildERC20PredicateContract,
// 			Receiver: rootERC20PredicateAddr,
// 			Data:     exitData1,
// 		},
// 	},
// 	{
// 		L2StateSyncedEvent: &contractsapi.L2StateSyncedEvent{
// 			ID:       big.NewInt(2),
// 			Sender:   contracts.ChildERC20PredicateContract,
// 			Receiver: rootERC20PredicateAddr,
// 			Data:     exitData2,
// 		},
// 	},
// }
// exitTree, err := createExitTree(exits)
// require.NoError(t, err)

// 	exits := []*ExitEvent{
// 		{
// 			ID:       1,
// 			Sender:   ethgo.Address(contracts.ChildERC20PredicateContract),
// 			Receiver: ethgo.Address(rootERC20PredicateAddr),
// 			Data:     exitData1,
// 		},
// 		{
// 			ID:       2,
// 			Sender:   ethgo.Address(contracts.ChildERC20PredicateContract),
// 			Receiver: ethgo.Address(rootERC20PredicateAddr),
// 			Data:     exitData2,
// 		},
// 	}
// 	exitTree, err := createExitTree(exits)
// 	require.NoError(t, err)

// 	eventRoot := exitTree.Hash()

// 	checkpointData := &CheckpointData{
// 		BlockRound:            blockRound,
// 		EpochNumber:           epochNumber,
// 		CurrentValidatorsHash: accSetHash,
// 		NextValidatorsHash:    accSetHash,
// 		EventRoot:             eventRoot,
// 	}

// 	checkpointHash, err := checkpointData.Hash(
// 		cm.blockchain.GetChainID(),
// 		blockRound,
// 		blockHash)
// 	require.NoError(t, err)

// 	i := uint64(0)
// 	bmp := bitmap.Bitmap{}
// 	signatures := bls.Signatures(nil)

// 	currentValidators.iterAcct(nil, func(v *testValidator) {
// 		signatures = append(signatures, v.mustSign(checkpointHash[:], bls.DomainCheckpointManager))
// 		bmp.Set(i)
// 		i++
// 	})

// 	aggSignature, err := signatures.Aggregate().Marshal()
// 	require.NoError(t, err)

// 	extra := &Extra{
// 		Checkpoint: checkpointData,
// 		Committed: &Signature{
// 			AggregatedSignature: aggSignature,
// 			Bitmap:              bmp,
// 		},
// 	}

// 	// submit a checkpoint
// 	submitCheckpointEncoded, err := cm.abiEncodeCheckpointBlock(blockNumber, blockHash, extra, accSet)
// 	require.NoError(t, err)

// 	result = transition.Call2(senderAddress, checkpointManagerAddr, submitCheckpointEncoded, big.NewInt(0), gasLimit)
// 	require.NoError(t, result.Err)
// 	require.Equal(t, getField(checkpointManagerAddr, contractsapi.CheckpointManager.Abi, "currentCheckpointBlockNumber")[31], uint8(1))

// proofExitEvent, err := exits[0].L2StateSyncedEvent.Encode()
// require.NoError(t, err)

// 	var exitEventAPI contractsapi.L2StateSyncedEvent
// 	proofExitEvent, err := exitEventAPI.Encode(exits[0])
// 	require.NoError(t, err)

// 	proof, err := exitTree.GenerateProof(proofExitEvent)
// 	require.NoError(t, err)

// 	leafIndex, err := exitTree.LeafIndex(proofExitEvent)
// 	require.NoError(t, err)

// 	exitFnInput, err := (&contractsapi.ExitExitHelperFn{
// 		BlockNumber:  new(big.Int).SetUint64(blockNumber),
// 		LeafIndex:    new(big.Int).SetUint64(leafIndex),
// 		UnhashedLeaf: proofExitEvent,
// 		Proof:        proof,
// 	}).EncodeAbi()
// 	require.NoError(t, err)

// 	result = transition.Call2(senderAddress, exitHelperContractAddress, exitFnInput, big.NewInt(0), gasLimit)
// 	require.NoError(t, result.Err)

// 	// check that first exit event is processed
// 	res = getField(exitHelperContractAddress, contractsapi.ExitHelper.Abi, "processedExits", exits[0].ID)
// 	require.Equal(t, 1, int(res[31]))

// 	res = getField(rootERC20Addr, contractsapi.RootERC20.Abi, "balanceOf", receiverAddr)
// 	require.Equal(t, amount1, new(big.Int).SetBytes(res))
// }

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
				Code: contractsapi.ChildValidatorSet.DeployedBytecode,
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
				Stake:        big.NewInt(int64(initialBalance)),
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

		// init ValidatorSet
		err = initValidatorSet(polyBFTConfig, transition)
		require.NoError(t, err)

		// delegate amounts to validators
		for valAddress, delegators := range valid2deleg {
			for _, delegator := range delegators {
				encoded, err := contractsapi.ChildValidatorSet.Abi.Methods["delegate"].Encode(
					[]interface{}{valAddress, false})
				require.NoError(t, err)

				result := transition.Call2(types.Address(delegator.Address()), contracts.ValidatorSetContract, encoded, delegateAmount, 1000000000000)
				require.False(t, result.Failed())
			}
		}

		commitEpochTxValue := createTestCommitEpochTxValue(t, transition)
		commitEpochInput := createTestCommitEpochInputWithVals(t, 1, accSet, polyBFTConfig.EpochSize)
		input, err := commitEpochInput.EncodeAbi()
		require.NoError(t, err)

		// Normally injecting balance to the system caller is handled by a higher order method in the executor.go
		// but here we use call2 directly so we need to do it manually
		transition.Txn().AddBalance(contracts.SystemCaller, commitEpochTxValue)

		// call commit epoch
		result := transition.Call2(contracts.SystemCaller, contracts.ValidatorSetContract, input, commitEpochTxValue, 10000000000)
		require.NoError(t, result.Err)
		t.Logf("Number of validators %d when we add %d of delegators, Gas used %+v\n", accSet.Len(), accSet.Len()*delegatorsPerValidator, result.GasUsed)

		commitEpochInput = createTestCommitEpochInputWithVals(t, 2, accSet, polyBFTConfig.EpochSize)
		input, err = commitEpochInput.EncodeAbi()
		require.NoError(t, err)

		transition.Txn().AddBalance(contracts.SystemCaller, commitEpochTxValue)

		// call commit epoch
		result = transition.Call2(contracts.SystemCaller, contracts.ValidatorSetContract, input, commitEpochTxValue, 10000000000)
		require.NoError(t, result.Err)
		t.Logf("Number of validators %d, Number of delegator %d, Gas used %+v\n", accSet.Len(), accSet.Len()*delegatorsPerValidator, result.GasUsed)
	}
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

	paddingSize := length - len(slice)
	padding := make([]byte, paddingSize, paddingSize)

	return append(padding, slice...)
}
