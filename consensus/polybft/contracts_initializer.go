package polybft

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/validator"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/state"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/umbracle/ethgo/abi"
)

var (
	initialMinStake, _  = new(big.Int).SetString("15000000000000000000000", 10)
	minDelegation int64 = 1e18

	contractCallGasLimit uint64 = 100_000_000
)

// initHydraChain initializes HydraChain SC
func initHydraChain(polyBFTConfig PolyBFTConfig, transition *state.Transition) error {
	initialValidators := make([]*contractsapi.ValidatorInit, len(polyBFTConfig.InitialValidatorSet))

	for i, validator := range polyBFTConfig.InitialValidatorSet {
		validatorData, err := validator.ToValidatorInitAPIBinding()
		if err != nil {
			return err
		}

		initialValidators[i] = validatorData
	}

	initFn := &contractsapi.InitializeHydraChainFn{
		NewValidators:         initialValidators,
		Governance:            polyBFTConfig.Governance,
		StakingContractAddr:   contracts.HydraChainContract,
		NewBls:                contracts.BLSContract,
	}

	input, err := initFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("HydraChain.initialize params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller,
		contracts.HydraChainContract, input, "HydraChain.initialize", transition)
}

// initHydraStaking initializes HydraStaking SC
func initHydraStaking(polyBFTConfig PolyBFTConfig, transition *state.Transition) error {
	initialStakers, err := validator.GetInitialStakers(polyBFTConfig.InitialValidatorSet)
	if (err != nil) {
		return err
	}

	initFn := &contractsapi.InitializeHydraStakingFn{
		InitialStakers: initialStakers,
		NewMinStake: initialMinStake,
		NewLiquidToken: contracts.LiquidityTokenContract,
		HydraChainAddr: contracts.HydraChainContract,
		AprCalculatorAddr: contracts.APRCalculatorContract,
		Governance: polyBFTConfig.Governance,
		DelegationContractAddr: contracts.HydraDelegationContract,
	}

	input, err := initFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("HydraStaking.initialize params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller,
		contracts.HydraStakingContract, input, "HydraStaking.initialize", transition)
}

// initHydraDelegation initializes HydraDelegation SC
func initHydraDelegation(polyBFTConfig PolyBFTConfig, transition *state.Transition) error {
	initialStakers, err := validator.GetInitialStakers(polyBFTConfig.InitialValidatorSet)
	if (err != nil) {
		return err
	}

	initFn := &contractsapi.InitializeHydraDelegationFn{
		InitialStakers: initialStakers,
		LiquidToken: contracts.LiquidityTokenContract,
		Governance: polyBFTConfig.Governance,
		AprCalculatorAddr: contracts.APRCalculatorContract,
		HydraStakingAddr: contracts.HydraStakingContract,
		EpochManagerAddr: contracts.HydraChainContract,
		VestingManagerFactoryAddr: contracts.VestingManagerFactoryContract,
	}

	input, err := initFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("HydraDelegation.initialize params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller,
		contracts.HydraDelegationContract, input, "HydraDelegation.initialize", transition)
}

// initVestingManagerFactory initializes VestingManagerFactory SC
func initVestingManagerFactory(polyBFTConfig PolyBFTConfig, transition *state.Transition) error {
	initFn := &contractsapi.InitializeVestingManagerFactoryFn{
		HydraDelegationAddr:  contracts.HydraDelegationContract,
	}

	input, err := initFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("VestingManagerFactory.initialize params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller,
		contracts.VestingManagerFactoryContract, input, "VestingManagerFactory.initialize", transition)
}

// initAPRCalculator initializes APRCalculator SC
func initAPRCalculator(polyBFTConfig PolyBFTConfig, transition *state.Transition) error {
	initFn := &contractsapi.InitializeAPRCalculatorFn{
		Manager:  polyBFTConfig.Governance,
	}

	input, err := initFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("APRCalculator.initialize params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller,
		contracts.APRCalculatorContract, input, "APRCalculator.initialize", transition)
}

