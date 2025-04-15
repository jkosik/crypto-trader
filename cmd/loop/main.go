package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	baseCoin := flag.String("coin", "", "Base coin to trade (e.g. BTC, SOL)")
	volume := flag.Float64("volume", 0.0, "Base coin volume to trade")
	iterations := flag.Int("iterations", 50, "Number of trades to execute")
	flag.Parse()

	if *baseCoin == "" || *volume == 0.0 {
		fmt.Println("Error: -coin and -volume flags are required")
		fmt.Println("Usage: ./loop -coin <COIN> -volume <AMOUNT> [-iterations <NUMBER>]")
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

	// Get the absolute path to the trader binary
	traderPath := filepath.Join("..", "trader", "main.go")

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
	}
}
