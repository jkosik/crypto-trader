// Checks balance for all coins

package main

import (
	"fmt"
	"os"
	"time"

	"github.com/jkosik/crypto-trader/internal/kraken"
)

func main() {
	// Check for required environment variables
	apiKey := os.Getenv("KRAKEN_API_KEY")
	apiSecret := os.Getenv("KRAKEN_PRIVATE_KEY")

	if apiKey == "" || apiSecret == "" {
		fmt.Println("Error: KRAKEN_API_KEY and KRAKEN_PRIVATE_KEY environment variables must be set")
		os.Exit(1)
	}

	// Get account balance
	nonce := time.Now().UnixNano() / int64(time.Millisecond)
	urlBase := "https://api.kraken.com"
	urlPath := "/0/private/BalanceEx"

	payload := fmt.Sprintf(`{
		"nonce": "%d"
	}`, nonce)

	signature, err := kraken.GetKrakenSignature(urlPath, payload, apiSecret)
	if err != nil {
		fmt.Printf("Error generating signature: %v\n", err)
		os.Exit(1)
	}

	balanceBody, err := kraken.MakePrivateRequest(urlBase+urlPath, "POST", payload, apiKey, signature)
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Account balance:")
	fmt.Println(string(balanceBody))

}
