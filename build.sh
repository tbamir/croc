#!/bin/bash

# TrustDrop Build Script for macOS and Linux
# Creates production-ready .app bundle with custom icon

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
    go build -v -ldflags "$LDFLAGS" -o "$MACOS_DIR/$APP_NAME" .

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
        ICON_KEY="<key>CFBundleIconFile</key>\n    <string>${APP_NAME}</string>"
    else
        echo "⚠️  Warning: image.png not found - app will use default icon"
        ICON_KEY=""
    fi

    echo "📝 Creating Info.plist..."
    cat > "$APP_BUNDLE/Contents/Info.plist" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>CFBundleName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleDisplayName</key>
    <string>${APP_NAME}</string>
    <key>CFBundleExecutable</key>
    <string>${APP_NAME}</string>
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
EOF

    # Add icon file reference if icon was created
    if [ -f "$RESOURCES_DIR/${APP_NAME}.icns" ]; then
        cat >> "$APP_BUNDLE/Contents/Info.plist" <<EOF
    <key>CFBundleIconFile</key>
    <string>${APP_NAME}</string>
EOF
    fi

    # Close the plist
    cat >> "$APP_BUNDLE/Contents/Info.plist" <<EOF
</dict>
</plist>
EOF

    # Set proper permissions
    chmod +x "$MACOS_DIR/$APP_NAME"

    echo "✅ macOS .app bundle created at: $APP_BUNDLE"
    echo "🎯 Double-click to run: $APP_BUNDLE"

    # Test the app can be launched
    echo "🧪 Testing app launch..."
    if [ -x "$MACOS_DIR/$APP_NAME" ]; then
        echo "✅ Binary is executable"
    else
        echo "❌ Binary is not executable - fixing permissions..."
        chmod +x "$MACOS_DIR/$APP_NAME"
    fi

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
    go build -v -ldflags "$LDFLAGS" -o "$BUILD_DIR/trustdrop" .

    chmod +x "$BUILD_DIR/trustdrop"
    echo "✅ Linux binary created at: $BUILD_DIR/trustdrop"

else
    echo "❌ Unsupported OS: $OS"
    exit 1
fi

# Build the ledger viewer tool
echo "🔨 Building ledger viewer..."
cd cmd/ledger-viewer
go build -v -ldflags "$LDFLAGS" -o "../../$BUILD_DIR/ledger-viewer" .
cd ../..

echo ""
echo "🎉 Build complete!"
echo "📁 Build directory: $BUILD_DIR/"
echo "🔍 Ledger viewer: $BUILD_DIR/ledger-viewer"
echo ""

if [ "$OS" == "Darwin" ]; then
    echo "🚀 To run the app:"
    echo "   Double-click: $BUILD_DIR/${APP_NAME}.app"
    echo "   Or via terminal: open $BUILD_DIR/${APP_NAME}.app"
    echo ""
    echo "🏥 For medical deployment:"
    echo "   1. Upload $BUILD_DIR/${APP_NAME}.app to Google Drive"
    echo "   2. Medical staff download and double-click to run"
    echo "   3. Allow when macOS asks for permission (first time)"
else
    echo "🚀 To run: ./$BUILD_DIR/trustdrop"
fi

echo ""
echo "🔍 To view blockchain ledger: ./$BUILD_DIR/ledger-viewer -view"
echo "🐛 For debugging: DEBUG=1 open $BUILD_DIR/${APP_NAME}.app"