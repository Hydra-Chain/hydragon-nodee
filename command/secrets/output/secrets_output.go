package output

import (
	"github.com/spf13/cobra"

	"github.com/0xPolygon/polygon-edge/command"
)

var outputParams = &OutputParams{}

func GetCommand() *cobra.Command {
	secretsOuputCmd := &cobra.Command{
		Use: "output",
		Short: "Retrieves the private keys for both the validator and the networking components " +
			"from the specified Secrets Manager. If the keys are encrypted, the prompted password " +
			"will be used for decryption. It will then gather the corresponding public keys " +
			"and output them to the console.",
		PreRunE: runPreRun,
		Run:     runCommand,
	}

	outputParams.setFlags(secretsOuputCmd)

	return secretsOuputCmd
}

func runPreRun(_ *cobra.Command, _ []string) error {
	return outputParams.validateFlags()
}

func runCommand(cmd *cobra.Command, _ []string) {
	outputter := command.InitializeOutputter(cmd)
	defer outputter.WriteOutput()

	if err := outputParams.initSecrets(); err != nil {
		outputter.SetError(err)

		return
	}

	outputter.SetCommandResult(outputParams.getResult())
}
