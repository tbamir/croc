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

	// FIXED: Improved working directory handling for macOS .app bundles
	if runtime.GOOS == "darwin" {
		if err := setupMacOSWorkingDirectory(); err != nil {
			log.Printf("Warning: Could not set proper working directory: %v", err)
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

// FIXED: Improved macOS working directory setup
func setupMacOSWorkingDirectory() error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	// Check if we're running from a .app bundle
	if filepath.Base(filepath.Dir(execPath)) == "MacOS" {
		// We're in .app/Contents/MacOS/
		contentsDir := filepath.Dir(execPath)
		appDir := filepath.Dir(contentsDir)
		
		// Try to use the directory containing the .app bundle
		parentDir := filepath.Dir(appDir)
		
		// Verify the parent directory is writable
		testFile := filepath.Join(parentDir, ".trustdrop_test")
		if f, err := os.Create(testFile); err == nil {
			f.Close()
			os.Remove(testFile)
			
			// Change to the parent directory (where .app is located)
			if err := os.Chdir(parentDir); err == nil {
				if os.Getenv("DEBUG") != "" {
					fmt.Printf("Set working directory to: %s\n", parentDir)
				}
				return nil
			}
		}
		
		// Fallback: Use user's home directory
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		
		trustDropDir := filepath.Join(homeDir, "TrustDrop")
		if err := os.MkdirAll(trustDropDir, 0755); err != nil {
			return err
		}
		
		if err := os.Chdir(trustDropDir); err != nil {
			return err
		}
		
		if os.Getenv("DEBUG") != "" {
			fmt.Printf("Set working directory to: %s\n", trustDropDir)
		}
	}
	
	return nil
}

func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return dir
}