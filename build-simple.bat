@echo off
REM TrustDrop Simple Build Script for Windows (No external tools required)
REM This builds TrustDrop.exe without requiring ImageMagick or windres

echo 🚀 Building TrustDrop for Windows (Simple)...

REM Set build directory
set BUILD_DIR=build
if not exist %BUILD_DIR% mkdir %BUILD_DIR%

REM Clean up existing build
if exist %BUILD_DIR%\TrustDrop.exe del %BUILD_DIR%\TrustDrop.exe

REM Build flags for Windows GUI application
set BUILD_FLAGS=-v -ldflags="-s -w -H=windowsgui"

REM Build the main application
echo 🔨 Building main application...
go build %BUILD_FLAGS% -o %BUILD_DIR%\TrustDrop.exe .
if errorlevel 1 (
    echo ❌ Build failed!
    exit /b 1
)

echo ✅ Windows .exe created: %BUILD_DIR%\TrustDrop.exe

REM Build the ledger viewer tool
echo 🔨 Building ledger viewer...
cd cmd\ledger-viewer
go build -v -ldflags="-s -w" -o ..\..\%BUILD_DIR%\ledger-viewer.exe .
if errorlevel 1 (
    echo ❌ Ledger viewer build failed!
    cd ..\..
    exit /b 1
)
cd ..\..

REM Create debug launcher
echo 📝 Creating debug launcher...
echo @echo off > %BUILD_DIR%\TrustDrop-Debug.bat
echo set DEBUG=1 >> %BUILD_DIR%\TrustDrop-Debug.bat
echo start TrustDrop.exe >> %BUILD_DIR%\TrustDrop-Debug.bat

echo.
echo 🎉 Build complete!
echo.
echo 📁 Output files:
echo   🎯 Main application: %BUILD_DIR%\TrustDrop.exe
echo   🔍 Ledger viewer: %BUILD_DIR%\ledger-viewer.exe
echo   🐛 Debug launcher: %BUILD_DIR%\TrustDrop-Debug.bat
echo.
echo 🏥 Ready for medical deployment!
echo   - Upload TrustDrop.exe to Google Drive
echo   - Medical staff download and double-click to run
echo.
echo 💡 Note: For custom icon, use build.bat with ImageMagick installed

pause 