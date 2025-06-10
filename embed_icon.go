//go:build windows
// +build windows

package main

// This file helps embed icon resources for Windows builds
// Two methods supported:
//
// Method 1 (Recommended): Windows Resource File
//   - Uses app.rc and icon.ico files
//   - Compile with: windres -i app.rc -o resource.syso
//   - Then build normally: go build
//
// Method 2 (Alternative): Direct ICO embedding
//   - Uses rsrc tool: go install github.com/akavel/rsrc@latest
//   - Usage: rsrc -ico icon.ico -o app.syso
//   - Then build normally: go build
//
// The build.bat script handles both methods automatically
// Resource embedding happens at build time via .syso files

// Windows icon requirements:
// - ICO format (not PNG)
// - Multiple sizes (16x16, 32x32, 48x48, 256x256)
// - Proper resource compilation
