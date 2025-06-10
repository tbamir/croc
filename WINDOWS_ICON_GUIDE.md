# Windows Icon Support Guide

This guide explains how to properly embed icons in Windows executables for TrustDrop.

## Overview

Windows applications display icons differently than macOS applications. To get proper icon display in Windows:

1. **File Explorer**: Shows the embedded executable icon
2. **Taskbar**: Shows the embedded executable icon  
3. **Alt+Tab**: Shows the embedded executable icon
4. **System dialogs**: Show the embedded executable icon

## Files Created

### Icon Files
- `icon.ico` - Multi-resolution Windows icon file (16x16, 32x32, 48x48, 256x256)
- `app.rc` - Windows resource script file
- `app.syso` - Compiled Go resource file (auto-generated during build)

### Build Scripts
- `build.bat` - Windows build script with icon embedding
- `generate_windows_icon.sh` - Cross-platform icon generation script

## Build Process

### On Windows (using build.bat)
```batch
# This automatically handles icon embedding
build.bat
```

The script will:
1. Look for `icon.ico` file
2. Try to use `windres` (if available) or `rsrc` tool
3. Generate `app.syso` resource file
4. Build executable with embedded icon
5. Clean up temporary files

### On macOS (cross-compilation)
```bash
# Generate Windows icon resources
./generate_windows_icon.sh

# Then build for Windows (requires Go cross-compilation setup)
GOOS=windows GOARCH=amd64 go build -ldflags="-H=windowsgui" -o TrustDrop.exe .
```

### Manual Process
```bash
# 1. Install rsrc tool
go install github.com/akavel/rsrc@latest

# 2. Generate Windows resource file
rsrc -ico icon.ico -o app.syso

# 3. Build executable
go build -ldflags="-H=windowsgui" -o TrustDrop.exe .

# 4. Clean up (optional)
rm app.syso
```

## Tools Required

### Option 1: rsrc (Go-based, cross-platform)
```bash
go install github.com/akavel/rsrc@latest
```

### Option 2: windres (Windows native, part of TDM-GCC or MinGW)
- Install TDM-GCC: https://jmeubank.github.io/tdm-gcc/
- Or install MinGW-w64

## Troubleshooting

### Icon Not Showing
1. **Check if .syso file exists**: The `app.syso` file must be present during build
2. **Verify ICO format**: Must be `.ico` format, not `.png`
3. **Build flags**: Use `-H=windowsgui` for GUI applications
4. **Windows cache**: Windows may cache old icons - restart Explorer or reboot

### Build Errors
1. **"rsrc: command not found"**: Install rsrc tool or use windres
2. **"windres: command not found"**: Install TDM-GCC or MinGW
3. **Cross-compilation issues**: Some dependencies don't support cross-compilation

### Testing Icon
```bash
# Check if icon is embedded (on Windows)
Get-ItemProperty "TrustDrop.exe" | Select-Object VersionInfo

# Or use Resource Hacker tool to inspect embedded resources
```

## Icon Requirements

### Size Requirements
- 16x16 pixels (small icons)
- 32x32 pixels (standard icons)  
- 48x48 pixels (large icons)
- 256x256 pixels (extra large icons)

### Format Requirements
- ICO format (not PNG, JPG, etc.)
- Multiple resolutions in single file
- Transparent background recommended
- True color (24-bit) or 32-bit with alpha

## Automated Workflow

The build process is now automated in the main build scripts:

1. **macOS Build** (`./build.sh`): Creates macOS app + Windows icon resources
2. **Windows Build** (`build.bat`): Creates Windows executable with embedded icon
3. **Manual Helper** (`./generate_windows_icon.sh`): Cross-platform icon generation

This ensures both platforms have proper native icon support. 