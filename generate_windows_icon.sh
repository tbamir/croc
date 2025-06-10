#!/bin/bash
# Generate Windows Icon Resource
# Run this on macOS before cross-compiling for Windows

echo "Generating Windows icon resource..."

# Check if image.png exists
if [ ! -f "image.png" ]; then
    echo "Error: image.png not found"
    exit 1
fi

# Check if ImageMagick is available
if ! command -v magick &> /dev/null; then
    echo "Error: ImageMagick not found. Install with: brew install imagemagick"
    exit 1
fi

# Generate ICO file from PNG
echo "Converting PNG to ICO..."
magick image.png -resize 16x16 temp16.png
magick image.png -resize 32x32 temp32.png  
magick image.png -resize 48x48 temp48.png
magick image.png -resize 256x256 temp256.png
magick temp16.png temp32.png temp48.png temp256.png icon.ico
rm temp16.png temp32.png temp48.png temp256.png

# Check if rsrc is available
if [ ! -f "$HOME/go/bin/rsrc" ]; then
    echo "Installing rsrc tool..."
    go install github.com/akavel/rsrc@latest
fi

# Generate Windows resource file
echo "Generating Windows resource file..."
$HOME/go/bin/rsrc -ico icon.ico -o app.syso

echo "Windows icon resource generated successfully!"
echo "Files created:"
echo "  - icon.ico (Windows icon file)"
echo "  - app.syso (Go resource file)"
echo ""
echo "You can now build for Windows with: GOOS=windows GOARCH=amd64 go build" 