package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jkosik/crypto-trader/internal/kraken"
)

const (
	// Trading conditions
	minSpreadPercent = 0.7    // Minimum spread percentage required to place orders
	minVolume24h     = 100000 // Minimum 24h volume in USD required to place orders
)

// Kraken crypto trading bot that executes spread trades on specified cryptocurrency pairs.
// The bot places simultaneous buy and sell orders to profit from the spread between bid and ask prices.
//
// Usage:
//   go run cmd/trader/main.go -coin BTC -volume 0.1 -order
//
// Flags:
//   -coin string      Base coin to trade (e.g. BTC, SOL)
//   -order           Place actual orders (default: false)
//   -untradeable     Place orders at untradeable prices (orders won't be executed)
//   -volume float    Base coin volume to trade
//   -spreadnarrow    Optional: How much to narrow the spread (0.0 to 1.0, default: 0.0)
//                    Permitted values:
//                    - 0.0: No narrowing, use full spread (default)
//                    - 0.25: Narrow to quarter of the spread
//                    - 0.5: Narrow to half of the spread
//                    - 0.75: Narrow to quarter of the spread
//                    - 1.0: Place orders at center price (minimum spread)
//
// Example:
//   # Place a real trade with full spread (no narrowing)
//   go run cmd/trader/main.go -coin SUNDOG -volume 300 -order
//
//   # Place a trade with half spread narrowing
//   go run cmd/trader/main.go -coin SUNDOG -volume 300 -order -spreadnarrow 0.5
//
//   # Place a trade with minimal spread narrowing (25%)
//   go run cmd/trader/main.go -coin SUNDOG -volume 300 -order -spreadnarrow 0.25
//
//   # Place a trade with maximum spread narrowing (center price)
//   go run cmd/trader/main.go -coin SUNDOG -volume 300 -order -spreadnarrow 1.0
//
//   # Simulate a trade without actually placing orders
//   go run cmd/trader/main.go -coin SUNDOG -volume 300
//
//   # Place untradeable orders in extreme prices (for testing)
//   go run cmd/trader/main.go -coin SUNDOG -volume 300 -order -untradeable
//
// Environment variables required:
//   KRAKEN_API_KEY
//   KRAKEN_PRIVATE_KEY
//   SLACK_WEBHOOK    (optional) Webhook URL for sending trade notifications to Slack

