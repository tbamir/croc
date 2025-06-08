@echo off
REM TrustDrop Build Script for Windows
REM This script builds the TrustDrop application

echo Building TrustDrop for Windows...

REM Set build directory
set BUILD_DIR=build
if not exist %BUILD_DIR% mkdir %BUILD_DIR%

REM Clean up existing build
if exist %BUILD_DIR%\TrustDrop.exe del %BUILD_DIR%\TrustDrop.exe
if exist %BUILD_DIR%\TrustDrop.ico del %BUILD_DIR%\TrustDrop.ico
if exist app_icon.rc del app_icon.rc
if exist app_icon.syso del app_icon.syso

REM Check for image.png and attempt icon conversion
if exist image.png (
    echo Converting image.png to .ico format...
    
    REM Check if ImageMagick is available
    where magick >nul 2>nul
    if errorlevel 1 (
        echo WARNING: ImageMagick not found. Building without custom icon.
        echo To add custom icon support:
        echo   Install ImageMagick: winget install ImageMagick.ImageMagick
        echo   Or manually convert image.png to TrustDrop.ico and place in build folder
        set ICON_FLAGS=-ldflags="-s -w -H=windowsgui"
    ) else (
        echo Creating Windows icon sizes...
        magick image.png -resize 256x256 temp_256.png
        magick image.png -resize 128x128 temp_128.png
        magick image.png -resize 64x64 temp_64.png
        magick image.png -resize 48x48 temp_48.png
        magick image.png -resize 32x32 temp_32.png
        magick image.png -resize 16x16 temp_16.png
        
        echo Creating .ico file...
        magick temp_16.png temp_32.png temp_48.png temp_64.png temp_128.png temp_256.png %BUILD_DIR%\TrustDrop.ico
        
        REM Clean up temp files
        del temp_*.png
        
        echo SUCCESS: Icon created at %BUILD_DIR%\TrustDrop.ico
        
        REM Create resource file for embedding icon
        echo Creating resource file...
        echo IDI_ICON1 ICON "%BUILD_DIR%\TrustDrop.ico" > app_icon.rc
        
        REM Check if windres is available
        where windres >nul 2>nul
        if errorlevel 1 (
            echo WARNING: windres not found. Icon will not be embedded in .exe
            echo To embed icons, install TDM-GCC or MinGW-w64
            set ICON_FLAGS=-ldflags="-s -w -H=windowsgui"
        ) else (
            echo Compiling resource file...
            windres -i app_icon.rc -o app_icon.syso
            if errorlevel 1 (
                echo WARNING: Resource compilation failed
                set ICON_FLAGS=-ldflags="-s -w -H=windowsgui"
            ) else (
                echo SUCCESS: Icon resource compiled
                set ICON_FLAGS=-ldflags="-s -w -H=windowsgui"
            )
        )
    )
) else (
    echo WARNING: image.png not found - .exe will use default icon
    set ICON_FLAGS=-ldflags="-s -w -H=windowsgui"
)

REM Build flags
if not defined ICON_FLAGS set ICON_FLAGS=-ldflags="-s -w -H=windowsgui"

REM Build the main application
echo Building main application...
go build -v %ICON_FLAGS% -o %BUILD_DIR%\TrustDrop.exe .
if errorlevel 1 (
    echo ERROR: Build failed!
    goto cleanup
)

echo SUCCESS: Windows .exe created at %BUILD_DIR%\TrustDrop.exe

REM Build the ledger viewer tool
echo Building ledger viewer...
if exist cmd\ledger-viewer (
    pushd cmd\ledger-viewer
    go build -v -ldflags="-s -w" -o ..\..\%BUILD_DIR%\ledger-viewer.exe .
    if errorlevel 1 (
        echo ERROR: Ledger viewer build failed!
        popd
        goto cleanup
    )
    popd
    echo SUCCESS: Ledger viewer created at %BUILD_DIR%\ledger-viewer.exe
) else (
    echo WARNING: Ledger viewer source not found, skipping...
)

REM Create debug launcher
echo Creating debug launcher...
echo @echo off > %BUILD_DIR%\TrustDrop-Debug.bat
echo set DEBUG=1 >> %BUILD_DIR%\TrustDrop-Debug.bat
echo start TrustDrop.exe >> %BUILD_DIR%\TrustDrop-Debug.bat

REM Create README for the build
echo Creating documentation...
echo TrustDrop - Secure Medical File Transfer > %BUILD_DIR%\README.txt
echo. >> %BUILD_DIR%\README.txt
echo To run TrustDrop: >> %BUILD_DIR%\README.txt
echo   - Double-click TrustDrop.exe >> %BUILD_DIR%\README.txt
echo. >> %BUILD_DIR%\README.txt
echo To view the blockchain ledger: >> %BUILD_DIR%\README.txt
echo   - Open Command Prompt in this folder >> %BUILD_DIR%\README.txt
echo   - Run: ledger-viewer.exe -view >> %BUILD_DIR%\README.txt
echo. >> %BUILD_DIR%\README.txt
echo For debugging: >> %BUILD_DIR%\README.txt
echo   - Run TrustDrop-Debug.bat to see console output >> %BUILD_DIR%\README.txt
echo. >> %BUILD_DIR%\README.txt
echo For medical deployment: >> %BUILD_DIR%\README.txt
echo   1. Copy TrustDrop.exe to target computers >> %BUILD_DIR%\README.txt
echo   2. Allow Windows SmartScreen if prompted >> %BUILD_DIR%\README.txt
echo   3. Double-click to run - no installation needed >> %BUILD_DIR%\README.txt

echo.
echo BUILD COMPLETE!
echo.
echo Output files:
echo   Main application: %BUILD_DIR%\TrustDrop.exe
if exist %BUILD_DIR%\ledger-viewer.exe echo   Ledger viewer: %BUILD_DIR%\ledger-viewer.exe
echo   Debug launcher: %BUILD_DIR%\TrustDrop-Debug.bat
if exist %BUILD_DIR%\TrustDrop.ico echo   Icon file: %BUILD_DIR%\TrustDrop.ico
echo.
echo Ready for medical deployment!
echo   - Upload %BUILD_DIR%\TrustDrop.exe to Google Drive
echo   - Medical staff download and double-click to run
echo.
echo TIP: To create an installer, use Inno Setup or NSIS

:cleanup
REM Clean up temporary files
if exist app_icon.rc del app_icon.rc
if exist app_icon.syso del app_icon.syso

pause