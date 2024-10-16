package sidechain

import (
	"errors"
	"fmt"
	"os"

	"github.com/0xPolygon/polygon-edge/command/polybftsecrets"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/wallet"
)

const (
	SelfFlag               = "self"
	AmountFlag             = "amount"
	InsecureLocalStoreFlag = "insecure"

	DefaultGasPrice = 1879048192 // 0x70000000
	MaxCommission   = 100
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
