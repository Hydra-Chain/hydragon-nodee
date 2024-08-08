package priceoracle

import "math/big"

type PriceFeed interface {
	// GetPrice returns the USD price per 1 HYDRA with 8 decimals precision
	GetPrice(day uint64) (*big.Int, error)
}

type dummyPriceFeed struct{}

func NewDummyPriceFeed() PriceFeed {
	return &dummyPriceFeed{}
}

func (d *dummyPriceFeed) GetPrice(day uint64) (*big.Int, error) {
	return big.NewInt(1000000000000000000), nil
}
