#!/bin/bash

# TrustDrop Build Script for macOS and Linux
# Creates production-ready .app bundle with custom icon and proper structure

set -e

echo "🚀 Building TrustDrop..."

OS=$(uname -s)
ARCH=$(uname -m)
APP_NAME="TrustDrop"
BUILD_DIR="build"
LDFLAGS="-s -w"

mkdir -p "$BUILD_DIR"

if [ "$OS" == "Darwin" ]; then
    echo "🍎 Detected macOS - Building .app bundle..."

    APP_BUNDLE="$BUILD_DIR/${APP_NAME}.app"
    MACOS_DIR="$APP_BUNDLE/Contents/MacOS"
    RESOURCES_DIR="$APP_BUNDLE/Contents/Resources"

    # Clean up any existing app bundle
    if [ -d "$APP_BUNDLE" ]; then
        echo "🧹 Cleaning up existing .app bundle..."
        rm -rf "$APP_BUNDLE"
    fi

    # Clean up any existing iconset
    if [ -d "${APP_NAME}.iconset" ]; then
        echo "🧹 Cleaning up existing iconset..."
        rm -rf "${APP_NAME}.iconset"
    fi

    mkdir -p "$MACOS_DIR"
    mkdir -p "$RESOURCES_DIR"

    echo "🔨 Compiling macOS GUI binary..."
    CGO_ENABLED=1 go build -v -ldflags "$LDFLAGS" -o "$MACOS_DIR/$APP_NAME" .

    # Convert image.png to .icns if it exists
    if [ -f "image.png" ]; then
        echo "🎨 Converting image.png to .icns format..."
        
        # Create iconset directory
        ICONSET_DIR="${APP_NAME}.iconset"
        mkdir -p "$ICONSET_DIR"

        # Generate all required icon sizes
        echo "📐 Generating icon sizes..."
        sips -z 16 16     image.png --out "$ICONSET_DIR/icon_16x16.png" > /dev/null 2>&1
        sips -z 32 32     image.png --out "$ICONSET_DIR/icon_16x16@2x.png" > /dev/null 2>&1
        sips -z 32 32     image.png --out "$ICONSET_DIR/icon_32x32.png" > /dev/null 2>&1
        sips -z 64 64     image.png --out "$ICONSET_DIR/icon_32x32@2x.png" > /dev/null 2>&1
        sips -z 128 128   image.png --out "$ICONSET_DIR/icon_128x128.png" > /dev/null 2>&1
        sips -z 256 256   image.png --out "$ICONSET_DIR/icon_128x128@2x.png" > /dev/null 2>&1
        sips -z 256 256   image.png --out "$ICONSET_DIR/icon_256x256.png" > /dev/null 2>&1
        sips -z 512 512   image.png --out "$ICONSET_DIR/icon_256x256@2x.png" > /dev/null 2>&1
        sips -z 512 512   image.png --out "$ICONSET_DIR/icon_512x512.png" > /dev/null 2>&1
        sips -z 1024 1024 image.png --out "$ICONSET_DIR/icon_512x512@2x.png" > /dev/null 2>&1

        # Convert iconset to icns
        echo "🔧 Creating .icns file..."
        iconutil -c icns "$ICONSET_DIR" -o "$RESOURCES_DIR/${APP_NAME}.icns"
        
        # Clean up iconset directory
        rm -rf "$ICONSET_DIR"
        
        echo "✅ Icon created: $RESOURCES_DIR/${APP_NAME}.icns"
    else
        echo "⚠️  Warning: image.png not found - app will use default icon"
    fi

    echo "📝 Creating Info.plist..."
    cat > "$APP_BUNDLE/Contents/Info.plist" << 'PLIST_EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>
    <string>TrustDrop</string>
    <key>CFBundleDisplayName</key>
    <string>TrustDrop</string>
    <key>CFBundleExecutable</key>
    <string>TrustDrop_launcher.sh</string>
    <key>CFBundleIdentifier</key>
    <string>com.trustdrop.app</string>
    <key>CFBundleVersion</key>
    <string>1.0.0</string>
    <key>CFBundleShortVersionString</key>
    <string>1.0</string>
    <key>CFBundleInfoDictionaryVersion</key>
    <string>6.0</string>
    <key>CFBundlePackageType</key>
    <string>APPL</string>
    <key>CFBundleSignature</key>
    <string>????</string>
    <key>LSMinimumSystemVersion</key>
    <string>10.13</string>
    <key>NSHighResolutionCapable</key>
    <true/>
    <key>LSUIElement</key>
    <false/>
    <key>NSAppTransportSecurity</key>
    <dict>
        <key>NSAllowsArbitraryLoads</key>
        <true/>
    </dict>
    <key>NSRequiresAquaSystemAppearance</key>
    <false/>
    <key>NSDocumentsFolderUsageDescription</key>
    <string>TrustDrop needs access to Documents folder to save received files securely.</string>
    <key>NSDownloadsFolderUsageDescription</key>
    <string>TrustDrop needs access to Downloads folder to save received files.</string>
    <key>NSRemovableVolumesUsageDescription</key>
    <string>TrustDrop needs access to external drives for file transfers.</string>
