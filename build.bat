@echo off
REM TrustDrop Build Script for Windows
REM This script builds the TrustDrop application with custom icon

echo 🚀 Building TrustDrop for Windows...

REM Set build directory
set BUILD_DIR=build
if not exist %BUILD_DIR% mkdir %BUILD_DIR%

REM Clean up existing build
if exist %BUILD_DIR%\TrustDrop.exe del %BUILD_DIR%\TrustDrop.exe
if exist %BUILD_DIR%\TrustDrop.ico del %BUILD_DIR%\TrustDrop.ico
if exist app_icon.rc del app_icon.rc
if exist app_icon.syso del app_icon.syso

REM Convert image.png to .ico if it exists
if exist image.png (
    echo 🎨 Converting image.png to .ico format...
    
    REM Check if ImageMagick is available
    where magick >nul 2>nul
    if errorlevel 1 (
        echo ⚠️  ImageMagick not found. Install ImageMagick or use online converter
        echo    To install: winget install ImageMagick.ImageMagick
        echo    Or download from: https://imagemagick.org/script/download.php#windows
        echo    Continuing without icon...
    ) else (
        echo 📐 Creating Windows icon sizes...
        magick image.png -resize 256x256 temp_256.png
        magick image.png -resize 128x128 temp_128.png
        magick image.png -resize 64x64 temp_64.png
        magick image.png -resize 48x48 temp_48.png
        magick image.png -resize 32x32 temp_32.png
        magick image.png -resize 16x16 temp_16.png
        
        echo 🔧 Creating .ico file...
        magick temp_16.png temp_32.png temp_48.png temp_64.png temp_128.png temp_256.png %BUILD_DIR%\TrustDrop.ico
        
        REM Clean up temp files
        del temp_*.png
        
        echo ✅ Icon created: %BUILD_DIR%\TrustDrop.ico
        
        REM Create resource file for embedding icon
        echo 📝 Creating resource file...
        echo IDI_ICON1 ICON "%BUILD_DIR%\TrustDrop.ico" > app_icon.rc
        
        REM Check if windres is available (from TDM-GCC or similar)
        where windres >nul 2>nul
        if errorlevel 1 (
            echo ⚠️  windres not found. Icon will not be embedded in .exe
            echo    To embed icons, install TDM-GCC or MinGW-w64
            set ICON_FLAGS=
        ) else (
            echo 🔧 Compiling resource file...
            windres -i app_icon.rc -o app_icon.syso
            if errorlevel 1 (
                echo ⚠️  Resource compilation failed
                set ICON_FLAGS=
            ) else (
                echo ✅ Icon resource compiled
                set ICON_FLAGS=-ldflags="-s -w -H=windowsgui"
            )
        )
    )
) else (
    echo ⚠️  Warning: image.png not found - .exe will use default icon
    set ICON_FLAGS=-ldflags="-s -w -H=windowsgui"
)

REM Build flags
if not defined ICON_FLAGS set ICON_FLAGS=-ldflags="-s -w -H=windowsgui"

REM Build the main application
echo 🔨 Building main application...
go build -v %ICON_FLAGS% -o %BUILD_DIR%\TrustDrop.exe .
if errorlevel 1 (
    echo ❌ Build failed!
    goto cleanup
)

echo ✅ Windows .exe created: %BUILD_DIR%\TrustDrop.exe

REM Build the ledger viewer tool
echo 🔨 Building ledger viewer...
cd cmd\ledger-viewer
go build -v -ldflags="-s -w" -o ..\..\%BUILD_DIR%\ledger-viewer.exe .
if errorlevel 1 (
    echo ❌ Ledger viewer build failed!
    cd ..\..
    goto cleanup
)
cd ..\..

REM Create a batch file to run TrustDrop with console output for debugging
echo 📝 Creating debug launcher...
echo @echo off > %BUILD_DIR%\TrustDrop-Debug.bat
echo set DEBUG=1 >> %BUILD_DIR%\TrustDrop-Debug.bat
echo start TrustDrop.exe >> %BUILD_DIR%\TrustDrop-Debug.bat

REM Create README for the build
echo 📝 Creating documentation...
echo TrustDrop - Secure Medical File Transfer > %BUILD_DIR%\README.txt
echo. >> %BUILD_DIR%\README.txt
echo 🚀 To run TrustDrop: >> %BUILD_DIR%\README.txt
echo   - Double-click TrustDrop.exe >> %BUILD_DIR%\README.txt
echo. >> %BUILD_DIR%\README.txt
echo 🔍 To view the blockchain ledger: >> %BUILD_DIR%\README.txt
echo   - Open Command Prompt in this folder >> %BUILD_DIR%\README.txt
echo   - Run: ledger-viewer.exe -view >> %BUILD_DIR%\README.txt
echo. >> %BUILD_DIR%\README.txt
echo 🐛 For debugging: >> %BUILD_DIR%\README.txt
echo   - Run TrustDrop-Debug.bat to see console output >> %BUILD_DIR%\README.txt
echo. >> %BUILD_DIR%\README.txt
echo 🏥 For medical deployment: >> %BUILD_DIR%\README.txt
echo   1. Copy TrustDrop.exe to target computers >> %BUILD_DIR%\README.txt
echo   2. Allow Windows SmartScreen if prompted >> %BUILD_DIR%\README.txt
echo   3. Double-click to run - no installation needed >> %BUILD_DIR%\README.txt

echo.
echo 🎉 Build complete!
echo.
echo 📁 Output files:
echo   🎯 Main application: %BUILD_DIR%\TrustDrop.exe
echo   🔍 Ledger viewer: %BUILD_DIR%\ledger-viewer.exe  
echo   🐛 Debug launcher: %BUILD_DIR%\TrustDrop-Debug.bat
if exist %BUILD_DIR%\TrustDrop.ico echo   🎨 Icon file: %BUILD_DIR%\TrustDrop.ico
echo.
echo 🏥 For medical deployment:
echo   1. Upload %BUILD_DIR%\TrustDrop.exe to Google Drive
echo   2. Medical staff download and double-click to run
echo   3. Allow Windows SmartScreen if prompted (first time)
echo.
echo 💡 Tip: To create an installer, use Inno Setup or NSIS

:cleanup
REM Clean up temporary files
if exist app_icon.rc del app_icon.rc
if exist app_icon.syso del app_icon.syso

pause