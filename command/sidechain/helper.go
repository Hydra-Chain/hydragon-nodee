package sidechain

import (
	"errors"
	"fmt"
	"math/big"
	"os"

	"github.com/0xPolygon/polygon-edge/command/polybftsecrets"
	"github.com/0xPolygon/polygon-edge/consensus/polybft"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/wallet"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/helper/hex"
	"github.com/0xPolygon/polygon-edge/txrelayer"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/umbracle/ethgo"
)

const (
	SelfFlag               = "self"
	AmountFlag             = "amount"
	InsecureLocalStoreFlag = "insecure"

	DefaultGasPrice  = 1879048192 // 0x70000000
	MaxCommission    = 100
	MaxVestingPeriod = 52
)

func CheckIfDirectoryExist(dir string) error {
	if _, err := os.Stat(dir); errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("provided directory '%s' doesn't exist", dir)
	}

	return nil
}

func ValidateSecretFlags(dataDir, config string) error {
	if config == "" {
		if dataDir == "" {
			return polybftsecrets.ErrInvalidParams
		} else {
			return CheckIfDirectoryExist(dataDir)
		}
	}

	return nil
}

// GetAccount resolves secrets manager and returns an account object
func GetAccount(
	accountDir, accountConfig string,
	insecureLocalStore bool,
) (*wallet.Account, error) {
	// resolve secrets manager instance and allow usage of insecure local secrets manager
	secretsManager, err := polybftsecrets.GetSecretsManager(
		accountDir,
		accountConfig,
		insecureLocalStore,
	)
	if err != nil {
		return nil, err
	}

	return wallet.NewAccountFromSecret(secretsManager)
}

// GetAccountFromDir returns an account object from local secrets manager
func GetAccountFromDir(accountDir string, insecureLocalStore bool) (*wallet.Account, error) {
	return GetAccount(accountDir, "", insecureLocalStore)
}

// CreateTransaction is a helper function that creates a standard transaction based on the input parameters
func CreateTransaction(sender ethgo.Address, to *ethgo.Address, input []byte, value *big.Int) *ethgo.Transaction {
	txn := &ethgo.Transaction{
		From:  sender,
		To:    to,
		Input: input,
		Value: value,
	}

	return txn
}

// Define the ValidatorStatus enum
type ValidatorStatus int

const (
	None ValidatorStatus = iota
	Registered
	Active
	Banned
)

// Function to get ValidatorStatus based on number value
func GetStatus(value int) ValidatorStatus {
	switch value {
	case 1:
		return Active
	case 2:
		return Active
	case 3:
		return Banned
	default:
		return None
	}
}

// GetValidatorInfo queries HydraChain smart contract to retrieve the validator info for given address
func GetValidatorInfo(txRelayer txrelayer.TxRelayer, validatorAddr ethgo.Address) (*polybft.ValidatorInfo, error) {
	var getValidatorFn = &contractsapi.GetValidatorHydraChainFn{
		ValidatorAddress: types.Address(validatorAddr),
	}

	encoded, err := getValidatorFn.EncodeAbi()
	if err != nil {
		return nil, err
	}

	response, err := txRelayer.Call(validatorAddr, (ethgo.Address)(contracts.HydraChainContract), encoded)
	if err != nil {
		return nil, err
	}

	byteResponse, err := hex.DecodeHex(response)
	if err != nil {
		return nil, fmt.Errorf("unable to decode hex response, %w", err)
	}

	getValidatorMethod := contractsapi.HydraChain.Abi.GetMethod("getValidator")

	decoded, err := getValidatorMethod.Outputs.Decode(byteResponse)
	if err != nil {
		return nil, err
	}

	decodedOutputsMap, ok := decoded.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("could not convert decoded outputs to map")
	}

	stake, ok := decodedOutputsMap["stake"].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("could not convert stake to big.Int")
	}

	withdrawableRewards, ok := decodedOutputsMap["withdrawableRewards"].(*big.Int)
	if !ok {
		return nil, fmt.Errorf("could not convert withdrawableRewards to big.Int")
	}

	status, ok := decodedOutputsMap["status"].(uint8)
	if !ok {
		return nil, fmt.Errorf("could not convert status to uint8")
	}

	validatorInfo := &polybft.ValidatorInfo{
		Address:             validatorAddr,
		Stake:               stake,
		WithdrawableRewards: withdrawableRewards,
		IsActive:            Active == GetStatus(int(status)),
	}

	return validatorInfo, nil
}
