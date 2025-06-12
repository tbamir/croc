#!/bin/bash

echo "Building TrustDrop for multiple platforms..."

# Install required tools for cross-compilation
echo "Installing fyne package tool..."
go install fyne.io/fyne/v2/cmd/fyne@latest

# Windows 64-bit with CGO
echo "Building for Windows..."
CC=x86_64-w64-mingw32-gcc \
CXX=x86_64-w64-mingw32-g++ \
GOOS=windows \
GOARCH=amd64 \
CGO_ENABLED=1 \
go build -ldflags="-w -s -H windowsgui" -o builds/TrustDrop.exe

# Check if Windows build succeeded
if [ ! -f builds/TrustDrop.exe ]; then
    echo "❌ Windows build failed, trying alternative method..."
    
    # Try using fyne package command
    fyne package -os windows -icon icon.png -name TrustDrop
    mv TrustDrop.exe builds/ 2>/dev/null || echo "Fyne package also failed"
fi

# macOS (your current platform) - should work fine
echo "Building for macOS..."
GOOS=darwin GOARCH=amd64 go build -ldflags="-w -s" -o builds/TrustDrop-macos

echo "Build results:"
ls -la builds/