package whitelist

import (
	"fmt"
	"time"

	"github.com/0xPolygon/polygon-edge/command"
	"github.com/0xPolygon/polygon-edge/command/helper"
	"github.com/0xPolygon/polygon-edge/command/polybftsecrets"
	"github.com/0xPolygon/polygon-edge/command/sidechain"
	sidechainHelper "github.com/0xPolygon/polygon-edge/command/sidechain"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/txrelayer"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/spf13/cobra"
	"github.com/umbracle/ethgo"
)

var (
	whitelistFn       = contractsapi.ValidatorSet.Abi.Methods["addToWhitelist"]
	whitelistEventABI = contractsapi.ValidatorSet.Abi.Events["AddedToWhitelist"]
)

var params whitelistParams

func GetCommand() *cobra.Command {
	registerCmd := &cobra.Command{
		Use:     "whitelist-validator",
		Short:   "whitelist a new validator",
		PreRunE: runPreRun,
		RunE:    runCommand,
	}

	setFlags(registerCmd)

	return registerCmd
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

	cmd.Flags().StringVar(
		&params.newValidatorAddress,
		newValidatorAddressFlag,
		"",
		"account address of a possible validator",
	)

	cmd.Flags().BoolVar(
		&params.insecureLocalStore,
		sidechain.InsecureLocalStoreFlag,
		false,
		"a flag to indicate if the secrets used are encrypted. If set to true, the secrets are stored in plain text.",
	)

	cmd.MarkFlagsMutuallyExclusive(polybftsecrets.AccountDirFlag, polybftsecrets.AccountConfigFlag)
	helper.RegisterJSONRPCFlag(cmd)
}

func runPreRun(cmd *cobra.Command, _ []string) error {
	params.jsonRPC = helper.GetJSONRPCAddress(cmd)

	return params.validateFlags()
}

func runCommand(cmd *cobra.Command, _ []string) error {
	outputter := command.InitializeOutputter(cmd)
	defer outputter.WriteOutput()

	governanceAccount, err := sidechainHelper.GetAccount(params.accountDir, params.accountConfig, params.insecureLocalStore)
	if err != nil {
		return fmt.Errorf("enlist validator failed: %w", err)
	}

	txRelayer, err := txrelayer.NewTxRelayer(txrelayer.WithIPAddress(params.jsonRPC),
		txrelayer.WithReceiptTimeout(150*time.Millisecond))
	if err != nil {
		return fmt.Errorf("enlist validator failed: %w", err)
	}

	encoded, err := whitelistFn.Encode([]interface{}{
		[]types.Address{types.StringToAddress(params.newValidatorAddress)},
	})

	txn := &ethgo.Transaction{
		From:     governanceAccount.Ecdsa.Address(),
		Input:    encoded,
		To:       (*ethgo.Address)(&contracts.ValidatorSetContract),
		GasPrice: sidechainHelper.DefaultGasPrice,
	}

	receipt, err := txRelayer.SendTransaction(txn, governanceAccount.Ecdsa)
	if err != nil {
		return fmt.Errorf("enlist validator failed %w", err)
	}

	if receipt.Status == uint64(types.ReceiptFailed) {
		return fmt.Errorf("enlist validator transaction failed on block %d", receipt.BlockNumber)
	}

	result := &enlistResult{}
	foundLog := false

	for _, log := range receipt.Logs {
		if whitelistEventABI.Match(log) {
			event, err := whitelistEventABI.ParseLog(log)
			if err != nil {
				return err
			}

			result.newValidatorAddress = event["validator"].(ethgo.Address).String() //nolint:forcetypeassert
			foundLog = true

			break
		}
	}

	if !foundLog {
		return fmt.Errorf("could not find an appropriate log in receipt that enlistment happened")
	}

	outputter.WriteCommandResult(result)

	return nil
}
