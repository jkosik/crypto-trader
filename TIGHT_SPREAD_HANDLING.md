# Handling Very Tight Spreads

## The Problem You Encountered

When trading ETH/BTC with `spreadAdjustFactor = 0.5`, you saw:

```
Market spread: 0.000010 (0.0326%)
Adjusted spread: 0.000010 (100.0000% of market spread)  ← Should be 50%!
Estimated profit: 0.00 BTC (0.0326% gain)                ← Should be 0.0163%!
```

**What happened?** The adjusted prices got **rounded back to market prices** due to decimal precision limits.

## Why This Happens

### Step-by-Step

1. **Market data**:
   - Bid: 0.030640 (5 decimals)
   - Ask: 0.030650 (5 decimals)
   - Spread: 0.000010
   - Center: 0.030645

2. **Calculate adjusted prices** with factor 0.5:
   ```
   Half spread: 0.000010 / 2 = 0.000005
   Adjusted half: 0.000005 × 0.5 = 0.0000025
   
   Adjusted buy:  0.030645 - 0.0000025 = 0.0306425
   Adjusted sell: 0.030645 + 0.0000025 = 0.0306475
   ```

3. **Round to 5 decimals** (Kraken's limit for ETH/BTC):
   ```
   Adjusted buy:  0.03064 (rounds to market bid!)
   Adjusted sell: 0.03065 (rounds to market ask!)
   ```

4. **Result**: You're placing orders at market bid/ask, getting **full spread** (100%), not half (50%)!

## The Fixes Applied

### Fix 1: Warning Message ⚠️

The bot now warns you when this happens:

```
⚠️  WARNING: Adjusted prices equal market prices (no spread adjustment due to rounding)
   Market spread (0.00001000) is too tight for factor 0.50 with 5 decimals
   Consider using factor 1.0 (full spread) or higher for this pair
```

### Fix 2: Untradeable Mode Decimal Handling ✅

**Before**: Untradeable mode created invalid decimals
```
0.030640 × 0.1 = 0.003064  (6 decimals - rejected by API!)
```

**After**: Respects decimal limits
```
0.030640 × 0.1 = 0.003064 → rounds to 0.00306 (5 decimals - accepted!)
```

## Solutions for Tight Spreads

### Option 1: Use Factor 1.0 (Full Spread) ⭐ Recommended

```go
const (
    spreadAdjustFactor = 1.0  // Use full market spread
)
```

**Result**:
- Buy at market bid (0.030640)
- Sell at market ask (0.030650)
- Profit: 0.000010 BTC per 1 ETH = 0.0326% gain
- **Orders fill quickly** (at market prices)

### Option 2: Extend the Spread (Factor > 1)

```go
const (
    spreadAdjustFactor = 1.5  // 50% wider than market
)
```

**Result**:
- Buy: 0.030635 (0.5 ticks below bid)
- Sell: 0.030655 (0.5 ticks above ask)
- Profit: 0.000020 BTC per 1 ETH = 0.0652% gain
- **Lower fill probability** (orders further from market)

### Option 3: Accept the Reality (Factor 0.5 = Factor 1.0)

Keep factor 0.5, but understand:
- Due to rounding, you get full spread anyway
- Effectively same as factor 1.0
- No harm, just not as tight as intended

## When Does This Happen?

### Conditions for Rounding Issues

This happens when:
```
(market_spread × factor) / 2 < 0.5 × 10^(-decimals)
```

**For ETH/BTC (5 decimals, spread 0.00001)**:
```
Factor 0.5:  (0.00001 × 0.5) / 2 = 0.0000025 < 0.000005  ← Rounds away!
Factor 1.0:  (0.00001 × 1.0) / 2 = 0.000005  = 0.000005  ← Exact!
Factor 2.0:  (0.00001 × 2.0) / 2 = 0.000010  > 0.000005  ← Works!
```

### Safe Factor Ranges by Pair

| Pair | Decimals | Typical Spread | Safe Factors |
|------|----------|----------------|--------------|
| **BTC/USD** | 1 | $10-50 | Any (0.1+) |
| **ETH/USD** | 2 | $0.50-2 | Any (0.1+) |
| **ETH/BTC** | 5 | 0.00001-0.00003 | 1.0+ only |
| **Altcoin/USD** | 4-5 | $0.0001-0.01 | 0.5+ usually OK |

## Recommendations by Pair Type

### Major Pairs with Tight Spreads (BTC/USD, ETH/USD)

```go
const (
    minSpreadPercent   = 0.0
    minVolume24h       = 1000000.0
    maxADX             = 20.0
    spreadAdjustFactor = 1.0    // Full spread - tight enough already
)
```

### Crypto-to-Crypto Pairs (ETH/BTC, SOL/BTC)

```go
const (
    minSpreadPercent   = 0.0
    minVolume24h       = 100.0
    maxADX             = 20.0
    spreadAdjustFactor = 1.0    // Full spread - avoid rounding issues
)
```

**Why 1.0?**
- Spreads are already very tight (0.03%)
- Factor < 1.0 rounds back to market prices anyway
- Factor 1.0 = market bid/ask = highest fill probability

### Altcoins with Wide Spreads (SUNDOG/USD, GHIBLI/USD)

```go
const (
    minSpreadPercent   = 0.0
    minVolume24h       = 1000.0
    maxADX             = 20.0
    spreadAdjustFactor = 0.7    // Narrow the spread - more fills
)
```

**Why 0.7?**
- Spreads are wide (0.5-5%)
- Factor 0.7 = significant narrowing
- Higher fill probability

## Your Specific Case: ETH/BTC

### Current Issue
```
Market spread: 0.000010 (0.0326%)
Factor: 0.5
Result: Rounds to market prices (no narrowing)
Profit: ~0 BTC (rounds to 0.00)
```

### Recommended Fix

```go
const (
    minSpreadPercent   = 0.0    // Accept any spread
    minVolume24h       = 100.0  // 100 BTC/day is fine for ETH/BTC
    maxADX             = 20.0   // Keep (wait for weak trend)
    spreadAdjustFactor = 1.0    // Use FULL spread (not 0.5)
)
```

### Expected Result

```
Market spread: 0.000010 (0.0326%)
Factor: 1.0
Adjusted buy:  0.030640 (market bid)
Adjusted sell: 0.030650 (market ask)
Profit: 0.000010 BTC per 1 ETH traded

For -volume 0.01:
Profit: 0.00000010 BTC = ~$0.0095 per trade
```

## Zero-Fee Strategy for Tight Spreads

If you have **zero fees** (market maker program):

### High Velocity Strategy

```go
spreadAdjustFactor = 1.0  // Full spread, very high fill rate
```

**Expected**:
- 50-100 fills/day (orders at market prices)
- $0.0095 × 75 fills = **~$0.71/day** per 0.01 ETH

### Lower Velocity, Higher Profit

```go
spreadAdjustFactor = 2.0  // Double spread, lower fill rate
```

**Expected**:
- 5-10 fills/day (orders further from market)
- $0.019 × 7 fills = **~$0.13/day** per 0.01 ETH

**Verdict**: Factor 1.0 (high velocity) wins! 🚀

## Testing Your Fix

After changing to `spreadAdjustFactor = 1.0`, you should see:

```
🔄 Placing spread orders for ETH/BTC:
Market spread: 0.000010 (0.0326%)
Spread adjust factor: 1.00
Adjusted buy price: 0.030640   (market bid)
Adjusted sell price: 0.030650  (market ask)
Adjusted spread: 0.000010 (100.00% of market spread)
Estimated profit: 0.00001 BTC (0.0326% gain)  ← Shows correctly now!
```

No warning message = working as intended! ✅

## Summary

**For ETH/BTC and other tight-spread pairs:**
- ✅ Use `spreadAdjustFactor = 1.0` (full market spread)
- ❌ Don't use factor < 1.0 (rounds back to market anyway)
- ✅ Untradeable mode now respects decimal limits
- ✅ Bot warns when rounding negates adjustment

**Your corrected config:**
```go
minSpreadPercent   = 0.0   // Accept tight spreads
minVolume24h       = 100.0 // ETH/BTC has ~167 BTC/day
maxADX             = 20.0  // Wait for ranging market  
spreadAdjustFactor = 1.0   // Full spread (0.5 doesn't work due to rounding)
```
