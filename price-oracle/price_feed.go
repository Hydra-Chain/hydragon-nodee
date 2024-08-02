package priceoracle

import "math/big"

type PriceFeed interface {
	GetPrice() *big.Int
}

type dummyPriceFeed struct{}

func NewDummyPriceFeed() PriceFeed {
	return &dummyPriceFeed{}
}

func (d *dummyPriceFeed) GetPrice() *big.Int {
	return big.NewInt(1000000000000000000)
}
