package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// Configuration parameters
const (
	MinVolumeUSD  = 1000000.0 // Minimum 24h volume in USD
	MinSpreadPct  = 0.5       // Minimum spread percentage
	TopPairsCount = 10        // Number of top pairs to show in each category
)

// ScannerTickerResponse represents the response from the Kraken API ticker endpoint
type ScannerTickerResponse struct {
	Error  []string                       `json:"error"`
	Result map[string]ScannerTickerResult `json:"result"`
}

// ScannerTickerResult represents the ticker data for a specific trading pair
type ScannerTickerResult struct {
	Ask  []string `json:"a"` // Ask price and volume
	Bid  []string `json:"b"` // Bid price and volume
	High []string `json:"h"` // High price
	Low  []string `json:"l"` // Low price
	Vol  []string `json:"v"` // Volume
}

// TradingPair represents a trading pair with its metrics
type TradingPair struct {
	Pair      string
	AskPrice  float64
	BidPrice  float64
	Spread    float64
	SpreadPct float64
	Volume24h float64
	VolumeUSD float64
}

// makePublicRequest makes a request to Kraken's public API endpoints
func makePublicRequest(url string, method string) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	req.Header.Add("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response: %v", err)
	}

	return body, nil
}

func scanPairs() {
	// Get all trading pairs
	url := "https://api.kraken.com/0/public/Ticker"
	body, err := makePublicRequest(url, "GET")
	if err != nil {
		fmt.Printf("Error getting ticker data: %v\n", err)
		return
	}

	var response ScannerTickerResponse
	if err := json.Unmarshal(body, &response); err != nil {
		fmt.Printf("Error parsing ticker response: %v\n", err)
		return
	}

	if len(response.Error) > 0 {
		fmt.Printf("API error: %v\n", response.Error)
		return
	}

	// Process each trading pair
	var pairs []TradingPair
	for pair, data := range response.Result {
		// Skip pairs that don't have USD as quote currency
		if !strings.HasSuffix(pair, "USD") {
			continue
		}

		// Parse prices and volume
		askPrice, _ := strconv.ParseFloat(data.Ask[0], 64)
		bidPrice, _ := strconv.ParseFloat(data.Bid[0], 64)
		volume24h, _ := strconv.ParseFloat(data.Vol[1], 64) // 24h volume

		// Calculate metrics
		spread := askPrice - bidPrice
		spreadPct := (spread / bidPrice) * 100
		volumeUSD := volume24h * bidPrice // Approximate USD volume

		pairs = append(pairs, TradingPair{
			Pair:      pair,
			AskPrice:  askPrice,
			BidPrice:  bidPrice,
			Spread:    spread,
			SpreadPct: spreadPct,
			Volume24h: volume24h,
			VolumeUSD: volumeUSD,
		})
	}

	// Sort by spread percentage (descending)
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].SpreadPct > pairs[j].SpreadPct
	})

	// Print top 10 pairs with highest spread
	fmt.Println("\nTop 10 Trading Pairs by Spread Percentage:")
	fmt.Println("=========================================")
	fmt.Printf("%-10s %-12s %-12s %-12s %-12s\n", "Pair", "Spread %", "Spread $", "24h Vol", "USD Vol")
	fmt.Println("-----------------------------------------")

	for _, pair := range pairs[:10] {
		fmt.Printf("%-10s %-12.4f %-12.4f %-12.2f %-12.2f\n",
			pair.Pair,
			pair.SpreadPct,
			pair.Spread,
			pair.Volume24h,
			pair.VolumeUSD)
	}

	// Sort by USD volume (descending)
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].VolumeUSD > pairs[j].VolumeUSD
	})

	// Print top 10 pairs with highest volume
	fmt.Println("\nTop 10 Trading Pairs by USD Volume:")
	fmt.Println("===================================")
	fmt.Printf("%-10s %-12s %-12s %-12s %-12s\n", "Pair", "Spread %", "Spread $", "24h Vol", "USD Vol")
	fmt.Println("-----------------------------------")

	for _, pair := range pairs[:10] {
		fmt.Printf("%-10s %-12.4f %-12.4f %-12.2f %-12.2f\n",
			pair.Pair,
			pair.SpreadPct,
			pair.Spread,
			pair.Volume24h,
			pair.VolumeUSD)
	}

	// Find pairs with both high volume and high spread
	fmt.Printf("\nPairs with High Volume (>$%.0f) and High Spread (>%.1f%%):\n", MinVolumeUSD, MinSpreadPct)
	fmt.Println("===================================================")
	fmt.Printf("%-10s %-12s %-12s %-12s %-12s\n", "Pair", "Spread %", "Spread $", "24h Vol", "USD Vol")
	fmt.Println("---------------------------------------------------")

	for _, pair := range pairs {
		if pair.VolumeUSD > MinVolumeUSD && pair.SpreadPct > MinSpreadPct {
			fmt.Printf("%-10s %-12.4f %-12.4f %-12.2f %-12.2f\n",
				pair.Pair,
				pair.SpreadPct,
				pair.Spread,
				pair.Volume24h,
				pair.VolumeUSD)
		}
	}
}

func main() {
	fmt.Printf("Scanning for trading pairs with:\n")
	fmt.Printf("- Minimum 24h volume: $%.0f USD\n", MinVolumeUSD)
	fmt.Printf("- Minimum spread: %.1f%%\n", MinSpreadPct)
	fmt.Printf("- Showing top %d pairs in each category\n\n", TopPairsCount)

	scanPairs()
}
