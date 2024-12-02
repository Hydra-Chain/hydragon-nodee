package staking

import (
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
	"github.com/0xPolygon/polygon-edge/helper/common"
	"github.com/0xPolygon/polygon-edge/txrelayer"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/spf13/cobra"
	"github.com/umbracle/ethgo"
)

var (
	params stakeParams

	stakeEventABI    = contractsapi.HydraStaking.Abi.Events["Staked"]
	delegateEventABI = contractsapi.HydraDelegation.Abi.Events["Delegated"]
)

func GetCommand() *cobra.Command {
	stakeCmd := &cobra.Command{
		Use:     "stake",
		Short:   "Stakes the amount sent for validator or delegates it to another account",
		PreRunE: runPreRun,
		RunE:    runCommand,
	}

	setFlags(stakeCmd)

	helper.SetRequiredFlags(stakeCmd, params.getRequiredFlags())

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

	cmd.Flags().StringVar(
		&params.amount,
		sidechain.AmountFlag,
		"",
		"a mandatory flag which indicates the amount to self stake or delegate to another account",
	)

	cmd.Flags().BoolVar(
		&params.self,
		sidechain.SelfFlag,
		false,
		"indicates if its a self stake action",
	)

	cmd.Flags().Uint64Var(
		&params.vestingPeriod,
		vestingPeriodFlag,
		0,
		"this flag is used to open a vested staking position. It indicates the vesting period in weeks. "+
			"It must be at least 1 week and the max period is 52 weeks (1 year).",
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

	helper.RegisterJSONRPCFlag(cmd)

	cmd.MarkFlagsMutuallyExclusive(sidechain.SelfFlag, delegateAddressFlag)
	cmd.MarkFlagsMutuallyExclusive(vestingPeriodFlag, delegateAddressFlag)
	cmd.MarkFlagsMutuallyExclusive(polybftsecrets.AccountDirFlag, polybftsecrets.AccountConfigFlag)
}

func runPreRun(cmd *cobra.Command, _ []string) error {
	params.jsonRPC = helper.GetJSONRPCAddress(cmd)

	return params.validateFlags()
}

func runCommand(cmd *cobra.Command, _ []string) error {
	outputter := command.InitializeOutputter(cmd)
	defer outputter.WriteOutput()

	validatorAccount, err := sidechain.GetAccount(
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

	txn, err := createStakeTransaction(validatorAccount)
	if err != nil {
		return err
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
		vestingPeriod:    params.vestingPeriod,
	}

	foundLog := false

	// check the logs to check for the result
	for _, log := range receipt.Logs {
		var event map[string]interface{}

		var match bool

		if match = stakeEventABI.Match(log); match {
			event, err = stakeEventABI.ParseLog(log)
			if err != nil {
				return err
			}

			result.isSelfStake = true
			result.amount = event["amount"].(*big.Int).String() //nolint:forcetypeassert

			if params.vestingPeriod > 0 {
				validatorInfo, err := sidechain.GetValidatorInfo(txRelayer, validatorAccount.Ecdsa.Address())
				if err != nil {
					fmt.Printf("was unable to get the validator info %s", result.validatorAddress)
				} else {
					result.amount = validatorInfo.Stake.String()
				}
			}
		} else if match = delegateEventABI.Match(log); match {
			event, err = delegateEventABI.ParseLog(log)
			if err != nil {
				return err
			}

			result.amount = event["amount"].(*big.Int).String()              //nolint:forcetypeassert
			result.delegatedTo = event["validator"].(ethgo.Address).String() //nolint:forcetypeassert
		}

		if match {
			foundLog = true

			break
		}
	}

	if !foundLog {
		return fmt.Errorf(
			"could not find an appropriate log in receipt that stake or delegate happened",
		)
	}

	outputter.WriteCommandResult(result)

	return nil
}

func createStakeTransaction(validatorAccount *wallet.Account) (*ethgo.Transaction, error) {
	var (
		encoded      []byte
		contractAddr *ethgo.Address
		err          error
	)

	if params.self {
		if params.vestingPeriod > 0 {
			var stakeWithVestingFn = &contractsapi.StakeWithVestingHydraStakingFn{
				DurationWeeks: new(big.Int).SetUint64(params.vestingPeriod),
			}

			encoded, err = stakeWithVestingFn.EncodeAbi()
		} else {
			var stakeFn = &contractsapi.StakeHydraStakingFn{}
			encoded, err = stakeFn.EncodeAbi()
		}

		contractAddr = (*ethgo.Address)(&contracts.HydraStakingContract)
	} else {
		var delegateFn = &contractsapi.DelegateHydraDelegationFn{
			Staker: types.StringToAddress(params.delegateAddress),
		}

		encoded, err = delegateFn.EncodeAbi()
		contractAddr = (*ethgo.Address)(&contracts.HydraDelegationContract)
	}

	if err != nil {
		return nil, err
	}

	parsedValue, err := common.ParseUint256orHex(&params.amount)
	if err != nil {
		return nil, fmt.Errorf("cannot parse \"amount\" value %s", params.amount)
	}

	txn := sidechain.CreateTransaction(
		validatorAccount.Ecdsa.Address(),
		contractAddr,
		encoded,
		parsedValue,
	)

	return txn, nil
}
