package kraken

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Balance represents a currency balance
type Balance struct {
	Currency  string
	Available float64
}

// GetBalance returns the available balance for a coin
func GetBalance(balanceBody []byte, coin string) (*Balance, error) {
	// Get balance string for the coin
	balanceStr, err := getCoinBalance(balanceBody, coin)
	if err != nil {
		return nil, fmt.Errorf("error getting %s balance: %v", coin, err)
	}

	// Convert balance to float64
	balanceFloat, err := strconv.ParseFloat(balanceStr, 64)
	if err != nil {
		return nil, fmt.Errorf("error converting %s balance: %v", coin, err)
	}

	return &Balance{
		Currency:  coin,
		Available: balanceFloat,
	}, nil
}

// getCoinBalance is a helper function to extract balance string from the response
func getCoinBalance(body []byte, coin string) (string, error) {
	var response struct {
		Error  []string `json:"error"`
		Result map[string]struct {
			Balance string `json:"balance"`
		} `json:"result"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("error parsing response: %v", err)
	}

	balanceData, exists := response.Result[coin]
	if !exists {
		return "", fmt.Errorf("balance for %s not found in response", coin)
	}

	return balanceData.Balance, nil
}

// KrakenAssetCode converts standard coin codes to Kraken's format
func KrakenAssetCode(standardCode string) (string, error) {
	hardcodedMap := map[string]string{
		"BTC":     "XBT.F",
		"ETH":     "ETH",
		"SOL":     "SOL.F",
		"SUNDOG":  "SUNDOG",
		"TRUMP":   "TRUMP",
		"GUN":     "GUN",
		"OCEAN":   "OCEAN",
		"GHIBLI":  "GHIBLI",
		"TITCOIN": "TITCOIN",
		"PAXG":    "PAXG",
		"FWOG":    "FWOG",
	}

	code, ok := hardcodedMap[strings.ToUpper(standardCode)]
	if !ok {
		return "", fmt.Errorf("unknown standard code: %s", standardCode)
	}
	return code, nil
}
