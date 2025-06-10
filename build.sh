#!/bin/bash

# TrustDrop Bulletproof Build Script
# Builds for Mac, Linux, and Windows

set -e  # Exit on any error

echo "🚀 TrustDrop Bulletproof Edition - Build Script"
echo "================================================"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Get version info
VERSION=$(date +%Y%m%d_%H%M%S)
BUILD_DIR="build"
APP_NAME="trustdrop-bulletproof"

echo -e "${BLUE}Version: ${VERSION}${NC}"
echo -e "${BLUE}Building: ${APP_NAME}${NC}"

# Clean previous builds
echo -e "${YELLOW}🧹 Cleaning previous builds...${NC}"
rm -rf ${BUILD_DIR}
mkdir -p ${BUILD_DIR}

# Function to build for a specific platform
build_platform() {
    local GOOS=$1
    local GOARCH=$2
    local PLATFORM_NAME=$3
    local EXT=$4
    
    echo -e "${BLUE}🔨 Building for ${PLATFORM_NAME}...${NC}"
    
    OUTPUT_NAME="${BUILD_DIR}/${APP_NAME}_${PLATFORM_NAME}_${VERSION}${EXT}"
    
    # Enable CGO only for current platform, disable for cross-compilation
    if [[ "${GOOS}" == "$(go env GOOS)" && "${GOARCH}" == "$(go env GOARCH)" ]]; then
        # Native build - keep CGO enabled for full Fyne support
        env GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=1 go build -v -ldflags="-s -w" -o "${OUTPUT_NAME}" .
    else
        # Cross-compilation - disable CGO
        env GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=0 go build -v -ldflags="-s -w" -o "${OUTPUT_NAME}" .
    fi
    
    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✅ ${PLATFORM_NAME} build successful: ${OUTPUT_NAME}${NC}"
        
        # Get file size
        SIZE=$(du -h "${OUTPUT_NAME}" | cut -f1)
        echo -e "${GREEN}   📦 Size: ${SIZE}${NC}"
        
        # Make executable on Unix systems
        if [[ "${GOOS}" != "windows" ]]; then
            chmod +x "${OUTPUT_NAME}"
        fi
    else
        echo -e "${RED}❌ ${PLATFORM_NAME} build failed${NC}"
        return 1
    fi
}

# Build for current platform first (Mac)
echo -e "${YELLOW}Building for current platform...${NC}"
build_platform "$(go env GOOS)" "$(go env GOARCH)" "current" ""

# Build for other platforms
echo -e "${YELLOW}Building for other platforms...${NC}"

# macOS (if not already built)
if [[ "$(go env GOOS)" != "darwin" ]]; then
    build_platform "darwin" "amd64" "macos_intel" ""
    build_platform "darwin" "arm64" "macos_apple_silicon" ""
fi

# Linux
build_platform "linux" "amd64" "linux_x64" ""
build_platform "linux" "arm64" "linux_arm64" ""

# Windows (Note: CGO might not work for cross-compilation)
echo -e "${YELLOW}🪟 Attempting Windows cross-compilation...${NC}"
echo -e "${YELLOW}Note: Windows build may require CGO_ENABLED=0 for cross-compilation${NC}"

# Try with CGO disabled for Windows
env GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -v -ldflags="-s -w" -o "${BUILD_DIR}/${APP_NAME}_windows_x64_${VERSION}.exe" .

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✅ Windows x64 build successful (CGO disabled)${NC}"
    SIZE=$(du -h "${BUILD_DIR}/${APP_NAME}_windows_x64_${VERSION}.exe" | cut -f1)
    echo -e "${GREEN}   📦 Size: ${SIZE}${NC}"
else
    echo -e "${RED}❌ Windows x64 build failed${NC}"
    echo -e "${YELLOW}💡 For full Windows support with CGO, build on Windows or use cross-compilation tools${NC}"
fi

# Create archive with all builds
echo -e "${YELLOW}📦 Creating release archive...${NC}"
ARCHIVE_NAME="${APP_NAME}_multi_platform_${VERSION}.tar.gz"
cd ${BUILD_DIR}
tar -czf "${ARCHIVE_NAME}" ${APP_NAME}_*
cd ..

echo -e "${GREEN}✅ Release archive created: ${BUILD_DIR}/${ARCHIVE_NAME}${NC}"

# Summary
echo -e "\n${GREEN}🎉 Build Summary${NC}"
echo -e "${GREEN}===============${NC}"
echo -e "${GREEN}Built files:${NC}"
ls -la ${BUILD_DIR}/${APP_NAME}_*

echo -e "\n${BLUE}📋 Installation Instructions:${NC}"
echo -e "${BLUE}Mac/Linux: chmod +x filename && ./filename${NC}"
echo -e "${BLUE}Windows: Double-click the .exe file${NC}"

echo -e "\n${GREEN}🔧 For testing Mac ↔ PC transfers:${NC}"
echo -e "${GREEN}1. Run the appropriate binary on each machine${NC}"
echo -e "${GREEN}2. On sender: Choose 'Send Files' and select files/folders${NC}"
echo -e "${GREEN}3. Share the transfer code with receiver${NC}"
echo -e "${GREEN}4. On receiver: Choose 'Receive Files' and enter the code${NC}"

echo -e "\n${GREEN}✨ TrustDrop Bulletproof is ready for testing!${NC}"