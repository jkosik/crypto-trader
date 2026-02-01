package kraken

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

// OrderResponse represents the Kraken API response for order placement
type OrderResponse struct {
	Error  []string `json:"error"`
	Result struct {
		Description struct {
			Order string `json:"order"`
		} `json:"descr"`
		TransactionIds []string `json:"txid"`
	} `json:"result"`
}

// OrderStatus represents the status of an order
// Possible status values:
// - open: Order is open and active
// - closed: Order has been closed
// - canceled: Order has been canceled
// - expired: Order has expired
// - pending: Order is pending
// - rejected: Order was rejected
// - partial: Order was partially filled
type OrderStatus struct {
	Status string `json:"status"`
	Descr  struct {
		Order string `json:"order"`
		Type  string `json:"type"`
		Price string `json:"price"`
		Pair  string `json:"pair"`
	} `json:"descr"`
	Vol     string `json:"vol"`
	VolExec string `json:"vol_exec"`
	Cost    string `json:"cost"`
	Fee     string `json:"fee"`
}

// OpenOrdersResponse represents the response from the Kraken API for open orders
type OpenOrdersResponse struct {
	Error  []string `json:"error"`
	Result struct {
		Open map[string]OrderStatus `json:"open"`
	} `json:"result"`
}

// PlaceLimitOrder places a limit order on Kraken
func PlaceLimitOrder(baseCoin, quoteCoin string, price float64, volume float64, isBuy bool, untradeable bool, decimals int) (string, error) {
	urlBase := "https://api.kraken.com"
	urlPath := "/0/private/AddOrder"

	// Create nonce
	nonce := time.Now().UnixNano() / int64(time.Millisecond)

	// Determine order type
	orderType := "sell"
	if isBuy {
		orderType = "buy"
	}

	// In untradeable mode, use extreme prices to prevent order filling. Estimated profit still shows the spread size.
	if untradeable {
		multiplier := math.Pow10(decimals)
		if isBuy {
			fmt.Printf("\nOriginal buy price: %.6f", price)
			price = price * 0.1 // 90% below market for buy orders
			// Round to maintain decimal limit
			price = math.Round(price*multiplier) / multiplier
			fmt.Printf("\nSetting untradeable buy price: %.*f\n", decimals, price)
		} else {
			fmt.Printf("\nOriginal sell price: %.6f", price)
			price = price * 10.0 // 900% above market for sell orders
			// Round to maintain decimal limit
			price = math.Round(price*multiplier) / multiplier
			fmt.Printf("\nSetting untradeable sell price: %.*f\n", decimals, price)
		}
	}

	// Create payload
	payload := fmt.Sprintf(`{
		"nonce": "%d",
		"ordertype": "limit",
		"type": "%s",
		"pair": "%s/%s",
		"price": %.6f,
		"volume": "%.5f"
	}`, nonce, orderType, baseCoin, quoteCoin, price, volume)

	// Debug: Print the payload
	// fmt.Printf("[DEBUG] Payload: %s\n", payload)

	// Get signature for the request
	signature, err := GetKrakenSignature(urlPath, payload, os.Getenv("KRAKEN_PRIVATE_KEY"))
	if err != nil {
		return "", fmt.Errorf("error generating signature: %v", err)
	}

	// Make request
	body, err := MakePrivateRequest(urlBase+urlPath, "POST", payload, os.Getenv("KRAKEN_API_KEY"), signature)
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
	}

	// Parse response
	var response OrderResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("error parsing response: %v", err)
	}

	if len(response.Error) > 0 {
		return "", fmt.Errorf("API error: %v", response.Error)
	}

	if len(response.Result.TransactionIds) == 0 {
		return "", fmt.Errorf("no transaction ID returned")
	}

	// Print order details
	fmt.Printf("\nPlaced %s order:\n", orderType)
	fmt.Printf("Price: %.6f\n", price)
	fmt.Printf("Volume: %.5f\n", volume)
	fmt.Printf("Order description: %s\n", response.Result.Description.Order)
	if untradeable {
		fmt.Println("UNTRADEABLE: Order placed with extreme price to prevent filling")
	}

	return response.Result.TransactionIds[0], nil
}

