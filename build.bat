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
if exist resource.syso del resource.syso

REM Create Windows resource file for icon
echo Creating Windows executable with icon...
if exist icon.ico (
    if exist app.rc (
        echo Compiling Windows resources...
        REM Try windres first (preferred method)
        where windres >nul 2>&1
        if %ERRORLEVEL% equ 0 (
            windres -i app.rc -o resource.syso
            echo Icon resource created with windres
        ) else (
            REM Fallback to rsrc if available
            if exist "%USERPROFILE%\go\bin\rsrc.exe" (
                "%USERPROFILE%\go\bin\rsrc.exe" -ico icon.ico -o app.syso
                echo Icon resource created with rsrc
            ) else (
                where rsrc >nul 2>&1
                if %ERRORLEVEL% equ 0 (
                    rsrc -ico icon.ico -o app.syso
                    echo Icon resource created with rsrc
                ) else (
                    echo Neither windres nor rsrc found, building without embedded icon
                    echo To embed icon: Install TDM-GCC or go install github.com/akavel/rsrc@latest
                )
            )
        )
    ) else (
        echo app.rc not found, trying direct ICO embedding...
        if exist "%USERPROFILE%\go\bin\rsrc.exe" (
            "%USERPROFILE%\go\bin\rsrc.exe" -ico icon.ico -o app.syso
            echo Icon resource created with rsrc
        ) else (
            where rsrc >nul 2>&1
            if %ERRORLEVEL% equ 0 (
                rsrc -ico icon.ico -o app.syso
                echo Icon resource created
            ) else (
                echo rsrc tool not found, building without embedded icon
                echo To embed icon: go install github.com/akavel/rsrc@latest
            )
        )
    )
) else (
    echo icon.ico not found, building without icon
    echo Note: Convert image.png to icon.ico for Windows icon support
)

REM Build with optimizations and version info
echo Building %APP_NAME%.exe...
go build -v -ldflags="-s -w -X main.appName=%APP_NAME% -X main.version=%VERSION% -H=windowsgui" -o "%APP_NAME%.exe" .

if %ERRORLEVEL% equ 0 (
    echo Build successful!
    for %%A in ("%APP_NAME%.exe") do echo    Executable Size: %%~zA bytes
    
    REM Clean up resource files
    if exist app.syso del app.syso
    if exist resource.syso del resource.syso
    
    echo.
    echo %APP_NAME%.exe is ready!
    echo To run: Double-click %APP_NAME%.exe
    echo Downloads will be saved to: Documents\TrustDrop Downloads\received
    echo.
    echo Installation: Copy %APP_NAME%.exe to desired location
) else (
    echo Build failed
    if exist app.syso del app.syso
    if exist resource.syso del resource.syso
    pause
    exit /b 1
)

echo.
pause