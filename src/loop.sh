#!/bin/bash

# runs crypto-trader in a loop and opens consecutive trades after the previous one is closed
# run as ./loop.sh SOL 25

COIN=$1
VOLUME=$2

report="trades-$COIN-$(date +%F-%H-%M).txt"

for i in {1..50}; do
    echo "Running iteration $i"
    go run . -coin $COIN -order -volume $VOLUME || { echo "Iteration $i failed at $(date)"; exit 1; }
    echo "$(date) - SUCCESSFUL TRADE $i" >> $report
done