PLIST_EOF

    # Add icon file reference if icon was created
    if [ -f "$RESOURCES_DIR/${APP_NAME}.icns" ]; then
        cat >> "$APP_BUNDLE/Contents/Info.plist" << 'ICON_EOF'
    <key>CFBundleIconFile</key>
    <string>TrustDrop</string>
ICON_EOF
    fi

    # Close the plist
    cat >> "$APP_BUNDLE/Contents/Info.plist" << 'PLIST_END'
</dict>
</plist>
PLIST_END

    # Create a launcher script to ensure proper working directory
    echo "📝 Creating launcher script..."
    cat > "$MACOS_DIR/${APP_NAME}_launcher.sh" << 'LAUNCHER_EOF'
#!/bin/bash

# TrustDrop Launcher Script
# Ensures proper working directory for .app bundle

# Get the directory containing this script (Contents/MacOS)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP_DIR="$(dirname "$(dirname "$SCRIPT_DIR")")"
PARENT_DIR="$(dirname "$APP_DIR")"

# Try to create TrustDrop directory in user's Documents
HOME_DOCS="$HOME/Documents/TrustDrop"
if mkdir -p "$HOME_DOCS" 2>/dev/null && [ -w "$HOME_DOCS" ]; then
    cd "$HOME_DOCS"
elif [ -w "$PARENT_DIR" ]; then
    cd "$PARENT_DIR"
else
    # Last resort: use temp directory
    TEMP_DIR="/tmp/TrustDrop-$$"
    mkdir -p "$TEMP_DIR"
    cd "$TEMP_DIR"
fi

# Execute the actual binary
exec "$SCRIPT_DIR/TrustDrop" "$@"
LAUNCHER_EOF

    chmod +x "$MACOS_DIR/${APP_NAME}_launcher.sh"

    # Set proper permissions
    chmod +x "$MACOS_DIR/$APP_NAME"

    echo "✅ macOS .app bundle created at: $APP_BUNDLE"
    echo "🎯 Double-click to run: $APP_BUNDLE"

    # Test the app can be launched
    echo "🧪 Testing app structure..."
    if [ -x "$MACOS_DIR/$APP_NAME" ]; then
        echo "✅ Binary is executable"
    else
        echo "❌ Binary is not executable - fixing permissions..."
        chmod +x "$MACOS_DIR/$APP_NAME"
    fi

    if [ -x "$MACOS_DIR/${APP_NAME}_launcher.sh" ]; then
        echo "✅ Launcher script is executable"
    else
        echo "❌ Launcher script is not executable - fixing permissions..."
        chmod +x "$MACOS_DIR/${APP_NAME}_launcher.sh"
    fi

    # Create debug launcher for troubleshooting
    echo "🐛 Creating debug launcher..."
    cat > "$BUILD_DIR/Debug-${APP_NAME}.sh" << DEBUG_EOF
#!/bin/bash
echo "=== TrustDrop Debug Launcher ==="
echo "Current directory: \$(pwd)"
echo "App bundle: $APP_BUNDLE"
echo ""
export DEBUG=1
open -W "$APP_BUNDLE"
DEBUG_EOF
    chmod +x "$BUILD_DIR/Debug-${APP_NAME}.sh"

    # Optional: Create DMG
    if command -v create-dmg &> /dev/null; then
        echo "📦 Creating DMG installer..."
        DMG_PATH="$BUILD_DIR/${APP_NAME}-Installer.dmg"
        if [ -f "$DMG_PATH" ]; then
            rm "$DMG_PATH"
        fi
        
        create-dmg \
            --volname "$APP_NAME" \
            --window-pos 200 120 \
            --window-size 800 400 \
            --icon-size 100 \
            --app-drop-link 600 185 \
            "$DMG_PATH" \
            "$APP_BUNDLE" 2>/dev/null || true
        
        if [ -f "$DMG_PATH" ]; then
            echo "✅ DMG created: $DMG_PATH"
        fi
    else
        echo "💡 Tip: Install create-dmg for DMG creation: brew install create-dmg"
    fi

