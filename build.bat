@echo off
REM TrustDrop Production Build Script - Windows Executable
REM Creates a proper .exe with icon ready for distribution

setlocal enabledelayedexpansion

echo TrustDrop Production Build - Windows
echo ===================================

REM App info
set APP_NAME=TrustDrop
for /f "tokens=2 delims==" %%I in ('wmic os get localdatetime /format:list ^| find "="') do set datetime=%%I
set VERSION=%datetime:~0,8%_%datetime:~8,6%

echo Building: %APP_NAME% v%VERSION%

REM Clean previous builds
echo Cleaning previous builds...
if exist %APP_NAME%.exe del %APP_NAME%.exe
if exist %APP_NAME%_*.exe del %APP_NAME%_*.exe
if exist app.syso del app.syso

REM Create Windows resource file for icon (if rsrc tool is available)
echo Creating Windows executable with icon...
if exist image.png (
    echo Converting icon for Windows...
    REM Try to create resource file with icon
    REM This requires rsrc tool: go install github.com/akavel/rsrc@latest
    where rsrc >nul 2>&1
    if %ERRORLEVEL% equ 0 (
        rsrc -ico image.png -o app.syso
        echo Icon resource created
    ) else (
        echo rsrc tool not found, building without embedded icon
        echo To embed icon: go install github.com/akavel/rsrc@latest
    )
) else (
    echo image.png not found, building without icon
)

REM Build with optimizations and version info
echo Building %APP_NAME%.exe...
go build -v -ldflags="-s -w -X main.appName=%APP_NAME% -X main.version=%VERSION% -H=windowsgui" -o "%APP_NAME%.exe" .

if %ERRORLEVEL% equ 0 (
    echo Build successful!
    for %%A in ("%APP_NAME%.exe") do echo    Executable Size: %%~zA bytes
    
    REM Clean up resource file
    if exist app.syso del app.syso
    
    echo.
    echo %APP_NAME%.exe is ready!
    echo To run: Double-click %APP_NAME%.exe
    echo Downloads will be saved to: Documents\TrustDrop Downloads\data\received
    echo.
    echo Installation: Copy %APP_NAME%.exe to desired location
) else (
    echo Build failed
    if exist app.syso del app.syso
    pause
    exit /b 1
)

echo.
pause