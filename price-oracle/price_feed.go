package priceoracle

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
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

func NewDummyPriceFeed() PriceFeed {
	return &dummyPriceFeed{}
}

func (d *dummyPriceFeed) GetPrice(header *types.Header) (*big.Int, error) {
	secretsManagerConfig, err := secrets.ReadConfig("./secretsManagerConfig.json")
	if err != nil {
		return nil, err
	}

	apiKey, ok := secretsManagerConfig.Extra[secrets.CoinGeckoAPIKey].(string)
	if !ok {
		return nil, fmt.Errorf(secrets.CoinGeckoAPIKey + " is not a string")
	}

	price, err := getCoingeckoPrice(2, apiKey)
	if err != nil || price.Cmp(big.NewInt(0)) == 0 {
		apiKey, ok = secretsManagerConfig.Extra[secrets.CoinMarketCapAPIKey].(string)
		if !ok {
			return nil, fmt.Errorf(secrets.CoinMarketCapAPIKey + " is not a string")
		}

		price, err = getCMCPrice(2, apiKey)
		if err != nil {
			return nil, fmt.Errorf("get price from third parties failed: %w", err)
		}
	}

	return price, nil
}

// getCoingeckoPrice fetches the current price of the Hydra cryptocurrency from the CoinGecko API.
// It takes a timeout as input to wait for the request, in minutes.
// It returns a big.Int representing the average price for the previous day.
func getCoingeckoPrice(timeoutInMinutes int, apiKey string) (*big.Int, error) {
	yesterday := getYesterdayFormatted()
	apiURL := fmt.Sprintf(`https://api.coingecko.com/api/v3/coins/hydra/history?date=%s`, yesterday)

	req, err := generateThirdPartyJSONRequest(apiURL, timeoutInMinutes)
	if err != nil {
		return nil, err
	}

	// Add the key in the header
	req.Header.Add("x-cg-demo-api-key", apiKey)

	// price := float64(0)
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

// getCMCPrice fetches the current price of the Hydra cryptocurrency from the CoinMarketCap API.
// It takes a timeout as input to wait for the request, in minutes.
// It returns a big.Int representing the current price.
func getCMCPrice(timeoutInMinutes int, apiKey string) (*big.Int, error) {
	apiURL := "https://pro-api.coinmarketcap.com/v2/cryptocurrency/quotes/latest"

	req, err := generateThirdPartyJSONRequest(apiURL, timeoutInMinutes)
	if err != nil {
		return nil, err
	}

	q := url.Values{}
	q.Add("slug", "hydra")
	q.Add("convert", "USD")

	req.Header.Add("X-CMC_PRO_API_KEY", apiKey)
	req.URL.RawQuery = q.Encode()

	price := float64(0)
	body, err := fetchPriceData(req)
	if err != nil {
		return nil, err
	}

	var priceData PriceDataCoinMarketCap
	err = json.Unmarshal(body, &priceData)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	for _, data := range priceData.Data {
		price = data.Quote.USD.Price
	}

	return common.ConvertFloatToBigInt(price, 18)
}

// generateThirdPartyJSONRequest creates a new HTTP request with a context that has a timeout
// for fetching data from a third-party API. The request is configured to accept JSON responses.
// It takes url which is the URL of the third-party API endpoint and timeoutInMinutes
// which is the duration in minutes for the request.
// Returns: The created HTTP request and any error that occurred.
func generateThirdPartyJSONRequest(url string, timeoutInMinutes int) (*http.Request, error) {
	// Discard the cancel function, because the context will be cleaned later
	ctx, _ := context.WithTimeout(
		context.Background(),
		time.Duration(timeoutInMinutes)*time.Minute,
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
	httpClient := &http.Client{}

	res, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	// Close the request when finish, too
	defer req.Context().Done()
	defer res.Body.Close()
	data, _ := io.ReadAll(res.Body)

	return data, nil
}

// getPreviousDayTimestamps returns the start and end timestamps for the previous day.
// It takes the current time as input and calculates the start and end timestamps
// for the previous day (00:00:00 to 23:59:59 of the previous day).
// vito - delete if not used
func getPreviousDayTimestamps(currentTime time.Time) (uint64, uint64) {
	// Get the previous day
	previousDay := currentTime.AddDate(0, 0, -1)

	// Set to start of the previous day (00:00:00)
	startOfPreviousDay := time.Date(
		previousDay.Year(),
		previousDay.Month(),
		previousDay.Day(),
		0,
		0,
		0,
		0,
		previousDay.Location(),
	)

	// Set to end of the previous day (23:59:59)
	endOfPreviousDay := startOfPreviousDay.Add(24*time.Hour - time.Second)

	// Convert back to uint64 timestamps
	startTimestamp := uint64(startOfPreviousDay.Unix())
	endTimestamp := uint64(endOfPreviousDay.Unix())

	return startTimestamp, endTimestamp
}

// getYesterdayFormatted returns the date in the format dd-mm-yyyy for the previous day.
func getYesterdayFormatted() string {
	yesterday := time.Now().UTC().AddDate(0, 0, -1)

	return yesterday.Format("02-01-2006")
}
