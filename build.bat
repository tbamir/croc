@echo off
setlocal enabledelayedexpansion

REM TrustDrop Bulletproof Edition - Production Build Script for Windows
REM Creates a proper .exe with embedded icon and resources

echo.
echo 🚀 TrustDrop - Windows Build
echo ========================

REM App configuration
set APP_NAME=TrustDrop
set DISPLAY_NAME=TrustDrop
set VERSION=1.0.0
set BUILD_VERSION=%date:~-4,4%%date:~-7,2%%date:~-10,2%_%time:~0,2%%time:~3,2%%time:~6,2%
set BUILD_VERSION=%BUILD_VERSION: =0%

echo Building: %DISPLAY_NAME% v%VERSION% (%BUILD_VERSION%)

REM Clean previous builds
echo 🧹 Cleaning previous builds...
if exist "%APP_NAME%.exe" del "%APP_NAME%.exe"
if exist "%APP_NAME%_*.exe" del "%APP_NAME%_*.exe"
if exist "icon.ico" del "icon.ico"
if exist "resource.syso" del "resource.syso"
if exist "trustdrop_icon.png" del "trustdrop_icon.png"
if exist "extract_icon.exe" del "extract_icon.exe"
if exist "build\" rmdir /s /q "build"

REM Create build directory
echo 📁 Creating build directory...
mkdir build 2>nul

REM Check for app icon
echo 🎨 Checking for app icon...
if exist "image.png" (
    echo ✅ Found image.png for app icon
) else (
    echo ❌ image.png not found - please add your app icon as image.png
    exit /b 1
)

REM Convert PNG to ICO format for Windows
echo 🎨 Creating Windows icon...
if exist "image.png" (
    REM Try using ImageMagick if available
    where magick >nul 2>nul
    if !errorlevel! equ 0 (
        echo    Using ImageMagick to convert PNG to ICO...
        magick image.png -resize 16x16 temp16.png
        magick image.png -resize 32x32 temp32.png
        magick image.png -resize 48x48 temp48.png
        magick image.png -resize 64x64 temp64.png
        magick image.png -resize 128x128 temp128.png
        magick image.png -resize 256x256 temp256.png
        magick temp16.png temp32.png temp48.png temp64.png temp128.png temp256.png icon.ico
        del temp16.png temp32.png temp48.png temp64.png temp128.png temp256.png >nul 2>nul
        echo ✅ Windows icon (.ico) created successfully
    ) else (
        echo ⚠️  ImageMagick not found, trying alternative method...
        REM Try using PowerShell as fallback
        powershell -command "Add-Type -AssemblyName System.Drawing; $img = [System.Drawing.Image]::FromFile('image.png'); $ico = [System.Drawing.Icon]::FromHandle($img.GetHIcon()); $ico.Save('icon.ico'); $img.Dispose()"
        if exist "icon.ico" (
            echo ✅ Windows icon created with PowerShell
        ) else (
            echo ⚠️  Icon conversion failed, continuing without icon
        )
    )
) else (
    echo ⚠️  PNG icon not found, skipping icon creation
)

REM Create Windows resource file for embedding icon and version info
echo 📄 Creating Windows resource file...
if exist "icon.ico" (
    echo #include "winres.h" > app.rc
    echo. >> app.rc
    echo // Icon >> app.rc
    echo IDI_ICON1 ICON "icon.ico" >> app.rc
    echo. >> app.rc
    echo // Version Information >> app.rc
    echo 1 VERSIONINFO >> app.rc
    echo FILEVERSION 1,0,0,0 >> app.rc
    echo PRODUCTVERSION 1,0,0,0 >> app.rc
    echo FILEFLAGSMASK 0x3fL >> app.rc
    echo FILEFLAGS 0x0L >> app.rc
    echo FILEOS 0x40004L >> app.rc
    echo FILETYPE 0x1L >> app.rc
    echo FILESUBTYPE 0x0L >> app.rc
    echo BEGIN >> app.rc
    echo     BLOCK "StringFileInfo" >> app.rc
    echo     BEGIN >> app.rc
    echo         BLOCK "040904b0" >> app.rc
    echo         BEGIN >> app.rc
    echo             VALUE "CompanyName", "TrustDrop" >> app.rc
    echo             VALUE "FileDescription", "%DISPLAY_NAME%" >> app.rc
    echo             VALUE "FileVersion", "%VERSION%" >> app.rc
    echo             VALUE "InternalName", "%APP_NAME%" >> app.rc
    echo             VALUE "LegalCopyright", "Copyright (C) 2024" >> app.rc
    echo             VALUE "OriginalFilename", "%APP_NAME%.exe" >> app.rc
    echo             VALUE "ProductName", "%DISPLAY_NAME%" >> app.rc
    echo             VALUE "ProductVersion", "%VERSION%" >> app.rc
    echo         END >> app.rc
    echo     END >> app.rc
    echo     BLOCK "VarFileInfo" >> app.rc
    echo     BEGIN >> app.rc
    echo         VALUE "Translation", 0x409, 1200 >> app.rc
    echo     END >> app.rc
    echo END >> app.rc

    REM Check if rsrc tool is available
    where rsrc >nul 2>nul
    if !errorlevel! neq 0 (
        echo 🔧 Installing rsrc tool for Windows resource embedding...
        go install github.com/akavel/rsrc@latest
    )

    REM Generate resource object file
    if exist "%GOPATH%\bin\rsrc.exe" (
        "%GOPATH%\bin\rsrc.exe" -manifest app.manifest -ico icon.ico -arch amd64 -o resource.syso
    ) else if exist "%USERPROFILE%\go\bin\rsrc.exe" (
        "%USERPROFILE%\go\bin\rsrc.exe" -manifest app.manifest -ico icon.ico -arch amd64 -o resource.syso
    ) else (
        rsrc -manifest app.manifest -ico icon.ico -arch amd64 -o resource.syso
    )

    if exist "resource.syso" (
        echo ✅ Windows resources embedded successfully
    ) else (
        echo ⚠️  Resource embedding failed, continuing without resources
    )
) else (
    echo ⚠️  No icon file found, skipping resource creation
)

