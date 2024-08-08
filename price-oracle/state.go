package priceoracle

import "github.com/0xPolygon/polygon-edge/consensus/polybft"

// PriceOracleState is an interface to interact with the price oracle contract in the chain
type PriceOracleState interface {
	// shouldVote returns does the given address should vote for the given day's price
	shouldVote(address string, dayNumber uint64) (shouldVote bool, falseReason string, err error)
}

type priceOracleState struct {
	polybft.SystemState
}

func newPriceOracleState(systemState polybft.SystemState) PriceOracleState {
	return &priceOracleState{systemState}
}

func (p priceOracleState) shouldVote(address string, dayNumber uint64) (bool, string, error) {
	return true, "", nil
}
