package kraken

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// BalanceData represents the balance information for a coin
type BalanceData struct {
	Balance   string `json:"balance"`
	HoldTrade string `json:"hold_trade"`
}

// Balance represents a currency balance with its trade hold amount
type Balance struct {
	Currency  string
	Balance   float64
	HoldTrade float64
	Available float64
}

// GetBalance calculates the available balance for a currency
// It handles both the main balance and any held amounts in related currencies
func GetBalance(balanceBody []byte, mainCurrency string, holdCurrency string) (*Balance, error) {
	// Get main currency balance
	mainBalance, err := getCoinBalance(balanceBody, mainCurrency)
	if err != nil {
		return nil, fmt.Errorf("error getting %s balance: %v", mainCurrency, err)
	}

	// Get hold currency balance if specified
	var holdBalance BalanceData
	if holdCurrency != "" {
		holdBalance, err = getCoinBalance(balanceBody, holdCurrency)
		if err != nil {
			return nil, fmt.Errorf("error getting %s balance: %v", holdCurrency, err)
		}
	}

	// Convert balances to float64
	balanceFloat, err := strconv.ParseFloat(mainBalance.Balance, 64)
	if err != nil {
		return nil, fmt.Errorf("error converting %s balance: %v", mainCurrency, err)
	}

	// Get main currency's hold_trade
	mainHoldTradeFloat, err := strconv.ParseFloat(mainBalance.HoldTrade, 64)
	if err != nil {
		return nil, fmt.Errorf("error converting %s hold_trade: %v", mainCurrency, err)
	}

	// Get hold currency's hold_trade if specified
	var holdTradeFloat float64
	if holdCurrency != "" {
		holdTradeFloat, err = strconv.ParseFloat(holdBalance.HoldTrade, 64)
		if err != nil {
			return nil, fmt.Errorf("error converting %s hold_trade: %v", holdCurrency, err)
		}
	}

	// Total hold is the sum of main currency's hold_trade and hold currency's hold_trade
	totalHold := mainHoldTradeFloat + holdTradeFloat

	return &Balance{
		Currency:  mainCurrency,
		Balance:   balanceFloat,
		HoldTrade: totalHold,
		Available: balanceFloat - totalHold,
	}, nil
}

// getCoinBalance is a helper function to extract balance data from the response
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