func initFeeHandler(polybftConfig PolyBFTConfig, transition *state.Transition) error {
	initFn := &contractsapi.InitializeFeeHandlerFn{
		Owner: polybftConfig.Governance,
	}

	input, err := initFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("FeeHandler.initialize params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller, contracts.FeeHandlerContract, input, "FeeHandler.initialize", transition)
}

func initLiquidityToken(polyBFTConfig PolyBFTConfig, transition *state.Transition) error {
	initFn := contractsapi.InitializeLiquidityTokenFn{
		Name_:            "Liquid Hydra",
		Symbol_:          "LYDRA",
		Governer:         polyBFTConfig.Governance,
		HydraStakingAddr: contracts.HydraStakingContract,
		HydraDelegationAddr: contracts.HydraDelegationContract,
	}

	input, err := initFn.EncodeAbi()
	if err != nil {
		return fmt.Errorf("LiquidityToken.initialize params encoding failed: %w", err)
	}

	return callContract(contracts.SystemCaller, contracts.LiquidityTokenContract, input, "LiquidityToken.initialize", transition)
}

// // getInitERC20PredicateInput builds initialization input parameters for child chain ERC20Predicate SC
// func getInitERC20PredicateInput(config *BridgeConfig, childChainMintable bool) ([]byte, error) {
// 	var params contractsapi.StateTransactionInput
// 	if childChainMintable {
// 		params = &contractsapi.InitializeRootMintableERC20PredicateFn{
// 			NewL2StateSender:       contracts.L2StateSenderContract,
// 			NewStateReceiver:       contracts.StateReceiverContract,
// 			NewChildERC20Predicate: config.ChildMintableERC20PredicateAddr,
// 			NewChildTokenTemplate:  config.ChildERC20Addr,
// 		}
// 	} else {
// 		params = &contractsapi.InitializeChildERC20PredicateFn{
// 			NewL2StateSender:          contracts.L2StateSenderContract,
// 			NewStateReceiver:          contracts.StateReceiverContract,
// 			NewRootERC20Predicate:     config.RootERC20PredicateAddr,
// 			NewChildTokenTemplate:     contracts.ChildERC20Contract,
// 			NewNativeTokenRootAddress: config.RootNativeERC20Addr,
// 		}
// 	}

// 	return params.EncodeAbi()
// }

// // getInitERC20PredicateACLInput builds initialization input parameters for child chain ERC20PredicateAccessList SC
// func getInitERC20PredicateACLInput(config *BridgeConfig, owner types.Address,
// 	childChainMintable bool) ([]byte, error) {
// 	var params contractsapi.StateTransactionInput
// 	if childChainMintable {
// 		params = &contractsapi.InitializeRootMintableERC20PredicateACLFn{
// 			NewL2StateSender:       contracts.L2StateSenderContract,
// 			NewStateReceiver:       contracts.StateReceiverContract,
// 			NewChildERC20Predicate: config.ChildMintableERC20PredicateAddr,
// 			NewChildTokenTemplate:  config.ChildERC20Addr,
// 			NewUseAllowList:        owner != contracts.SystemCaller,
// 			NewUseBlockList:        owner != contracts.SystemCaller,
// 			NewOwner:               owner,
// 		}
// 	} else {
// 		params = &contractsapi.InitializeChildERC20PredicateACLFn{
// 			NewL2StateSender:          contracts.L2StateSenderContract,
// 			NewStateReceiver:          contracts.StateReceiverContract,
// 			NewRootERC20Predicate:     config.RootERC20PredicateAddr,
// 			NewChildTokenTemplate:     contracts.ChildERC20Contract,
// 			NewNativeTokenRootAddress: config.RootNativeERC20Addr,
// 			NewUseAllowList:           owner != contracts.SystemCaller,
// 			NewUseBlockList:           owner != contracts.SystemCaller,
// 			NewOwner:                  owner,
// 		}
// 	}

// 	return params.EncodeAbi()
// }

// getInitERC721PredicateInput builds initialization input parameters for child chain ERC721Predicate SC
// func getInitERC721PredicateInput(config *BridgeConfig, childOriginatedTokens bool) ([]byte, error) {
// 	var params contractsapi.StateTransactionInput
// 	if childOriginatedTokens {
// 		params = &contractsapi.InitializeRootMintableERC721PredicateFn{
// 			NewL2StateSender:        contracts.L2StateSenderContract,
// 			NewStateReceiver:        contracts.StateReceiverContract,
// 			NewChildERC721Predicate: config.ChildMintableERC721PredicateAddr,
// 			NewChildTokenTemplate:   config.ChildERC721Addr,
// 		}
// 	} else {
// 		params = &contractsapi.InitializeChildERC721PredicateFn{
// 			NewL2StateSender:       contracts.L2StateSenderContract,
// 			NewStateReceiver:       contracts.StateReceiverContract,
// 			NewRootERC721Predicate: config.RootERC721PredicateAddr,
// 			NewChildTokenTemplate:  contracts.ChildERC721Contract,
// 		}
// 	}

// 	return params.EncodeAbi()
// }

// getInitERC721PredicateACLInput builds initialization input parameters
// for child chain ERC721PredicateAccessList SC
// func getInitERC721PredicateACLInput(config *BridgeConfig, owner types.Address,
// 	useAllowList, useBlockList, childChainMintable bool) ([]byte, error) {
// 	var params contractsapi.StateTransactionInput
// 	if childChainMintable {
// 		params = &contractsapi.InitializeRootMintableERC721PredicateACLFn{
// 			NewL2StateSender:        contracts.L2StateSenderContract,
// 			NewStateReceiver:        contracts.StateReceiverContract,
// 			NewChildERC721Predicate: config.ChildMintableERC721PredicateAddr,
// 			NewChildTokenTemplate:   config.ChildERC721Addr,
// 			NewUseAllowList:         useAllowList,
// 			NewUseBlockList:         useBlockList,
// 			NewOwner:                owner,
// 		}
// 	} else {
// 		params = &contractsapi.InitializeChildERC721PredicateACLFn{
// 			NewL2StateSender:       contracts.L2StateSenderContract,
// 			NewStateReceiver:       contracts.StateReceiverContract,
// 			NewRootERC721Predicate: config.RootERC721PredicateAddr,
// 			NewChildTokenTemplate:  contracts.ChildERC721Contract,
// 			NewUseAllowList:        useAllowList,
// 			NewUseBlockList:        useBlockList,
// 			NewOwner:               owner,
// 		}
// 	}

// 	return params.EncodeAbi()
// }

// getInitERC1155PredicateInput builds initialization input parameters for child chain ERC1155Predicate SC
// func getInitERC1155PredicateInput(config *BridgeConfig, childChainMintable bool) ([]byte, error) {
// 	var params contractsapi.StateTransactionInput
// 	if childChainMintable {
// 		params = &contractsapi.InitializeRootMintableERC1155PredicateFn{
// 			NewL2StateSender:         contracts.L2StateSenderContract,
// 			NewStateReceiver:         contracts.StateReceiverContract,
// 			NewChildERC1155Predicate: config.ChildMintableERC1155PredicateAddr,
// 			NewChildTokenTemplate:    config.ChildERC1155Addr,
// 		}
// 	} else {
// 		params = &contractsapi.InitializeChildERC1155PredicateFn{
// 			NewL2StateSender:        contracts.L2StateSenderContract,
// 			NewStateReceiver:        contracts.StateReceiverContract,
// 			NewRootERC1155Predicate: config.RootERC1155PredicateAddr,
// 			NewChildTokenTemplate:   contracts.ChildERC1155Contract,
// 		}
// 	}

// 	return params.EncodeAbi()
// }

// // getInitERC1155PredicateACLInput builds initialization input parameters
// // for child chain ERC1155PredicateAccessList SC
// func getInitERC1155PredicateACLInput(config *BridgeConfig, owner types.Address,
// 	useAllowList, useBlockList, childChainMintable bool) ([]byte, error) {
// 	var params contractsapi.StateTransactionInput
// 	if childChainMintable {
// 		params = &contractsapi.InitializeRootMintableERC1155PredicateACLFn{
// 			NewL2StateSender:         contracts.L2StateSenderContract,
// 			NewStateReceiver:         contracts.StateReceiverContract,
// 			NewChildERC1155Predicate: config.ChildMintableERC1155PredicateAddr,
// 			NewChildTokenTemplate:    config.ChildERC1155Addr,
// 			NewUseAllowList:          useAllowList,
// 			NewUseBlockList:          useBlockList,
// 			NewOwner:                 owner,
// 		}
// 	} else {
// 		params = &contractsapi.InitializeChildERC1155PredicateACLFn{
// 			NewL2StateSender:        contracts.L2StateSenderContract,
// 			NewStateReceiver:        contracts.StateReceiverContract,
// 			NewRootERC1155Predicate: config.RootERC1155PredicateAddr,
// 			NewChildTokenTemplate:   contracts.ChildERC1155Contract,
// 			NewUseAllowList:         useAllowList,
// 			NewUseBlockList:         useBlockList,
// 			NewOwner:                owner,
// 		}
// 	}

// 	return params.EncodeAbi()
// }

// mintRewardTokensToWallet mints configured amount of reward tokens to reward wallet address
// func mintRewardTokensToWallet(polyBFTConfig PolyBFTConfig, transition *state.Transition) error {
// 	if isNativeRewardToken(polyBFTConfig) {
// 		// if reward token is a native erc20 token, we don't need to mint an amount of tokens
// 		// for given wallet address to it since this is done in premine
// 		return nil
// 	}

// 	mintFn := contractsapi.MintRootERC20Fn{
// 		To:     polyBFTConfig.RewardConfig.WalletAddress,
// 		Amount: polyBFTConfig.RewardConfig.WalletAmount,
// 	}

// 	input, err := mintFn.EncodeAbi()
// 	if err != nil {
// 		return fmt.Errorf("RewardToken.mint params encoding failed: %w", err)
// 	}

// 	return callContract(contracts.SystemCaller, polyBFTConfig.RewardConfig.TokenAddress, input,
// 		"RewardToken.mint", transition)
// }

// approveRewardPoolAsSpender approves reward pool contract as reward token spender
// since reward pool distributes rewards.
// func approveRewardPoolAsSpender(polyBFTConfig PolyBFTConfig, transition *state.Transition) error {
// 	approveFn := &contractsapi.ApproveRootERC20Fn{
// 		Spender: contracts.RewardPoolContract,
// 		Amount:  polyBFTConfig.RewardConfig.WalletAmount,
// 	}

// 	input, err := approveFn.EncodeAbi()
// 	if err != nil {
// 		return fmt.Errorf("RewardToken.approve params encoding failed: %w", err)
// 	}

// 	return callContract(polyBFTConfig.RewardConfig.WalletAddress,
// 		polyBFTConfig.RewardConfig.TokenAddress, input, "RewardToken.approve", transition)
// }

// callContract calls given smart contract function, encoded in input parameter
func callContract(from, to types.Address, input []byte, contractName string, transition *state.Transition) error {
	result := transition.Call2(from, to, input, big.NewInt(0), contractCallGasLimit)
	if result.Failed() {
		if result.Reverted() {
			if revertReason, err := abi.UnpackRevertError(result.ReturnValue); err == nil {
				return fmt.Errorf("%s contract call was reverted: %s", contractName, revertReason)
			}
		}

		return fmt.Errorf("%s contract call failed: %w, Revert reason hex: %s",
			contractName,
			result.Err,
			hex.EncodeToString(result.ReturnValue),
		)
	}

	return nil
}

// isNativeRewardToken returns true in case a native token is used as a reward token as well
func isNativeRewardToken(cfg PolyBFTConfig) bool {
	return cfg.RewardConfig.TokenAddress == contracts.NativeERC20TokenContract
}
