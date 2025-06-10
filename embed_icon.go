//go:build windows
// +build windows

package main

// This file helps embed icon resources for Windows builds
// The rsrc tool will generate app.syso file with embedded icon
// Usage: rsrc -ico image.png -o app.syso

// Resource embedding happens at build time via app.syso