REM Create Windows manifest for modern app appearance
echo 📄 Creating Windows manifest...
echo ^<?xml version="1.0" encoding="UTF-8" standalone="yes"?^> > app.manifest
echo ^<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0"^> >> app.manifest
echo   ^<assemblyIdentity version="%VERSION%.0" name="%APP_NAME%" type="win32"/^> >> app.manifest
echo   ^<description^>%DISPLAY_NAME%^</description^> >> app.manifest
echo   ^<dependency^> >> app.manifest
echo     ^<dependentAssembly^> >> app.manifest
echo       ^<assemblyIdentity type="win32" name="Microsoft.Windows.Common-Controls" version="6.0.0.0" processorArchitecture="*" publicKeyToken="6595b64144ccf1df" language="*"/^> >> app.manifest
echo     ^</dependentAssembly^> >> app.manifest
echo   ^</dependency^> >> app.manifest
echo   ^<trustInfo xmlns="urn:schemas-microsoft-com:asm.v2"^> >> app.manifest
echo     ^<security^> >> app.manifest
echo       ^<requestedPrivileges^> >> app.manifest
echo         ^<requestedExecutionLevel level="asInvoker" uiAccess="false"/^> >> app.manifest
echo       ^</requestedPrivileges^> >> app.manifest
echo     ^</security^> >> app.manifest
echo   ^</trustInfo^> >> app.manifest
echo   ^<application xmlns="urn:schemas-microsoft-com:asm.v3"^> >> app.manifest
echo     ^<windowsSettings^> >> app.manifest
echo       ^<dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings"^>true^</dpiAware^> >> app.manifest
echo       ^<dpiAwareness xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings"^>permonitorv2^</dpiAwareness^> >> app.manifest
echo     ^</windowsSettings^> >> app.manifest
echo   ^</application^> >> app.manifest
echo ^</assembly^> >> app.manifest

REM Build the Windows executable
echo 🔨 Building %APP_NAME%.exe...
set CGO_ENABLED=1
set GOOS=windows
set GOARCH=amd64

REM Build with version information and optimizations
go build -v ^
    -ldflags "-s -w -H=windowsgui" ^
    -o "build\%APP_NAME%.exe" ^
    main.go

if !errorlevel! neq 0 (
    echo ❌ Build failed
    exit /b 1
)

echo ✅ Binary built successfully

REM Move final executable to root directory
echo 📦 Finalizing executable...
move "build\%APP_NAME%.exe" "%APP_NAME%.exe" >nul

REM Clean up temporary files
echo 🧹 Cleaning up temporary files...
del icon.ico >nul 2>nul
del app.rc >nul 2>nul
del app.manifest >nul 2>nul
del resource.syso >nul 2>nul
rmdir /s /q build >nul 2>nul

REM Get executable size
for /f %%i in ('dir "%APP_NAME%.exe" ^| findstr "%APP_NAME%.exe"') do set SIZE=%%~zi
set /a SIZE_KB=%SIZE%/1024
set /a SIZE_MB=%SIZE_KB%/1024

echo.
echo 🎉 BUILD SUCCESSFUL! 🎉
echo ================================
echo App Name: %DISPLAY_NAME%
echo Version: %VERSION% (%BUILD_VERSION%)
echo File Size: %SIZE_MB% MB (%SIZE_KB% KB)
echo Location: %cd%\%APP_NAME%.exe
echo.
echo 📱 Installation Instructions:
echo    1. Double-click %APP_NAME%.exe to run
echo    2. Downloads saved to: Documents\TrustDrop Downloads\
echo    3. No installation required - portable executable
echo.
echo 🔒 Security Notes:
echo    - Windows Defender may scan the file on first run
echo    - If blocked, add to Windows Defender exclusions
echo    - Some antivirus may flag due to network functionality
echo.
echo 🚀 Ready for GitHub Release! 🚀
echo Upload %APP_NAME%.exe to your GitHub release

endlocal
pause 