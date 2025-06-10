#!/bin/bash

# TrustDrop Production Build Script - macOS App Bundle
# Creates a proper .app bundle ready for distribution

set -e  # Exit on any error

echo "TrustDrop Production Build - macOS"
echo "================================="

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# App info
APP_NAME="TrustDrop"
VERSION=$(date +%Y%m%d_%H%M%S)
BUNDLE_ID="com.trustdrop.app"

echo -e "${BLUE}Building: ${APP_NAME} v${VERSION}${NC}"

# Clean previous builds
echo -e "${YELLOW}Cleaning previous builds...${NC}"
rm -rf ${APP_NAME}.app
rm -f ${APP_NAME}
rm -f ${APP_NAME}_*

# Create app bundle structure
echo -e "${YELLOW}Creating app bundle structure...${NC}"
mkdir -p "${APP_NAME}.app/Contents/MacOS"
mkdir -p "${APP_NAME}.app/Contents/Resources"

# Build the binary
echo -e "${YELLOW}Building ${APP_NAME}...${NC}"
go build -v -ldflags="-s -w -X main.appName=${APP_NAME} -X main.version=${VERSION}" -o "${APP_NAME}.app/Contents/MacOS/${APP_NAME}" .

if [ $? -ne 0 ]; then
    echo -e "${RED}Build failed${NC}"
    exit 1
fi

# Convert PNG to ICNS for macOS icon
echo -e "${YELLOW}Creating app icon...${NC}"
if command -v sips >/dev/null 2>&1; then
    # Create iconset directory
    mkdir -p "${APP_NAME}.iconset"
    
    # Generate different icon sizes
    sips -z 16 16 image.png --out "${APP_NAME}.iconset/icon_16x16.png" >/dev/null 2>&1
    sips -z 32 32 image.png --out "${APP_NAME}.iconset/icon_16x16@2x.png" >/dev/null 2>&1
    sips -z 32 32 image.png --out "${APP_NAME}.iconset/icon_32x32.png" >/dev/null 2>&1
    sips -z 64 64 image.png --out "${APP_NAME}.iconset/icon_32x32@2x.png" >/dev/null 2>&1
    sips -z 128 128 image.png --out "${APP_NAME}.iconset/icon_128x128.png" >/dev/null 2>&1
    sips -z 256 256 image.png --out "${APP_NAME}.iconset/icon_128x128@2x.png" >/dev/null 2>&1
    sips -z 256 256 image.png --out "${APP_NAME}.iconset/icon_256x256.png" >/dev/null 2>&1
    sips -z 512 512 image.png --out "${APP_NAME}.iconset/icon_256x256@2x.png" >/dev/null 2>&1
    sips -z 512 512 image.png --out "${APP_NAME}.iconset/icon_512x512.png" >/dev/null 2>&1
    sips -z 1024 1024 image.png --out "${APP_NAME}.iconset/icon_512x512@2x.png" >/dev/null 2>&1
    
    # Create ICNS file
    if command -v iconutil >/dev/null 2>&1; then
        iconutil -c icns "${APP_NAME}.iconset" -o "${APP_NAME}.app/Contents/Resources/${APP_NAME}.icns"
        rm -rf "${APP_NAME}.iconset"
        echo -e "${GREEN}App icon created successfully${NC}"
    else
        echo -e "${YELLOW}iconutil not found, skipping icon creation${NC}"
    fi
else
    echo -e "${YELLOW}sips not found, skipping icon creation${NC}"
fi

# Create Info.plist
cat > "${APP_NAME}.app/Contents/Info.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>${APP_NAME}</string>
    <key>CFBundleIconFile</key>
    <string>${APP_NAME}</string>
    <key>CFBundleIdentifier</key>
    <string>${BUNDLE_ID}</string>
    <key>CFBundleName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleDisplayName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleVersion</key>
    <string>${VERSION}</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleSignature</key>
    <string>TRDR</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.15</string>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>LSUIElement</key>
    <false/>
</dict>
</plist>
EOF

# Prepare Windows icon resources (for cross-compilation)
echo -e "${YELLOW}Preparing Windows icon resources...${NC}"
if [ -f "image.png" ]; then
    if command -v magick >/dev/null 2>&1; then
        # Generate ICO file for Windows
        magick image.png -resize 16x16 temp16.png
        magick image.png -resize 32x32 temp32.png  
        magick image.png -resize 48x48 temp48.png
        magick image.png -resize 256x256 temp256.png
        magick temp16.png temp32.png temp48.png temp256.png icon.ico
        rm temp16.png temp32.png temp48.png temp256.png
        echo -e "${GREEN}Windows icon (icon.ico) created${NC}"
        
        # Also prepare the rsrc tool for Windows builds
        if [ ! -f "$HOME/go/bin/rsrc" ]; then
            echo -e "${YELLOW}Installing rsrc tool for Windows builds...${NC}"
            go install github.com/akavel/rsrc@latest
        fi
        echo -e "${GREEN}Windows build tools ready${NC}"
    else
        echo -e "${YELLOW}ImageMagick not found, skipping Windows icon generation${NC}"
        echo -e "${YELLOW}Install with: brew install imagemagick${NC}"
    fi
else
    echo -e "${YELLOW}image.png not found, skipping Windows icon generation${NC}"
fi

echo -e "${GREEN}Build successful!${NC}"

# Get file size
SIZE=$(du -h "${APP_NAME}.app" | tail -1 | cut -f1)
echo -e "${GREEN}   App Bundle Size: ${SIZE}${NC}"

# Make executable
chmod +x "${APP_NAME}.app/Contents/MacOS/${APP_NAME}"

echo ""
echo "TrustDrop.app is ready!"
echo "To run: Double-click TrustDrop.app or open TrustDrop.app"
echo "Downloads will be saved to: ~/Documents/TrustDrop Downloads/received"
echo ""
echo "Installation: Drag TrustDrop.app to /Applications folder"