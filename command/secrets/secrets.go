package secrets

import (
	"github.com/0xPolygon/polygon-edge/command/helper"
	"github.com/0xPolygon/polygon-edge/command/polybftsecrets"
	"github.com/0xPolygon/polygon-edge/command/secrets/generate"
	outputpublic "github.com/0xPolygon/polygon-edge/command/secrets/output-private"
	outputprivate "github.com/0xPolygon/polygon-edge/command/secrets/output-public"
	"github.com/spf13/cobra"
)

func GetCommand() *cobra.Command {
	secretsCmd := &cobra.Command{
		Use:   "secrets",
		Short: "Top level SecretsManager command for interacting with secrets functionality. Only accepts subcommands.",
	}

	helper.RegisterGRPCAddressFlag(secretsCmd)

	registerSubcommands(secretsCmd)

	return secretsCmd
}

func registerSubcommands(baseCmd *cobra.Command) {
	baseCmd.AddCommand(
		// secrets init
		polybftsecrets.GetCommand(),
		// secrets generate
		generate.GetCommand(),
		// output private keys
		outputprivate.GetCommand(),
		// secrets output private and public data
		outputpublic.GetCommand(),
	)
}
