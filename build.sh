#!/bin/bash

# TrustDrop Production Build Script
# Creates a clean application binary ready for distribution

set -e  # Exit on any error

echo "🚀 TrustDrop Production Build"
echo "=============================="

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# App info
APP_NAME="TrustDrop"
VERSION=$(date +%Y%m%d_%H%M%S)

echo -e "${BLUE}Building: ${APP_NAME} v${VERSION}${NC}"

# Clean previous builds
echo -e "${YELLOW}🧹 Cleaning previous builds...${NC}"
rm -f ${APP_NAME}
rm -f ${APP_NAME}.exe
rm -f ${APP_NAME}_*

echo -e "${YELLOW}🔨 Building ${APP_NAME}...${NC}"

# Build for current platform with optimizations
go build -v -ldflags="-s -w -X main.appName=${APP_NAME} -X main.version=${VERSION}" -o "${APP_NAME}" .

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Build successful!${NC}"
    
    # Get file size
    SIZE=$(du -h "${APP_NAME}" | cut -f1)
    echo -e "${GREEN}   📦 Size: ${SIZE}${NC}"
    
    # Make executable
    chmod +x "${APP_NAME}"
    
    echo -e "\n${GREEN}🎉 ${APP_NAME} is ready!${NC}"
    echo -e "${GREEN}💡 To run: ./${APP_NAME}${NC}"
    echo -e "${GREEN}📂 Downloads will be saved to: ~/Documents/TrustDrop Downloads/data/received${NC}"
else
    echo -e "${RED}❌ Build failed${NC}"
    exit 1
fi