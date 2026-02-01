package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jkosik/crypto-trader/internal/kraken"
)

const (
	// Trading conditions
	minSpreadPercent = 0.0  // Minimum spread percentage required to place orders (0.0 = accept any spread)
	minVolume24h     = 10.0 // Minimum 24h trading pair volume (measured in QUOTE currency: BTC for ETH/BTC, USD for BTC/USD)
	// This is a liquidity filter: for ETH/BTC use 100, for BTC/USD use 1000000, for altcoins use 1000
	maxADX             = 20.0 // Maximum ADX value (0-20 = weak trend, ideal for spread trading)
	adxPeriod          = 14   // ADX calculation period (standard is 14)
	spreadAdjustFactor = 0.5  // Spread adjustment: 0=no spread, 0.5=half spread, 1=full spread, 2=double spread, etc.
)

// Kraken crypto trading bot that executes spread trades on specified cryptocurrency pairs.
// The bot places simultaneous buy and sell orders to profit from the spread between bid and ask prices.
//
// Usage:
//   go run cmd/trader/main.go -coin BTC -volume 0.1 -order
//
// Flags:
//   -coin string      Base coin to trade (e.g. BTC, SOL)
//   -order            Place actual orders (default: false)
//   -untradeable      Place orders at untradeable prices (orders won't be executed)
//   -volume float     Base coin volume to trade
//
// Example:
//   # Place a real trade
//   go run cmd/trader/main.go -coin SUNDOG -volume 300 -order
//
//   # Simulate a trade without actually placing orders
//   go run cmd/trader/main.go -coin SUNDOG -volume 300
//
//   # Place untradeable orders in extreme prices (for testing)
//   go run cmd/trader/main.go -coin SUNDOG -volume 300 -order -untradeable

