package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"trustdrop/gui"
	"trustdrop/internal"
)

func main() {
	// Set up better logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Fix working directory for macOS .app bundles
	if runtime.GOOS == "darwin" {
		// When launched as .app bundle, working directory might be wrong
		if execPath, err := os.Executable(); err == nil {
			// Check if we're running from a .app bundle
			if filepath.Base(filepath.Dir(execPath)) == "MacOS" {
				// We're in .app/Contents/MacOS/, move to .app/Contents/Resources/
				resourcesDir := filepath.Join(filepath.Dir(filepath.Dir(execPath)), "Resources")
				if _, err := os.Stat(resourcesDir); err == nil {
					// Resources dir exists, but we should use the parent of .app
					appDir := filepath.Dir(filepath.Dir(filepath.Dir(execPath)))
					os.Chdir(appDir)
				}
			}
		}
	}

	// Only show console output in debug mode
	if os.Getenv("DEBUG") != "" {
		fmt.Printf("TrustDrop - Secure File Transfer\n")
		fmt.Printf("===============================\n")
		fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("Go Version: %s\n", runtime.Version())
		fmt.Printf("Working Directory: %s\n", getCurrentDir())
		fmt.Printf("\n")
	}

	// Ensure data directories exist with secure permissions
	if err := internal.EnsureDataDirectory(); err != nil {
		log.Fatalf("Failed to create data directories: %v", err)
	}

	// Enable croc debug logging only if DEBUG env var is set
	if os.Getenv("DEBUG") != "" {
		os.Setenv("CROC_DEBUG", "1")
		fmt.Println("Debug mode enabled")
		fmt.Println("Starting TrustDrop application...")
		fmt.Println("")
		fmt.Println("=== USAGE NOTES ===")
		fmt.Println("• To SEND: Click 'Send Files', copy your code, then select files")
		fmt.Println("• To RECEIVE: Click 'Receive Files' and enter the sender's code")
		fmt.Println("• All transfers use AES-256 encryption and blockchain logging")
		fmt.Println("• Received files are saved to: data/received/")
		fmt.Println("===================")
		fmt.Println("")
	}

	// Create and run the app - this blocks on the main thread
	app, err := gui.NewTrustDropApp()
	if err != nil {
		log.Fatalf("Failed to initialize TrustDrop: %v", err)
	}

	// This call blocks until the app is closed - critical for .app bundles
	app.Run()
}

func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return dir
}
