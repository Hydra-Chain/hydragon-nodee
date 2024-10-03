package outputpublic

import (
	"bytes"
	"fmt"

	"github.com/0xPolygon/polygon-edge/command/helper"
)

type OutputResult struct {
	ValidatorAddress string `json:"validator_address"`
	BLSPublicKey     string `json:"bls_public_key"`
	NodeID           string `json:"node_id"`
}

func (or *OutputResult) GetOutput() string {
	var buffer bytes.Buffer

	buffer.WriteString("\n[PUBLIC DATA OUTPUT]\n")
	buffer.WriteString(helper.FormatKV([]string{
		fmt.Sprintf("Validator Address|%s", or.ValidatorAddress),
		fmt.Sprintf("BLS Public Key|%s", or.BLSPublicKey),
		fmt.Sprintf("Node ID|%s", or.NodeID),
	}))

	return buffer.String()
}
