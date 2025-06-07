@echo off
REM TrustDrop Build Script for Windows
REM Builds the main application and ledger viewer tool

echo Building TrustDrop...
echo ====================

REM Check if Go is installed
where go >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo Error: Go is not installed. Please install Go 1.21 or later.
    exit /b 1
)

REM Get dependencies
echo Getting dependencies...
go mod download

REM Build main application
echo Building TrustDrop...
go build -ldflags -H=windowsgui -o trustdrop.exe main.go

REM Build ledger viewer
echo Building ledger viewer...
if not exist cmd\ledger-viewer mkdir cmd\ledger-viewer
go build -o ledger-viewer.exe cmd\ledger-viewer\main.go

echo.
echo Build complete!
echo.
echo Executables created:
echo   - trustdrop.exe (main application)
echo   - ledger-viewer.exe (blockchain viewer tool)
echo.
echo To run TrustDrop: trustdrop.exe
echo To view ledger: ledger-viewer.exe -help