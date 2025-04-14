package main

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// OHLCResponse represents the response from the Kraken API OHLC endpoint
type OHLCResponse struct {
	Error  []string               `json:"error"`
	Result map[string]interface{} `json:"result"`
}

// OHLCData represents a single OHLC candle
type OHLCData struct {
	Time   int64
	Open   float64
	High   float64
	Low    float64
	Close  float64
	Volume float64
}

// GetOHLCData retrieves OHLC data for a given trading pair
func GetOHLCData(coin string, duration time.Duration) error {
	// Convert coin to Kraken pair format (e.g., "SUNDOG" -> "SUNDOG/USD")
	pair := coin + "/USD"

	// Limit duration to 8 hours
	if duration > 8*time.Hour {
		duration = 8 * time.Hour
		fmt.Printf("Note: Duration limited to 8 hours\n")
	}

	// Calculate number of candles needed (1 candle per minute)
	minutesNeeded := int(duration.Minutes())
	candlesNeeded := minutesNeeded + 1 // +1 for current candle

	// Get OHLC data from public API
	url := fmt.Sprintf("https://api.kraken.com/0/public/OHLC?pair=%s&interval=1", pair)

	body, err := makePublicRequest(url, "GET")
	if err != nil {
		return fmt.Errorf("error getting OHLC data: %v", err)
	}

	var response OHLCResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("error parsing OHLC response: %v", err)
	}

	if len(response.Error) > 0 {
		return fmt.Errorf("API error: %v", response.Error)
	}

	// Get the first (and only) pair from the result
	var ohlcData []interface{}
	for _, data := range response.Result {
		if dataArray, ok := data.([]interface{}); ok {
			ohlcData = dataArray
			break
		}
	}

	if len(ohlcData) < candlesNeeded {
		return fmt.Errorf("insufficient OHLC data: got %d candles, need at least %d", len(ohlcData), candlesNeeded)
	}

	// Get current and historical data
	currentData, err := parseOHLCData(ohlcData[len(ohlcData)-1])
	if err != nil {
		return fmt.Errorf("error parsing current OHLC data: %v", err)
	}

	oldData, err := parseOHLCData(ohlcData[len(ohlcData)-candlesNeeded])
	if err != nil {
		return fmt.Errorf("error parsing old OHLC data: %v", err)
	}

	// Calculate price change
	priceChange := ((currentData.Close - oldData.Close) / oldData.Close) * 100

	// Print the information
	fmt.Printf("\n%s/USD Price Change in imeframe %s (OHLC API):\n", coin, duration)
	fmt.Printf("Current Price: %.8f\n", currentData.Close)
	fmt.Printf("Price %s ago: %.8f\n", duration, oldData.Close)
	fmt.Printf("Price Change: %.2f%%\n", priceChange)
	fmt.Printf("Time: %s\n", time.Unix(currentData.Time, 0).Format(time.RFC3339))
	fmt.Printf("Time %s ago: %s\n", duration, time.Unix(oldData.Time, 0).Format(time.RFC3339))

	// Check if price change is significant (e.g., more than 5%)
	priceChangeThreshold := 5.0
	if priceChange > priceChangeThreshold {
		fmt.Printf("WARNING: Price increased by more than %.1f%% in the last %s\n", priceChangeThreshold, duration)
	} else if priceChange < -priceChangeThreshold {
		fmt.Printf("WARNING: Price decreased by more than %.1f%% in the last %s\n", priceChangeThreshold, duration)
	}

	return nil
}

// parseOHLCData converts raw OHLC data to structured format
func parseOHLCData(data interface{}) (OHLCData, error) {
	values, ok := data.([]interface{})
	if !ok {
		return OHLCData{}, fmt.Errorf("invalid data type: expected []interface{}, got %T", data)
	}

	if len(values) < 7 {
		return OHLCData{}, fmt.Errorf("insufficient data points: got %d, need 7", len(values))
	}

	// Parse time
	timeFloat, ok := values[0].(float64)
	if !ok {
		return OHLCData{}, fmt.Errorf("invalid time format: expected float64, got %T", values[0])
	}
	time := int64(timeFloat)

	// Parse OHLC values
	open, err := strconv.ParseFloat(values[1].(string), 64)
	if err != nil {
		return OHLCData{}, fmt.Errorf("error parsing open price: %v", err)
	}

	high, err := strconv.ParseFloat(values[2].(string), 64)
	if err != nil {
		return OHLCData{}, fmt.Errorf("error parsing high price: %v", err)
	}

	low, err := strconv.ParseFloat(values[3].(string), 64)
	if err != nil {
		return OHLCData{}, fmt.Errorf("error parsing low price: %v", err)
	}

	close, err := strconv.ParseFloat(values[4].(string), 64)
	if err != nil {
		return OHLCData{}, fmt.Errorf("error parsing close price: %v", err)
	}

	// Parse volume
	volume, err := strconv.ParseFloat(values[6].(string), 64)
	if err != nil {
		return OHLCData{}, fmt.Errorf("error parsing volume: %v", err)
	}

	return OHLCData{
		Time:   time,
		Open:   open,
		High:   high,
		Low:    low,
		Close:  close,
		Volume: volume,
	}, nil
}
