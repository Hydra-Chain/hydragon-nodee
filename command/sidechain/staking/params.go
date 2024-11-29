package staking

import (
	"bytes"
	"fmt"

	"github.com/0xPolygon/polygon-edge/command/helper"
	sidechainHelper "github.com/0xPolygon/polygon-edge/command/sidechain"
)

var (
	delegateAddressFlag = "delegate"
	vestingFlag         = "vesting"
	vestingPeriodFlag   = "vesting-period"
)

type stakeParams struct {
	accountDir         string
	accountConfig      string
	jsonRPC            string
	amount             string
	self               bool
	vesting            bool
	vestingPeriod      uint64
	delegateAddress    string
	insecureLocalStore bool
}

func (sp *stakeParams) getRequiredFlags() []string {
	return []string{
		sidechainHelper.AmountFlag,
	}
}

func (sp *stakeParams) validateFlags() error {
	if _, err := helper.ParseJSONRPCAddress(sp.jsonRPC); err != nil {
		return fmt.Errorf("failed to parse json rpc address. Error: %w", err)
	}

	if sp.vesting && (sp.vestingPeriod < 1 || sp.vestingPeriod > sidechainHelper.MaxVestingPeriod) {
		return fmt.Errorf(
			"invalid vesting period '%d'. The period must between 1 and '%d' weeks",
			sp.vestingPeriod,
			sidechainHelper.MaxVestingPeriod,
		)
	}

	return sidechainHelper.ValidateSecretFlags(sp.accountDir, sp.accountConfig)
}

type stakeResult struct {
	validatorAddress string
	isSelfStake      bool
	isVesting        bool
	amount           string
	vestingPeriod    uint64
	delegatedTo      string
}

func (sr stakeResult) GetOutput() string {
	var buffer bytes.Buffer

	var title string

	var vals []string

	if sr.isSelfStake {
		title = "\n[SELF STAKE]\n"

		vals = []string{
			fmt.Sprintf("Validator Address|%s", sr.validatorAddress),
			fmt.Sprintf("Amount Staked|%v", sr.amount),
		}

		if sr.isVesting {
			title = "\n[VESTED STAKING ACTIVATED]\n"

			vals = append(vals, fmt.Sprintf("Vesting Period|%d", sr.vestingPeriod))
		}
	} else {
		title = "\n[DELEGATED AMOUNT]\n"

		vals = []string{
			fmt.Sprintf("Validator Address|%s", sr.validatorAddress),
			fmt.Sprintf("Amount Delegated|%v", sr.amount),
			fmt.Sprintf("Delegated To|%s", sr.delegatedTo),
		}
	}

	buffer.WriteString(title)
	buffer.WriteString(helper.FormatKV(vals))
	buffer.WriteString("\n")

	return buffer.String()
}
