package priceoracle

import (
	"fmt"

	"github.com/0xPolygon/polygon-edge/consensus/polybft"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/contractsapi"
	"github.com/0xPolygon/polygon-edge/consensus/polybft/wallet"
	"github.com/0xPolygon/polygon-edge/contracts"
	"github.com/0xPolygon/polygon-edge/types"
	"github.com/umbracle/ethgo"
	"github.com/umbracle/ethgo/contract"
)

// PriceOracleState is an interface to interact with the price oracle contract in the chain
type PriceOracleState interface {
	// shouldVote returns does the given address should vote for the given day's price
	shouldVote(
		validatorAccount *wallet.Account,
		dayNumber uint64,
	) (shouldVote bool, falseReason string, err error)
}

type priceOracleState struct {
	polybft.SystemState
	priceOracleContract *contract.Contract
}

func newPriceOracleState(
	systemState polybft.SystemState,
	priceOracleAddr types.Address,
	provider contract.Provider,
) PriceOracleState {
	s := &priceOracleState{systemState, nil}

	s.priceOracleContract = contract.NewContract(
		ethgo.Address(priceOracleAddr),
		contractsapi.PriceOracle.Abi, contract.WithProvider(provider),
	)

	return s
}

func (p priceOracleState) shouldVote(
	validatorAccount *wallet.Account,
	dayNumber uint64,
) (bool, string, error) {
	rawOutput, err := p.priceOracleContract.Call("shouldVote", ethgo.Latest, dayNumber)
	if err != nil {
		return false, "", err
	}

	shouldVote, ok := rawOutput["0"].(bool)
	if !ok {
		return false, "", fmt.Errorf("failed to decode shouldVote result")
	}

	if !shouldVote {
		reason, ok := rawOutput["1"].(string)
		if !ok {
			return false, "", fmt.Errorf("failed to decode shouldVote reason")
		}

		return shouldVote, reason, nil
	}

	return shouldVote, "", nil
}

type PriceOracleStateProvider interface {
	GetPriceOracleState(
		header *types.Header,
	) (PriceOracleState, error)
}

type priceOracleStateProvider struct {
	blockchain blockchainBackend
}

func NewPriceOracleStateProvider(blockchain blockchainBackend) PriceOracleStateProvider {
	return &priceOracleStateProvider{blockchain: blockchain}
}

func (p priceOracleStateProvider) GetPriceOracleState(
	header *types.Header,
) (PriceOracleState, error) {
	provider, err := p.blockchain.GetStateProviderForBlock(header)
	if err != nil {
		return nil, err
	}

	return newPriceOracleState(p.blockchain.GetSystemState(provider), contracts.PriceOracleContract, provider), nil
}