// PlaceSpreadOrders places a spread of buy and sell orders
// spreadAdjustFactor controls the spread size:
// - 0.0 = no spread (orders at center price, no profit potential)
// - 0.5 = half of the original spread
// - 1.0 = full original spread (use market bid/ask)
// - 2.0 = double the original spread
// - 3.0 = triple the original spread
// Values > 1.0 extend the spread beyond market prices
func PlaceSpreadOrders(baseCoin, quoteCoin string, spreadInfo *SpreadInfo, volume float64, untradeable bool, spreadAdjustFactor float64, adx float64, adxPeriod int) (string, string, float64, float64, error) {
	// Ensure spreadAdjustFactor is non-negative
	if spreadAdjustFactor < 0 {
		spreadAdjustFactor = 0
	}

	fmt.Printf("\nBid price: %.6f\n", spreadInfo.BidPrice)
	fmt.Printf("Ask price: %.6f\n", spreadInfo.AskPrice)

	// Calculate the center price and half spread
	centerPrice := (spreadInfo.AskPrice + spreadInfo.BidPrice) / 2
	halfSpread := (spreadInfo.AskPrice - spreadInfo.BidPrice) / 2

	// Check decimal places in both bid and ask prices
	bidStr := strconv.FormatFloat(spreadInfo.BidPrice, 'f', -1, 64)
	askStr := strconv.FormatFloat(spreadInfo.AskPrice, 'f', -1, 64)

	bidDecimals := 0
	if idx := strings.Index(bidStr, "."); idx != -1 {
		bidDecimals = len(bidStr) - idx - 1
	}

	askDecimals := 0
	if idx := strings.Index(askStr, "."); idx != -1 {
		askDecimals = len(askStr) - idx - 1
	}

	// Use the higher number of decimals
	decimals := max(bidDecimals, askDecimals)

	fmt.Printf("\nBid: %s (%d decimals)\n", bidStr, bidDecimals)
	fmt.Printf("Ask: %s (%d decimals)\n", askStr, askDecimals)
	fmt.Printf("Using %d decimal places\n", decimals)

	// Check if spreadAdjustFactor will produce meaningful results with this decimal precision
	// Minimum representable change is 1 unit at the decimal place (e.g., 0.00001 for 5 decimals)
	minPrecision := math.Pow10(-decimals)
	adjustedHalfSpread := halfSpread * spreadAdjustFactor
	
	// For the adjustment to be meaningful, it should be at least 0.5 × minPrecision
	// (so after rounding, there's at least a 1-unit change from center)
	minMeaningfulAdjustment := 0.5 * minPrecision
	
	if adjustedHalfSpread < minMeaningfulAdjustment && spreadAdjustFactor > 0 {
		fmt.Printf("\n⚠️  WARNING: spreadAdjustFactor %.2f may not produce meaningful results\n", spreadAdjustFactor)
		fmt.Printf("   Half spread: %.8f, Adjusted: %.8f, Min precision: %.8f\n", halfSpread, adjustedHalfSpread, minPrecision)
		fmt.Printf("   Prices will likely round to center (%.6f) or market prices\n", centerPrice)
		
		// Suggest minimum working factor
		minWorkingFactor := minMeaningfulAdjustment / halfSpread
		if minWorkingFactor < 1.0 {
			fmt.Printf("   Suggested minimum factor for narrowing: %.1f (or use 1.0 for full spread)\n\n", math.Ceil(minWorkingFactor*10)/10)
		} else {
			fmt.Printf("   Suggested: Use factor 1.0 (full spread) or higher\n\n")
		}
	}

	// Calculate new buy and sell prices based on the adjust factor
	// newBuyPrice = center - (halfSpread * factor)
	// newSellPrice = center + (halfSpread * factor)
	newBuyPrice := centerPrice - (halfSpread * spreadAdjustFactor)
	newSellPrice := centerPrice + (halfSpread * spreadAdjustFactor)

	// Round to detected decimal places
	multiplier := math.Pow10(decimals)
	newBuyPrice = math.Round(newBuyPrice*multiplier) / multiplier
	newSellPrice = math.Round(newSellPrice*multiplier) / multiplier

	// Warn if adjusted prices equal market prices due to rounding
	if newBuyPrice == spreadInfo.BidPrice && newSellPrice == spreadInfo.AskPrice && spreadAdjustFactor != 1.0 {
		marketSpreadPercent := (spreadInfo.Spread / centerPrice) * 100
		intendedSpreadPercent := marketSpreadPercent * spreadAdjustFactor
		
		if spreadAdjustFactor < 1.0 {
			// Narrowing failed - got full spread instead of narrower
			fmt.Printf("\nℹ️  INFO: Adjusted prices rounded to market prices (narrowing ineffective)\n")
			fmt.Printf("   Intended:  Buy %.6f, Sell %.6f (factor %.2f)\n", 
				centerPrice-(halfSpread*spreadAdjustFactor), 
				centerPrice+(halfSpread*spreadAdjustFactor), 
				spreadAdjustFactor)
			fmt.Printf("   After rounding to %d decimals: Buy %.6f, Sell %.6f (factor 1.0)\n", decimals, newBuyPrice, newSellPrice)
			fmt.Printf("   Result: MORE profit than intended (%.4f%% vs %.4f%%), no risk\n\n", 
				marketSpreadPercent, 
				intendedSpreadPercent)
		} else {
			// Extension failed - got market spread instead of wider
			fmt.Printf("\n⚠️  WARNING: Adjusted prices rounded to market prices (extension ineffective)\n")
			fmt.Printf("   Intended:  Buy %.6f, Sell %.6f (factor %.2f)\n", 
				centerPrice-(halfSpread*spreadAdjustFactor), 
				centerPrice+(halfSpread*spreadAdjustFactor), 
				spreadAdjustFactor)
			fmt.Printf("   After rounding to %d decimals: Buy %.6f, Sell %.6f (factor 1.0)\n", decimals, newBuyPrice, newSellPrice)
			fmt.Printf("   Result: LESS profit than intended (%.4f%% vs %.4f%%)\n", 
				marketSpreadPercent, 
				intendedSpreadPercent)
			fmt.Printf("   For %.0fx spread, use factor %.1f or higher\n\n", 
				math.Ceil(spreadAdjustFactor), 
				math.Ceil(spreadAdjustFactor*2)/2)
		}
	}

	// Check if adjusted prices are too close or equal (only happens with factor = 0)
	if newSellPrice <= newBuyPrice {
		// Send Slack notification about the error
		slackErr := SendSlackMessage(fmt.Sprintf(
			"❌ Trade %s/%s cancelled\n"+
				"Reason: Adjusted prices are too close (buy: %.6f, sell: %.6f)\n"+
				"spreadAdjustFactor: %.2f\n",
			baseCoin,
			quoteCoin,
			newBuyPrice,
			newSellPrice,
			spreadAdjustFactor,
		))
		if slackErr != nil {
			fmt.Printf("Warning: Failed to send Slack notification: %v\n", slackErr)
		}

		return "", "", 0, 0, fmt.Errorf("adjusted prices are too close or equal (buy: %.6f, sell: %.6f) with spreadAdjustFactor %.2f", newBuyPrice, newSellPrice, spreadAdjustFactor)
	}

	// Calculate estimated profit based on the new prices
	estimatedProfit := (newSellPrice - newBuyPrice) * volume

	// Calculate estimated percent gain based on the buy price
	estimatedPercentGain := ((newSellPrice - newBuyPrice) / newBuyPrice) * 100

	// Calculate the new spread
	newSpread := newSellPrice - newBuyPrice

	// Print spread information
	fmt.Printf("\n🔄 Placing spread orders for %s/%s:\n", baseCoin, quoteCoin)
	fmt.Printf("Volume: %.5f\n", volume)
	fmt.Printf("Market bid price: %.6f\n", spreadInfo.BidPrice)
	fmt.Printf("Market ask price: %.6f\n", spreadInfo.AskPrice)
	fmt.Printf("Market spread: %.6f (%.4f%%)\n", spreadInfo.Spread, (spreadInfo.Spread/spreadInfo.BidPrice)*100)
	fmt.Printf("ADX(%d) at entry: %.2f\n", adxPeriod, adx)
	fmt.Printf("Spread adjust factor: %.2f\n", spreadAdjustFactor)
	fmt.Printf("Center price: %.6f\n", centerPrice)
	fmt.Printf("Adjusted buy price: %.6f\n", newBuyPrice)
	fmt.Printf("Adjusted sell price: %.6f\n", newSellPrice)
	fmt.Printf("Adjusted spread: %.6f (%.4f%% of market spread)\n", newSpread, (newSpread/spreadInfo.Spread)*100)
	fmt.Printf("Estimated profit: %.8f %s (%.4f%% gain)\n", estimatedProfit, quoteCoin, estimatedPercentGain)

	// Place buy order at the new buy price
	buyTxId, err := PlaceLimitOrder(baseCoin, quoteCoin, newBuyPrice, volume, true, untradeable, decimals)
	if err != nil {
		return "", "", 0, 0, fmt.Errorf("error placing buy order: %v", err)
	}

	// Place sell order at the new sell price
	sellTxId, err := PlaceLimitOrder(baseCoin, quoteCoin, newSellPrice, volume, false, untradeable, decimals)
	if err != nil {
		return "", "", 0, 0, fmt.Errorf("error placing sell order: %v", err)
	}

	fmt.Printf("\nOrders placed successfully:\n")
	fmt.Printf("Buy Order ID: %s\n", buyTxId)
	fmt.Printf("Sell Order ID: %s\n", sellTxId)

	// Send Slack notification about placed orders
	slackErr := SendSlackMessage(fmt.Sprintf(
		"🔄 Placing spread orders for %s/%s\n"+
			"Volume: %.5f\n"+
			"Market bid price: %.6f\n"+
			"Market ask price: %.6f\n"+
			"Market spread: %.6f (%.4f%%)\n"+
			"ADX(%d) at entry: %.2f\n"+
			"Spread adjust factor: %.2f\n"+
			"Center price: %.6f\n"+
			"Adjusted buy price: %.6f\n"+
			"Adjusted sell price: %.6f\n"+
			"Adjusted spread: %.6f (%.2f%% of market)\n"+
			"Estimated profit: %.8f %s (%.4f%% gain)\n"+
			"Buy Order ID: %s\n"+
			"Sell Order ID: %s",
		baseCoin,
		quoteCoin,
		volume,
		spreadInfo.BidPrice,
		spreadInfo.AskPrice,
		spreadInfo.Spread,
		(spreadInfo.Spread/spreadInfo.BidPrice)*100,
		adxPeriod,
		adx,
		spreadAdjustFactor,
		centerPrice,
		newBuyPrice,
		newSellPrice,
		newSpread,
		(newSpread/spreadInfo.Spread)*100,
		estimatedProfit,
		quoteCoin,
		estimatedPercentGain,
		buyTxId,
		sellTxId,
	))
	if slackErr != nil {
		fmt.Printf("Warning: Failed to send Slack notification: %v\n", slackErr)
	}

	return buyTxId, sellTxId, estimatedProfit, estimatedPercentGain, nil
}

