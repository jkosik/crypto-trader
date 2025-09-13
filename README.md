# Crypto Trader

A trading bot for cryptocurrency markets that executes trades based on price spreads.

## Features

- Automated trading based on price spreads
- Support for multiple cryptocurrencies
- Price monitoring and limit order placement
- Narrowing spread to increase probability of trades to be executed
- Detailed logging and reporting

## What is spread trading
Any exchange (crypto or stock) joins buyers who are placing the **buy orders for the Bid price** and sellers who are placing **sell orders for the Ask price**. Bid and Ask price oscilate around **mid price, which can be considered as the market price**. All bids and asks are collected in the **order book** of the exchanage and wait for execution. When the Bid or Ask price is far away frome the market price, the order may be never executed.

**Spread** is the difference between the Bid and Ask price closest to the mid price. These prices has the highest probability of being executed. Spread size depends on market conditions, asset volatility and liquidity and is mostly between 0.01 - 0.5%.

### Example:
![Spread](readme/spread.png)

Red: Ask price (Sell orders) 10.279
Green: Bid price (Buy orders) 10.276

Ask price is always higher than the market price. Sellers (asset owners) want to sell for a higher price.
Bid price is always lower than the market price. Buyers want always to buy cheaper.

The logic behind the spread trading is mimicking the buyers and sellers - buy slightly below the mid price and sell slightly above the mid price and profit based on small price movements.
There are also some risks associated (e.g. sudden market volatility, trading fees etc.)

## Setup

1. Generate API key in the Settings of your Kraken profile with the following permissions:
   - `Query`
   - `Query open orders & trades`
   - `Query closed orders & trades`
   - `Create & modify orders`

2. Export your API credentials:
   ```bash
   export KRAKEN_API_KEY=your_api_key
   export KRAKEN_PRIVATE_KEY=your_private_key
   export SLACK_WEBHOOK=your_webhook_url  # Optional
   ```

3. Build the binaries:
   ```bash
   go mod tidy
   go build -o bin/trader ./cmd/trader
   go build -o bin/loop ./cmd/loop
   ```

## Usage

### Trader Bot
Execute single trade:
```bash
go run cmd/trader/main.go -coin <COIN> -volume <AMOUNT> [-order] [-untradeable] [-editorder]
```

#### Examples of a single trade
```bash
# Simulate a trade without actually placing orders (to see balance and asset codes)
go run cmd/trader/main.go -coin GHIBLI -volume 3000.0

# Place a real trade
go run cmd/trader/main.go -coin GHIBLI -volume 3000.0 -order


# Place untradeable orders in extreme prices (for testing)
go run cmd/trader/main.go -coin GHIBLI -volume 3000.0 -order -untradeable
```

### Loop Bot
Execute trades in a loop:
```bash
cd cmd/loop
go run main.go -coin <COIN> -volume <AMOUNT> [-iterations <NUMBER>]
```

### Flags
- `-coin`: Base coin to trade (e.g. BTC, SOL)
- `-volume`: Base coin volume to trade
- `-order`: Place actual orders (default: false)
- `-untradeable`: Place orders at untradeable prices (orders won't be executed - close them manually)

Further tuning can be done in `cmd/trader/main.go`:
- minSpreadPercent   = 0.5    // Minimum spread percentage required to place orders
- minVolume24h       = 100000 // Minimum 24h volume in USD required to place orders
- spreadNarrowFactor = 0.7    // How much to narrow the spread (0.0 to 1.0)

## Utils
```
go run cmd/utils/check-balance.go
go run cmd/utils/volume-spread-scanner.go
```

### Trading Strategy
The bot uses a fixed spread narrowing factor of 0.7 (70%) to place orders closer to the center price. This means:
- Buy orders are placed 70% of the way from the bid price towards the center price
- Sell orders are placed 70% of the way from the ask price towards the center price
- This helps increase the probability of order execution while maintaining a profitable spread


## Asset Codes
Trading API endpoints use human
Some Kraken API endpoints needs conversion from human-readable codes to asset codes. For example:
- BTC → XBT.F
- ETH → ETH
- SOL → SOL.F
- SUNDOG → SUNDOG

If unsure, dry-run the crypto-trader by omittun `-order` flag and check the balance JSON output.
Add the pair to the `KrakenAssetCode` function in `internal/kraken/api.go` if needed.

Example:
```go
func KrakenAssetCode(standardCode string) (string, error) {
    hardcodedMap := map[string]string{
        "BTC":    "XBT.F",
        "ETH":    "ETH",
        "SOL":    "SOL.F",
        "SUNDOG": "SUNDOG",
        "TRUMP":  "TRUMP",
        "GUN":    "GUN",
        "OCEAN":  "OCEAN",
        "GHIBLI": "GHIBLI",
    }
    // Add your new pair here
    // ...
}
```
