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
    
    # Make the binary executable
    chmod +x $BUILD_DIR/TrustDrop
    
    echo "macOS binary created at: $BUILD_DIR/TrustDrop"
    
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
            "$BUILD_DIR/TrustDrop"
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
    echo "  open $BUILD_DIR/TrustDrop"
else
    echo "  ./$BUILD_DIR/trustdrop"
fi
echo ""
echo "To view the blockchain ledger:"
echo "  ./$BUILD_DIR/ledger-viewer -view"
echo ""
echo "Note: For debugging, set DEBUG=1 before running to see detailed logs"