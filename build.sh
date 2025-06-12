#!/bin/bash

# TrustDrop Bulletproof Edition - Production Build Script for macOS
# Creates a proper .app bundle with code signing to avoid quarantine issues

set -e  # Exit on any error

echo "🚀 TrustDrop - macOS Build"
echo "=========================="

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
RED='\033[0;31m'
PURPLE='\033[0;35m'
NC='\033[0m' # No Color

# App configuration
APP_NAME="TrustDrop"
DISPLAY_NAME="TrustDrop"
VERSION="1.0.0"
BUILD_VERSION=$(date +%Y%m%d_%H%M%S)
BUNDLE_ID="com.trustdrop.app"

echo -e "${BLUE}Building: ${DISPLAY_NAME} v${VERSION} (${BUILD_VERSION})${NC}"
echo -e "${BLUE}Bundle ID: ${BUNDLE_ID}${NC}"

# Clean previous builds
echo -e "${YELLOW}🧹 Cleaning previous builds...${NC}"
rm -rf "${APP_NAME}.app"
rm -f "${APP_NAME}"
rm -f "${APP_NAME}_"*
rm -f *.dmg
rm -rf build/


# Create build directory structure
echo -e "${YELLOW}📁 Creating app bundle structure...${NC}"
mkdir -p "build/${APP_NAME}.app/Contents/MacOS"
mkdir -p "build/${APP_NAME}.app/Contents/Resources"

# Check for app icon
echo -e "${YELLOW}🎨 Checking for app icon...${NC}"
if [ -f "image.png" ]; then
    echo -e "${GREEN}✅ Found image.png for app icon${NC}"
else
    echo -e "${RED}❌ image.png not found - please add your app icon as image.png${NC}"
    exit 1
fi

# Build the main binary with optimizations
echo -e "${YELLOW}🔨 Building ${APP_NAME} binary...${NC}"
export CGO_ENABLED=1
export GOOS=darwin

# Build the binary
go build -v \
    -ldflags "-s -w" \
    -o "build/${APP_NAME}.app/Contents/MacOS/${APP_NAME}" \
    main.go

if [ $? -ne 0 ]; then
    echo -e "${RED}❌ Build failed${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Binary built successfully${NC}"

# Create macOS icon set properly
echo -e "${YELLOW}🎨 Setting up app icon...${NC}"

# Create macOS icon set if tools are available
if command -v sips >/dev/null 2>&1 && command -v iconutil >/dev/null 2>&1; then
    echo -e "${BLUE}   Creating proper macOS icon...${NC}"
    mkdir -p "${APP_NAME}.iconset"
    
    # Generate icon sizes - all required sizes for macOS
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
    
    # Create ICNS file with the proper name
    if iconutil -c icns "${APP_NAME}.iconset" -o "app_icon.icns"; then
        mv "app_icon.icns" "build/${APP_NAME}.app/Contents/Resources/app_icon.icns"
        rm -rf "${APP_NAME}.iconset"
        echo -e "${GREEN}✅ macOS icon (.icns) created successfully${NC}"
    else
        echo -e "${YELLOW}⚠️  iconutil failed, using PNG fallback${NC}"
        cp image.png "build/${APP_NAME}.app/Contents/Resources/app_icon.icns"
    fi
else
    echo -e "${YELLOW}⚠️  Icon tools not available, using PNG fallback${NC}"
    cp image.png "build/${APP_NAME}.app/Contents/Resources/app_icon.icns"
fi

# Create Info.plist
echo -e "${YELLOW}📄 Creating Info.plist...${NC}"
cat > "build/${APP_NAME}.app/Contents/Info.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>${APP_NAME}</string>
    <key>CFBundleIconFile</key>
    <string>app_icon</string>
    <key>CFBundleIdentifier</key>
    <string>${BUNDLE_ID}</string>
    <key>CFBundleName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleDisplayName</key>
    <string>${DISPLAY_NAME}</string>
    <key>CFBundleVersion</key>
    <string>${BUILD_VERSION}</string>
    <key>CFBundleShortVersionString</key>
    <string>${VERSION}</string>
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
    <key>NSAppTransportSecurity</key>
    <dict>
        <key>NSAllowsArbitraryLoads</key>
        <true/>
    </dict>
    <key>NSNetworkVolumesUsageDescription</key>
    <string>TrustDrop needs network access for secure P2P file transfers.</string>
    <key>NSDownloadsFolderUsageDescription</key>
    <string>TrustDrop saves received files to your Downloads folder.</string>
    <key>NSDocumentsFolderUsageDescription</key>
    <string>TrustDrop may save files to your Documents folder.</string>
    <key>NSDesktopFolderUsageDescription</key>
    <string>TrustDrop may save files to your Desktop.</string>
    <key>NSSupportsAutomaticGraphicsSwitching</key>
    <true/>
</dict>
</plist>
EOF

# Make binary executable
chmod +x "build/${APP_NAME}.app/Contents/MacOS/${APP_NAME}"

