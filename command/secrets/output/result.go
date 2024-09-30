package output

import (
	"bytes"
	"fmt"

	"github.com/0xPolygon/polygon-edge/command/helper"
)

type SecretsOutputResult struct {
	NetworkKey       string `json:"network_key"`
	PrivateKey       string `json:"private_key"`
	BLSPrivateKey    string `json:"bls_private_key"`
	ValidatorAddress string `json:"validator_address"`
	BLSPublicKey     string `json:"bls_public_key"`
	NodeID           string `json:"node_id"`
}

func (r *SecretsOutputResult) GetOutput() string {
	var buffer bytes.Buffer

	buffer.WriteString("\n[SECRET AND PUBLIC KEYS]\n")
	buffer.WriteString(helper.FormatKV([]string{
		fmt.Sprintf("Network Key|%s", r.NetworkKey),
		fmt.Sprintf("Private Key|%s", r.PrivateKey),
		fmt.Sprintf("BLS Private Key|%s", r.BLSPrivateKey),
		fmt.Sprintf("Validator Address|%s", r.ValidatorAddress),
		fmt.Sprintf("BLS Public Key|%s", r.BLSPublicKey),
		fmt.Sprintf("Node ID|%s", r.NodeID),
	}))

	return buffer.String()
}
