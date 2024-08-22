package contractsapi

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArtifactNotEmpty(t *testing.T) {
	require.NotEmpty(t, BLS.Bytecode)
	require.NotEmpty(t, BLS.DeployedBytecode)
	require.NotEmpty(t, BLS.Abi)

	require.NotEmpty(t, HydraChain.Bytecode)
	require.NotEmpty(t, HydraChain.DeployedBytecode)
	require.NotEmpty(t, HydraChain.Abi)

	require.NotEmpty(t, HydraStaking.Bytecode)
	require.NotEmpty(t, HydraStaking.DeployedBytecode)
	require.NotEmpty(t, HydraStaking.Abi)

	require.NotEmpty(t, HydraDelegation.Bytecode)
	require.NotEmpty(t, HydraDelegation.DeployedBytecode)
	require.NotEmpty(t, HydraDelegation.Abi)

	require.NotEmpty(t, VestingManagerFactory.Bytecode)
	require.NotEmpty(t, VestingManagerFactory.DeployedBytecode)
	require.NotEmpty(t, VestingManagerFactory.Abi)

	require.NotEmpty(t, APRCalculator.Bytecode)
	require.NotEmpty(t, APRCalculator.DeployedBytecode)
	require.NotEmpty(t, APRCalculator.Abi)

	require.NotEmpty(t, PriceOracle.Bytecode)
	require.NotEmpty(t, PriceOracle.DeployedBytecode)
	require.NotEmpty(t, PriceOracle.Abi)

	require.NotEmpty(t, LiquidityToken.Bytecode)
	require.NotEmpty(t, LiquidityToken.DeployedBytecode)
	require.NotEmpty(t, LiquidityToken.Abi)

	require.NotEmpty(t, GenesisProxy.Bytecode)
	require.NotEmpty(t, GenesisProxy.DeployedBytecode)
	require.NotEmpty(t, GenesisProxy.Abi)

	require.NotEmpty(t, TransparentUpgradeableProxy.Bytecode)
	require.NotEmpty(t, TransparentUpgradeableProxy.DeployedBytecode)
	require.NotEmpty(t, TransparentUpgradeableProxy.Abi)
}
