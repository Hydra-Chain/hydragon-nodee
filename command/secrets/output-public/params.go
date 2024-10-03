package outputpublic

import (
	"github.com/0xPolygon/polygon-edge/command"
	outputprivate "github.com/0xPolygon/polygon-edge/command/secrets/output-private"
	"github.com/0xPolygon/polygon-edge/secrets/helper"
)

type outputResult struct {
	validatorAddress string
	blsPublicKey     string
	nodeID           string
}

func (or *outputResult) initPublicData(outputParams *outputprivate.OutputParams) error {
	if err := outputParams.InitSecretsManager(); err != nil {
		return err
	}

	// fetch the public key
	validatorAddress, err := helper.GetValidatorAddress(outputParams.SecretsManager)
	if err != nil {
		return err
	}
	or.validatorAddress = validatorAddress

	// fetch the public bls key
	blsPublicKey, err := helper.GetBLSPublicKey(outputParams.SecretsManager)
	if err != nil {
		return err
	}
	or.blsPublicKey = blsPublicKey

	// fetch the node id
	nodeID, err := helper.LoadNodeID(outputParams.SecretsManager)
	if err != nil {
		return err
	}
	or.nodeID = nodeID

	return nil
}

func (or *outputResult) getResult() command.CommandResult {
	return &OutputResult{
		ValidatorAddress: or.validatorAddress,
		BLSPublicKey:     or.blsPublicKey,
		NodeID:           or.nodeID,
	}
}