elif [ "$OS" == "Linux" ]; then
    echo "🐧 Detected Linux - Building CLI binary..."

    echo "🔨 Compiling Linux binary..."
    CGO_ENABLED=1 go build -v -ldflags "$LDFLAGS" -o "$BUILD_DIR/trustdrop" .

    chmod +x "$BUILD_DIR/trustdrop"
    echo "✅ Linux binary created at: $BUILD_DIR/trustdrop"

    # Create a desktop entry for Linux
    if command -v desktop-file-install &> /dev/null; then
        echo "🖥️ Creating desktop entry..."
        DESKTOP_FILE="$BUILD_DIR/TrustDrop.desktop"
        cat > "$DESKTOP_FILE" << DESKTOP_EOF
[Desktop Entry]
Version=1.0
Type=Application
Name=TrustDrop
Comment=Secure Medical File Transfer
Exec=$(pwd)/$BUILD_DIR/trustdrop
Icon=application-default-icon
Terminal=false
Categories=Network;FileTransfer;
StartupWMClass=TrustDrop
DESKTOP_EOF
        echo "✅ Desktop entry created: $DESKTOP_FILE"
        echo "💡 To install: desktop-file-install --dir=\$HOME/.local/share/applications $DESKTOP_FILE"
    fi

else
    echo "❌ Unsupported OS: $OS"
    exit 1
fi

# Build the ledger viewer tool
echo "🔨 Building ledger viewer..."
if [ -d "cmd/ledger-viewer" ]; then
    cd cmd/ledger-viewer
    CGO_ENABLED=0 go build -v -ldflags "$LDFLAGS" -o "../../$BUILD_DIR/ledger-viewer" .
    cd ../..
    chmod +x "$BUILD_DIR/ledger-viewer"
    echo "✅ Ledger viewer created at: $BUILD_DIR/ledger-viewer"
else
    echo "⚠️ Ledger viewer source not found, skipping..."
fi

# Create comprehensive README
echo "📚 Creating documentation..."
cat > "$BUILD_DIR/README.md" << README_EOF
# TrustDrop - Secure Medical File Transfer

## Quick Start

### macOS
- **Run**: Double-click \`TrustDrop.app\`
- **Debug**: Run \`./Debug-TrustDrop.sh\` for console output
- **Files**: Received files saved to \`~/Documents/TrustDrop/data/received/\`

### Linux
- **Run**: \`./trustdrop\`
- **Files**: Received files saved to current directory's \`data/received/\`

## Features
- **Security**: AES-256 encryption + SHA-256 file verification
- **Blockchain**: Immutable audit trail for all transfers
- **Cross-Platform**: Works through firewalls between Europe/US
- **Large Files**: Supports transfers up to 5TB

## Usage
1. **Sender**: Click "Send Files" → Copy code → Select files → Share code
2. **Receiver**: Click "Receive Files" → Enter code → Files download automatically

## Troubleshooting
- **macOS Permissions**: System Preferences → Privacy → Files and Folders
- **Network Issues**: Application uses multiple relay servers automatically
- **Debug Mode**: Set \`DEBUG=1\` environment variable

## Blockchain Ledger
View transfer history: \`./ledger-viewer -view\`

## Medical Deployment
1. Download \`TrustDrop.app\` (macOS) or \`trustdrop\` (Linux)
2. No installation required - just double-click to run
3. All transfers are automatically logged for compliance
README_EOF

echo ""
echo "🎉 BUILD COMPLETE!"
echo "📁 Build directory: $BUILD_DIR/"
echo "🔍 Ledger viewer: $BUILD_DIR/ledger-viewer"
echo ""

if [ "$OS" == "Darwin" ]; then
    echo "🚀 To run the app:"
    echo "   Double-click: $BUILD_DIR/${APP_NAME}.app"
    echo "   Debug mode: ./$BUILD_DIR/Debug-${APP_NAME}.sh"
    echo ""
    echo "🏥 For medical deployment:"
    echo "   1. Upload $BUILD_DIR/${APP_NAME}.app to Google Drive"
    echo "   2. Medical staff download and double-click to run"
    echo "   3. Allow permissions when macOS asks (first time only)"
    echo "   4. Files save to ~/Documents/TrustDrop/data/received/"
else
    echo "🚀 To run: ./$BUILD_DIR/trustdrop"
    echo ""
    echo "🏥 For medical deployment:"
    echo "   1. Copy trustdrop binary to target systems"
    echo "   2. Run: ./trustdrop (no installation needed)"
    echo "   3. Files save to ./data/received/"
fi

echo ""
echo "🔍 To view blockchain ledger: ./$BUILD_DIR/ledger-viewer -view"
echo "📚 Full documentation: $BUILD_DIR/README.md"
echo ""
echo "✅ Ready for secure medical file transfers!"