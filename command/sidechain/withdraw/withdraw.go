package withdraw

import (
	"fmt"
	"math/big"
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
)

func GetCommand() *cobra.Command {
	unstakeCmd := &cobra.Command{
		Use: "withdraw",
		Short: "Processes pending withdrawals or initiates the withdrawal of penalized funds " +
			"for the specified validator",
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
		&params.penalizedFunds,
		penalizedFundsFlag,
		false,
		"a flag indicating that the withdrawal of penalized funds should be initiated",
	)

	cmd.Flags().BoolVar(
		&params.insecureLocalStore,
		sidechain.InsecureLocalStoreFlag,
		false,
		"a flag to indicate if the secrets used are encrypted. If set to true, the secrets are stored in plain text",
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

	if params.penalizedFunds {
		var initiatePenalizedFundsWithdrawal = &contractsapi.InitiatePenalizedFundsWithdrawalHydraStakingFn{}
		encoded, err = initiatePenalizedFundsWithdrawal.EncodeAbi()
	} else {
		var withdrawFn = &contractsapi.WithdrawHydraStakingFn{
			To: (types.Address)(validatorAccount.Ecdsa.Address()),
		}

		encoded, err = withdrawFn.EncodeAbi()
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
		withdrawalRegisteredEventABI = contractsapi.HydraStaking.Abi.Events["WithdrawalRegistered"]
		withdrawalFinishedEventABI   = contractsapi.HydraDelegation.Abi.Events["WithdrawalFinished"]
		foundLog                     bool
	)

	result := &withdrawResult{
		ValidatorAddress: validatorAccount.Ecdsa.Address().String(),
		Registered:       false,
	}

	// check the logs to check for the result
	for _, log := range receipt.Logs {
		result.BlockNumber = receipt.BlockNumber

		var event map[string]interface{}

		var match bool

		if match = withdrawalRegisteredEventABI.Match(log); match {
			event, err = withdrawalRegisteredEventABI.ParseLog(log)
			if err != nil {
				return err
			}

			result.Amount = event["amount"].(*big.Int).String() //nolint:forcetypeassert
			result.Registered = true
		} else if match = withdrawalFinishedEventABI.Match(log); match {
			event, err = withdrawalFinishedEventABI.ParseLog(log)
			if err != nil {
				return err
			}

			result.Amount = event["amount"].(*big.Int).String() //nolint:forcetypeassert
		}

		if match {
			foundLog = true

			break
		}
	}

	if !foundLog {
		return fmt.Errorf("could not find the appropriate log for withdraw registration or " +
			"finalized withdrawal on HydraStaking")
	}

	outputter.WriteCommandResult(result)

	return nil
}
