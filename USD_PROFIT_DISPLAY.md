# USD Profit Display Feature

## Overview

All profit displays now show **both the quote currency and USD equivalent** for easy tracking.

## How It Works

### For Non-USD Pairs (e.g., ETH/BTC)

Shows profit in both quote currency and USD:
```
Estimated profit: 0.00000010 BTC ($0.0078) (0.0326% gain)
Actual profit: 0.00000012 BTC ($0.0094) (0.0391% gain)
Net profit (after fees): 0.00000010 BTC ($0.0078)
```

### For USD Pairs (e.g., BTC/USD, ETH/USD)

Shows only USD (no redundant conversion):
```
Estimated profit: $5.25 (0.0326% gain)
Actual profit: $5.50 (0.0342% gain)
Net profit (after fees): $5.30
```

## Examples by Trading Pair

### ETH/BTC Trading

**Console Output:**
```
🔄 Placing spread orders for ETH/BTC:
Volume: 0.01000
Market bid price: 0.030640
Market ask price: 0.030650
Market spread: 0.000010 (0.0326%)
Spread adjust factor: 1.00
Center price: 0.030645
Adjusted buy price: 0.030640
Adjusted sell price: 0.030650
Adjusted spread: 0.000010 (100.00% of market spread)
Estimated profit: 0.00000010 BTC ($0.0078) (0.0326% gain)  ← Shows both!
```

**Slack Notification:**
```
✅ Trade ETH/BTC executed
Volume: 0.01000
Buy price: 0.030640
Sell price: 0.030650
Actual profit: 0.00000010 BTC ($0.0078) (0.0326% gain)     ← Shows both!
Fees: 0.00000002 BTC ($0.0016) (Buy: 0.00000001, Sell: 0.00000001 BTC)
Net profit: 0.00000008 BTC ($0.0062)                       ← Shows both!
```

### BTC/USD Trading

**Console Output:**
```
🔄 Placing spread orders for BTC/USD:
Volume: 0.00100
Market bid price: 78000.0
Market ask price: 78010.0
Market spread: 10.0 (0.0128%)
Estimated profit: $0.01 (0.0128% gain)  ← USD only
```

**Slack Notification:**
```
✅ Trade BTC/USD executed
Volume: 0.00100
Buy price: 78000.0
Sell price: 78010.0
Actual profit: $0.01 (0.0128% gain)     ← USD only
Net profit: $0.01                       ← USD only
```

### SOL/ETH Trading

**Console Output:**
```
Estimated profit: 0.00050000 ETH ($1.1843) (0.5% gain)  ← Shows both!
```

## Implementation Details

### Simple API

Added `internal/kraken/usd.go` with two functions:

#### 1. GetUSDPrice(currency string)
Fetches current USD exchange rate for any currency:
```go
btcPrice, err := kraken.GetUSDPrice("BTC")
// Returns: 78013.10 (current BTC/USD price)

usdPrice, err := kraken.GetUSDPrice("USD")
// Returns: 1.0 (USD is already USD)
```

#### 2. FormatProfitWithUSD(profit float64, currency string)
Formats profit with USD equivalent:
```go
kraken.FormatProfitWithUSD(0.00000010, "BTC")
// Returns: "0.00000010 BTC ($0.0078)"

kraken.FormatProfitWithUSD(5.25, "USD")
// Returns: "$5.25"
```

## Where It's Applied

✅ **Estimated profit** (console)  
✅ **Estimated profit** (Slack)  
✅ **Actual profit** (console)  
✅ **Actual profit** (Slack)  
✅ **Total fees** (console)  
✅ **Total fees** (Slack)  
✅ **Net profit** (console)  
✅ **Net profit** (Slack)  

## Benefits

### 1. Easy Profit Tracking
```
Before: "0.00000010 BTC" (need calculator to know USD value)
After:  "0.00000010 BTC ($0.0078)" (instant USD understanding)
```

### 2. Compare Across Pairs
```
ETH/BTC: 0.00000010 BTC ($0.0078) per trade
BTC/USD: $0.01 per trade
SOL/USD: $0.05 per trade

Now you can easily see: SOL/USD > BTC/USD > ETH/BTC in profit per trade
```

### 3. Real-Time USD Conversion
Uses **live Kraken prices** at the moment of display, so USD values are always current.

### 4. No Configuration Needed
Automatically detects quote currency and converts as needed. Works for any pair!

## Error Handling

If USD price is unavailable (e.g., API error, unsupported pair):
```
0.00000010 OBSCURECOIN (USD price unavailable)
```

Gracefully degrades - you still see the profit in native currency.

## API Calls

The implementation is efficient:
- **0 extra API calls** for USD pairs (BTC/USD, ETH/USD, etc.)
- **1 extra API call** per non-USD pair display (e.g., ETH/BTC → fetches BTC/USD once)
- Kraken's public API is used (no authentication needed, high rate limits)

## Supported Quote Currencies

Works with any quote currency that has a /USD trading pair on Kraken:

✅ BTC → BTC/USD  
✅ ETH → ETH/USD  
✅ SOL → SOL/USD  
✅ USDT → USDT/USD  
✅ USDC → USDC/USD  
✅ EUR → EUR/USD  
✅ Any crypto with USD pair  

## Technical Notes

### Why It's Simple

1. **Single file added** (`internal/kraken/usd.go` - 70 lines)
2. **No external dependencies** (uses standard Go + existing Kraken API patterns)
3. **No configuration** (automatic detection)
4. **No breaking changes** (existing code still works)

### Precision

- Native currency: 8 decimals (crypto standard)
- USD conversion: 4 decimals (e.g., $0.0078)
- For USD-only: 2 decimals (e.g., $5.25)

## Real-World Examples

### Small Volume ETH/BTC Trade (0.01 ETH)

**Before:**
```
Estimated profit: 0.00000010 BTC (0.0326% gain)
→ Need to check: "What's 0.0000001 BTC in USD?"
→ Manual calculation: ~$0.0078
```

**After:**
```
Estimated profit: 0.00000010 BTC ($0.0078) (0.0326% gain)
→ Instant understanding: "Less than a penny per trade"
```

### High-Frequency Trading (100 trades/day)

**Before:**
```
Trade 1: 0.00000010 BTC
Trade 2: 0.00000010 BTC
...
→ Daily total: Need spreadsheet to sum and convert
```

**After:**
```
Trade 1: 0.00000010 BTC ($0.0078)
Trade 2: 0.00000010 BTC ($0.0078)
...
→ Daily total: ~$0.78/day (instant mental math)
```

## Summary

Simple solution that adds massive convenience:
- ✅ 1 new file (70 lines)
- ✅ Real-time USD conversion
- ✅ Works with all pairs
- ✅ Minimal API overhead
- ✅ Graceful error handling
- ✅ No configuration needed

**Result: You always know your profit in USD!** 💰
