package rewards

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/umbracle/ethgo"

	"github.com/0xPolygon/polygon-edge/command"
	"github.com/0xPolygon/polygon-edge/command/helper"
	"github.com/0xPolygon/polygon-edge/command/polybftsecrets"
	"github.com/0xPolygon/polygon-edge/command/sidechain"
	sidechainHelper "github.com/0xPolygon/polygon-edge/command/sidechain"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/txrelayer"
	"github.com/0xPolygon/polygon-edge/types"
)

var params withdrawRewardsParams

func GetCommand() *cobra.Command {
	unstakeCmd := &cobra.Command{
		Use:     "claim-rewards",
		Short:   "Claim rewards for given validator",
		PreRunE: runPreRun,
		RunE:    runCommand,
	}

	helper.RegisterJSONRPCFlag(unstakeCmd)
	setFlags(unstakeCmd)

	return unstakeCmd
}

func setFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&params.accountDir,
		polybftsecrets.AccountDirFlag,
		"",
		polybftsecrets.AccountDirFlagDesc,
	)

	cmd.Flags().StringVar(
		&params.accountConfig,
		polybftsecrets.AccountConfigFlag,
		"",
		polybftsecrets.AccountConfigFlagDesc,
	)

	cmd.Flags().BoolVar(
		&params.insecureLocalStore,
		sidechain.InsecureLocalStoreFlag,
		false,
		"a flag to indicate if the secrets used are encrypted. If set to true, the secrets are stored in plain text.",
	)

	cmd.MarkFlagsMutuallyExclusive(polybftsecrets.AccountDirFlag, polybftsecrets.AccountConfigFlag)
}

func runPreRun(cmd *cobra.Command, _ []string) error {
	params.jsonRPC = helper.GetJSONRPCAddress(cmd)

	return params.validateFlags()
}

func runCommand(cmd *cobra.Command, _ []string) error {
	outputter := command.InitializeOutputter(cmd)
	defer outputter.WriteOutput()

	validatorAccount, err := sidechainHelper.GetAccount(params.accountDir, params.accountConfig, params.insecureLocalStore)
	if err != nil {
		return err
	}

	validatorAddr := validatorAccount.Ecdsa.Address()

	txRelayer, err := txrelayer.NewTxRelayer(txrelayer.WithIPAddress(params.jsonRPC),
		txrelayer.WithReceiptTimeout(150*time.Millisecond))
	if err != nil {
		return err
	}

	claimRewardsFn := contractsapi.ClaimValidatorRewardRewardPoolFn{}
	encoded, err := claimRewardsFn.EncodeAbi()
	if err != nil {
		return err
	}

	txn := &ethgo.Transaction{
		From:  validatorAddr,
		Input: encoded,
		To:    (*ethgo.Address)(&contracts.RewardPoolContract),
	}

	receipt, err := txRelayer.SendTransaction(txn, validatorAccount.Ecdsa)
	if err != nil {
		return err
	}

	if receipt.Status != uint64(types.ReceiptSuccess) {
		return fmt.Errorf("claim rewards transaction failed on block: %d", receipt.BlockNumber)
	}

	var (
		claimRewardsEvent contractsapi.ValidatorRewardClaimedEvent
		foundLog          bool
	)

	result := &withdrawRewardResult{
		ValidatorAddress: validatorAddr.String(),
	}

	// check the logs to check for the result
	for _, log := range receipt.Logs {
		doesMatch, err := claimRewardsEvent.ParseLog(log)
		if err != nil {
			return err
		}

		if doesMatch {
			foundLog = true
			result.RewardAmount = claimRewardsEvent.Amount.String()

			break
		}
	}

	if !foundLog {
		return fmt.Errorf("could not find an appropriate log in receipt that rewards claim happened (claim rewards)")
	}

	outputter.WriteCommandResult(result)

	return nil
}
