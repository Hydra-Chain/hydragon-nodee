package polybft

import (
	// H_MODIFY: Registration module is moved to sidechain

	"github.com/0xPolygon/polygon-edge/command/sidechain/registration"
	"github.com/0xPolygon/polygon-edge/command/sidechain/staking"
	terminateban "github.com/0xPolygon/polygon-edge/command/sidechain/terminate-ban"
	"github.com/0xPolygon/polygon-edge/command/sidechain/whitelist"

	"github.com/0xPolygon/polygon-edge/command/sidechain/commission"
	"github.com/0xPolygon/polygon-edge/command/sidechain/rewards"
	"github.com/0xPolygon/polygon-edge/command/sidechain/unstaking"
	sidechainWithdraw "github.com/0xPolygon/polygon-edge/command/sidechain/withdraw"
	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	polybftCmd := &cobra.Command{
		Use: "hydragon",
		Short: "Executes HydraChain's Hydragon consensus commands, including staking, unstaking, " +
			"rewards management, and validator operations.",
	}

	// Hydra modification: modify sidechain commands and remove rootchain commands
	polybftCmd.AddCommand(
		// sidechain (hydra staking) command to stake on child chain
		staking.GetCommand(),
		// sidechain (hydra staking) command to unstake on child chain
		unstaking.GetCommand(),
		// sidechain (hydra staking) command to withdraw stake on child chain
		sidechainWithdraw.GetCommand(),
		// sidechain (hydra staking) command to withdraw pending rewards
		rewards.GetCommand(),
		// sidechain (hydra chain) command to register validator
		registration.GetCommand(),
		// sidechain (hydra chain) command to whitelist validators
		whitelist.GetCommand(),
		// sidechain (hydra chain) command to terminate ban for validator
		terminateban.GetCommand(),
		// sidechain (hydra delegation) command to set commission
		commission.GetCommand(),
	)

	return polybftCmd
}
