package kraken

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// GetUSDPrice fetches the current USD price for a given currency
// Returns 1.0 for USD itself, or the current exchange rate for other currencies
func GetUSDPrice(currency string) (float64, error) {
	// If already USD, return 1.0
	if currency == "USD" || currency == "ZUSD" {
		return 1.0, nil
	}

	// Fetch ticker for currency/USD pair
	pair := currency + "/USD"
	url := fmt.Sprintf("https://api.kraken.com/0/public/Ticker?pair=%s", pair)

	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("error fetching %s price: %v", pair, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("error reading response: %v", err)
	}

	var tickerResp struct {
		Error  []string `json:"error"`
		Result map[string]struct {
			C []string `json:"c"` // Last trade closed array [price, lot volume]
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &tickerResp); err != nil {
		return 0, fmt.Errorf("error parsing JSON: %v", err)
	}

	if len(tickerResp.Error) > 0 {
		return 0, fmt.Errorf("API error: %v", tickerResp.Error)
	}

	// Get the first (and should be only) result
	for _, ticker := range tickerResp.Result {
		if len(ticker.C) > 0 {
			var price float64
			if _, err := fmt.Sscanf(ticker.C[0], "%f", &price); err != nil {
				return 0, fmt.Errorf("error parsing price: %v", err)
			}
			return price, nil
		}
	}

	return 0, fmt.Errorf("no price data available for %s", pair)
}

// FormatProfitWithUSD formats profit to show both native currency and USD equivalent
// Example: "0.00000010 BTC ($0.01)" or "0.50 USD"
func FormatProfitWithUSD(profit float64, currency string) string {
	// If already USD, just show USD
	if currency == "USD" || currency == "ZUSD" {
		return fmt.Sprintf("$%.2f", profit)
	}

	// Get USD price for the currency
	usdPrice, err := GetUSDPrice(currency)
	if err != nil {
		// If we can't get USD price, just show the native currency
		return fmt.Sprintf("%.8f %s (USD price unavailable)", profit, currency)
	}

	profitUSD := profit * usdPrice
	return fmt.Sprintf("%.8f %s ($%.4f)", profit, currency, profitUSD)
}
