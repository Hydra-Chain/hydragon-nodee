package outputprivate

import (
	"errors"

	"github.com/0xPolygon/polygon-edge/command"
	"github.com/0xPolygon/polygon-edge/command/polybftsecrets"
	"github.com/0xPolygon/polygon-edge/secrets"
	"github.com/0xPolygon/polygon-edge/secrets/helper"
	"github.com/spf13/cobra"
)

type OutputParams struct {
	DataDir            string
	ConfigPath         string
	InsecureLocalStore bool

	SecretsManager secrets.SecretsManager
	SecretsConfig  *secrets.SecretsManagerConfig
}

type outputResult struct {
	networkKey    string
	privateKey    string
	blsPrivateKey string
}

var (
	errInvalidParams   = errors.New("no config file or data directory passed in")
	errInvalidConfig   = errors.New("invalid secrets configuration")
	errUnsupportedType = errors.New("unsupported secrets manager")
)

func (op *OutputParams) ValidateFlags() error {
	if op.DataDir == "" && op.ConfigPath == "" {
		return errInvalidParams
	}

	return nil
}

func (op *OutputParams) SetFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&op.DataDir,
		polybftsecrets.AccountDirFlag,
		"",
		polybftsecrets.AccountDirFlagDesc,
	)

	cmd.Flags().StringVar(
		&op.ConfigPath,
		polybftsecrets.AccountConfigFlag,
		"",
		polybftsecrets.AccountConfigFlagDesc,
	)

	cmd.Flags().BoolVar(
		&op.InsecureLocalStore,
		polybftsecrets.InsecureLocalStoreFlag,
		false,
		"the flag indicates if the stored secrets are encrypted or not",
	)

	// Don't accept data-dir and config flags because they are related to different secrets managers.
	// data-dir is about the local FS as secrets storage, config is about remote secrets manager.
	cmd.MarkFlagsMutuallyExclusive(polybftsecrets.AccountDirFlag, polybftsecrets.AccountConfigFlag)
}

func (or *outputResult) initSecrets(outputParams *OutputParams) error {
	if err := outputParams.InitSecretsManager(); err != nil {
		return err
	}

	// load the encoded ecdsa private key
	encodedKey, err := helper.LoadEncodedPrivateKey(outputParams.SecretsManager, secrets.ValidatorKey)
	if err != nil {
		return err
	}

	or.privateKey = string(encodedKey)

	// load the encoded bls private key
	encodedKey, err = helper.LoadEncodedPrivateKey(outputParams.SecretsManager, secrets.ValidatorBLSKey)
	if err != nil {
		return err
	}

	or.blsPrivateKey = string(encodedKey)

	// load the encoded network private key
	encodedKey, err = helper.LoadEncodedPrivateKey(outputParams.SecretsManager, secrets.NetworkKey)
	if err != nil {
		return err
	}

	or.networkKey = string(encodedKey)

	return nil
}

func (op *OutputParams) InitSecretsManager() error {
	secretsManager, err := polybftsecrets.GetSecretsManager(op.DataDir, op.ConfigPath, op.InsecureLocalStore)
	if err != nil {
		return err
	}

	op.SecretsManager = secretsManager

	return nil
}

func (or *outputResult) getResult() command.CommandResult {
	return &SecretsOutputResult{
		NetworkKey:    or.networkKey,
		PrivateKey:    or.privateKey,
		BLSPrivateKey: or.blsPrivateKey,
	}
}
