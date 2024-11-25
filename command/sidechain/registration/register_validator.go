package registration

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"time"

	"github.com/0xPolygon/polygon-edge/bls"
	"github.com/0xPolygon/polygon-edge/command"
	"github.com/0xPolygon/polygon-edge/command/helper"
	"github.com/0xPolygon/polygon-edge/command/polybftsecrets"
	"github.com/0xPolygon/polygon-edge/command/sidechain"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/wallet"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/helper/common"
	"github.com/0xPolygon/polygon-edge/secrets"
	"github.com/0xPolygon/polygon-edge/txrelayer"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/spf13/cobra"
	"github.com/umbracle/ethgo"
)

var (
	params registerParams

	hydraChain                = contracts.HydraChainContract
	stakeManager              = contracts.HydraStakingContract
	newValidatorEventABI      = contractsapi.HydraChain.Abi.Events["NewValidator"]
	commissionUpdatedEventABI = contractsapi.HydraDelegation.Abi.Events["CommissionUpdated"]
	stakeEventABI             = contractsapi.HydraStaking.Abi.Events["Staked"]
)

func GetCommand() *cobra.Command {
	registerCmd := &cobra.Command{
		Use:     "register-validator",
		Short:   "Registers and stake an enlisted validator",
		PreRunE: runPreRun,
		RunE:    runCommand,
	}

	setFlags(registerCmd)
	helper.SetRequiredFlags(registerCmd, params.getRequiredFlags())

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
		&params.stake,
		stakeFlag,
		"",
		"stake represents amount which is going to be staked by the new validator account",
	)

	cmd.Flags().Uint64Var(
		&params.commission,
		command.CommissionFlag,
		0,
		"a mandatory flag that represents the commission percentage of the new validator",
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

	newValidatorAccount, err := wallet.NewAccountFromSecret(secretsManager)
	if err != nil {
		return err
	}

	sRaw, err := secretsManager.GetSecret(secrets.ValidatorBLSSignature)
	if err != nil {
		return err
	}

	sb, err := hex.DecodeString(string(sRaw))
	if err != nil {
		return err
	}

	blsSignature, err := bls.UnmarshalSignature(sb)
	if err != nil {
		return err
	}

	receipt, err := registerValidator(txRelayer, newValidatorAccount, blsSignature)
	if err != nil {
		return err
	}

	if receipt.Status != uint64(types.ReceiptSuccess) {
		return errors.New("register validator transaction failed")
	}

	result := &registerResult{}
	foundNewValidatorLog := false

	for _, log := range receipt.Logs {
		if newValidatorEventABI.Match(log) {
			event, err := newValidatorEventABI.ParseLog(log)
			if err != nil {
				return err
			}

			result.validatorAddress = event["validator"].(ethgo.Address).String() //nolint:forcetypeassert
			foundNewValidatorLog = true
		}

		if commissionUpdatedEventABI.Match(log) {
			event, err := commissionUpdatedEventABI.ParseLog(log)
			if err != nil {
				return err
			}

			result.commission = event["newCommission"].(*big.Int).Uint64() //nolint:forcetypeassert
		}
	}

	if !foundNewValidatorLog {
		return fmt.Errorf(
			"could not find an appropriate log in the receipt that validates the registration has happened",
		)
	}

	if params.stake != "" {
		receipt, err := stake(txRelayer, newValidatorAccount)
		if err != nil {
			result.stakeResult = fmt.Sprintf("Failed to execute stake transaction: %s", err.Error())
		} else {
			populateStakeResults(receipt, result)
		}
	}

	outputter.WriteCommandResult(result)

	return nil
}

func registerValidator(
	sender txrelayer.TxRelayer,
	account *wallet.Account,
	signature *bls.Signature,
) (*ethgo.Receipt, error) {
	sigMarshal, err := signature.ToBigInt()
	if err != nil {
		return nil, fmt.Errorf("register validator failed: %w", err)
	}

	registerFn := &contractsapi.RegisterHydraChainFn{
		Signature:         sigMarshal,
		Pubkey:            account.Bls.PublicKey().ToBigInt(),
		InitialCommission: new(big.Int).SetUint64(params.commission),
	}

	encoded, err := registerFn.EncodeAbi()
	if err != nil {
		return nil, fmt.Errorf("register validator failed: %w", err)
	}

	txn := sidechain.CreateTransaction(
		account.Ecdsa.Address(),
		(*ethgo.Address)(&hydraChain),
		encoded,
		nil,
	)

	return sender.SendTransaction(txn, account.Ecdsa)
}

func stake(sender txrelayer.TxRelayer, account *wallet.Account) (*ethgo.Receipt, error) {
	stakeFn := &contractsapi.StakeHydraStakingFn{}

	encoded, err := stakeFn.EncodeAbi()
	if err != nil {
		return nil, err
	}

	stake, err := common.ParseUint256orHex(&params.stake)
	if err != nil {
		return nil, err
	}

	txn := &ethgo.Transaction{
		Input: encoded,
		To:    (*ethgo.Address)(&stakeManager),
		Value: stake,
	}

	receipt, err := sender.SendTransaction(txn, account.Ecdsa)
	if err != nil {
		// retry execution. Issue: https://github.com/valyala/fasthttp/issues/189
		receipt, err = sender.SendTransaction(txn, account.Ecdsa)
	}

	return receipt, err
}

func populateStakeResults(receipt *ethgo.Receipt, result *registerResult) {
	if receipt.Status != uint64(types.ReceiptSuccess) {
		result.stakeResult = fmt.Sprintf("Stake transaction failed. Tx: %s", receipt.TransactionHash.String())
		result.amount = "0"

		return
	}

	// check the logs to verify stake
	for _, log := range receipt.Logs {
		if stakeEventABI.Match(log) {
			event, err := stakeEventABI.ParseLog(log)
			if err != nil {
				result.stakeResult = "Failed to parse stake log"

				return
			}

			result.amount = event["amount"].(*big.Int).String() //nolint:forcetypeassert
			result.stakeResult = "Stake succeeded"

			return
		}
	}

	result.stakeResult = "Could not find an appropriate log in receipt that stake happened"
}
