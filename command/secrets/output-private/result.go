package outputprivate

import (
	"bytes"
	"fmt"

	"github.com/0xPolygon/polygon-edge/command/helper"
)

type SecretsOutputResult struct {
	NetworkKey    string `json:"network_key"`
	PrivateKey    string `json:"private_key"`
	BLSPrivateKey string `json:"bls_private_key"`
}

func (r *SecretsOutputResult) GetOutput() string {
	var buffer bytes.Buffer

	buffer.WriteString("\n[PRIVATE KEYS OUTPUT]\n")
	buffer.WriteString(helper.FormatKV([]string{
		fmt.Sprintf("Network Private Key|%s", r.NetworkKey),
		fmt.Sprintf("EVM Private Key|%s", r.PrivateKey),
		fmt.Sprintf("Validator BLS Private Key|%s", r.BLSPrivateKey),
	}))

	return buffer.String()
}
