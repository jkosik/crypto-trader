package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// BalanceData represents the balance information for a coin
type BalanceData struct {
	Balance   string `json:"balance"`
	HoldTrade string `json:"hold_trade"`
}

func getKrakenSignature(urlPath string, payload string, secret string) (string, error) {
	// Parse the JSON payload
	var jsonData map[string]interface{}
	if err := json.Unmarshal([]byte(payload), &jsonData); err != nil {
		return "", fmt.Errorf("Failed to parse JSON payload: %v", err)
	}

	// Get nonce from the parsed JSON
	nonce, ok := jsonData["nonce"].(string)
	if !ok {
		return "", fmt.Errorf("Nonce not found in payload or not a string")
	}

	// Create the encoded data string
	encodedData := nonce + payload

	sha := sha256.New()
	sha.Write([]byte(encodedData))
	shaSum := sha.Sum(nil)

	message := append([]byte(urlPath), shaSum...)

	decodedSecret, err := base64.StdEncoding.DecodeString(secret)
	if err != nil {
		return "", fmt.Errorf("Failed to decode secret: %v", err)
	}

	mac := hmac.New(sha512.New, decodedSecret)
	mac.Write(message)
	macSum := mac.Sum(nil)
	sigDigest := base64.StdEncoding.EncodeToString(macSum)
	return sigDigest, nil
}

