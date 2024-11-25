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

	delegationManager              = contracts.HydraDelegationContract
	pendingCommissionAddedEventABI = contractsapi.HydraDelegation.Abi.Events["PendingCommissionAdded"]
	commissionUpdatedEventABI      = contractsapi.HydraDelegation.Abi.Events["CommissionUpdated"]
	commissionClaimedEventABI      = contractsapi.HydraDelegation.Abi.Events["CommissionClaimed"]
)

func GetCommand() *cobra.Command {
	setCommissionCmd := &cobra.Command{
		Use:     "commission",
		Short:   "Set a commission for a validator (staker)",
		PreRunE: runPreRun,
		RunE:    runCommand,
	}

	setFlags(setCommissionCmd)

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
		command.CommissionFlag,
		0,
		"flag that represents the commission percentage of the validator (staker)",
	)

	cmd.Flags().BoolVar(
		&params.apply,
		applyFlag,
		false,
		"a flag to indicate whether to set or apply the commission. "+
			"If the flag is set, it will apply the pending commission, otherwise will set new pending commission.",
	)

	cmd.Flags().BoolVar(
		&params.claim,
		claimFlag,
		false,
		"a flag to indicate whether to claim the commission generated from the delegators.",
	)

	cmd.Flags().BoolVar(
		&params.insecureLocalStore,
		sidechain.InsecureLocalStoreFlag,
		false,
		"a flag to indicate if the secrets used are encrypted. If set to true, the secrets are stored in plain text.",
	)

	helper.RegisterJSONRPCFlag(cmd)

	cmd.MarkFlagsMutuallyExclusive(polybftsecrets.AccountConfigFlag, polybftsecrets.AccountDirFlag)
	cmd.MarkFlagsMutuallyExclusive(command.CommissionFlag, applyFlag, claimFlag)
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

	var input []byte
	if params.apply { //nolint:gocritic
		input, err = generateApplyPendingCommissionFn()
	} else if params.claim {
		input, err = generateClaimCommissionFn(validatorAccount)
	} else {
		input, err = generateSetPendingCommissionFn()
	}

	if err != nil {
		return err
	}

	receipt, err := sendCommissionTx(txRelayer, validatorAccount, input)
	if err != nil {
		return err
	}

	if receipt.Status != uint64(types.ReceiptSuccess) {
		return errors.New("commission transaction failed")
	}

	result := &setCommissionResult{}
	foundPendingCommissionLog := false
	foundCommissionUpdatedLog := false
	foundClaimCommissionLog := false

	for _, log := range receipt.Logs {
		if pendingCommissionAddedEventABI.Match(log) {
			event, err := pendingCommissionAddedEventABI.ParseLog(log)
			if err != nil {
				return err
			}

			result.staker = event["staker"].(ethgo.Address).String()       //nolint:forcetypeassert
			result.commission = event["newCommission"].(*big.Int).Uint64() //nolint:forcetypeassert
			result.isPending = true
			foundPendingCommissionLog = true
		}

		if commissionUpdatedEventABI.Match(log) {
			event, err := commissionUpdatedEventABI.ParseLog(log)
			if err != nil {
				return err
			}

			result.staker = event["staker"].(ethgo.Address).String()       //nolint:forcetypeassert
			result.commission = event["newCommission"].(*big.Int).Uint64() //nolint:forcetypeassert
			foundCommissionUpdatedLog = true
		}

		if commissionClaimedEventABI.Match(log) {
			event, err := commissionClaimedEventABI.ParseLog(log)
			if err != nil {
				return err
			}

			result.staker = event["to"].(ethgo.Address).String()    //nolint:forcetypeassert
			result.commission = event["amount"].(*big.Int).Uint64() //nolint:forcetypeassert
			result.isClaimed = true
			foundClaimCommissionLog = true
		}
	}

	if params.apply && !foundCommissionUpdatedLog { //nolint:gocritic
		return fmt.Errorf("could not find an appropriate log in the receipt that validates the new commission update")
	} else if params.claim && !foundClaimCommissionLog {
		return fmt.Errorf("could not find an appropriate log in the receipt that validates the commission claim")
	} else if !foundPendingCommissionLog {
		return fmt.Errorf("could not find an appropriate log in the receipt that validates the new pending commission")
	}

	outputter.WriteCommandResult(result)

	return nil
}

func sendCommissionTx(sender txrelayer.TxRelayer, account *wallet.Account, input []byte) (*ethgo.Receipt, error) {
	txn := sidechain.CreateTransaction(
		account.Ecdsa.Address(),
		(*ethgo.Address)(&delegationManager),
		input,
		nil,
	)

	receipt, err := sender.SendTransaction(txn, account.Ecdsa)
	if err != nil {
		// retry execution. Issue: https://github.com/valyala/fasthttp/issues/189
		receipt, err = sender.SendTransaction(txn, account.Ecdsa)
	}

	return receipt, err
}

func generateSetPendingCommissionFn() ([]byte, error) {
	setPendingCommisionFn := &contractsapi.SetPendingCommissionHydraDelegationFn{
		NewCommission: new(big.Int).SetUint64(params.commission),
	}

	encoded, err := setPendingCommisionFn.EncodeAbi()
	if err != nil {
		return nil, fmt.Errorf("encoding set pending commission function failed: %w", err)
	}

	return encoded, err
}

func generateApplyPendingCommissionFn() ([]byte, error) {
	applyPendingCommisionFn := &contractsapi.ApplyPendingCommissionHydraDelegationFn{}

	encoded, err := applyPendingCommisionFn.EncodeAbi()
	if err != nil {
		return nil, fmt.Errorf("encoding apply pending commission function failed: %w", err)
	}

	return encoded, err
}

func generateClaimCommissionFn(account *wallet.Account) ([]byte, error) {
	claimCommisionFn := &contractsapi.ClaimCommissionHydraDelegationFn{
		To: (types.Address)(account.Ecdsa.Address()),
	}

	encoded, err := claimCommisionFn.EncodeAbi()
	if err != nil {
		return nil, fmt.Errorf("encoding claim commission function failed: %w", err)
	}

	return encoded, err
}
