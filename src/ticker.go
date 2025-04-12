package main

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// TickerResponse represents the response from the Kraken API ticker endpoint
type TickerResponse struct {
	Error  []string                `json:"error"`
	Result map[string]TickerResult `json:"result"`
}

// TickerResult represents the ticker data for a specific trading pair
type TickerResult struct {
	Ask  []string `json:"a"` // Ask price and volume
	Bid  []string `json:"b"` // Bid price and volume
	High []string `json:"h"` // High price
	Low  []string `json:"l"` // Low price
}

// TickerInfo represents the current ticker information for a trading pair
type TickerInfo struct {
	Pair      string
	AskPrice  float64
	BidPrice  float64
	Spread    float64
	HighPrice float64
	LowPrice  float64
}

// SpreadInfo contains the spread boundary information
type SpreadInfo struct {
	BidPrice  float64
	AskPrice  float64
	Spread    float64
	HighPrice float64
	LowPrice  float64
}

// Returns spread and daily price change
func Ticker(coin string) (*SpreadInfo, error) {
	// Convert coin to Kraken pair format (e.g., "SUNDOG" -> "SUNDOG/USD")
	pair := coin + "/USD"

	// Get ticker data from public API
	url := fmt.Sprintf("https://api.kraken.com/0/public/Ticker?pair=%s", pair)

	body, err := makePublicRequest(url, "GET")
	if err != nil {
		return nil, fmt.Errorf("error getting ticker data: %v", err)
	}

	var response TickerResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("error parsing ticker response: %v", err)
	}

	if len(response.Error) > 0 {
		return nil, fmt.Errorf("API error: %v", response.Error)
	}

	// Get the first (and only) pair from the result
	var pairData TickerResult
	for _, data := range response.Result {
		pairData = data
		break
	}

	if len(pairData.Bid) < 1 || len(pairData.Ask) < 1 || len(pairData.High) < 1 || len(pairData.Low) < 1 {
		return nil, fmt.Errorf("insufficient order book data")
	}

	// Parse bid and ask prices
	bidPrice, err := strconv.ParseFloat(pairData.Bid[0], 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing bid price: %v", err)
	}

	askPrice, err := strconv.ParseFloat(pairData.Ask[0], 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing ask price: %v", err)
	}

	highPrice, err := strconv.ParseFloat(pairData.High[0], 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing high price: %v", err)
	}

	lowPrice, err := strconv.ParseFloat(pairData.Low[0], 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing low price: %v", err)
	}

	spread := askPrice - bidPrice

	return &SpreadInfo{
		BidPrice:  bidPrice,
		AskPrice:  askPrice,
		Spread:    spread,
		HighPrice: highPrice,
		LowPrice:  lowPrice,
	}, nil
}

// PrintTickerInfo prints the spread information for a trading pair
func PrintTickerInfo(info *SpreadInfo, coin string) {
	spreadPercent := (info.Spread / info.BidPrice) * 100
	fmt.Printf("\n%s/USD Spread & High/LowInformation:\n", coin)
	fmt.Printf("Bid Price: %.8f\n", info.BidPrice)
	fmt.Printf("Ask Price: %.8f\n", info.AskPrice)
	fmt.Printf("Spread: %.8f (%.4f%%)\n", info.Spread, spreadPercent)
	fmt.Printf("24h High: %.8f\n", info.HighPrice)
	fmt.Printf("24h Low: %.8f\n", info.LowPrice)
}

// GetTickerInfo returns the current ticker information for the specified trading pair
func GetTickerInfo(coin string) (*SpreadInfo, error) {
	// Convert coin to Kraken pair format (e.g., "SUNDOG" -> "SUNDOG/USD")
	pair := coin + "/USD"

	// Get ticker data from public API
	url := fmt.Sprintf("https://api.kraken.com/0/public/Ticker?pair=%s", pair)

	body, err := makePublicRequest(url, "GET")
	if err != nil {
		return nil, fmt.Errorf("error getting ticker data: %v", err)
	}

	var response TickerResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("error parsing ticker response: %v", err)
	}

	if len(response.Error) > 0 {
		return nil, fmt.Errorf("API error: %v", response.Error)
	}

	// Get the first (and only) pair from the result
	var pairData TickerResult
	for _, data := range response.Result {
		pairData = data
		break
	}

	if len(pairData.Bid) < 1 || len(pairData.Ask) < 1 || len(pairData.High) < 1 || len(pairData.Low) < 1 {
		return nil, fmt.Errorf("insufficient order book data")
	}

	// Parse bid and ask prices
	bidPrice, err := strconv.ParseFloat(pairData.Bid[0], 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing bid price: %v", err)
	}

	askPrice, err := strconv.ParseFloat(pairData.Ask[0], 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing ask price: %v", err)
	}

	highPrice, err := strconv.ParseFloat(pairData.High[0], 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing high price: %v", err)
	}

	lowPrice, err := strconv.ParseFloat(pairData.Low[0], 64)
	if err != nil {
		return nil, fmt.Errorf("error parsing low price: %v", err)
	}

	spread := askPrice - bidPrice

	return &SpreadInfo{
		BidPrice:  bidPrice,
		AskPrice:  askPrice,
		Spread:    spread,
		HighPrice: highPrice,
		LowPrice:  lowPrice,
	}, nil
}
