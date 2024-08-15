package priceoracle

import (
	"github.com/0xPolygon/polygon-edge/consensus/polybft"
	"github.com/0xPolygon/polygon-edge/txrelayer"
)

// PriceOracleState is an interface to interact with the price oracle contract in the chain
type PriceOracleState interface {
	// shouldVote returns does the given address should vote for the given day's price
	shouldVote(
		address string,
		dayNumber uint64,
		jsonRPC string,
	) (shouldVote bool, falseReason string, err error)
}

type priceOracleState struct {
	polybft.SystemState
	txRelayer txrelayer.TxRelayer
}

func newPriceOracleState(
	systemState polybft.SystemState,
	txRelayer txrelayer.TxRelayer,
) PriceOracleState {
	return &priceOracleState{systemState, txRelayer}
}

func (p priceOracleState) shouldVote(
	address string,
	dayNumber uint64,
	jsonRPC string,
) (bool, string, error) {
	if p.txRelayer == nil {
		newRelayer, err := NewTxRelayer(jsonRPC)
		if err != nil {
			return false, "", err
		}

		p.txRelayer = newRelayer
	}

	return true, "", nil
}
