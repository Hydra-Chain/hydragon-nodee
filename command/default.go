package command

import (
	"fmt"
	"math/big"

	"github.com/umbracle/ethgo"

	"github.com/0xPolygon/polygon-edge/chain"
	"github.com/0xPolygon/polygon-edge/server"
)

const (
	DefaultGenesisFileName           = "genesis.json"
	DefaultChainName                 = "hydra-chain"
	DefaultChainID                   = 187
	DefaultConsensus                 = server.PolyBFTConsensus
	DefaultGenesisGasUsed            = 458752  // 0x70000
	DefaultGenesisGasLimit           = 5242880 // 0x500000
	DefaultGenesisBaseFeeEM          = chain.GenesisBaseFeeEM
	DefaultGenesisBaseFeeChangeDenom = chain.BaseFeeChangeDenom
	DefaultSecretsConfigPath         = "./secretsManagerConfig.json"
	DefaultSecretsConfigPathDesc     = "the path to the SecretsManager config file. " +
		"Used for Coingecko API key and others. If omitted, the local FS secrets manager is used"
)

var (
	DefaultStake                = ethgo.Ether(15000)
	DefaultPremineBalance       = ethgo.Ether(1e6)
	DefaultGenesisBaseFee       = chain.GenesisBaseFee
	DefaultGenesisBaseFeeConfig = fmt.Sprintf(
		"%d:%d:%d",
		DefaultGenesisBaseFee,
		DefaultGenesisBaseFeeEM,
		DefaultGenesisBaseFeeChangeDenom,
	)
	// DefaultNumerator is the default numerator for the voting power exponent
	DefaultNumerator = big.NewInt(5000)
)

const (
	JSONOutputFlag  = "json"
	GRPCAddressFlag = "grpc-address"
	JSONRPCFlag     = "jsonrpc"
)

// GRPCAddressFlagLEGACY Legacy flag that needs to be present to preserve backwards
// compatibility with running clients
const (
	GRPCAddressFlagLEGACY = "grpc"
)
