package withdraw

import (
	"bytes"
	"fmt"

	"github.com/0xPolygon/polygon-edge/command/helper"
	sidechainHelper "github.com/0xPolygon/polygon-edge/command/sidechain"
)

var (
	penalizedFundsFlag = "penalized-funds"
)

type withdrawParams struct {
	accountDir         string
	accountConfig      string
	jsonRPC            string
	penalizedFunds     bool
	insecureLocalStore bool
}

func (w *withdrawParams) validateFlags() error {
	if _, err := helper.ParseJSONRPCAddress(w.jsonRPC); err != nil {
		return fmt.Errorf("failed to parse json rpc address. Error: %w", err)
	}

	return sidechainHelper.ValidateSecretFlags(w.accountDir, w.accountConfig)
}

type withdrawResult struct {
	ValidatorAddress string `json:"validatorAddress"`
	Amount           string `json:"amount"`
	BlockNumber      uint64 `json:"blockNumber"`
	Registered       bool   `json:"registered"`
}

func (r *withdrawResult) GetOutput() string {
	var buffer bytes.Buffer

	if r.Registered {
		buffer.WriteString("\n[WITHDRAWAL REGISTERED]\n")
	} else {
		buffer.WriteString("\n[WITHDRAWAL SUCCESSFUL]\n")
	}

	vals := make([]string, 0, 4)
	vals = append(vals, fmt.Sprintf("Validator Address|%s", r.ValidatorAddress))
	vals = append(vals, fmt.Sprintf("Amount Withdrawn|%s", r.Amount))
	vals = append(vals, fmt.Sprintf("Inclusion Block Number|%d", r.BlockNumber))

	buffer.WriteString(helper.FormatKV(vals))
	buffer.WriteString("\n")

	return buffer.String()
}
