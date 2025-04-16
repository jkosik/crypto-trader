package main

import (
	"context"
	"crypto-trader/internal/kraken"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Loop trading bot that executes multiple trades in sequence using the trader bot.
// This program runs the trader bot multiple times with the same parameters and logs the results.
//
// Usage:
//   go run cmd/loop/main.go -coin BTC -volume 0.1 -iterations 20
//
// Flags:
//   -coin string      Base coin to trade (e.g. BTC, SOL)
//   -volume float     Base coin volume to trade
//   -iterations int   Number of trades to execute (default: 10)
//   -limitprice float Maximum price limit for the base coin
//
// Example:
//   # Execute N iterations of trades
//   go run cmd/loop/main.go -coin SUNDOG -volume 300 -iterations 2
//
//   # Execute 50 trades (default iteration count)
//   go run cmd/loop/main.go -coin SUNDOG -volume 300
//
// Note: This program requires the same environment variables as the trader bot:
//   KRAKEN_API_KEY
//   KRAKEN_PRIVATE_KEY
//   SLACK_WEBHOOK    (optional) Webhook URL for sending trade notifications to Slack

func main() {
	baseCoin := flag.String("coin", "", "Base coin to trade (e.g. BTC, SOL)")
	volume := flag.Float64("volume", 0.0, "Base coin volume to trade")
	iterations := flag.Int("iterations", 10, "Number of trades to execute")
	limitPrice := flag.Float64("limitprice", 0.0, "Maximum price limit for the base coin")
	flag.Parse()

	if *baseCoin == "" || *volume == 0.0 || *limitPrice == 0.0 {
		fmt.Println("Error: -coin, -volume, and -limitprice flags are required")
		fmt.Println("Usage: ./loop -coin <COIN> -volume <AMOUNT> -limitprice <PRICE> [-iterations <NUMBER>]")
		os.Exit(1)
	}

	// Create report file
	report := fmt.Sprintf("trades-%s-%s.txt", *baseCoin, time.Now().Format("2006-01-02-15-04"))
	reportFile, err := os.Create(report)
	if err != nil {
		fmt.Printf("Error creating report file: %v\n", err)
		os.Exit(1)
	}
	defer reportFile.Close()

	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a cancellation flag with mutex protection
	var (
		canceled bool
		mu       sync.Mutex
		wg       sync.WaitGroup
	)

	// Start price monitoring in a separate goroutine
	wg.Add(1)
	go monitorPrice(ctx, *baseCoin, *limitPrice, reportFile, &canceled, &mu, cancel, &wg)

	// Get the absolute path to the trader binary
	traderPath := filepath.Join("..", "trader", "main.go")

	for i := 1; i <= *iterations; i++ {
		// Check if we've been canceled
		mu.Lock()
		if canceled {
			mu.Unlock()
			fmt.Println("Price limit exceeded. Waiting for cleanup...")
			wg.Wait() // Wait for the goroutine to finish its work
			fmt.Println("Cleanup complete. Stopping trading.")
			return
		}
		mu.Unlock()

		fmt.Printf("Running iteration %d\n", i)

		// Run the trader command
		cmd := exec.Command("go", "run", traderPath, "-coin", *baseCoin, "-order", "-volume", fmt.Sprintf("%f", *volume))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			fmt.Printf("Iteration %d failed at %s\n", i, time.Now().Format("2006-01-02 15:04:05"))
			os.Exit(1)
		}

		// Log successful trade
		successMsg := fmt.Sprintf("%s - SUCCESSFUL TRADE %d\n", time.Now().Format("2006-01-02 15:04:05"), i)
		if _, err := reportFile.WriteString(successMsg); err != nil {
			fmt.Printf("Error writing to report file: %v\n", err)
		}
	}
}

