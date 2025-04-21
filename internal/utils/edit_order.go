package main

import (
	"fmt"
	"github.com/jkosik/crypto-trader/internal/kraken"
)

func main() {
	// Hardcoded values for debugging
	txId := "OKCADW-OIOLH-3FG5S2" // Replace with your actual order ID
	price := 0.7                  // Replace with your desired price
	volume := 100                 // Replace with your desired volume

	fmt.Printf("Attempting to edit order:\n")
	fmt.Printf("Order ID: %s\n", txId)
	fmt.Printf("New Price: %.8f\n", price)
	fmt.Printf("Volume: %.5f\n", float64(volume))

	if err := kraken.EditOrder(txId, price, float64(volume)); err != nil {
		fmt.Printf("Error editing order: %v\n", err)
		return
	}

	fmt.Println("✅ Order edited successfully")
}