func main() {
	// Define command line flags
	baseCoin := flag.String("coin", "", "Base coin to trade (e.g. BTC, SOL)")
	orderFlag := flag.Bool("order", false, "Place actual orders (default: false)")
	untradeable := flag.Bool("untradeable", false, "Place orders at untradeable prices (orders won't be executed - close them manually)")
	volume := flag.Float64("volume", 0.0, "Base coin volume to trade")
	spreadNarrow := flag.Float64("spreadnarrow", 0.0, "Optional: How much to narrow the spread (0.0 to 1.0, default: 0.0)")

	// Parse command line flags
	flag.Parse()

	// Check if required flags are set
	if *baseCoin == "" || *volume == 0.0 {
		fmt.Println("Error: -coin flag is required")
		fmt.Println("Usage: go run cmd/trader/main.go -coin <COIN> -volume <AMOUNT> [-order] [-untradeable] [-spreadnarrow <FACTOR>]")
		fmt.Println("\nFlags:")
		fmt.Println("  -coin <COIN>    Base coin to trade (e.g. BTC, SOL)")
		fmt.Println("  -order         Place actual orders (default: false)")
		fmt.Println("  -untradeable   Place orders at untradeable prices (orders won't be executed - close them manually)")
		fmt.Println("  -spreadnarrow  Optional: How much to narrow the spread (0.0 to 1.0, default: 0.0)")
		os.Exit(1)
	}

	// Get Kraken asset code for the selected coin
	baseCoinBalanceCode, err := kraken.KrakenAssetCode(*baseCoin)
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

	signature, err := kraken.GetKrakenSignature(urlPath, payload, apiSecret)
	if err != nil {
		fmt.Println("Error generating signature:", err)
		os.Exit(1)
	}

	balanceBody, err := kraken.MakePrivateRequest(urlBase+urlPath, "POST", payload, apiKey, signature)
	if err != nil {
		fmt.Println("Error making request:", err)
		os.Exit(1)
	}

	fmt.Println("Account balance:")
	fmt.Println(string(balanceBody))

	// Get spread boundary for base coin
	spreadInfo, err := kraken.GetTickerInfo(*baseCoin)
	if err != nil {
		fmt.Println("Error getting spread boundary:", err)
		os.Exit(1)
	}

	// Get OHLC data for price comparison. Hard cap on 8 hours
	if err := kraken.GetOHLCData(*baseCoin, 4*time.Hour); err != nil {
		fmt.Printf("Error getting OHLC data: %v\n", err)
	}

	// Check if we have sufficient balance and place the order
	// Check balance for the base coin
	// We pass empty string as holdCurrency because crypto coins (like BTC, SOL, etc.) don't have
	// alternative currency codes for holds like USD does (USD.F vs ZUSD)
	baseBalance, err := kraken.GetBalance(balanceBody, baseCoinBalanceCode, "")
	if err != nil {
		fmt.Printf("Error getting %s balance: %v\n", baseCoinBalanceCode, err)
		os.Exit(1)
	}
	fmt.Printf("\nAvailable %s: %.8f\n", baseCoinBalanceCode, baseBalance.Available)

	if baseBalance.Available < *volume {
		fmt.Printf("\nInsufficient %s balance (have: %.8f, need: %.8f)\n",
			*baseCoin, baseBalance.Available, *volume)
		os.Exit(1)
	}

	// Check USD balance (handles both USD.F and ZUSD)
	usdBalance, err := kraken.GetBalance(balanceBody, "USD.F", "ZUSD")
	if err != nil {
		fmt.Printf("Error getting USD balance: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Available USD: %.2f\n", usdBalance.Available)

	requiredUSD := *volume * spreadInfo.BidPrice
	if usdBalance.Available < requiredUSD {
		fmt.Printf("\nInsufficient USD balance (have: %.2f, need: %.2f)\n",
			usdBalance.Available, requiredUSD)
		os.Exit(1)
	}

	// Place spread orders
	if *orderFlag {
		// Place order only if spread is within the boundaries
		for {
			// Calculate spread percentage
			fmt.Println("\nGetting fresh spread boundary to assess max. spread and min. volume...")
			spreadInfo, err := kraken.GetTickerInfo(*baseCoin)
			if err != nil {
				fmt.Println("Error getting spread boundary:", err)
				os.Exit(1)
			}

			spreadPercent := (spreadInfo.Spread / spreadInfo.BidPrice) * 100
			fmt.Printf("\nCurrent spread: %.4f%%\n", spreadPercent)

			// Get 24h volume
			volume24h, err := kraken.Get24hVolume(*baseCoin)
			if err != nil {
				fmt.Printf("Error getting 24h volume: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("24h Volume: %.2f USD\n", volume24h)

			// Skip and re-try if spread and volume are not within the boundaries
			if spreadPercent < minSpreadPercent {
				fmt.Println("❌ Spread is not within the boundaries. Sleeping for a while...")
				time.Sleep(10 * time.Second)
				continue
			}
			if volume24h < minVolume24h {
				fmt.Println("❌ 24h volume is not within the boundaries. Sleeping for a while...")
				time.Sleep(10 * time.Second)
				continue
			}

			fmt.Println("✅ Spread and volume are within the boundaries. Placing orders.")
			break
		}

		buyTxId, sellTxId, estimatedProfit, estimatedPercentGain, err := kraken.PlaceSpreadOrders(*baseCoin, spreadInfo, *volume, *untradeable, *spreadNarrow)
		if err != nil {
			fmt.Printf("Error placing spread orders: %v\n", err)
			os.Exit(1)
		}

		// Estimated profit ignores -untradeable flag and always shows the spread size.
		fmt.Printf("\nEstimated profit: %.8f USD (%.4f%%)\n", estimatedProfit, estimatedPercentGain)
		fmt.Printf("\nBuy Order TXID: %s", buyTxId)
		fmt.Printf("\nSell Order TXID: %s\n", sellTxId)

		// Check status of both orders until both are closed
		for {
			time.Sleep(10 * time.Second)

			fmt.Printf("\n🟢 BUY %s status check\n", *baseCoin)
			buyOrder, err := kraken.CheckOrderStatus(buyTxId)
			if err != nil {
				fmt.Printf("Error checking buy order status: %v\n", err)
				continue
			}

			fmt.Printf("\n🔴 SELL %s status check\n", *baseCoin)
			sellOrder, err := kraken.CheckOrderStatus(sellTxId)
			if err != nil {
				fmt.Printf("Error checking sell order status: %v\n", err)
				continue
			}

			// If both orders are closed, print success message and exit
			if buyOrder.Status == "closed" && sellOrder.Status == "closed" {
				fmt.Println("\n🎉 🎉 🎉 TRADE COMPLETE! 🎉 🎉 🎉")
				fmt.Println("Both buy and sell orders have been successfully executed.")

				// Get current spread information
				currentSpreadInfo, err := kraken.GetTickerInfo(*baseCoin)
				if err != nil {
					fmt.Printf("Error getting current spread info: %v\n", err)
				}

				// Calculate spread information
				spread := currentSpreadInfo.Spread
				spreadPercent := (spread / currentSpreadInfo.BidPrice) * 100

				// Get 24h volume
				volume24h, err := kraken.Get24hVolume(*baseCoin)
				if err != nil {
					fmt.Printf("Error getting 24h volume: %v\n", err)
				}

				// Calculate total fees
				buyFee, _ := strconv.ParseFloat(buyOrder.Fee, 64)
				sellFee, _ := strconv.ParseFloat(sellOrder.Fee, 64)
				totalFees := buyFee + sellFee

				// Get actual executed prices
				buyPrice, _ := strconv.ParseFloat(buyOrder.Descr.Price, 64)
				sellPrice, _ := strconv.ParseFloat(sellOrder.Descr.Price, 64)
				realProfit := (sellPrice - buyPrice) * *volume
				realProfitPercent := (sellPrice - buyPrice) / buyPrice * 100

				fmt.Printf("Actual Profit: %.2f USD (Gain:%.2f%%)\n", realProfit, realProfitPercent)
				fmt.Printf("Total Fees: %.2f USD (Buy: %.2f, Sell: %.2f)\n", totalFees, buyFee, sellFee)
				slackErr := kraken.SendSlackMessage(fmt.Sprintf(
					"Trade %s in the volume %.5f executed\n"+
						"Expected Profit: $%.2f (%.2f%%)\n"+
						"Real Profit: $%.2f (%.2f%%)\n"+
						"Spread now: %.8f (%.4f%%)\n"+
						"24h Volume: %.2f\n"+
						"Fees: $%.2f (Buy: $%.2f, Sell: $%.2f)\n"+
						"Buy price: %.8f\n"+
						"Sell price: %.8f",
					*baseCoin,
					*volume,
					estimatedProfit,
					estimatedPercentGain,
					realProfit,
					realProfitPercent,
					spread,
					spreadPercent,
					volume24h,
					totalFees,
					buyFee,
					sellFee,
					buyPrice,
					sellPrice,
				))
				if slackErr != nil {
					fmt.Printf("Error sending Slack message: %v\n", slackErr)
				}
				os.Exit(0)
			}

			if buyOrder.Status == "canceled" && sellOrder.Status == "canceled" {
				fmt.Println("\n=== TRADE CANCELED! ===")
				fmt.Println("Both buy and sell orders have been canceled.")
				fmt.Printf("Unrealised Profit: %.2f USD (Gain: %.2f%%)\n", estimatedProfit, estimatedPercentGain)
				os.Exit(0)
			}
		}
	} else {
		fmt.Println("\nOrder (-order) flag not set. Skipping order placement.")
	}
}
