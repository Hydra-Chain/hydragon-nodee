package outputprivate

import (
	"github.com/spf13/cobra"

	"github.com/0xPolygon/polygon-edge/command"
)

var outputParams = &OutputParams{}
var result = &outputResult{}

func GetCommand() *cobra.Command {
	secretsOuputCmd := &cobra.Command{
		Use: "output-private",
		Short: "Retrieves the private keys for both the validator and networking components " +
			"from the specified Secrets Manager. If the keys are encrypted, you'll be prompted for a password " +
			"to decrypt them. The private keys are then output to the console.",
		PreRunE: runPreRun,
		Run:     runCommand,
	}

	outputParams.SetFlags(secretsOuputCmd)

	return secretsOuputCmd
}

func runPreRun(_ *cobra.Command, _ []string) error {
	return outputParams.ValidateFlags()
}

func runCommand(cmd *cobra.Command, _ []string) {
	outputter := command.InitializeOutputter(cmd)
	defer outputter.WriteOutput()

	if err := result.initSecrets(outputParams); err != nil {
		outputter.SetError(err)

		return
	}

	outputter.SetCommandResult(result.getResult())
}
