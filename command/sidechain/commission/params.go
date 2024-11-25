package commission

import (
	"bytes"
	"fmt"

	"github.com/0xPolygon/polygon-edge/command"
	"github.com/0xPolygon/polygon-edge/command/helper"
	sidechainHelper "github.com/0xPolygon/polygon-edge/command/sidechain"
)

const (
	applyFlag = "apply"
	claimFlag = "claim"
)

type setCommissionParams struct {
	accountDir         string
	accountConfig      string
	commission         uint64
	apply              bool
	claim              bool
	jsonRPC            string
	insecureLocalStore bool
}

type setCommissionResult struct {
	staker     string
	commission uint64
	isPending  bool
	isClaimed  bool
}

func (scp *setCommissionParams) getRequiredFlags() []string {
	return []string{
		command.CommissionFlag,
	}
}

func (scp *setCommissionParams) validateFlags() error {
	if err := sidechainHelper.ValidateSecretFlags(scp.accountDir, scp.accountConfig); err != nil {
		return err
	}

	if _, err := helper.ParseJSONRPCAddress(scp.jsonRPC); err != nil {
		return fmt.Errorf("failed to parse json rpc address. Error: %w", err)
	}

	if params.commission > sidechainHelper.MaxCommission {
		return fmt.Errorf(
			"provided commission '%d' is higher than the maximum of '%d'",
			params.commission,
			sidechainHelper.MaxCommission,
		)
	}

	return nil
}

func (scr setCommissionResult) GetOutput() string {
	var buffer bytes.Buffer

	var addressString string

	var valueString string

	if scr.isPending { //nolint:gocritic
		buffer.WriteString("\n[NEW COMMISSION SET AND WILL REMAIN PENDING FOR 15 DAYS]\n")

		addressString = "Staker Address"
		valueString = "New Commission"
	} else if scr.isClaimed {
		buffer.WriteString("\n[COMMISSION CLAIMED]\n")

		addressString = "Received Address"
		valueString = "Claimed Commission (wei)"
	} else {
		buffer.WriteString("\n[COMMISSION APPLIED]\n")

		addressString = "Staker Address"
		valueString = "Applied Commission"
	}

	vals := make([]string, 0, 2)
	vals = append(vals, fmt.Sprintf("%s|%s", addressString, scr.staker))
	vals = append(vals, fmt.Sprintf("%s|%v", valueString, scr.commission))

	buffer.WriteString(helper.FormatKV(vals))
	buffer.WriteString("\n")

	return buffer.String()
}
