package priceoracle

import (
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
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

func NewPriceFeed() (PriceFeed, error) {
	secretsManagerConfig, err := secrets.ReadConfig("./secretsManagerConfig.json")
	if err != nil {
		return nil, err
	}

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

	req, err := generateThirdPartyJSONRequest(apiURL)
	if err != nil {
		return nil, err
	}

	// Add the key in the header
	req.Header.Add("x-cg-demo-api-key", apiKey)

	body, err := fetchPriceData(req)
	if err != nil {
		return nil, err
	}

	var priceData PriceDataCoinGecko
	err = json.Unmarshal(body, &priceData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	price := priceData.MarketData.CurrentPrice.USD

	return common.ConvertFloatToBigInt(price, 18)
}

// generateThirdPartyJSONRequest creates a new HTTP request with a context that has a timeout
// for fetching data from a third-party API. The request is configured to accept JSON responses.
// It takes url which is the URL of the third-party API endpoint and timeoutInMinutes
// which is the duration in minutes for the request.
// Returns: The created HTTP request and any error that occurred.
func generateThirdPartyJSONRequest(url string) (*http.Request, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("accept", "application/json")

	return req, nil
}

// fetchPriceData makes an HTTP request to fetch price data and returns the response body.
// It takes the HTTP request to make as an input.
// Returns: The response body as a byte slice, or an error if the request failed.
func fetchPriceData(req *http.Request) ([]byte, error) {
	httpClient := &http.Client{
		Timeout: time.Minute * time.Duration(2),
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Close the request when finish, too
	defer res.Body.Close()
	data, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// getYesterdayFormatted returns the date in the format dd-mm-yyyy for the previous day.
func getYesterdayFormatted() string {
	yesterday := time.Now().UTC().AddDate(0, 0, -1)

	return yesterday.Format("02-01-2006")
}
