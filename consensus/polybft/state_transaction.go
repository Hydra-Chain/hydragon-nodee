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
		distributeVaultFundsFn contractsapi.DistributeDAOIncentiveHydraChainFn
		SyncValidatorsDataFn   contractsapi.SyncValidatorsDataHydraChainFn
		obj                    contractsapi.StateTransactionInput
	)

	switch {
	case bytes.Equal(sig, commitEpochFn.Sig()):
		// commit epoch
		obj = &contractsapi.CommitEpochHydraChainFn{}
	case bytes.Equal(sig, fundRewardWalletFn.Sig()):
		// fund reward wallet
		obj = &contractsapi.FundRewardWalletFn{}
	case bytes.Equal(sig, distributeRewardsFn.Sig()):
		// distribute rewards
		obj = &contractsapi.DistributeRewardsForHydraStakingFn{}
	case bytes.Equal(sig, distributeVaultFundsFn.Sig()):
		// distribute vault funds
		obj = &contractsapi.DistributeDAOIncentiveHydraChainFn{}
	case bytes.Equal(sig, SyncValidatorsDataFn.Sig()):
		// sync the validators voting power data
		obj = &contractsapi.SyncValidatorsDataHydraChainFn{}
	default:
		return nil, fmt.Errorf("unknown state transaction")
	}

	if err := obj.DecodeAbi(txData); err != nil {
		return nil, err
	}

	return obj, nil
}
