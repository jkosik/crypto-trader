package kraken

import (
	"encoding/json"
	"fmt"
	"math"
)

// CalculateADX calculates the Average Directional Index (ADX) for a given trading pair
// ADX measures trend strength: 0-20 = weak/no trend, 20-40 = strong trend, 40+ = very strong trend
// Returns ADX value and error if any
func CalculateADX(baseCoin, quoteCoin string, period int) (float64, error) {
	// ADX requires at least 2 * period + 1 candles for accurate calculation
	candlesNeeded := 2*period + 15 // Extra buffer for smoothing

	// Get OHLC data from Kraken (using 1-minute candles)
	pair := baseCoin + "/" + quoteCoin
	url := fmt.Sprintf("https://api.kraken.com/0/public/OHLC?pair=%s&interval=1", pair)

	body, err := MakePublicRequest(url, "GET")
	if err != nil {
		return 0, fmt.Errorf("error getting OHLC data for ADX: %v", err)
	}

	var response OHLCResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return 0, fmt.Errorf("error parsing OHLC response: %v", err)
	}

	if len(response.Error) > 0 {
		return 0, fmt.Errorf("API error: %v", response.Error)
	}

	// Extract OHLC data array
	var ohlcDataRaw []interface{}
	for _, data := range response.Result {
		if dataArray, ok := data.([]interface{}); ok {
			ohlcDataRaw = dataArray
			break
		}
	}

	if len(ohlcDataRaw) < candlesNeeded {
		return 0, fmt.Errorf("insufficient OHLC data for ADX: got %d candles, need at least %d", len(ohlcDataRaw), candlesNeeded)
	}

	// Parse OHLC data
	var ohlcData []OHLCData
	// Use the most recent candles
	startIdx := len(ohlcDataRaw) - candlesNeeded
	for i := startIdx; i < len(ohlcDataRaw); i++ {
		parsed, err := parseOHLCData(ohlcDataRaw[i])
		if err != nil {
			return 0, fmt.Errorf("error parsing OHLC data: %v", err)
		}
		ohlcData = append(ohlcData, parsed)
	}

	// Calculate ADX
	adx := calculateADXFromOHLC(ohlcData, period)

	return adx, nil
}

// calculateADXFromOHLC performs the actual ADX calculation
func calculateADXFromOHLC(data []OHLCData, period int) float64 {
	if len(data) < 2*period+1 {
		return 0
	}

	// Step 1: Calculate +DM, -DM, and TR
	var plusDM, minusDM, tr []float64

	for i := 1; i < len(data); i++ {
		// Calculate directional movements
		highDiff := data[i].High - data[i-1].High
		lowDiff := data[i-1].Low - data[i].Low

		// +DM
		if highDiff > lowDiff && highDiff > 0 {
			plusDM = append(plusDM, highDiff)
		} else {
			plusDM = append(plusDM, 0)
		}

		// -DM
		if lowDiff > highDiff && lowDiff > 0 {
			minusDM = append(minusDM, lowDiff)
		} else {
			minusDM = append(minusDM, 0)
		}

		// True Range
		tr1 := data[i].High - data[i].Low
		tr2 := math.Abs(data[i].High - data[i-1].Close)
		tr3 := math.Abs(data[i].Low - data[i-1].Close)
		tr = append(tr, math.Max(tr1, math.Max(tr2, tr3)))
	}

	// Step 2: Smooth +DM, -DM, and TR using Wilder's smoothing (like EMA)
	smoothPlusDM := wilderSmooth(plusDM, period)
	smoothMinusDM := wilderSmooth(minusDM, period)
	smoothTR := wilderSmooth(tr, period)

	// Step 3: Calculate +DI and -DI
	var plusDI, minusDI, dx []float64
	for i := 0; i < len(smoothPlusDM); i++ {
		if smoothTR[i] == 0 {
			plusDI = append(plusDI, 0)
			minusDI = append(minusDI, 0)
			dx = append(dx, 0)
			continue
		}

		pdi := (smoothPlusDM[i] / smoothTR[i]) * 100
		mdi := (smoothMinusDM[i] / smoothTR[i]) * 100
		plusDI = append(plusDI, pdi)
		minusDI = append(minusDI, mdi)

		// Step 4: Calculate DX
		diSum := pdi + mdi
		if diSum == 0 {
			dx = append(dx, 0)
		} else {
			dx = append(dx, (math.Abs(pdi-mdi)/diSum)*100)
		}
	}

	// Step 5: Calculate ADX (smoothed DX)
	if len(dx) < period {
		return 0
	}

	adxValues := wilderSmooth(dx, period)
	if len(adxValues) == 0 {
		return 0
	}

	// Return the most recent ADX value
	return adxValues[len(adxValues)-1]
}

// wilderSmooth applies Wilder's smoothing method (similar to EMA with alpha = 1/period)
func wilderSmooth(data []float64, period int) []float64 {
	if len(data) < period {
		return []float64{}
	}

	var smoothed []float64

	// Calculate first smoothed value (simple average of first 'period' values)
	sum := 0.0
	for i := 0; i < period; i++ {
		sum += data[i]
	}
	firstSmooth := sum / float64(period)
	smoothed = append(smoothed, firstSmooth)

	// Apply Wilder's smoothing for the rest
	for i := period; i < len(data); i++ {
		nextSmooth := (smoothed[len(smoothed)-1]*(float64(period)-1) + data[i]) / float64(period)
		smoothed = append(smoothed, nextSmooth)
	}

	return smoothed
}

// GetADXInfo retrieves and prints ADX information for a given trading pair
func GetADXInfo(baseCoin, quoteCoin string, period int) (float64, error) {
	adx, err := CalculateADX(baseCoin, quoteCoin, period)
	if err != nil {
		return 0, fmt.Errorf("error calculating ADX: %v", err)
	}

	// Interpret ADX value
	var trendStrength string
	if adx < 20 {
		trendStrength = "Weak/No Trend (Good for spread trading)"
	} else if adx < 25 {
		trendStrength = "Developing Trend"
	} else if adx < 40 {
		trendStrength = "Strong Trend"
	} else if adx < 50 {
		trendStrength = "Very Strong Trend"
	} else {
		trendStrength = "Extremely Strong Trend"
	}

	fmt.Printf("\n%s/%s ADX(%d) Indicator:\n", baseCoin, quoteCoin, period)
	fmt.Printf("ADX Value: %.2f\n", adx)
	fmt.Printf("Interpretation: %s\n", trendStrength)

	return adx, nil
}
