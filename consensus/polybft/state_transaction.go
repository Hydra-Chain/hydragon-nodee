package polybft

import (
	"bytes"
	"fmt"

	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
)

const abiMethodIDLength = 4

func decodeStateTransaction(txData []byte) (contractsapi.StateTransactionInput, error) {
	if len(txData) < abiMethodIDLength {
		return nil, fmt.Errorf("state transactions have input")
	}

	sig := txData[:abiMethodIDLength]

	var (
		commitEpochFn          contractsapi.CommitEpochHydraChainFn
		fundRewardWalletFn     contractsapi.FundRewardWalletFn
		distributeRewardsFn    contractsapi.DistributeRewardsForHydraStakingFn
		distributeVaultFundsFn contractsapi.DistributeVaultFundsHydraChainFn
		obj                    contractsapi.StateTransactionInput
	)

	if bytes.Equal(sig, commitEpochFn.Sig()) {
		// commit epoch
		obj = &contractsapi.CommitEpochHydraChainFn{}
	} else if bytes.Equal(sig, fundRewardWalletFn.Sig()) {
		// fund reward wallet
		obj = &contractsapi.FundRewardWalletFn{}
	} else if bytes.Equal(sig, distributeRewardsFn.Sig()) {
		// distribute rewards
		obj = &contractsapi.DistributeRewardsForHydraStakingFn{}
	} else if bytes.Equal(sig, distributeVaultFundsFn.Sig()) {
		// distribute vault funds
		obj = &contractsapi.DistributeVaultFundsHydraChainFn{}
	} else {
		return nil, fmt.Errorf("unknown state transaction")
	}

	if err := obj.DecodeAbi(txData); err != nil {
		return nil, err
	}

	return obj, nil
}
