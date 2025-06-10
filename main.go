package main

import (
	"fmt"
	"os"
	"path/filepath"

	"trustdrop-bulletproof/gui"
	"trustdrop-bulletproof/internal"
	"trustdrop-bulletproof/transfer"
)

func main() {
	// Create TrustDrop Downloads folder in user's Documents or Desktop
	var targetDataDir string
	var err error

	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Warning: Could not get user home directory: %v\n", err)
		targetDataDir = "."
	} else {
		// Try Documents first, then Desktop as fallback
		documentsDir := filepath.Join(homeDir, "Documents", "TrustDrop Downloads")
		if err := os.MkdirAll(documentsDir, 0755); err == nil {
			targetDataDir = documentsDir
		} else {
			// Fallback to Desktop
			desktopDir := filepath.Join(homeDir, "Desktop", "TrustDrop Downloads")
			if err := os.MkdirAll(desktopDir, 0755); err == nil {
				targetDataDir = desktopDir
			} else {
				// Final fallback to current directory
				fmt.Printf("Warning: Could not create TrustDrop Downloads folder: %v\n", err)
				targetDataDir = "."
			}
		}
	}

	// Create data directory if needed
	if err := internal.EnsureDataDirectoryAtPath(targetDataDir); err != nil {
		fmt.Printf("Warning: %v\n", err)
	}

	fmt.Printf("TrustDrop starting...\n")
	fmt.Printf("Downloads will be saved to: %s\n", filepath.Join(targetDataDir, "received"))

	// Initialize transfer manager with data directory
	fmt.Printf("Initializing transfer manager...\n")
	transferManager, err := transfer.NewBulletproofTransferManager(targetDataDir)
	if err != nil {
		fmt.Printf("Failed to create transfer manager: %v\n", err)
		return
	}
	defer transferManager.Close()

	// Adapt settings to network conditions
	transferManager.SetStatusCallback(func(status string) {
		fmt.Printf("Status: %s\n", status)
	})

	transferManager.SetProgressCallback(func(current, total int64, fileName string) {
		if total > 0 {
			progress := float64(current) / float64(total) * 100
			fmt.Printf("Progress: %.1f%% - %s\n", progress, fileName)
		}
	})

	fmt.Printf("Transfer manager ready\n")

	// Create GUI
	fmt.Printf("Creating GUI...\n")
	app := gui.NewAppWithBulletproofManager(transferManager, targetDataDir)
	if app == nil {
		fmt.Printf("Failed to create GUI\n")
		return
	}
	fmt.Printf("GUI ready\n")

	// Show network status
	status := transferManager.GetNetworkStatus()
	fmt.Printf("Network Status:\n")
	if transportStatus, ok := status["transport_status"].(map[string]interface{}); ok {
		fmt.Printf("   Available Transports: %d\n", len(transportStatus))
	}

	fmt.Printf("TrustDrop is ready!\n")
	fmt.Printf("Your downloads will be saved to:\n   %s\n", filepath.Join(targetDataDir, "received"))

	// Run the application
	app.Run()
}