// CheckOrderStatus checks and prints the status of a transaction ID
func CheckOrderStatus(txId string) (*OrderStatus, error) {
	urlBase := "https://api.kraken.com"
	urlPath := "/0/private/QueryOrders"

	// Create nonce
	nonce := time.Now().UnixNano() / int64(time.Millisecond)

	// Create payload with transaction ID
	payload := fmt.Sprintf(`{
		"nonce": "%d",
		"txid": "%s"
	}`, nonce, txId)

	// Get signature for the request
	signature, err := GetKrakenSignature(urlPath, payload, os.Getenv("KRAKEN_PRIVATE_KEY"))
	if err != nil {
		return nil, fmt.Errorf("error generating signature: %v", err)
	}

	// Make request
	body, err := MakePrivateRequest(urlBase+urlPath, "POST", payload, os.Getenv("KRAKEN_API_KEY"), signature)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}

	// Parse response
	var response struct {
		Error  []string               `json:"error"`
		Result map[string]OrderStatus `json:"result"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("error parsing response: %v", err)
	}

	if len(response.Error) > 0 {
		return nil, fmt.Errorf("API error: %v", response.Error)
	}

	// Get order status
	order, exists := response.Result[txId]
	if !exists {
		return nil, fmt.Errorf("order not found")
	}

	// Check if order is successfully closed
	if order.Status == "closed" {
		fmt.Println("✅ TRADE SUCCESSFUL: Order has been fully executed")
	} else if order.Status == "partial" {
		fmt.Printf("⚠️ PARTIAL FILL: %.2f%% of the order has been executed\n",
			parseFloat(order.VolExec)/parseFloat(order.Vol)*100)
	} else if order.Status == "canceled" {
		fmt.Println("❌ TRADE CANCELED: Order was canceled")
	} else if order.Status == "rejected" {
		fmt.Println("❌ TRADE REJECTED: Order was rejected")
	} else if order.Status == "expired" {
		fmt.Println("❌ TRADE EXPIRED: Order has expired")
	} else if order.Status == "open" {
		fmt.Println("⏳ ORDER OPEN: Waiting for execution")
	}

	return &order, nil
}

// Helper function to parse float from string
func parseFloat(s string) float64 {
	f, _ := strconv.ParseFloat(s, 64)
	return f
}

// GetOpenOrders retrieves all open orders for a given trading pair
func GetOpenOrders(baseCoin, quoteCoin string) (map[string]OrderStatus, error) {
	urlBase := "https://api.kraken.com"
	urlPath := "/0/private/OpenOrders"

	// Create nonce
	nonce := time.Now().UnixNano() / int64(time.Millisecond)

	// Create payload
	payload := fmt.Sprintf(`{
		"nonce": "%d"
	}`, nonce)

	// Get signature for the request
	signature, err := GetKrakenSignature(urlPath, payload, os.Getenv("KRAKEN_PRIVATE_KEY"))
	if err != nil {
		return nil, fmt.Errorf("error generating signature: %v", err)
	}

	// Make request
	body, err := MakePrivateRequest(urlBase+urlPath, "POST", payload, os.Getenv("KRAKEN_API_KEY"), signature)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}

	// Debug: Print raw response body
	// fmt.Printf("\n[DEBUG] Raw API response body:\n%s\n", string(body))

	// Parse response
	var response OpenOrdersResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("error parsing response: %v", err)
	}

	if len(response.Error) > 0 {
		return nil, fmt.Errorf("API error: %v", response.Error)
	}

	// Debug: Print all orders before filtering
	// if len(response.Result.Open) == 0 {
	// 	fmt.Println("[DEBUG] No open orders found in the account")
	// } else {
	// 	fmt.Printf("[DEBUG] Found %d total open orders (of any pairs) in the account\n", len(response.Result.Open))
	// 	for txId, order := range response.Result.Open {
	// 		fmt.Printf("[DEBUG] Order %s: Status=%s, Description=%s, Type=%s, Price=%s, Volume=%s\n", txId, order.Status, order.Descr.Order, order.Descr.Type, order.Descr.Price, order.Vol)
	// 	}
	// }

	// Filter orders for the specific trading pair
	filteredOrders := make(map[string]OrderStatus)
	pair := baseCoin + quoteCoin
	for txId, order := range response.Result.Open {
		// Skip empty orders
		if order.Status == "" || order.Descr.Order == "" {
			// fmt.Printf("[DEBUG] Skipping empty order %s\n", txId)
			continue
		}
		// Check if the order description contains the pair
		if strings.Contains(order.Descr.Order, pair) {
			filteredOrders[txId] = order
			// fmt.Printf("[DEBUG] Found matching order %s: %s\n", txId, order.Descr.Order)
		} else {
			// fmt.Printf("[DEBUG] Order %s does not match pair %s: %s\n", txId, pair, order.Descr.Order)
		}
	}

	return filteredOrders, nil
}

// CancelOrder cancels a specific order by its transaction ID
func CancelOrder(txId string) error {
	urlBase := "https://api.kraken.com"
	urlPath := "/0/private/CancelOrder"

	// Create nonce
	nonce := time.Now().UnixNano() / int64(time.Millisecond)

	// Create payload
	payload := fmt.Sprintf(`{
		"nonce": "%d",
		"txid": "%s"
	}`, nonce, txId)

	// Get signature for the request
	signature, err := GetKrakenSignature(urlPath, payload, os.Getenv("KRAKEN_PRIVATE_KEY"))
	if err != nil {
		return fmt.Errorf("error generating signature: %v", err)
	}

	// Make request
	body, err := MakePrivateRequest(urlBase+urlPath, "POST", payload, os.Getenv("KRAKEN_API_KEY"), signature)
	if err != nil {
		return fmt.Errorf("error making request: %v", err)
	}

	// Parse response
	var response struct {
		Error  []string `json:"error"`
		Result struct {
			Count int `json:"count"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return fmt.Errorf("error parsing response: %v", err)
	}

	if len(response.Error) > 0 {
		return fmt.Errorf("API error: %v", response.Error)
	}

	if response.Result.Count == 0 {
		return fmt.Errorf("no orders were canceled")
	}

	return nil
}

// Helper functions for min/max
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
