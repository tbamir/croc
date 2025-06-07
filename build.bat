@echo off
REM TrustDrop Build Script for Windows
REM This script builds the TrustDrop application for Windows

echo Building TrustDrop for Windows...

REM Set build directory
set BUILD_DIR=build
if not exist %BUILD_DIR% mkdir %BUILD_DIR%

REM Build flags
set BUILD_FLAGS=-v -ldflags="-s -w -H=windowsgui"

REM Build the main application
echo Building main application...
go build %BUILD_FLAGS% -o %BUILD_DIR%\TrustDrop.exe .
if errorlevel 1 (
    echo Build failed!
    exit /b 1
)

REM Build the ledger viewer tool
echo Building ledger viewer...
cd cmd\ledger-viewer
go build -v -ldflags="-s -w" -o ..\..\%BUILD_DIR%\ledger-viewer.exe .
if errorlevel 1 (
    echo Ledger viewer build failed!
    cd ..\..
    exit /b 1
)
cd ..\..

REM Create a batch file to run TrustDrop with console output for debugging
echo @echo off > %BUILD_DIR%\TrustDrop-Debug.bat
echo start TrustDrop.exe >> %BUILD_DIR%\TrustDrop-Debug.bat

REM Create README for the build
echo TrustDrop - Secure File Transfer > %BUILD_DIR%\README.txt
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

echo.
echo Build complete!
echo.
echo Output files:
echo   Main application: %BUILD_DIR%\TrustDrop.exe
echo   Ledger viewer: %BUILD_DIR%\ledger-viewer.exe
echo   Debug launcher: %BUILD_DIR%\TrustDrop-Debug.bat
echo.
echo To create an installer, use a tool like Inno Setup or NSIS
echo with the files in the %BUILD_DIR% directory.

pause