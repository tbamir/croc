#!/bin/bash

# TrustDrop Build Script for macOS and Linux
# This script builds the TrustDrop application with proper icons and settings

set -e

echo "Building TrustDrop..."

# Detect OS
OS=$(uname -s)
ARCH=$(uname -m)

# Set build directory
BUILD_DIR="build"
mkdir -p $BUILD_DIR

# Common build flags
LDFLAGS="-s -w"

# Build the main application
echo "Building main application..."
if [ "$OS" == "Darwin" ]; then
    # macOS build
    echo "Building for macOS..."
    
    # Build the binary
    go build -v -ldflags "$LDFLAGS" -o $BUILD_DIR/TrustDrop .
    
    # Create app bundle
    APP_DIR="$BUILD_DIR/TrustDrop.app"
    mkdir -p "$APP_DIR/Contents/MacOS"
    mkdir -p "$APP_DIR/Contents/Resources"
    
    # Move binary to app bundle
    mv $BUILD_DIR/TrustDrop "$APP_DIR/Contents/MacOS/TrustDrop"
    
    # Create Info.plist
    cat > "$APP_DIR/Contents/Info.plist" << EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleExecutable</key>
    <string>TrustDrop</string>
    <key>CFBundleIdentifier</key>
    <string>com.trustdrop.app</string>
    <key>CFBundleName</key>
    <string>TrustDrop</string>
    <key>CFBundleDisplayName</key>
    <string>TrustDrop</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0.0</string>
    <key>CFBundleVersion</key>
    <string>1</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleSignature</key>
    <string>????</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.12</string>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>NSHumanReadableCopyright</key>
    <string>Copyright © 2024 TrustDrop. All rights reserved.</string>
</dict>
</plist>
EOF
    
    # Make the app executable
    chmod +x "$APP_DIR/Contents/MacOS/TrustDrop"
    
    echo "macOS app bundle created at: $APP_DIR"
    
    # Create a DMG (optional)
    if command -v create-dmg &> /dev/null; then
        echo "Creating DMG..."
        create-dmg \
            --volname "TrustDrop" \
            --window-pos 200 120 \
            --window-size 800 400 \
            --icon-size 100 \
            --app-drop-link 600 185 \
            "$BUILD_DIR/TrustDrop.dmg" \
            "$APP_DIR"
        echo "DMG created at: $BUILD_DIR/TrustDrop.dmg"
    else
        echo "Note: Install create-dmg to build DMG installer (brew install create-dmg)"
    fi
    
elif [ "$OS" == "Linux" ]; then
    # Linux build
    echo "Building for Linux..."
    go build -v -ldflags "$LDFLAGS" -o $BUILD_DIR/trustdrop .
    
    # Make executable
    chmod +x $BUILD_DIR/trustdrop
    
    echo "Linux binary created at: $BUILD_DIR/trustdrop"
    
    # Create AppImage (optional)
    if command -v appimagetool &> /dev/null; then
        echo "Creating AppImage..."
        APPDIR="$BUILD_DIR/TrustDrop.AppDir"
        mkdir -p "$APPDIR/usr/bin"
        mkdir -p "$APPDIR/usr/share/applications"
        mkdir -p "$APPDIR/usr/share/icons/hicolor/256x256/apps"
        
        # Copy binary
        cp $BUILD_DIR/trustdrop "$APPDIR/usr/bin/"
        
        # Create desktop file
        cat > "$APPDIR/usr/share/applications/trustdrop.desktop" << EOF
[Desktop Entry]
Type=Application
Name=TrustDrop
Exec=trustdrop
Icon=trustdrop
Categories=Network;FileTransfer;
Comment=Secure peer-to-peer file transfer
EOF
        
        # Create AppRun
        cat > "$APPDIR/AppRun" << 'EOF'
#!/bin/bash
HERE="$(dirname "$(readlink -f "${0}")")"
exec "${HERE}/usr/bin/trustdrop" "$@"
EOF
        chmod +x "$APPDIR/AppRun"
        
        # Build AppImage
        appimagetool "$APPDIR" "$BUILD_DIR/TrustDrop.AppImage"
        echo "AppImage created at: $BUILD_DIR/TrustDrop.AppImage"
    else
        echo "Note: Install appimagetool to build AppImage"
    fi
else
    echo "Unsupported OS: $OS"
    exit 1
fi

# Build the ledger viewer tool
echo "Building ledger viewer..."
cd cmd/ledger-viewer
go build -v -ldflags "$LDFLAGS" -o ../../$BUILD_DIR/ledger-viewer .
cd ../..

echo ""
echo "Build complete!"
echo "Main application: $BUILD_DIR/"
echo "Ledger viewer: $BUILD_DIR/ledger-viewer"
echo ""
echo "To run TrustDrop:"
if [ "$OS" == "Darwin" ]; then
    echo "  open $APP_DIR"
else
    echo "  ./$BUILD_DIR/trustdrop"
fi
echo ""
echo "To view the blockchain ledger:"
echo "  ./$BUILD_DIR/ledger-viewer -view"
echo ""
echo "Note: For debugging, set DEBUG=1 before running to see detailed logs"