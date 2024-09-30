package output

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/0xPolygon/polygon-edge/command"
	"github.com/0xPolygon/polygon-edge/command/polybftsecrets"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/wallet"
	"github.com/0xPolygon/polygon-edge/network"
	"github.com/0xPolygon/polygon-edge/secrets"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/spf13/cobra"
)

type OutputParams struct {
	dataDir            string
	configPath         string
	insecureLocalStore bool

	secretsManager secrets.SecretsManager
	secretsConfig  *secrets.SecretsManagerConfig

	networkKey       string
	privateKey       string
	blsPrivateKey    string
	validatorAddress string
	blsPublicKey     string
	nodeID           string
}

var (
	errInvalidParams   = errors.New("no config file or data directory passed in")
	errInvalidConfig   = errors.New("invalid secrets configuration")
	errUnsupportedType = errors.New("unsupported secrets manager")
)

func (op *OutputParams) validateFlags() error {
	if op.dataDir == "" && op.configPath == "" {
		return errInvalidParams
	}

	return nil
}

func (op *OutputParams) setFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&op.dataDir,
		polybftsecrets.AccountDirFlag,
		"",
		polybftsecrets.AccountDirFlagDesc,
	)

	cmd.Flags().StringVar(
		&op.configPath,
		polybftsecrets.AccountConfigFlag,
		"",
		polybftsecrets.AccountConfigFlagDesc,
	)

	cmd.Flags().BoolVar(
		&op.insecureLocalStore,
		polybftsecrets.InsecureLocalStoreFlag,
		false,
		"the flag indicates if the stored secrets are encrypted or not",
	)

	// Don't accept data-dir and config flags because they are related to different secrets managers.
	// data-dir is about the local FS as secrets storage, config is about remote secrets manager.
	cmd.MarkFlagsMutuallyExclusive(polybftsecrets.AccountDirFlag, polybftsecrets.AccountConfigFlag)
}

func (op *OutputParams) initSecrets() error {
	if err := op.initSecretsManager(); err != nil {
		return err
	}

	if !op.secretsManager.HasSecret(secrets.NetworkKey) {
		return fmt.Errorf("network key does not exist")
	}

	if !op.secretsManager.HasSecret(secrets.ValidatorKey) {
		return fmt.Errorf("validator key does not exist")
	}

	if !op.secretsManager.HasSecret(secrets.ValidatorBLSKey) {
		return fmt.Errorf("validator bls key does not exist")
	}

	// decrypt the validator key
	encodedKey, err := op.secretsManager.GetSecret(secrets.ValidatorKey)
	if err != nil {
		return err
	}

	op.privateKey = string(encodedKey)

	account, err := wallet.NewAccountFromSecret(op.secretsManager)
	if err != nil {
		return err
	}

	op.validatorAddress = account.Address().String()

	// decrypt the validator bls key
	encodedKey, err = op.secretsManager.GetSecret(secrets.ValidatorBLSKey)
	if err != nil {
		return err
	}

	op.blsPrivateKey = string(encodedKey)
	op.blsPublicKey = hex.EncodeToString(account.Bls.PublicKey().Marshal())

	// decrypt the network key and get node ID
	encodedKey, err = op.secretsManager.GetSecret(secrets.NetworkKey)
	if err != nil {
		return err
	}

	op.networkKey = string(encodedKey)

	parsedKey, err := network.ParseLibp2pKey(encodedKey)
	if err != nil {
		return err
	}

	nodeID, err := peer.IDFromPrivateKey(parsedKey)
	if err != nil {
		return err
	}

	op.nodeID = nodeID.String()

	return nil
}

func (op *OutputParams) initSecretsManager() error {
	secretsManager, err := polybftsecrets.GetSecretsManager(op.dataDir, op.configPath, op.insecureLocalStore)
	if err != nil {
		return err
	}

	op.secretsManager = secretsManager

	return nil
}

func (op *OutputParams) getResult() command.CommandResult {
	return &SecretsOutputResult{
		NetworkKey:       op.networkKey,
		PrivateKey:       op.privateKey,
		BLSPrivateKey:    op.blsPrivateKey,
		ValidatorAddress: op.validatorAddress,
		BLSPublicKey:     op.blsPublicKey,
		NodeID:           op.nodeID,
	}
}
