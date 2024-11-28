package sidechain

import (
	"errors"
	"fmt"
	"math/big"
	"os"

	"github.com/0xPolygon/polygon-edge/command/polybftsecrets"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/wallet"
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
