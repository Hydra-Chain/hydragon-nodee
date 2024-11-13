package priceoracle

import (
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	"github.com/0xPolygon/polygon-edge/helper/common"
	"github.com/0xPolygon/polygon-edge/secrets"
	"github.com/0xPolygon/polygon-edge/types"
)

type PriceFeed interface {
	// GetPrice returns the USD price per 1 HYDRA with 8 decimals precision
	GetPrice(header *types.Header) (*big.Int, error)
}

type dummyPriceFeed struct{}

func NewDummyPriceFeed() (PriceFeed, error) {
	return &dummyPriceFeed{}, nil
}

func (d *dummyPriceFeed) GetPrice(header *types.Header) (*big.Int, error) {
	return nil, nil
}

type priceFeed struct {
	coinGeckoAPIKey string
}

func NewPriceFeed(secretsManagerConfig *secrets.SecretsManagerConfig) (PriceFeed, error) {
	apiKey, ok := secretsManagerConfig.Extra[secrets.CoinGeckoAPIKey].(string)
	if !ok {
		return nil, fmt.Errorf(secrets.CoinGeckoAPIKey + " is not a string")
	}

	return &priceFeed{coinGeckoAPIKey: apiKey}, nil
}

func (p *priceFeed) GetPrice(header *types.Header) (*big.Int, error) {
	price, err := getCoingeckoPrice(p.coinGeckoAPIKey)
	if err != nil {
		return nil, fmt.Errorf("get price from CoinGecko failed: %w", err)
	}

	return price, nil
}

type PriceDataCoinGecko struct {
	ID         string `json:"id"`
	Symbol     string `json:"symbol"`
	Name       string `json:"name"`
	MarketData struct {
		CurrentPrice struct {
			USD float64 `json:"usd"`
		} `json:"current_price"`
	} `json:"market_data"`
}

// getCoingeckoPrice fetches the current price of the Hydra cryptocurrency from the CoinGecko API.
// It takes a timeout as input to wait for the request, in minutes.
// It returns a big.Int representing the average price for the previous day.
func getCoingeckoPrice(apiKey string) (*big.Int, error) {
	yesterday := getYesterdayFormatted()
	apiURL := fmt.Sprintf(`https://api.coingecko.com/api/v3/coins/hydra/history?date=%s`, yesterday)

	req, err := common.GenerateThirdPartyJSONRequest(apiURL)
	if err != nil {
		return nil, err
	}

	// Add the key in the header
	req.Header.Add("x-cg-demo-api-key", apiKey)

	body, err := common.FetchData(req)
	if err != nil {
		return nil, err
	}

	var priceData PriceDataCoinGecko

	err = json.Unmarshal(body, &priceData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	price := priceData.MarketData.CurrentPrice.USD

	return common.ConvertFloatToBigInt(price, 8)
}

// getYesterdayFormatted returns the date in the format dd-mm-yyyy for the previous day.
func getYesterdayFormatted() string {
	yesterday := time.Now().UTC().AddDate(0, 0, -1)

	return yesterday.Format("02-01-2006")
}