func main() {
	// Define command line flags
	pair := flag.String("pair", "", "Trading pair (e.g. ETH/BTC, BTC/USD, SUNDOG/USD)")
	orderFlag := flag.Bool("order", false, "Place actual orders (default: false)")
	untradeable := flag.Bool("untradeable", false, "Place orders at untradeable prices (orders won't be executed - close them manually)")
	volume := flag.Float64("volume", 0.0, "Base coin volume to trade")

	// Parse command line flags
	flag.Parse()

	// Check if required flags are set
	if *pair == "" || *volume == 0.0 {
		fmt.Println("Error: -pair and -volume flags are required")
		fmt.Println("Usage: go run cmd/trader/main.go -pair <PAIR> -volume <AMOUNT> [-order] [-untradeable]")
		fmt.Println("\nFlags:")
		fmt.Println("  -pair <PAIR>    Trading pair (e.g. ETH/BTC, BTC/USD, SUNDOG/USD)")
		fmt.Println("  -volume <AMOUNT> Base coin volume to trade")
		fmt.Println("  -order          Place actual orders (default: false)")
		fmt.Println("  -untradeable    Place orders at untradeable prices (orders won't be executed - close them manually)")
		os.Exit(1)
	}

	// Parse the trading pair
	parts := strings.Split(*pair, "/")
	if len(parts) != 2 {
		fmt.Println("Error: -pair must be in format BASE/QUOTE (e.g. ETH/BTC, BTC/USD)")
		fmt.Println("Examples:")
		fmt.Println("  -pair BTC/USD")
		fmt.Println("  -pair ETH/BTC")
		fmt.Println("  -pair SUNDOG/USD")
		os.Exit(1)
	}
	baseCoin := parts[0]
	quoteCoin := parts[1]

	fmt.Printf("\nTrading %s/%s\n", baseCoin, quoteCoin)
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

	// Get spread boundary for trading pair
	spreadInfo, err := kraken.GetTickerInfo(baseCoin, quoteCoin)
	if err != nil {
		fmt.Println("Error getting spread boundary:", err)
		os.Exit(1)
	}

	// Get OHLC data for price comparison. Hard cap on 8 hours
	if err := kraken.GetOHLCData(baseCoin, quoteCoin, 4*time.Hour); err != nil {
		fmt.Printf("Error getting OHLC data: %v\n", err)
	}

	// Some asset codes differ submitted on CLI from those recognized by Kraken
	baseCoinBalanceCode, err := kraken.KrakenAssetCode(baseCoin)
	if err != nil {
		fmt.Printf("Error getting Kraken asset code: %v\n", err)
		os.Exit(1)
	}

	// Check available balance for the base coin (ignoring holds from open trades)
	baseBalance, err := kraken.GetBalance(balanceBody, baseCoinBalanceCode)
	if err != nil {
		fmt.Printf("Error getting %s balance: %v\n", baseCoinBalanceCode, err)
		os.Exit(1)
	}
	fmt.Printf("\nAvailable %s: %.8f\n", baseCoin, baseBalance.Available)

	if baseBalance.Available < *volume {
		fmt.Printf("\nInsufficient %s balance (have: %.8f, need: %.8f)\n",
			baseCoin, baseBalance.Available, *volume)
		os.Exit(1)
	}

	// Check quote currency balance (e.g., USD, BTC, ETH)
	quoteCoinBalanceCode, err := kraken.KrakenAssetCode(quoteCoin)
	if err != nil {
		fmt.Printf("Error getting Kraken asset code for %s: %v\n", quoteCoin, err)
		os.Exit(1)
	}

	quoteBalance, err := kraken.GetBalance(balanceBody, quoteCoinBalanceCode)
	if err != nil {
		fmt.Printf("Error getting %s balance: %v\n", quoteCoin, err)
		os.Exit(1)
	}
	fmt.Printf("Available %s: %.8f\n", quoteCoin, quoteBalance.Available)

	requiredQuote := *volume * spreadInfo.BidPrice
	if quoteBalance.Available < requiredQuote {
		fmt.Printf("\nInsufficient %s balance (have: %.8f, need: %.8f)\n",
			quoteCoin, quoteBalance.Available, requiredQuote)
		os.Exit(1)
	}

	// Place spread orders
	if *orderFlag {
		// Variables to store conditions when trade is placed
		var finalADX float64
		var spreadInfo *kraken.SpreadInfo

		// Place order only if all conditions are met: spread, volume, and ADX
		for {
			fmt.Println("\n=== Checking Trading Conditions ===")

			// Check 1: Spread percentage
			var err error
			spreadInfo, err = kraken.GetTickerInfo(baseCoin, quoteCoin)
			if err != nil {
				fmt.Println("Error getting spread info:", err)
				os.Exit(1)
			}

			spreadPercent := (spreadInfo.Spread / spreadInfo.BidPrice) * 100
			fmt.Printf("Current spread: %.4f%% (min required: %.2f%%)\n", spreadPercent, minSpreadPercent)

			// Check 2: 24h trading pair volume (liquidity check)
			// Note: Volume is measured in quote currency (BTC for ETH/BTC, USD for BTC/USD, etc.)
			volume24h, err := kraken.Get24hVolume(baseCoin, quoteCoin)
			if err != nil {
				fmt.Printf("Error getting 24h volume: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("24h %s/%s Volume: %.2f %s (min required: %.2f %s)\n", baseCoin, quoteCoin, volume24h, quoteCoin, minVolume24h, quoteCoin)

			// Check 3: ADX indicator
			adx, err := kraken.GetADXInfo(baseCoin, quoteCoin, adxPeriod)
			if err != nil {
				fmt.Printf("Error calculating ADX: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("ADX(%d): %.2f (max allowed: %.2f)\n", adxPeriod, adx, maxADX)

			// Validate all conditions
			conditionsMet := true

			if spreadPercent < minSpreadPercent {
				fmt.Printf("❌ Spread too low (%.4f%% < %.2f%%)\n", spreadPercent, minSpreadPercent)
				conditionsMet = false
			} else {
				fmt.Printf("✅ Spread OK (%.4f%% >= %.2f%%)\n", spreadPercent, minSpreadPercent)
			}

			if volume24h < minVolume24h {
				fmt.Printf("❌ %s/%s volume too low (%.2f < %.2f %s) - insufficient pair liquidity\n", baseCoin, quoteCoin, volume24h, minVolume24h, quoteCoin)
				conditionsMet = false
			} else {
				fmt.Printf("✅ %s/%s volume OK (%.2f >= %.2f %s) - sufficient pair liquidity\n", baseCoin, quoteCoin, volume24h, minVolume24h, quoteCoin)
			}

			if adx > maxADX {
				fmt.Printf("❌ ADX too high (%.2f > %.2f) - strong trend detected, not ideal for spread trading\n", adx, maxADX)
				conditionsMet = false
			} else {
				fmt.Printf("✅ ADX OK (%.2f <= %.2f) - weak trend, good for spread trading\n", adx, maxADX)
			}

			if !conditionsMet {
				fmt.Println("\n⏳ Conditions not met. Waiting 30 seconds before rechecking...")
				time.Sleep(30 * time.Second)
				continue
			}

			// Store ADX value when conditions are met
			finalADX = adx

			fmt.Println("\n✅ All conditions met! Placing orders...")
			break
		}

		buyTxId, sellTxId, estimatedProfit, estimatedPercentGain, err := kraken.PlaceSpreadOrders(baseCoin, quoteCoin, spreadInfo, *volume, *untradeable, spreadAdjustFactor, finalADX, adxPeriod)
		if err != nil {
			fmt.Printf("Error placing spread orders: %v\n", err)
			os.Exit(1)
		}

		// Check status of both orders until both are closed
		for {
			time.Sleep(10 * time.Second)

			fmt.Printf("\n🟢 BUY %s/%s status check\n", baseCoin, quoteCoin)
			buyOrder, err := kraken.CheckOrderStatus(buyTxId)
			if err != nil {
				fmt.Printf("Error checking buy order status: %v\n", err)
				continue
			}

			fmt.Printf("\n🔴 SELL %s/%s status check\n", baseCoin, quoteCoin)
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
				currentSpreadInfo, err := kraken.GetTickerInfo(baseCoin, quoteCoin)
				if err != nil {
					fmt.Printf("Error getting current spread info: %v\n", err)
				}

				// Calculate spread information
				spread := currentSpreadInfo.Spread
				spreadPercent := (spread / currentSpreadInfo.BidPrice) * 100

				// Get 24h volume
				volume24h, err := kraken.Get24hVolume(baseCoin, quoteCoin)
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

				// Calculate actual profit from executed prices
				actualProfit := (sellPrice - buyPrice) * (*volume)
				actualPercentGain := ((sellPrice - buyPrice) / buyPrice) * 100
				netProfit := actualProfit - totalFees

				fmt.Printf("Actual profit: %.2f %s (%.4f%% gain)\n", actualProfit, quoteCoin, actualPercentGain)
				fmt.Printf("Total Fees: %.2f %s (Buy: %.2f, Sell: %.2f)\n", totalFees, quoteCoin, buyFee, sellFee)
				fmt.Printf("Net profit (after fees): %.2f %s\n", netProfit, quoteCoin)
				slackErr := kraken.SendSlackMessage(fmt.Sprintf(
					"✅ Trade %s/%s executed\n"+
						"Volume: %.5f\n"+
						"Buy price: %.6f\n"+
						"Sell price: %.6f\n"+
						"Actual profit: %.2f %s (%.4f%% gain)\n"+
						"Fees: %.2f %s (Buy: %.2f, Sell: %.2f)\n"+
						"Net profit: %.2f %s\n"+
						"Buy Order ID: %s\n"+
						"Sell Order ID: %s\n"+
						"Current spread: %.6f (%.4f%%)\n"+
						"24h Volume: %.2f %s\n"+
						"ADX(%d) at entry: %.2f",
					baseCoin,
					quoteCoin,
					*volume,
					buyPrice,
					sellPrice,
					actualProfit,
					quoteCoin,
					actualPercentGain,
					totalFees,
					quoteCoin,
					buyFee,
					sellFee,
					netProfit,
					quoteCoin,
					buyTxId,
					sellTxId,
					spread,
					spreadPercent,
					volume24h,
					quoteCoin,
					adxPeriod,
					finalADX,
				))
				if slackErr != nil {
					fmt.Printf("Error sending Slack message: %v\n", slackErr)
				}
				os.Exit(0)
			}

			if buyOrder.Status == "canceled" && sellOrder.Status == "canceled" {
				fmt.Println("\n=== TRADE CANCELED! ===")
				fmt.Println("Both buy and sell orders have been canceled.")
				fmt.Printf("Unrealised Profit: %.2f %s (Gain: %.4f%%)\n", estimatedProfit, quoteCoin, estimatedPercentGain)
				os.Exit(0)
			}
		}
	} else {
		fmt.Println("\nOrder (-order) flag not set. Skipping order placement.")
	}
}
