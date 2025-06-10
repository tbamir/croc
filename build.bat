@echo off
REM TrustDrop Bulletproof Build Script for Windows
REM Builds for Windows, Mac, and Linux

setlocal enabledelayedexpansion

echo 🚀 TrustDrop Bulletproof Edition - Windows Build Script
echo =========================================================

REM Get version info
for /f "tokens=2 delims==" %%I in ('wmic os get localdatetime /format:list ^| find "="') do set datetime=%%I
set VERSION=%datetime:~0,8%_%datetime:~8,6%
set BUILD_DIR=build
set APP_NAME=trustdrop-bulletproof

echo Version: %VERSION%
echo Building: %APP_NAME%

REM Clean previous builds
echo 🧹 Cleaning previous builds...
if exist %BUILD_DIR% rmdir /s /q %BUILD_DIR%
mkdir %BUILD_DIR%

REM Build for Windows first
echo 🔨 Building for Windows x64...
set OUTPUT_NAME=%BUILD_DIR%\%APP_NAME%_windows_x64_%VERSION%.exe
go build -v -ldflags="-s -w" -o "%OUTPUT_NAME%" .

if %ERRORLEVEL% equ 0 (
    echo ✅ Windows x64 build successful: %OUTPUT_NAME%
    for %%A in ("%OUTPUT_NAME%") do echo    📦 Size: %%~zA bytes
) else (
    echo ❌ Windows x64 build failed
    pause
    exit /b 1
)

REM Cross-compile for other platforms
echo 🔨 Cross-compiling for other platforms...

REM macOS Intel
echo Building for macOS Intel...
set GOOS=darwin
set GOARCH=amd64
set CGO_ENABLED=0
go build -v -ldflags="-s -w" -o "%BUILD_DIR%\%APP_NAME%_macos_intel_%VERSION%" .
if %ERRORLEVEL% equ 0 (
    echo ✅ macOS Intel build successful
) else (
    echo ❌ macOS Intel build failed
)

REM macOS Apple Silicon
echo Building for macOS Apple Silicon...
set GOOS=darwin
set GOARCH=arm64
set CGO_ENABLED=0
go build -v -ldflags="-s -w" -o "%BUILD_DIR%\%APP_NAME%_macos_apple_silicon_%VERSION%" .
if %ERRORLEVEL% equ 0 (
    echo ✅ macOS Apple Silicon build successful
) else (
    echo ❌ macOS Apple Silicon build failed
)

REM Linux x64
echo Building for Linux x64...
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
go build -v -ldflags="-s -w" -o "%BUILD_DIR%\%APP_NAME%_linux_x64_%VERSION%" .
if %ERRORLEVEL% equ 0 (
    echo ✅ Linux x64 build successful
) else (
    echo ❌ Linux x64 build failed
)

REM Linux ARM64
echo Building for Linux ARM64...
set GOOS=linux
set GOARCH=arm64
set CGO_ENABLED=0
go build -v -ldflags="-s -w" -o "%BUILD_DIR%\%APP_NAME%_linux_arm64_%VERSION%" .
if %ERRORLEVEL% equ 0 (
    echo ✅ Linux ARM64 build successful
) else (
    echo ❌ Linux ARM64 build failed
)

REM Reset environment
set GOOS=
set GOARCH=
set CGO_ENABLED=

echo.
echo 🎉 Build Summary
echo ===============
echo Built files:
dir /b %BUILD_DIR%\%APP_NAME%_*

echo.
echo 📋 Installation Instructions:
echo Windows: Double-click the .exe file
echo Mac/Linux: chmod +x filename ^&^& ./filename

echo.
echo 🔧 For testing Windows ↔ Mac transfers:
echo 1. Run the appropriate binary on each machine
echo 2. On sender: Choose 'Send Files' and select files/folders
echo 3. Share the transfer code with receiver
echo 4. On receiver: Choose 'Receive Files' and enter the code

echo.
echo ✨ TrustDrop Bulletproof is ready for testing!
echo.
pause