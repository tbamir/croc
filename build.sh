#!/bin/bash

# TrustDrop Build Script
# Builds the main application and ledger viewer tool

set -e

echo "Building TrustDrop..."
echo "===================="

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed. Please install Go 1.21 or later."
    exit 1
fi

# Get dependencies
echo "Getting dependencies..."
go mod download

# Build main application
echo "Building TrustDrop..."
go build -o trustdrop main.go

# Build ledger viewer
echo "Building ledger viewer..."
mkdir -p cmd/ledger-viewer
go build -o ledger-viewer cmd/ledger-viewer/main.go

# Platform-specific builds
if [[ "$OSTYPE" == "darwin"* ]]; then
    echo "Building macOS app..."
    go build -o TrustDrop.app/Contents/MacOS/trustdrop main.go
elif [[ "$OSTYPE" == "msys" ]] || [[ "$OSTYPE" == "win32" ]]; then
    echo "Building Windows exe..."
    go build -o trustdrop.exe main.go
    go build -o ledger-viewer.exe cmd/ledger-viewer/main.go
fi

echo ""
echo "Build complete!"
echo ""
echo "Executables created:"
echo "  - trustdrop (main application)"
echo "  - ledger-viewer (blockchain viewer tool)"
echo ""
echo "To run TrustDrop: ./trustdrop"
echo "To view ledger: ./ledger-viewer -help"