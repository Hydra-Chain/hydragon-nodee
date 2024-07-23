package contracts

import "github.com/0xPolygon/polygon-edge/types"

var (
	// HydraChainContract is an address of HydraChain's proxy contract deployed on the chain
	HydraChainContract = types.StringToAddress("0x101")
	// HydraChainContractV1 is an address of HydraChain's implementation contract deployed on the chain
	HydraChainContractV1 = types.StringToAddress("0x1011")
	// BLSContract is an address of BLS proxy contract on the child chain
	BLSContract = types.StringToAddress("0x102")
	// BLSContractV1 is an address of BLS contract on the child chain
	BLSContractV1 = types.StringToAddress("0x1021")
	// MerkleContract is an address of Merkle proxy contract on the child chain
	MerkleContract = types.StringToAddress("0x103")
	// MerkleContractV1 is an address of Merkle contract on the child chain
	MerkleContractV1 = types.StringToAddress("0x1031")
	// HydraStakingContract is an address of the HydraStaking's proxy contract on the chain
	HydraStakingContract = types.StringToAddress("0x104")
	// HydraStakingContractV1 is an address of the HydraStaking's implementation contract deployed on the chain
	HydraStakingContractV1 = types.StringToAddress("0x1041")
	// DefaultBurnContract is an address of eip1559 default proxy contract
	DefaultBurnContract = types.StringToAddress("0x105")
	// FeeHandlerContract is an address of the fee handler proxy contract
	FeeHandlerContract = types.StringToAddress("0x106")
	// FeeHandlerContract is an address of fee handler implementation contract
	FeeHandlerContractV1 = types.StringToAddress("0x1061")
	// HydraDelegationContract is an address of the HydraDelegation's proxy contract on the chain
	HydraDelegationContract = types.StringToAddress("0x107")
	// HydraDelegationContractV1 is an address of the HydraDelegation's implementation contract deployed on the chain
	HydraDelegationContractV1 = types.StringToAddress("0x1071")
	// VestingManagerFactoryContract is an address of the VestingManagerFactory's proxy contract on the chain
	VestingManagerFactoryContract = types.StringToAddress("0x108")
	// VestingManagerFactoryContractV1 is an address of the VestingManagerFactoryContract's implementation contract deployed on the chain
	VestingManagerFactoryContractV1 = types.StringToAddress("0x1081")
	// APRCalculatorContract is an address of the APRCalculator's proxy contract on the chain
	APRCalculatorContract = types.StringToAddress("0x109")
	// APRCalculatorContractV1 is an address of the APRCalculator's implementation contract deployed on the chain
	APRCalculatorContractV1 = types.StringToAddress("1091")
	// RewardWalletContract is an address of the RewardWallet's proxy contract on the chain
	RewardWalletContract = types.StringToAddress("0x110")
	// RewardWalletContractV1 is an address of the RewardWallet's implementation contract deployed on the chain
	RewardWalletContractV1 = types.StringToAddress("1101")
	// StateReceiverContract is an address of bridge proxy contract on the child chain
	StateReceiverContract = types.StringToAddress("0x1001")
	// StateReceiverContractV1 is an address of bridge implementation contract on the child chain
	StateReceiverContractV1 = types.StringToAddress("0x10011")
	// NativeERC20TokenContract is an address of bridge proxy contract
	// (used for transferring ERC20 native tokens on child chain)
	NativeERC20TokenContract = types.StringToAddress("0x1010")
	// NativeERC20TokenContractV1 is an address of bridge contract
	// (used for transferring ERC20 native tokens on child chain)
	NativeERC20TokenContractV1 = types.StringToAddress("0x10101")
	// L2StateSenderContract is an address of bridge proxy contract to the rootchain
	L2StateSenderContract = types.StringToAddress("0x1002")
	// L2StateSenderContractV1 is an address of bridge contract to the rootchain
	L2StateSenderContractV1 = types.StringToAddress("0x10021")
	// ChildERC20Contract is an address of bridgable ERC20 token contract on the child chain
	ChildERC20Contract = types.StringToAddress("0x1003")
	// ChildERC20PredicateContract is an address of child ERC20 proxy predicate contract on the child chain
	ChildERC20PredicateContract = types.StringToAddress("0x1004")
	// ChildERC20PredicateContractV1 is an address of child ERC20 predicate contract on the child chain
	ChildERC20PredicateContractV1 = types.StringToAddress("0x10041")
	// ChildERC721Contract is an address of bridgable ERC721 token contract on the child chain
	ChildERC721Contract = types.StringToAddress("0x1005")
	// ChildERC721PredicateContract is an address of child ERC721 proxy predicate contract on the child chain
	ChildERC721PredicateContract = types.StringToAddress("0x1006")
	// ChildERC721PredicateContractV1 is an address of child ERC721 predicate contract on the child chain
	ChildERC721PredicateContractV1 = types.StringToAddress("0x10061")
	// ChildERC1155Contract is an address of bridgable ERC1155 token contract on the child chain
	ChildERC1155Contract = types.StringToAddress("0x1007")
	// ChildERC1155PredicateContract is an address of child ERC1155 proxy predicate contract on the child chain
	ChildERC1155PredicateContract = types.StringToAddress("0x1008")
	// ChildERC1155PredicateContractV1 is an address of child ERC1155 predicate contract on the child chain
	ChildERC1155PredicateContractV1 = types.StringToAddress("0x10081")
	// RootMintableERC20PredicateContract is an address of mintable ERC20 proxy predicate on the child chain
	RootMintableERC20PredicateContract = types.StringToAddress("0x1009")
	// RootMintableERC20PredicateContractV1 is an address of mintable ERC20 predicate on the child chain
	RootMintableERC20PredicateContractV1 = types.StringToAddress("0x10091")
	// RootMintableERC721PredicateContract is an address of mintable ERC721 proxy predicate on the child chain
	RootMintableERC721PredicateContract = types.StringToAddress("0x100a")
	// RootMintableERC721PredicateContractV1 is an address of mintable ERC721 predicate on the child chain
	RootMintableERC721PredicateContractV1 = types.StringToAddress("0x100a1")
	// RootMintableERC1155PredicateContract is an address of mintable ERC1155 proxy predicate on the child chain
	RootMintableERC1155PredicateContract = types.StringToAddress("0x100b")
	// RootMintableERC1155PredicateContractV1 is an address of mintable ERC1155 predicate on the child chain
	RootMintableERC1155PredicateContractV1 = types.StringToAddress("0x100b1")

	// SystemCaller is address of account, used for system calls to smart contracts
	SystemCaller = types.StringToAddress("0xffffFFFfFFffffffffffffffFfFFFfffFFFfFFfE")

	// NativeTransferPrecompile is an address of native transfer precompile
	NativeTransferPrecompile = types.StringToAddress("0x2020")
	// BLSAggSigsVerificationPrecompile is an address of BLS aggregated signatures verificatin precompile
	BLSAggSigsVerificationPrecompile = types.StringToAddress("0x2030")
	// ConsolePrecompile is and address of Hardhat console precompile
	ConsolePrecompile = types.StringToAddress("0x000000000000000000636F6e736F6c652e6c6f67")
	// AllowListContractsAddr is the address of the contract deployer allow list
	AllowListContractsAddr = types.StringToAddress("0x0200000000000000000000000000000000000000")
	// BlockListContractsAddr is the address of the contract deployer block list
	BlockListContractsAddr = types.StringToAddress("0x0300000000000000000000000000000000000000")
	// AllowListTransactionsAddr is the address of the transactions allow list
	AllowListTransactionsAddr = types.StringToAddress("0x0200000000000000000000000000000000000002")
	// BlockListTransactionsAddr is the address of the transactions block list
	BlockListTransactionsAddr = types.StringToAddress("0x0300000000000000000000000000000000000002")
	// AllowListBridgeAddr is the address of the bridge allow list
	AllowListBridgeAddr = types.StringToAddress("0x0200000000000000000000000000000000000004")
	// BlockListBridgeAddr is the address of the bridge block list
	BlockListBridgeAddr = types.StringToAddress("0x0300000000000000000000000000000000000004")

	// Hydra: Additional system contracts and special addresses
	HydraBurnAddress       = types.StringToAddress("0x0000000000000000000000000000000000000000")
	LiquidityTokenContract = types.StringToAddress("0x1013")
)

// GetProxyImplementationMapping retrieves the addresses of proxy contracts that should be deployed unconditionally
func GetProxyImplementationMapping() map[types.Address]types.Address {
	return map[types.Address]types.Address{
		BLSContract:                   BLSContractV1,
		HydraChainContract:            HydraChainContractV1,
		HydraStakingContract:          HydraStakingContractV1,
		HydraDelegationContract:       HydraDelegationContractV1,
		VestingManagerFactoryContract: VestingManagerFactoryContractV1,
		APRCalculatorContract:         APRCalculatorContractV1,
		FeeHandlerContract:            FeeHandlerContractV1,
		RewardWalletContract:          RewardWalletContractV1,
	}
}
