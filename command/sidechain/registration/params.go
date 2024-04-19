package registration

import (
	"bytes"
	"fmt"

	"github.com/0xPolygon/polygon-edge/command/helper"
	sidechainHelper "github.com/0xPolygon/polygon-edge/command/sidechain"
	"github.com/0xPolygon/polygon-edge/helper/common"
)

const (
	stakeFlag   = "stake"
	chainIDFlag = "chain-id"
	commissionFlag = "commission"
)

const (
	maxCommission = 100
)

type registerParams struct {
	accountDir         string
	accountConfig      string
	jsonRPC            string
	stake              string
	commission         uint64
	chainID            int64
	insecureLocalStore bool
}

func (rp *registerParams) validateFlags() error {
	if err := sidechainHelper.ValidateSecretFlags(rp.accountDir, rp.accountConfig); err != nil {
		return err
	}

	if _, err := helper.ParseJSONRPCAddress(rp.jsonRPC); err != nil {
		return fmt.Errorf("failed to parse json rpc address. Error: %w", err)
	}

	if rp.stake != "" {
		_, err := common.ParseUint256orHex(&rp.stake)
		if err != nil {
			return fmt.Errorf("provided stake '%s' isn't valid", rp.stake)
		}
	}

	if (rp.commission > maxCommission) {
		return fmt.Errorf("provided commission '%d' is higher than the maximum of '%d'", rp.commission, maxCommission)
	}

	return nil
}

type registerResult struct {
	validatorAddress string
	stakeResult      string
	amount           string
	commission       uint64
}

func (rr registerResult) GetOutput() string {
	var buffer bytes.Buffer

	var vals []string

	buffer.WriteString("\n[REGISTRATION]\n")

	vals = make([]string, 0, 3)
	vals = append(vals, fmt.Sprintf("Validator Address|%s", rr.validatorAddress))
	vals = append(vals, fmt.Sprintf("Staking Result|%s", rr.stakeResult))
	vals = append(vals, fmt.Sprintf("Amount Staked|%v", rr.amount))
	vals = append(vals, fmt.Sprintf("Commission |%v", rr.commission))

	buffer.WriteString(helper.FormatKV(vals))
	buffer.WriteString("\n")

	return buffer.String()
}
