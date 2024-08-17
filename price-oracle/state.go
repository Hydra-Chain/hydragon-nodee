package priceoracle

import (
	"github.com/0xPolygon/polygon-edge/consensus/polybft"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/wallet"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/umbracle/ethgo"
)

// PriceOracleState is an interface to interact with the price oracle contract in the chain
type PriceOracleState interface {
	// shouldVote returns does the given address should vote for the given day's price
	shouldVote(
		validatorAccount *wallet.Account,
		jsonRPC string,
	) (shouldVote bool, falseReason string, err error)
}

type priceOracleState struct {
	polybft.SystemState
}

func newPriceOracleState(
	systemState polybft.SystemState,
) PriceOracleState {
	return &priceOracleState{systemState}
}

func (p priceOracleState) shouldVote(
	validatorAccount *wallet.Account,
	jsonRPC string,
) (bool, string, error) {
	txRelayer, err := NewTxRelayer(jsonRPC)
	if err != nil {
		return false, "", err
	}

	isValidValidatorVoteFn := &contractsapi.IsValidValidatorVotePriceOracleFn{}
	input, err := isValidValidatorVoteFn.EncodeAbi()
	if err != nil {
		return false, "", err
	}

	txn := &ethgo.Transaction{
		From:  validatorAccount.Ecdsa.Address(),
		Input: input,
		To:    (*ethgo.Address)(&contracts.PriceOracleContract),
	}

	_, err = txRelayer.SendTransaction(txn, validatorAccount.Ecdsa)
	if err != nil {
		return false, "validator is not active or already voted", nil
	}

	return true, "", nil
}
