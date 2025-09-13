package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
//
// Example:
//   # Execute N iterations of trades
//   go run cmd/loop/main.go -coin SUNDOG -volume 300 -iterations 2
//
//   # Execute 10 trades (default iteration count)
//   go run cmd/loop/main.go -coin SUNDOG -volume 300

func main() {
	baseCoin := flag.String("coin", "", "Base coin to trade (e.g. BTC, SOL)")
	volume := flag.Float64("volume", 0.0, "Base coin volume to trade")
	iterations := flag.Int("iterations", 10, "Number of trades to execute")
	flag.Parse()

	if *baseCoin == "" || *volume == 0.0 {
		fmt.Println("Error: -coin and -volume flags are required")
		fmt.Println("Usage: ./loop -coin <COIN> -volume <AMOUNT> [-iterations <NUMBER>]")
		fmt.Println("\nFlags:")
		fmt.Println("  -coin <COIN>    Base coin to trade (e.g. BTC, SOL)")
		fmt.Println("  -volume <AMOUNT> Base coin volume to trade")
		fmt.Println("  -iterations <NUMBER> Number of trades to execute (default: 10)")
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

	// Get the path to the trader binary, working from both root and cmd/loop
	traderPath, err := getTraderPath()
	if err != nil {
		fmt.Printf("Error finding trader path: %v\n", err)
		os.Exit(1)
	}

	for i := 1; i <= *iterations; i++ {
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

		// Add a delay between iterations to prevent too rapid execution
		if i < *iterations {
			delayMinutes := 5
			fmt.Printf("\nWaiting %d minutes before next iteration...\n", delayMinutes)
			time.Sleep(time.Duration(delayMinutes) * time.Minute)
		}
	}
}

// getTraderPath returns the correct path to the trader binary based on current directory
// to allow running from both root and cmd/loop
func getTraderPath() (string, error) {
	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("error getting current directory: %v", err)
	}

	// Check if we're in the project root (look for go.mod)
	if _, err := os.Stat("go.mod"); err == nil {
		// We're in project root, trader is at cmd/trader/main.go
		return "cmd/trader/main.go", nil
	}

	// Check if we're in cmd/loop directory
	if filepath.Base(cwd) == "loop" && filepath.Base(filepath.Dir(cwd)) == "cmd" {
		// We're in cmd/loop, trader is at ../trader/main.go
		return filepath.Join("..", "trader", "main.go"), nil
	}

	// Try to find go.mod by walking up the directory tree
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			// Found go.mod, construct path from project root
			return filepath.Join(dir, "cmd", "trader", "main.go"), nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root without finding go.mod
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("could not find project root (go.mod not found)")
}
