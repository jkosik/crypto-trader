#!/bin/bash

# Create bin directory if it doesn't exist
mkdir -p bin

# Build trader
echo "Building trader..."
go build -o bin/trader ./cmd/trader

# Build loop
echo "Building loop..."
go build -o bin/loop ./cmd/loop


echo "Build complete! Binaries are in the bin/ directory"