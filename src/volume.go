package main

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// VolumeResponse represents the response from the Kraken API ticker endpoint for volume
type VolumeResponse struct {
	Error  []string                `json:"error"`
	Result map[string]VolumeResult `json:"result"`
}

// VolumeResult represents the volume data for a specific trading pair
// According to Kraken API documentation:
// - Vol[0]: Today's 24h volume
// - Vol[1]: Last 24h volume
type VolumeResult struct {
	Vol []string `json:"v"` // Volume array
	Bid []string `json:"b"` // Bid price
}

// Get24hVolume returns the 24-hour trading volume in USD for a given coin
// It uses the last 24h volume (Vol[1]) from Kraken's ticker API and multiplies it by the bid price
func Get24hVolume(coin string) (float64, error) {
	// Convert coin to Kraken pair format (e.g., "SUNDOG" -> "SUNDOG/USD")
	pair := coin + "/USD"

	// Get ticker data from public API
	url := fmt.Sprintf("https://api.kraken.com/0/public/Ticker?pair=%s", pair)

	body, err := makePublicRequest(url, "GET")
	if err != nil {
		return 0, fmt.Errorf("error making request: %v", err)
	}

	// Parse response
	var response VolumeResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return 0, fmt.Errorf("error parsing response: %v", err)
	}

	if len(response.Error) > 0 {
		return 0, fmt.Errorf("API error: %v", response.Error)
	}

	// Get volume for the pair
	result, exists := response.Result[pair]
	if !exists {
		return 0, fmt.Errorf("pair %s not found in response", pair)
	}

	if len(result.Vol) < 2 {
		return 0, fmt.Errorf("insufficient volume data for pair %s", pair)
	}

	if len(result.Bid) < 1 {
		return 0, fmt.Errorf("insufficient bid price data for pair %s", pair)
	}

	// Convert last 24h volume string to float64
	coinVolume, err := strconv.ParseFloat(result.Vol[1], 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing volume: %v", err)
	}

	// Get bid price
	bidPrice, err := strconv.ParseFloat(result.Bid[0], 64)
	if err != nil {
		return 0, fmt.Errorf("error parsing bid price: %v", err)
	}

	// Calculate USD volume using bid price
	usdVolume := coinVolume * bidPrice

	return usdVolume, nil
}
