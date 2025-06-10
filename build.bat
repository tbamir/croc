@echo off
REM TrustDrop Production Build Script for Windows
REM Creates a clean application binary ready for distribution

setlocal enabledelayedexpansion

echo 🚀 TrustDrop Production Build
echo ==============================

REM App info
set APP_NAME=TrustDrop
for /f "tokens=2 delims==" %%I in ('wmic os get localdatetime /format:list ^| find "="') do set datetime=%%I
set VERSION=%datetime:~0,8%_%datetime:~8,6%

echo Building: %APP_NAME% v%VERSION%

REM Clean previous builds
echo 🧹 Cleaning previous builds...
if exist %APP_NAME%.exe del %APP_NAME%.exe
if exist %APP_NAME%_*.exe del %APP_NAME%_*.exe

echo 🔨 Building %APP_NAME%...

REM Build with optimizations
go build -v -ldflags="-s -w -X main.appName=%APP_NAME% -X main.version=%VERSION%" -o "%APP_NAME%.exe" .

if %ERRORLEVEL% equ 0 (
    echo ✅ Build successful!
    for %%A in ("%APP_NAME%.exe") do echo    📦 Size: %%~zA bytes
    
    echo.
    echo 🎉 %APP_NAME% is ready!
    echo 💡 To run: %APP_NAME%.exe
    echo 📂 Downloads will be saved to: Documents\TrustDrop Downloads\data\received
) else (
    echo ❌ Build failed
    pause
    exit /b 1
)

echo.
pause