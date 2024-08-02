package contractsapi

import (
	"embed"
	"log"
	"path"

	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi/artifact"
)

const (
	testContractsDir = "test-contracts"
)

var (
	// core-contracts smart contracts
	CheckpointManager               *artifact.Artifact
	ExitHelper                      *artifact.Artifact
	StateSender                     *artifact.Artifact
	RootERC20Predicate              *artifact.Artifact
	RootERC721Predicate             *artifact.Artifact
	RootERC1155Predicate            *artifact.Artifact
	ChildMintableERC20Predicate     *artifact.Artifact
	ChildMintableERC721Predicate    *artifact.Artifact
	ChildMintableERC1155Predicate   *artifact.Artifact
	BLS                             *artifact.Artifact
	BLS256                          *artifact.Artifact
	System                          *artifact.Artifact
	Merkle                          *artifact.Artifact
	NativeERC20                     *artifact.Artifact
	NativeERC20Mintable             *artifact.Artifact
	StateReceiver                   *artifact.Artifact
	ChildERC20                      *artifact.Artifact
	ChildERC20Predicate             *artifact.Artifact
	ChildERC20PredicateACL          *artifact.Artifact
	RootMintableERC20Predicate      *artifact.Artifact
	RootMintableERC20PredicateACL   *artifact.Artifact
	ChildERC721                     *artifact.Artifact
	ChildERC721Predicate            *artifact.Artifact
	ChildERC721PredicateACL         *artifact.Artifact
	RootMintableERC721Predicate     *artifact.Artifact
	RootMintableERC721PredicateACL  *artifact.Artifact
	ChildERC1155                    *artifact.Artifact
	ChildERC1155Predicate           *artifact.Artifact
	ChildERC1155PredicateACL        *artifact.Artifact
	RootMintableERC1155Predicate    *artifact.Artifact
	RootMintableERC1155PredicateACL *artifact.Artifact
	L2StateSender                   *artifact.Artifact
	CustomSupernetManager           *artifact.Artifact
	StakeManager                    *artifact.Artifact
	HydraChain                      *artifact.Artifact
	HydraStaking                    *artifact.Artifact
	HydraDelegation                 *artifact.Artifact
	VestingManagerFactory           *artifact.Artifact
	APRCalculator                   *artifact.Artifact
	RewardWallet                    *artifact.Artifact
	RootERC721                      *artifact.Artifact
	RootERC1155                     *artifact.Artifact
	EIP1559Burn                     *artifact.Artifact
	GenesisProxy                    *artifact.Artifact
	TransparentUpgradeableProxy     *artifact.Artifact

	// test smart contracts
	// go:embed test-contracts/*
	testContracts          embed.FS
	TestWriteBlockMetadata *artifact.Artifact
	RootERC20              *artifact.Artifact
	TestSimple             *artifact.Artifact
	LiquidityToken         *artifact.Artifact
	HydraVault             *artifact.Artifact
)

func init() {
	var err error

	BLS, err = artifact.DecodeArtifact([]byte(BLSArtifact))
	if err != nil {
		log.Fatal(err)
	}

	HydraChain, err = artifact.DecodeArtifact([]byte(HydraChainArtifact))
	if err != nil {
		log.Fatal(err)
	}

	HydraStaking, err = artifact.DecodeArtifact([]byte(HydraStakingArtifact))
	if err != nil {
		log.Fatal(err)
	}

	HydraDelegation, err = artifact.DecodeArtifact([]byte(HydraDelegationArtifact))
	if err != nil {
		log.Fatal(err)
	}

	VestingManagerFactory, err = artifact.DecodeArtifact([]byte(VestingManagerFactoryArtifact))
	if err != nil {
		log.Fatal(err)
	}

	APRCalculator, err = artifact.DecodeArtifact([]byte(APRCalculatorArtifact))
	if err != nil {
		log.Fatal(err)
	}

	RewardWallet, err = artifact.DecodeArtifact([]byte(RewardWalletArtifact))
	if err != nil {
		log.Fatal(err)
	}

	LiquidityToken, err = artifact.DecodeArtifact([]byte(LiquidityTokenArtifact))
	if err != nil {
		log.Fatal(err)
	}

	HydraVault, err = artifact.DecodeArtifact([]byte(HydraVaultArtifact))
	if err != nil {
		log.Fatal(err)
	}

	GenesisProxy, err = artifact.DecodeArtifact([]byte(GenesisProxyArtifact))
	if err != nil {
		log.Fatal(err)
	}

	TransparentUpgradeableProxy, err = artifact.DecodeArtifact(
		[]byte(TransparentUpgradeableProxyArtifact),
	)
	if err != nil {
		log.Fatal(err)
	}
}

func readTestContractContent(contractFileName string) []byte {
	contractRaw, err := testContracts.ReadFile(path.Join(testContractsDir, contractFileName))
	if err != nil {
		log.Fatal(err)
	}

	return contractRaw
}
