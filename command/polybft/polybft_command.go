package polybft

import (
	// H_MODIFY: Registration module is moved to sidechain
	withdraw "github.com/0xPolygon/polygon-edge/command/bridge/withdraw/erc721"
	"github.com/0xPolygon/polygon-edge/command/sidechain/registration"
	"github.com/0xPolygon/polygon-edge/command/sidechain/staking"
	"github.com/0xPolygon/polygon-edge/command/sidechain/whitelist"

	"github.com/0xPolygon/polygon-edge/command/sidechain/rewards"
	"github.com/0xPolygon/polygon-edge/command/sidechain/unstaking"
	sidechainWithdraw "github.com/0xPolygon/polygon-edge/command/sidechain/withdraw"
	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	polybftCmd := &cobra.Command{
		Use:   "hydragon",
		Short: "Executes HydraChain's Hydragon consensus commands, including staking, unstaking, rewards management, and validator operations.",
	}

	// Hydra modification: modify sidechain commands and remove rootchain commands
	polybftCmd.AddCommand(
		// sidechain (validator set) command to stake on child chain
		staking.GetCommand(),
		// sidechain (validator set) command to unstake on child chain
		unstaking.GetCommand(),
		// sidechain (validator set) command to withdraw stake on child chain
		sidechainWithdraw.GetCommand(),
		// sidechain (reward pool) command to withdraw pending rewards
		rewards.GetCommand(),
		// sidechain (validator set) command to register validator
		registration.GetCommand(),
		// sidechain (validator set) command to whitelist validators
		whitelist.GetCommand(),
		// sidechain (validator set) command to witdhraw amount on child chain
		withdraw.GetCommand(),
	)

	return polybftCmd
}