// Balance and Ticker API endpoints expect different asset codes. Conversion needed.
func krakenAssetCode(standardCode string) (string, error) {
	hardcodedMap := map[string]string{
		"BTC":    "XBT.F",
		"ETH":    "ETH",
		"SOL":    "SOL.F",
		"SUNDOG": "SUNDOG",
		"TRUMP":  "TRUMP",
		"GUN":    "GUN",
	}

	code, ok := hardcodedMap[strings.ToUpper(standardCode)]
	if !ok {
		return "", fmt.Errorf("unknown standard code: %s", standardCode)
	}
	return code, nil
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

// makePrivateRequest makes a request to Kraken's private API endpoints with auth
func makePrivateRequest(url string, method string, payload string, apiKey string, signature string) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, strings.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Add headers for private API
	req.Header.Add("API-Key", apiKey)
	req.Header.Add("API-Sign", signature)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("Content-Type", "application/json")

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

func getCoinBalance(body []byte, coin string) (BalanceData, error) {
	var response struct {
		Error  []string               `json:"error"`
		Result map[string]BalanceData `json:"result"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return BalanceData{}, fmt.Errorf("error parsing response: %v", err)
	}

	balanceData, exists := response.Result[coin]
	if !exists {
		return BalanceData{}, fmt.Errorf("balance for %s not found in response", coin)
	}

	return balanceData, nil
}

func main() {
	// Define command line flags
	baseCoin := flag.String("coin", "", "Base coin to trade (e.g. BTC, SOL)")
	orderFlag := flag.Bool("order", false, "Place actual orders (default: false)")
	untradeable := flag.Bool("untradeable", false, "Place orders at untradeable prices (orders won't be executed - close them manually)")
	volume := flag.Float64("volume", 0.0, "Base coin volume to trade")

	// Parse command line flags
	flag.Parse()

	// Check if required flags are set
	if *baseCoin == "" || *volume == 0.0 {
		fmt.Println("Error: -coin flag is required")
		fmt.Println("Usage: ./k-bot -coin <COIN> [-order] [-volume <AMOUNT>] [-untradeable]")
		fmt.Println("\nFlags:")
		fmt.Println("  -coin <COIN>    Base coin to trade (e.g. BTC, SOL)")
		fmt.Println("  -order         Place actual orders (default: false)")
		fmt.Println("  -untradeable   Place orders at untradeable prices (orders won't be executed - close them manually)")
		fmt.Println("  -volume <AMOUNT> Base coin volume to trade.")
		os.Exit(1)
	}

	// Get Kraken asset code for the selected coin
	baseCoinBalanceCode, err := krakenAssetCode(*baseCoin)
	if err != nil {
		fmt.Printf("Error getting Kraken asset code: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\nTrading %s/USD\n", *baseCoin)
	fmt.Println("Traded volume:", *volume)
	if *untradeable {
		fmt.Println("Running in untradeable mode (orders will be placed at extreme prices)")
	}

	// Grab env variables
	apiKey := os.Getenv("KRAKEN_API_KEY")
	apiSecret := os.Getenv("KRAKEN_PRIVATE_KEY")
	// Nonce is used for signature process
	nonce := time.Now().UnixNano() / int64(time.Millisecond)
	urlBase := "https://api.kraken.com"

	if apiKey == "" || apiSecret == "" {
		fmt.Println("Error: KRAKEN_API_KEY and KRAKEN_PRIVATE_KEY environment variables must be set")
		os.Exit(1)
	}

	// Get account balance
	urlPath := "/0/private/BalanceEx"
	payload := fmt.Sprintf(`{
		"nonce": "%d"
	}`, nonce)

	signature, err := getKrakenSignature(urlPath, payload, apiSecret)
	if err != nil {
		fmt.Println("Error generating signature:", err)
		os.Exit(1)
	}

	balanceBody, err := makePrivateRequest(urlBase+urlPath, "POST", payload, apiKey, signature)
	if err != nil {
		fmt.Println("Error making request:", err)
		os.Exit(1)
	}

	fmt.Println("Account balance:")
	fmt.Println(string(balanceBody))

	// Get spread boundary for base coin
	spreadInfo, err := GetTickerInfo(*baseCoin)
	if err != nil {
		fmt.Println("Error getting spread boundary:", err)
		os.Exit(1)
	}

	PrintTickerInfo(spreadInfo, *baseCoin)

	// Get OHLC data for price comparison. Hard cap on 8 hours
	if err := GetOHLCData(*baseCoin, 4*time.Hour); err != nil {
		fmt.Printf("Error getting OHLC data: %v\n", err)
	}

	// Check if we have sufficient balance and place the order
	// Check balance for the base coin
	balanceBase, err := getCoinBalance(balanceBody, baseCoinBalanceCode)
	if err != nil {
		fmt.Printf("Error getting %s balance: %v", baseCoinBalanceCode, err)
		os.Exit(1)
	}
	fmt.Printf("\nAvailable %s: %s\n", baseCoinBalanceCode, balanceBase.Balance)
	balanceBaseFloat, err := strconv.ParseFloat(balanceBase.Balance, 64)
	if err != nil {
		fmt.Println("Error converting string to float64:", err)
		os.Exit(1)
	}
	if balanceBaseFloat < *volume {
		fmt.Printf("\nInsufficient %s balance (have: %s, need: %.2f)\n", *baseCoin, balanceBase.Balance, *volume)
		os.Exit(1)
	}

	// Check balance for USD
	balanceUSDF, err := getCoinBalance(balanceBody, "USD.F")
	if err != nil {
		fmt.Println("Error getting USD.F balance:", err)
		os.Exit(1)
	}

	balanceZUSD, err := getCoinBalance(balanceBody, "ZUSD")
	if err != nil {
		fmt.Println("Error getting ZUSD balance:", err)
		os.Exit(1)
	}

	// Calculate true available USD (USD.F balance - ZUSD hold_trade)
	usdBalanceFloat, err := strconv.ParseFloat(balanceUSDF.Balance, 64)
	if err != nil {
		fmt.Println("Error converting USD.F balance:", err)
		os.Exit(1)
	}

	usdHoldTradeFloat, err := strconv.ParseFloat(balanceZUSD.HoldTrade, 64)
	if err != nil {
		fmt.Println("Error converting ZUSD hold_trade:", err)
		os.Exit(1)
	}

	availableUSD := usdBalanceFloat - usdHoldTradeFloat
	requiredUSD := *volume * spreadInfo.BidPrice

	// fmt.Printf("USD.F Balance: %s\n", balanceUSDF.Balance)
	// fmt.Printf("ZUSD Hold Trade: %s\n", balanceZUSD.HoldTrade)
	fmt.Printf("Available USD: %.2f\n", availableUSD)

	if availableUSD < requiredUSD {
		fmt.Printf("\nInsufficient USD balance (have: %.2f, need: %.2f)\n", availableUSD, requiredUSD)
		os.Exit(1)
	}

	// Place spread orders
	if *orderFlag {
		buyTxId, sellTxId, estimatedProfit, estimatedPercentGain, err := PlaceSpreadOrders(*baseCoin, spreadInfo, *volume, *untradeable)
		if err != nil {
			fmt.Printf("Error placing orders: %v\n", err)
			os.Exit(1)
		}

		// Estimated profit ignores -untradeable flag and always shows the spread size.
		fmt.Printf("\nEstimated Profit: %.2f USD (gain: %.2f%%)\n", estimatedProfit, estimatedPercentGain)
		fmt.Printf("\nBuy Order TXID: %s\n", buyTxId)
		fmt.Printf("Sell Order TXID: %s\n", sellTxId)

		// Check status of both orders until both are closed
		for {
			fmt.Printf("\n🟢 BUY %s status check\n", *baseCoin)
			buyStatus, err := CheckOrderStatus(buyTxId)
			if err != nil {
				fmt.Printf("Error checking buy order status: %v\n", err)
			}

			fmt.Printf("\n🔴 SELL %s status check\n", *baseCoin)
			sellStatus, err := CheckOrderStatus(sellTxId)
			if err != nil {
				fmt.Printf("Error checking sell order status: %v\n", err)
			}

			// If both orders are closed, print success message and exit
			if buyStatus == "closed" && sellStatus == "closed" {
				fmt.Println("\n🎉 🎉 🎉 TRADE COMPLETE! 🎉 🎉 🎉")
				fmt.Println("Both buy and sell orders have been successfully executed.")
				fmt.Printf("Actual Profit: %.2f USD (Gain: %.2f%%)\n", estimatedProfit, estimatedPercentGain)
				err := SendSlackMessage(fmt.Sprintf("Trade %s in the volume %.5f executed (Profit: $%.2f, Gain: %.2f%%)", *baseCoin, *volume, estimatedProfit, estimatedPercentGain))
				if err != nil {
					fmt.Printf("Error sending Slack message: %v\n", err)
				}
				os.Exit(0)
			}

			if buyStatus == "canceled" && sellStatus == "canceled" {
				fmt.Println("\n=== TRADE CANCELED! ===")
				fmt.Println("Both buy and sell orders have been canceled.")
				fmt.Printf("Unrealised Profit: %.2f USD (Gain: %.2f%%)\n", estimatedProfit, estimatedPercentGain)
				os.Exit(0)
			}

			// Wait before next iteration
			time.Sleep(20 * time.Second)
		}
	} else {
		fmt.Println("\nOrder (-order) flag not set. Skipping order placement.")
	}
}
