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
	minSpreadPercent   = 0        // Minimum spread percentage required to place orders
	minVolume24h       = 100000.0 // Minimum 24h volume in USD required to place orders
	maxADX             = 20.0     // Maximum ADX value (0-20 = weak trend, ideal for spread trading)
	adxPeriod          = 14       // ADX calculation period (standard is 14)
	spreadAdjustFactor = 0.5      // Spread adjustment: 0=no spread, 0.5=half spread, 1=full spread, 2=double spread, etc.
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
	baseCoin := flag.String("coin", "", "Base coin to trade (e.g. BTC, SOL)")
	orderFlag := flag.Bool("order", false, "Place actual orders (default: false)")
	untradeable := flag.Bool("untradeable", false, "Place orders at untradeable prices (orders won't be executed - close them manually)")
	volume := flag.Float64("volume", 0.0, "Base coin volume to trade")

	// Parse command line flags
	flag.Parse()

	// Check if required flags are set
	if *baseCoin == "" || *volume == 0.0 {
		fmt.Println("Error: -coin flag is required")
		fmt.Println("Usage: go run cmd/trader/main.go -coin <COIN> -volume <AMOUNT> [-order] [-untradeable]")
		fmt.Println("\nFlags:")
		fmt.Println("  -coin <COIN>    Base coin to trade (e.g. BTC, SOL)")
		fmt.Println("  -order         Place actual orders (default: false)")
		fmt.Println("  -untradeable   Place orders at untradeable prices (orders won't be executed - close them manually)")
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

	// Some asset codes differ submited on CLI differ from those recognized by Kraken.
	baseCoinBalanceCode, err := kraken.KrakenAssetCode(*baseCoin)
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
	fmt.Printf("\nAvailable %s: %.8f\n", baseCoinBalanceCode, baseBalance.Available)

	if baseBalance.Available < *volume {
		fmt.Printf("\nInsufficient %s balance (have: %.8f, need: %.8f)\n",
			*baseCoin, baseBalance.Available, *volume)
		os.Exit(1)
	}

	// Check USD balance
	usdBalance, err := kraken.GetBalance(balanceBody, "ZUSD")
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
		// Variables to store conditions when trade is placed
		var finalADX float64
		var spreadInfo *kraken.SpreadInfo

		// Place order only if all conditions are met: spread, volume, and ADX
		for {
			fmt.Println("\n=== Checking Trading Conditions ===")

			// Check 1: Spread percentage
			var err error
			spreadInfo, err = kraken.GetTickerInfo(*baseCoin)
			if err != nil {
				fmt.Println("Error getting spread info:", err)
				os.Exit(1)
			}

			spreadPercent := (spreadInfo.Spread / spreadInfo.BidPrice) * 100
			fmt.Printf("Current spread: %.4f%% (min required: %.2f%%)\n", spreadPercent, minSpreadPercent)

			// Check 2: 24h volume
			volume24h, err := kraken.Get24hVolume(*baseCoin)
			if err != nil {
				fmt.Printf("Error getting 24h volume: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("24h Volume: %.2f USD (min required: %.2f USD)\n", volume24h, minVolume24h)

			// Check 3: ADX indicator
			adx, err := kraken.GetADXInfo(*baseCoin, adxPeriod)
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
				fmt.Printf("❌ Volume too low (%.2f < %.2f USD)\n", volume24h, minVolume24h)
				conditionsMet = false
			} else {
				fmt.Printf("✅ Volume OK (%.2f >= %.2f USD)\n", volume24h, minVolume24h)
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

		buyTxId, sellTxId, estimatedProfit, estimatedPercentGain, err := kraken.PlaceSpreadOrders(*baseCoin, spreadInfo, *volume, *untradeable, spreadAdjustFactor, finalADX, adxPeriod)
		if err != nil {
			fmt.Printf("Error placing spread orders: %v\n", err)
			os.Exit(1)
		}

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

				// Calculate actual profit from executed prices
				actualProfit := (sellPrice - buyPrice) * (*volume)
				actualPercentGain := ((sellPrice - buyPrice) / buyPrice) * 100
				netProfit := actualProfit - totalFees

				fmt.Printf("Actual profit: %.2f USD (%.4f%% gain)\n", actualProfit, actualPercentGain)
				fmt.Printf("Total Fees: %.2f USD (Buy: %.2f, Sell: %.2f)\n", totalFees, buyFee, sellFee)
				fmt.Printf("Net profit (after fees): %.2f USD\n", netProfit)
				slackErr := kraken.SendSlackMessage(fmt.Sprintf(
					"✅ Trade %s/USD executed\n"+
						"Volume: %.5f\n"+
						"Buy price: %.6f\n"+
						"Sell price: %.6f\n"+
						"Actual profit: %.2f USD (%.4f%% gain)\n"+
						"Fees: %.2f USD (Buy: %.2f, Sell: %.2f)\n"+
						"Net profit: %.2f USD\n"+
						"Buy Order ID: %s\n"+
						"Sell Order ID: %s\n"+
						"Current spread: %.6f (%.4f%%)\n"+
						"24h Volume: %.2f USD\n"+
						"ADX(%d) at entry: %.2f",
					*baseCoin,
					*volume,
					buyPrice,
					sellPrice,
					actualProfit,
					actualPercentGain,
					totalFees,
					buyFee,
					sellFee,
					netProfit,
					buyTxId,
					sellTxId,
					spread,
					spreadPercent,
					volume24h,
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
				fmt.Printf("Unrealised Profit: %.2f USD (Gain: %.4f%%)\n", estimatedProfit, estimatedPercentGain)
				os.Exit(0)
			}
		}
	} else {
		fmt.Println("\nOrder (-order) flag not set. Skipping order placement.")
	}
}