# Try to code sign the app to avoid quarantine issues
echo -e "${YELLOW}🔐 Attempting to code sign app...${NC}"
SIGNING_IDENTITY=""

# Check for available signing identities
if command -v security >/dev/null 2>&1; then
    # Look for Developer ID Application certificates
    DEVELOPER_CERTS=$(security find-identity -v -p codesigning | grep "Developer ID Application" | head -1)
    if [ ! -z "$DEVELOPER_CERTS" ]; then
        SIGNING_IDENTITY=$(echo "$DEVELOPER_CERTS" | sed 's/.*"\(.*\)".*/\1/')
        echo -e "${GREEN}📝 Found Developer ID: ${SIGNING_IDENTITY}${NC}"
    else
        # Look for any valid codesigning certificate
        ANY_CERT=$(security find-identity -v -p codesigning | grep -E "(iPhone|Apple|Developer|Mac)" | head -1)
        if [ ! -z "$ANY_CERT" ]; then
            SIGNING_IDENTITY=$(echo "$ANY_CERT" | sed 's/.*"\(.*\)".*/\1/')
            echo -e "${YELLOW}📝 Using certificate: ${SIGNING_IDENTITY}${NC}"
        fi
    fi
fi

if [ ! -z "$SIGNING_IDENTITY" ]; then
    echo -e "${BLUE}🔐 Code signing with: ${SIGNING_IDENTITY}${NC}"
    
    # Sign the main executable first with timeout
    timeout 10 codesign --force --sign "${SIGNING_IDENTITY}" \
        "build/${APP_NAME}.app/Contents/MacOS/${APP_NAME}" >/dev/null 2>&1
    EXEC_SIGN_RESULT=$?
    
    # Then sign the entire app bundle with timeout
    timeout 10 codesign --force --sign "${SIGNING_IDENTITY}" \
        "build/${APP_NAME}.app" >/dev/null 2>&1
    APP_SIGN_RESULT=$?
    
    if [ $EXEC_SIGN_RESULT -eq 0 ] && [ $APP_SIGN_RESULT -eq 0 ]; then
        echo -e "${GREEN}✅ Code signing successful!${NC}"
        echo -e "${GREEN}🎉 App should NOT require quarantine removal${NC}"
    elif [ $EXEC_SIGN_RESULT -eq 124 ] || [ $APP_SIGN_RESULT -eq 124 ]; then
        echo -e "${YELLOW}⚠️  Code signing timed out (keychain access may be required)${NC}"
        echo -e "${YELLOW}⚠️  App may require quarantine removal${NC}"
    else
        echo -e "${YELLOW}⚠️  Code signing failed, app may require quarantine removal${NC}"
    fi
else
    echo -e "${YELLOW}⚠️  No code signing certificate found${NC}"
    echo -e "${YELLOW}⚠️  App may trigger quarantine and require manual removal${NC}"
fi

# Move final app bundle to root directory
echo -e "${YELLOW}📦 Finalizing app bundle...${NC}"
mv "build/${APP_NAME}.app" .

# Clean up temporary files
rm -rf build/

# Get app bundle size
SIZE=$(du -h "${APP_NAME}.app" | tail -1 | cut -f1)
BINARY_SIZE=$(du -h "${APP_NAME}.app/Contents/MacOS/${APP_NAME}" | tail -1 | cut -f1)

echo ""
echo -e "${GREEN}🎉 BUILD SUCCESSFUL! 🎉${NC}"
echo -e "${GREEN}================================${NC}"
echo -e "${GREEN}App Name: ${DISPLAY_NAME}${NC}"
echo -e "${GREEN}Version: ${VERSION} (${BUILD_VERSION})${NC}"
echo -e "${GREEN}Bundle Size: ${SIZE}${NC}"
echo -e "${GREEN}Binary Size: ${BINARY_SIZE}${NC}"
echo -e "${GREEN}Location: $(pwd)/${APP_NAME}.app${NC}"
echo ""
echo -e "${BLUE}📱 Installation Instructions:${NC}"
echo -e "${BLUE}   1. Drag ${APP_NAME}.app to /Applications folder${NC}"
echo -e "${BLUE}   2. Double-click to run${NC}"
echo -e "${BLUE}   3. Downloads saved to: ~/Documents/TrustDrop Downloads/${NC}"
echo ""

if [ -z "$SIGNING_IDENTITY" ]; then
    echo -e "${YELLOW}⚠️  QUARANTINE FIX (if needed):${NC}"
    echo -e "${YELLOW}   If macOS says app is damaged, run:${NC}"
    echo -e "${YELLOW}   sudo xattr -d com.apple.quarantine $(pwd)/${APP_NAME}.app${NC}"
    echo ""
fi

echo -e "${PURPLE}🚀 Ready for GitHub Release! 🚀${NC}"
echo -e "${PURPLE}Upload ${APP_NAME}.app to your GitHub release${NC}"