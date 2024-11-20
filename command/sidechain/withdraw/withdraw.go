package withdraw

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"github.com/umbracle/ethgo"

	"github.com/0xPolygon/polygon-edge/command"
	"github.com/0xPolygon/polygon-edge/command/helper"
	"github.com/0xPolygon/polygon-edge/command/polybftsecrets"
	"github.com/0xPolygon/polygon-edge/command/sidechain"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/txrelayer"
	"github.com/0xPolygon/polygon-edge/types"
)

var (
	params withdrawParams

	withdrawBannedFundsFn = contractsapi.HydraStaking.Abi.Methods["withdrawBannedFunds"]
	withdrawFn            = contractsapi.HydraStaking.Abi.Methods["withdraw"]
)

func GetCommand() *cobra.Command {
	unstakeCmd := &cobra.Command{
		Use:     "withdraw",
		Short:   "Withdraws pending withdrawals for given validator",
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
		&params.bannedFunds,
		bannedFundsFlag,
		false,
		"a flag to indicate if you want to withdraw banned funds.",
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

	validatorAccount, err := sidechain.GetAccount(params.accountDir, params.accountConfig, params.insecureLocalStore)
	if err != nil {
		return err
	}

	txRelayer, err := txrelayer.NewTxRelayer(txrelayer.WithIPAddress(params.jsonRPC),
		txrelayer.WithReceiptTimeout(150*time.Millisecond))
	if err != nil {
		return err
	}

	var encoded []byte

	if params.bannedFunds {
		encoded, err = withdrawBannedFundsFn.Encode([]interface{}{})
	} else {
		encoded, err = withdrawFn.Encode([]interface{}{
			(types.Address)(validatorAccount.Ecdsa.Address()),
		})
	}

	if err != nil {
		return err
	}

	txn := sidechain.CreateTransaction(
		validatorAccount.Ecdsa.Address(),
		(*ethgo.Address)(&contracts.HydraStakingContract),
		encoded,
		nil,
	)

	receipt, err := txRelayer.SendTransaction(txn, validatorAccount.Ecdsa)
	if err != nil {
		return err
	}

	if receipt.Status != uint64(types.ReceiptSuccess) {
		return fmt.Errorf("withdraw transaction failed on block: %d", receipt.BlockNumber)
	}

	var (
		withdrawalEvent contractsapi.WithdrawalFinishedEvent
		foundLog        bool
	)

	// check the logs to check for the result
	for _, log := range receipt.Logs {
		doesMatch, err := withdrawalEvent.ParseLog(log)
		if err != nil {
			return err
		}

		if doesMatch {
			foundLog = true

			break
		}
	}

	if !foundLog {
		return fmt.Errorf("could not find an appropriate log in receipt that withdraw happened on HydraStaking")
	}

	outputter.WriteCommandResult(
		&withdrawResult{
			ValidatorAddress: validatorAccount.Ecdsa.Address().String(),
			Amount:           withdrawalEvent.Amount.String(),
			BlockNumber:      receipt.BlockNumber,
		})

	return nil
}
