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

	// FIXED: Force working directory to user's Documents for built applications
	if err := setupTrustDropWorkingDirectory(); err != nil {
		log.Printf("Warning: Could not set working directory: %v", err)
		// Don't exit - try to continue with current directory
	}

	// Always show current working directory in debug mode
	if os.Getenv("DEBUG") != "" {
		fmt.Printf("TrustDrop - Secure File Transfer\n")
		fmt.Printf("===============================\n")
		fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		fmt.Printf("Go Version: %s\n", runtime.Version())
		fmt.Printf("Working Directory: %s\n", getCurrentDir())
		fmt.Printf("Data Directory: %s\n", getDataDirectory())
		fmt.Printf("\n")
	}

	// Ensure data directories exist with secure permissions
	if err := internal.EnsureDataDirectory(); err != nil {
		log.Fatalf("Failed to create data directories: %v", err)
	}

	// Verify directories were actually created
	dataDir := getDataDirectory()
	if !directoryExists(dataDir) {
		log.Fatalf("Data directory was not created: %s", dataDir)
	}

	receivedDir := filepath.Join(dataDir, "received")
	if !directoryExists(receivedDir) {
		log.Fatalf("Received directory was not created: %s", receivedDir)
	}

	// Enable croc debug logging only if DEBUG env var is set
	if os.Getenv("DEBUG") != "" {
		os.Setenv("CROC_DEBUG", "1")
		fmt.Println("Debug mode enabled")
		fmt.Printf("Data directory: %s\n", dataDir)
		fmt.Printf("Received files will be saved to: %s\n", receivedDir)
		fmt.Println("Starting TrustDrop application...")
		fmt.Println("")
		fmt.Println("=== USAGE NOTES ===")
		fmt.Println("• To SEND: Click 'Send Files', copy your code, then select files")
		fmt.Println("• To RECEIVE: Click 'Receive Files' and enter the sender's code")
		fmt.Println("• All transfers use AES-256 encryption and blockchain logging")
		fmt.Printf("• Received files are saved to: %s\n", receivedDir)
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

// FIXED: Simplified working directory setup that always works
func setupTrustDropWorkingDirectory() error {
	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Create TrustDrop directory in Documents (or home on Linux)
	var trustDropDir string
	switch runtime.GOOS {
	case "darwin", "windows":
		trustDropDir = filepath.Join(homeDir, "Documents", "TrustDrop")
	case "linux":
		trustDropDir = filepath.Join(homeDir, ".trustdrop")
	default:
		trustDropDir = filepath.Join(homeDir, "TrustDrop")
	}

	// Create the directory if it doesn't exist
	if err := os.MkdirAll(trustDropDir, 0755); err != nil {
		return fmt.Errorf("failed to create TrustDrop directory: %w", err)
	}

	// Test write permissions
	testFile := filepath.Join(trustDropDir, ".write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("TrustDrop directory is not writable: %w", err)
	}
	f.Close()
	os.Remove(testFile)

	// Change to the TrustDrop directory
	if err := os.Chdir(trustDropDir); err != nil {
		return fmt.Errorf("failed to change to TrustDrop directory: %w", err)
	}

	if os.Getenv("DEBUG") != "" {
		fmt.Printf("Set working directory to: %s\n", trustDropDir)
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

func getDataDirectory() string {
	wd, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return filepath.Join(wd, "data")
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}