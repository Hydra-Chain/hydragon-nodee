package staking

import (
	"fmt"
	"math/big"
	"time"

	"github.com/0xPolygon/polygon-edge/command"
	"github.com/0xPolygon/polygon-edge/command/helper"
	"github.com/0xPolygon/polygon-edge/command/polybftsecrets"
	"github.com/0xPolygon/polygon-edge/command/sidechain"
	sidechainHelper "github.com/0xPolygon/polygon-edge/command/sidechain"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/helper/common"
	"github.com/0xPolygon/polygon-edge/txrelayer"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/spf13/cobra"
	"github.com/umbracle/ethgo"
)

var (
	params           stakeParams
	stakeEventABI    = contractsapi.ValidatorSet.Abi.Events["Staked"]
	delegateEventABI = contractsapi.ValidatorSet.Abi.Events["Delegated"]
)

func GetCommand() *cobra.Command {
	stakeCmd := &cobra.Command{
		Use:     "stake",
		Short:   "Stakes the amount sent for validator or delegates it to another account",
		PreRunE: runPreRun,
		RunE:    runCommand,
	}

	helper.RegisterJSONRPCFlag(stakeCmd)
	setFlags(stakeCmd)

	return stakeCmd
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
		&params.self,
		sidechainHelper.SelfFlag,
		false,
		"indicates if its a self stake action",
	)

	cmd.Flags().StringVar(
		&params.amount,
		sidechainHelper.AmountFlag,
		"0",
		"amount to stake or delegate to another account",
	)

	cmd.Flags().StringVar(
		&params.delegateAddress,
		delegateAddressFlag,
		"",
		"account address to which stake should be delegated",
	)

	cmd.Flags().BoolVar(
		&params.insecureLocalStore,
		sidechain.InsecureLocalStoreFlag,
		false,
		"a flag to indicate if the secrets used are encrypted. If set to true, the secrets are stored in plain text.",
	)

	cmd.MarkFlagsMutuallyExclusive(sidechainHelper.SelfFlag, delegateAddressFlag)
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

	txRelayer, err := txrelayer.NewTxRelayer(txrelayer.WithIPAddress(params.jsonRPC),
		txrelayer.WithReceiptTimeout(150*time.Millisecond))
	if err != nil {
		return err
	}

	var encoded []byte
	if params.self {
		encoded, err = contractsapi.ValidatorSet.Abi.Methods["stake"].Encode([]interface{}{})
		if err != nil {
			return err
		}
	} else {
		delegateToAddress := types.StringToAddress(params.delegateAddress)
		encoded, err = contractsapi.ValidatorSet.Abi.Methods["delegate"].Encode(
			[]interface{}{ethgo.Address(delegateToAddress), false})
		if err != nil {
			return err
		}
	}

	parsedValue, err := common.ParseUint256orHex(&params.amount)
	if err != nil {
		return fmt.Errorf("cannot parse \"amount\" value %s", params.amount)
	}

	txn := &ethgo.Transaction{
		From:     validatorAccount.Ecdsa.Address(),
		Input:    encoded,
		To:       (*ethgo.Address)(&contracts.ValidatorSetContract),
		Value:    parsedValue,
		GasPrice: sidechainHelper.DefaultGasPrice,
	}

	receipt, err := txRelayer.SendTransaction(txn, validatorAccount.Ecdsa)
	if err != nil {
		return err
	}

	if receipt.Status == uint64(types.ReceiptFailed) {
		return fmt.Errorf("staking transaction failed on block %d", receipt.BlockNumber)
	}

	result := &stakeResult{
		validatorAddress: validatorAccount.Ecdsa.Address().String(),
	}

	foundLog := false

	// check the logs to check for the result
	for _, log := range receipt.Logs {
		if stakeEventABI.Match(log) {
			event, err := stakeEventABI.ParseLog(log)
			if err != nil {
				return err
			}

			result.isSelfStake = true
			result.amount = event["amount"].(*big.Int).Uint64() //nolint:forcetypeassert

			foundLog = true

			break
		} else if delegateEventABI.Match(log) {
			event, err := delegateEventABI.ParseLog(log)
			if err != nil {
				return err
			}

			result.amount = event["amount"].(*big.Int).Uint64()              //nolint:forcetypeassert
			result.delegatedTo = event["validator"].(ethgo.Address).String() //nolint:forcetypeassert

			foundLog = true

			break
		}
	}

	if !foundLog {
		return fmt.Errorf("could not find an appropriate log in receipt that stake or delegate happened")
	}

	outputter.WriteCommandResult(result)

	return nil
}
