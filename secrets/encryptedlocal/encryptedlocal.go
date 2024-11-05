package encryptedlocal

import (
	"errors"

	"github.com/0xPolygon/polygon-edge/secrets"
	"github.com/0xPolygon/polygon-edge/secrets/local"
	"github.com/hashicorp/go-hclog"
)

// EncryptedLocalSecretsManager is a SecretsManager that
// stores secrets encrypted locally on disk
type EncryptedLocalSecretsManager struct {
	prompt *Prompt
	logger hclog.Logger
	*local.LocalSecretsManager
	encryption Encryption
	pwd        []byte
}

// SecretsManagerFactory implements the factory method
func SecretsManagerFactory(
	_ *secrets.SecretsManagerConfig,
	params *secrets.SecretsManagerParams,
) (secrets.SecretsManager, error) {
	baseSM, err := local.SecretsManagerFactory(
		nil, // Local secrets manager doesn't require a config
		params)
	if err != nil {
		return nil, err
	}

	localSM, ok := baseSM.(*local.LocalSecretsManager)
	if !ok {
		return nil, errors.New("invalid type assertion")
	}

	prompt := NewPrompt()
	encryption := NewEncryption()
	logger := params.Logger.Named(string(secrets.EncryptedLocal))
	esm := &EncryptedLocalSecretsManager{
		prompt,
		logger,
		localSM,
		encryption,
		nil,
	}

	return esm, nil
}

func (esm *EncryptedLocalSecretsManager) SetSecret(name string, value []byte) error {
	esm.logger.Info("Configuring secret", "name", name)
	onSetHandler, ok := onSetHandlers[name]
	if ok {
		res, err := onSetHandler(esm, name, value)
		if err != nil {
			return err
		}

		value = res
	}

	return esm.LocalSecretsManager.SetSecret(name, value)
}

func (esm *EncryptedLocalSecretsManager) GetSecret(name string) ([]byte, error) {
	value, err := esm.LocalSecretsManager.GetSecret(name)
	if err != nil {
		return nil, err
	}

	onGetHandler, ok := onGetHandlers[name]
	if ok {
		value, err = onGetHandler(esm, name, value)
		if err != nil {
			return nil, err
		}
	}

	return value, nil
}

// --------- Custom Additional handlers ---------

type AdditionalHandlerFunc func(esm *EncryptedLocalSecretsManager, name string, value []byte) ([]byte, error)

var onSetHandlers = map[string]AdditionalHandlerFunc{
	secrets.NetworkKey:      baseOnSetHandler,
	secrets.ValidatorBLSKey: baseOnSetHandler,
	secrets.ValidatorKey:    baseOnSetHandler,
}

func baseOnSetHandler(
	esm *EncryptedLocalSecretsManager,
	name string,
	value []byte,
) ([]byte, error) {
	esm.logger.Info(
		"Here is the raw hex value of your secret. \nPlease copy it and store it in a safe place.",
		name,
		string(value),
	)
	confirmValue, err := esm.prompt.DefaultPrompt(
		`Please re-type the secret key value (present above, after the "=") to confirm that you have backed it up in a safe location.`,
		"",
	)
	if err != nil {
		return nil, err
	}

	if confirmValue != string(value) {
		esm.logger.Error(
			"The secret value you entered does not match the original value. Please try again.",
		)

		return nil, errors.New("secret value mismatch")
	} else {
		esm.logger.Info("The secret value you entered matches the original value. Continuing.")
	}

	if esm.pwd == nil || len(esm.pwd) == 0 {
		esm.pwd, err = esm.prompt.GeneratePassword()
		if err != nil {
			return nil, err
		}
	}

	encryptedValue, err := esm.encryption.Encrypt(value, esm.pwd)
	if err != nil {
		return nil, err
	}

	return encryptedValue, nil
}

var onGetHandlers = map[string]AdditionalHandlerFunc{
	secrets.NetworkKey:      baseOnGetHandler,
	secrets.ValidatorBLSKey: baseOnGetHandler,
	secrets.ValidatorKey:    baseOnGetHandler,
}

func baseOnGetHandler(
	esm *EncryptedLocalSecretsManager,
	name string,
	value []byte,
) ([]byte, error) {
	if esm.pwd == nil || len(esm.pwd) == 0 {
		var err error
		esm.pwd, err = esm.prompt.InputPassword(false)
		if err != nil {
			return nil, err
		}
	}

	return esm.encryption.Decrypt(value, esm.pwd)
}
