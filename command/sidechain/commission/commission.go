package commission

import (
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/0xPolygon/polygon-edge/command"
	"github.com/0xPolygon/polygon-edge/command/helper"
	"github.com/0xPolygon/polygon-edge/command/polybftsecrets"
	"github.com/0xPolygon/polygon-edge/command/sidechain"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/wallet"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/txrelayer"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/spf13/cobra"
	"github.com/umbracle/ethgo"
)

var (
	params setCommissionParams

	delegationManager         = contracts.HydraDelegationContract
	setCommissionFn           = contractsapi.HydraDelegation.Abi.Methods["setCommission"]
	commissionUpdatedEventABI = contractsapi.HydraDelegation.Abi.Events["CommissionUpdated"]
)

func GetCommand() *cobra.Command {
	setCommissionCmd := &cobra.Command{
		Use:     "commission",
		Short:   "Set a commission for a validator (staker)",
		PreRunE: runPreRun,
		RunE:    runCommand,
	}

	setFlags(setCommissionCmd)
	helper.SetRequiredFlags(setCommissionCmd, params.getRequiredFlags())

	return setCommissionCmd
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

	cmd.Flags().Uint64Var(
		&params.commission,
		commissionFlag,
		0,
		"mandatory flag that represents the commission percentage of the validator (staker)",
	)

	cmd.Flags().BoolVar(
		&params.insecureLocalStore,
		sidechain.InsecureLocalStoreFlag,
		false,
		"a flag to indicate if the secrets used are encrypted. If set to true, the secrets are stored in plain text.",
	)

	helper.RegisterJSONRPCFlag(cmd)

	cmd.MarkFlagsMutuallyExclusive(polybftsecrets.AccountConfigFlag, polybftsecrets.AccountDirFlag)
}

func runPreRun(cmd *cobra.Command, _ []string) error {
	params.jsonRPC = helper.GetJSONRPCAddress(cmd)

	return params.validateFlags()
}

func runCommand(cmd *cobra.Command, _ []string) error {
	outputter := command.InitializeOutputter(cmd)
	defer outputter.WriteOutput()

	secretsManager, err := polybftsecrets.GetSecretsManager(
		params.accountDir,
		params.accountConfig,
		params.insecureLocalStore,
	)
	if err != nil {
		return err
	}

	txRelayer, err := txrelayer.NewTxRelayer(
		txrelayer.WithIPAddress(params.jsonRPC),
		txrelayer.WithReceiptTimeout(150*time.Millisecond),
	)
	if err != nil {
		return err
	}

	validatorAccount, err := wallet.NewAccountFromSecret(secretsManager)
	if err != nil {
		return err
	}

	receipt, err := setCommission(txRelayer, validatorAccount)
	if err != nil {
		return err
	}

	if receipt.Status != uint64(types.ReceiptSuccess) {
		return errors.New("register validator transaction failed")
	}

	result := &setCommissionResult{}
	foundCommissionUpdatedLog := false

	for _, log := range receipt.Logs {
		if commissionUpdatedEventABI.Match(log) {
			event, err := commissionUpdatedEventABI.ParseLog(log)
			if err != nil {
				return err
			}

			result.staker = event["staker"].(ethgo.Address).String()          //nolint:forcetypeassert
			result.newCommission = event["newCommission"].(*big.Int).Uint64() //nolint:forcetypeassert
			foundCommissionUpdatedLog = true
		}
	}

	if !foundCommissionUpdatedLog {
		return fmt.Errorf("could not find an appropriate log in the receipt that validates the new commission update")
	}

	outputter.WriteCommandResult(result)

	return nil
}

func setCommission(sender txrelayer.TxRelayer, account *wallet.Account) (*ethgo.Receipt, error) {
	encoded, err := setCommissionFn.Encode([]interface{}{
		new(big.Int).SetUint64(params.commission),
	})
	if err != nil {
		return nil, fmt.Errorf("encoding set commission function failed: %w", err)
	}

	txn := sidechain.CreateTransaction(
		account.Ecdsa.Address(),
		(*ethgo.Address)(&delegationManager),
		encoded,
		nil,
	)

	receipt, err := sender.SendTransaction(txn, account.Ecdsa)
	if err != nil {
		// retry execution. Issue: https://github.com/valyala/fasthttp/issues/189
		receipt, err = sender.SendTransaction(txn, account.Ecdsa)
	}

	return receipt, err
}
