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
	fmt.Println("🌍 TrustDrop Bulletproof Edition - International Lab Transfer System")

	// Create TrustDrop Downloads folder with international naming
	var targetDataDir string
	var err error

	// Get user's home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Warning: Could not get user home directory: %v\n", err)
		targetDataDir = "."
	} else {
		// Try Documents first, then Desktop as fallback
		documentsDir := filepath.Join(homeDir, "Documents", "TrustDrop International")
		if err := os.MkdirAll(documentsDir, 0755); err == nil {
			targetDataDir = documentsDir
		} else {
			// Fallback to Desktop
			desktopDir := filepath.Join(homeDir, "Desktop", "TrustDrop International")
			if err := os.MkdirAll(desktopDir, 0755); err == nil {
				targetDataDir = desktopDir
			} else {
				// Final fallback to current directory
				fmt.Printf("Warning: Could not create TrustDrop International folder: %v\n", err)
				targetDataDir = "."
			}
		}
	}

	// Create data directory if needed
	if err := internal.EnsureDataDirectoryAtPath(targetDataDir); err != nil {
		fmt.Printf("Warning: %v\n", err)
	}

	fmt.Printf("🌍 TrustDrop International Lab Transfer System starting...\n")
	fmt.Printf("📁 Downloads will be saved to: %s\n", filepath.Join(targetDataDir, "received"))

	// Initialize transfer manager with international configuration
	fmt.Printf("🔧 Initializing international transfer manager...\n")
	transferManager, err := transfer.NewBulletproofTransferManager(targetDataDir)
	if err != nil {
		fmt.Printf("Failed to create international transfer manager: %v\n", err)
		return
	}
	defer transferManager.Close()

	// Set international-optimized callbacks
	transferManager.SetStatusCallback(func(status string) {
		fmt.Printf("🌍 International Status: %s\n", status)
	})

	transferManager.SetProgressCallback(func(current, total int64, fileName string) {
		if total > 0 {
			progress := float64(current) / float64(total) * 100
			fmt.Printf("🌍 International Progress: %.1f%% - %s\n", progress, fileName)
		}
	})

	fmt.Printf("✅ International transfer manager ready\n")

	// Create GUI with international branding
	fmt.Printf("🖥️  Creating international GUI...\n")
	app := gui.NewAppWithBulletproofManager(transferManager, targetDataDir)
	if app == nil {
		fmt.Printf("Failed to create international GUI\n")
		return
	}
	fmt.Printf("✅ International GUI ready\n")

	// Show international network status
	status := transferManager.GetNetworkStatus()
	fmt.Printf("🌍 International Network Status:\n")
	if transportStatus, ok := status["transport_status"].(map[string]interface{}); ok {
		fmt.Printf("   Available International Transports: %d\n", len(transportStatus))
	}

	fmt.Printf("🚀 TrustDrop International Lab Transfer System is ready!\n")
	fmt.Printf("📁 Your downloads will be saved to:\n   %s\n", filepath.Join(targetDataDir, "received"))
	fmt.Printf("🌍 Optimized for global lab-to-lab transfers through corporate firewalls\n")

	// Run the application
	app.Run()
}
