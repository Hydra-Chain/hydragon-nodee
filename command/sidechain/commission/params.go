package commission

import (
	"bytes"
	"fmt"

	"github.com/0xPolygon/polygon-edge/command/helper"
	sidechainHelper "github.com/0xPolygon/polygon-edge/command/sidechain"
)

const (
	commissionFlag = "commission"
)

type setCommissionParams struct {
	accountDir         string
	accountConfig      string
	commission         uint64
	jsonRPC            string
	insecureLocalStore bool
}

type setCommissionResult struct {
	staker        string
	newCommission uint64
}

func (scp *setCommissionParams) getRequiredFlags() []string {
	return []string{
		commissionFlag,
	}
}

func (scp *setCommissionParams) validateFlags() error {
	if err := sidechainHelper.ValidateSecretFlags(scp.accountDir, scp.accountConfig); err != nil {
		return err
	}

	if _, err := helper.ParseJSONRPCAddress(scp.jsonRPC); err != nil {
		return fmt.Errorf("failed to parse json rpc address. Error: %w", err)
	}

	return validateCommission(params.commission)
}

func validateCommission(commission uint64) error {
	if commission > sidechainHelper.MaxCommission {
		return fmt.Errorf(
			"provided commission '%d' is higher than the maximum of '%d'",
			commission,
			sidechainHelper.MaxCommission,
		)
	}

	return nil
}

func (scr setCommissionResult) GetOutput() string {
	var buffer bytes.Buffer

	var vals []string

	buffer.WriteString("\n[COMMISSION SET]\n")

	vals = make([]string, 0, 3)
	vals = append(vals, fmt.Sprintf("Staker Address|%s", scr.staker))
	vals = append(vals, fmt.Sprintf("New Commission |%v", scr.newCommission))

	buffer.WriteString(helper.FormatKV(vals))
	buffer.WriteString("\n")

	return buffer.String()
}
