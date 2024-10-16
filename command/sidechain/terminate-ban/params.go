package terminateban

import (
	"bytes"
	"fmt"

	"github.com/0xPolygon/polygon-edge/command/helper"
	sidechainHelper "github.com/0xPolygon/polygon-edge/command/sidechain"
)

type terminateBanParams struct {
	accountDir         string
	accountConfig      string
	jsonRPC            string
	insecureLocalStore bool
}

func (tbp *terminateBanParams) validateFlags() error {
	if _, err := helper.ParseJSONRPCAddress(tbp.jsonRPC); err != nil {
		return fmt.Errorf("failed to parse json rpc address. Error: %w", err)
	}

	return sidechainHelper.ValidateSecretFlags(tbp.accountDir, tbp.accountConfig)
}

type terminateBanResult struct {
	validatorAddress string
}

func (tbr terminateBanResult) GetOutput() string {
	var buffer bytes.Buffer

	var vals []string

	if tbr.validatorAddress != "" {
		buffer.WriteString("\n[BAN TERMINATED]\n")

		vals = make([]string, 0, 1)
		vals = append(vals, fmt.Sprintf("Validator Address|%s", tbr.validatorAddress))
	}

	buffer.WriteString(helper.FormatKV(vals))
	buffer.WriteString("\n")

	return buffer.String()
}
