package kraken

import (
	"encoding/json"
	"fmt"
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
func PlaceLimitOrder(coin string, price float64, volume float64, isBuy bool, untradeable bool) (string, error) {
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
		if isBuy {
			fmt.Printf("\nOriginal buy price: %.8f", price)
			price = price * 0.1 // 90% below market for buy orders
			fmt.Printf("\nSetting untradeable buy price: %.8f\n", price)
		} else {
			fmt.Printf("\nOriginal sell price: %.8f", price)
			price = price * 10.0 // 900% above market for sell orders
			fmt.Printf("\nSetting untradeable sell price: %.8f\n", price)
		}
	}

	// Create payload
	payload := fmt.Sprintf(`{
		"nonce": "%d",
		"ordertype": "limit",
		"type": "%s",
		"pair": "%s/USD",
		"price": %.8f,
		"volume": "%.5f"
	}`, nonce, orderType, coin, price, volume)

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
	fmt.Printf("Price: %.8f\n", price)
	fmt.Printf("Volume: %.5f\n", volume)
	fmt.Printf("Order description: %s\n", response.Result.Description.Order)
	if untradeable {
		fmt.Println("UNTRADEABLE: Order placed with extreme price to prevent filling")
	}

	return response.Result.TransactionIds[0], nil
}

// PlaceSpreadOrders places a spread of buy and sell orders
func PlaceSpreadOrders(coin string, spreadInfo *SpreadInfo, volume float64, untradeable bool) (string, string, float64, float64, error) {
	// Calculate estimated profit. Bid and ask prices are in USD and the differences is per one trading base coin unit.
	estimatedProfit := (spreadInfo.AskPrice - spreadInfo.BidPrice) * volume

	// Calculate estimated percent gain based on the buy price
	estimatedPercentGain := ((spreadInfo.AskPrice - spreadInfo.BidPrice) / spreadInfo.BidPrice) * 100

	// Place buy order at bid price
	buyTxId, err := PlaceLimitOrder(coin, spreadInfo.BidPrice, volume, true, untradeable)
	if err != nil {
		return "", "", 0, 0, fmt.Errorf("error placing buy order: %v", err)
	}

	// Place sell order at ask price
	sellTxId, err := PlaceLimitOrder(coin, spreadInfo.AskPrice, volume, false, untradeable)
	if err != nil {
		return "", "", 0, 0, fmt.Errorf("error placing sell order: %v", err)
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
func GetOpenOrders(coin string) (map[string]OrderStatus, error) {
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

	// Filter orders for the specific coin
	filteredOrders := make(map[string]OrderStatus)
	pair := coin + "USD"
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

// CancelAllOrders cancels all open orders for a given coin
func CancelAllOrders(coin string) error {
	// Get all open orders for the coin
	orders, err := GetOpenOrders(coin)
	if err != nil {
		return fmt.Errorf("error getting open orders: %v", err)
	}

	if len(orders) == 0 {
		return nil
	}

	// Print orders to be canceled
	fmt.Printf("\n[LOOP] Canceling %d orders for %s:\n", len(orders), coin)
	for txId, order := range orders {
		fmt.Printf("[LOOP] Canceling order %s: Type=%s, Price=%s, Volume=%s\n",
			txId, order.Descr.Type, order.Descr.Price, order.Vol)
	}

	// Cancel each order
	for txId := range orders {
		if err := CancelOrder(txId); err != nil {
			return fmt.Errorf("error canceling order %s: %v", txId, err)
		}
	}

	fmt.Printf("[LOOP] Successfully initiated cancellation of %d orders for %s\n", len(orders), coin)
	return nil
}

// PlaceMarketOrder places a market order on Kraken
func PlaceMarketOrder(coin string, volume float64, isBuy bool) (string, error) {
	urlBase := "https://api.kraken.com"
	urlPath := "/0/private/AddOrder"

	// Create nonce
	nonce := time.Now().UnixNano() / int64(time.Millisecond)

	// Determine order type
	orderType := "sell"
	if isBuy {
		orderType = "buy"
	}

	// Create payload
	payload := fmt.Sprintf(`{
		"nonce": "%d",
		"ordertype": "market",
		"type": "%s",
		"pair": "%s/USD",
		"volume": "%.5f"
	}`, nonce, orderType, coin, volume)

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
	fmt.Printf("\nPlaced market %s order:\n", orderType)
	fmt.Printf("Volume: %.5f\n", volume)
	fmt.Printf("Order description: %s\n", response.Result.Description.Order)

	return response.Result.TransactionIds[0], nil
}