func monitorPrice(ctx context.Context, baseCoin string, limitPrice float64, reportFile *os.File, canceled *bool, mu *sync.Mutex, cancel context.CancelFunc, wg *sync.WaitGroup) {
	defer wg.Done() // Signal that we're done when the function exits

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Get current price using Kraken package
			fmt.Printf("\n[LOOP] Checking current price for potential exit order for %s\n", baseCoin)
			spreadInfo, err := kraken.GetTickerInfo(baseCoin)
			if err != nil {
				fmt.Printf("[LOOP] Error getting price: %v\n", err)
				continue
			}

			currentPrice := spreadInfo.BidPrice
			fmt.Printf("[LOOP] Current price of %s: %.8f (Limit: %.8f)\n", baseCoin, currentPrice, limitPrice)

			// Check if price exceeds limit
			if currentPrice > limitPrice {
				fmt.Printf("[LOOP] Price exceeded limit! Current: %.8f, Limit: %.8f\n", currentPrice, limitPrice)

				// Set the cancellation flag
				mu.Lock()
				*canceled = true
				mu.Unlock()

				// Cancel all open orders for the base coin
				openOrders, err := kraken.GetOpenOrders(baseCoin)
				if err != nil {
					fmt.Printf("[LOOP] Error getting open orders: %v\n", err)
					continue
				}

				if len(openOrders) > 0 {
					fmt.Printf("[LOOP] Found %d open orders for %s:\n", len(openOrders), baseCoin)
					for txId, order := range openOrders {
						fmt.Printf("[LOOP] ID: %s, Type: %s, Volume: %s %s\n",
							txId, order.Descr.Type, order.Vol, baseCoin)
					}

					if err := kraken.CancelAllOrders(baseCoin); err != nil {
						fmt.Printf("[LOOP] Error canceling orders: %v\n", err)
						continue
					}

					// Verify that all orders are canceled
					openOrders, err = kraken.GetOpenOrders(baseCoin)
					if err != nil {
						fmt.Printf("[LOOP] Error checking open orders: %v\n", err)
						continue
					}

					if len(openOrders) > 0 {
						fmt.Printf("[LOOP] Warning: %d orders still open after cancellation attempt:\n", len(openOrders))
						for txId, order := range openOrders {
							fmt.Printf("[LOOP] ID: %s, Type: %s, Volume: %s %s\n",
								txId, order.Descr.Type, order.Vol, baseCoin)
						}
						// Try to cancel remaining orders one more time
						if err := kraken.CancelAllOrders(baseCoin); err != nil {
							fmt.Printf("[LOOP] Error canceling remaining orders: %v\n", err)
							continue
						}
						// Check again
						openOrders, err = kraken.GetOpenOrders(baseCoin)
						if err != nil {
							fmt.Printf("[LOOP] Error checking open orders after second attempt: %v\n", err)
							continue
						}
						if len(openOrders) > 0 {
							fmt.Printf("[LOOP] Error: Failed to cancel all orders after two attempts\n")
							continue
						}
					}

					fmt.Printf("[LOOP] Successfully canceled all open orders\n")
				} else {
					fmt.Printf("[LOOP] No open orders found for %s\n", baseCoin)
				}

				// Get account balance
				nonce := time.Now().UnixNano() / int64(time.Millisecond)
				urlPath := "/0/private/BalanceEx"
				payload := fmt.Sprintf(`{"nonce": "%d"}`, nonce)

				signature, err := kraken.GetKrakenSignature(urlPath, payload, os.Getenv("KRAKEN_PRIVATE_KEY"))
				if err != nil {
					fmt.Printf("[LOOP] Error generating signature: %v\n", err)
					continue
				}

				balanceBody, err := kraken.MakePrivateRequest("https://api.kraken.com"+urlPath, "POST", payload, os.Getenv("KRAKEN_API_KEY"), signature)
				if err != nil {
					fmt.Printf("[LOOP] Error getting balance: %v\n", err)
					continue
				}

				// Get base coin balance
				baseCoinCode, err := kraken.KrakenAssetCode(baseCoin)
				if err != nil {
					fmt.Printf("[LOOP] Error getting asset code: %v\n", err)
					continue
				}

				balance, err := kraken.GetBalance(balanceBody, baseCoinCode, "")
				if err != nil {
					fmt.Printf("[LOOP] Error getting balance: %v\n", err)
					continue
				}

				if balance.Available > 0 {
					// sellPrice := currentPrice*10 // untradeable (for testing)
					sellPrice := currentPrice
					_, err := kraken.PlaceLimitOrder(baseCoin, sellPrice, balance.Available, false, false)
					if err != nil {
						fmt.Printf("[LOOP] Error placing sell order: %v\n", err)
					} else {
						fmt.Printf("[LOOP] Placed full exit sell order for %.8f %s at %.2f USD\n", balance.Available, baseCoin, sellPrice)

						// Log the event
						eventMsg := fmt.Sprintf("%s - PRICE LIMIT EXCEEDED - Sold all %s at %.2f\n",
							time.Now().Format("2006-01-02 15:04:05"), baseCoin, sellPrice)
						if _, err := reportFile.WriteString(eventMsg); err != nil {
							fmt.Printf("[LOOP] Error writing to report file: %v\n", err)
						}
					}
				}

				// Cancel the context
				cancel()
				return
			}
		}
	}
}

func parsePrice(output string) float64 {
	var price float64
	fmt.Sscanf(strings.TrimSpace(output), "%f", &price)
	return price
}
