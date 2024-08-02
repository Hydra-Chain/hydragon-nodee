package priceoracle

import (
	"github.com/0xPolygon/polygon-edge/blockchain"
	"github.com/hashicorp/go-hclog"
)

type PriceOracle struct {
	logger hclog.Logger
	// closeCh is used to signal that the price oracle is stopped
	closeCh    chan struct{}
	blockchain *blockchain.Blockchain
	priceFeed  PriceFeed
}

func NewPriceOracle(blockchain *blockchain.Blockchain, logger hclog.Logger) *PriceOracle {
	priceFeed := NewDummyPriceFeed()

	return &PriceOracle{
		logger:     logger.Named("price-oracle"),
		blockchain: blockchain,
		priceFeed:  priceFeed,
	}
}

func (p *PriceOracle) Start() error {
	p.logger.Info("starting price oracle")

	go p.StartOracleProcess()

	return nil
}

func (p *PriceOracle) StartOracleProcess() {
	newBlockSub := p.blockchain.SubscribeEvents()
	defer p.blockchain.UnsubscribeEvents(newBlockSub)

	eventCh := newBlockSub.GetEventCh()

	for {
		select {
		case <-p.closeCh:
			return
		case ev := <-eventCh:
			// The blockchain notification system can eventually deliver
			// stale block notifications or fork blocks. These should be ignored
			if ev.NewChain[0].Number < p.blockchain.Header().Number || (ev.Type == blockchain.EventFork) {
				continue
			}

			p.logger.Debug("received new block notification", "block", ev.NewChain[0].Number)

			err := p.handleNewBlock()
			if err != nil {
				p.logger.Error("failed to handle new block", "err", err)
			}
		}
	}
}

// TODO: Ensure the close has worked before returning from the Close() function.
func (p *PriceOracle) Close() error {
	close(p.closeCh)
	p.logger.Info("price oracle stopped")

	return nil
}

func (p *PriceOracle) handleNewBlock() error {
	return nil
}
