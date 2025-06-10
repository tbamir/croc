package main

import (
	"fmt"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/widget"

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

	// Ensure the data directory structure exists
	err = internal.EnsureDataDirectoryAtPath(targetDataDir)
	if err != nil {
		fmt.Printf("Error creating data directory: %v\n", err)
		targetDataDir = "." // Fallback to current directory
	}

	fmt.Printf("🚀 TrustDrop starting...\n")
	fmt.Printf("📂 Downloads will be saved to: %s\n", filepath.Join(targetDataDir, "received"))

	// Initialize bulletproof transfer manager
	fmt.Printf("⚙️  Initializing transfer manager...\n")
	transferManager, err := transfer.NewBulletproofTransferManager(targetDataDir)
	if err != nil {
		fmt.Printf("❌ Failed to initialize transfer manager: %v\n", err)
		// Fallback to basic GUI without bulletproof features
		basicApp := app.New()
		basicWindow := basicApp.NewWindow("TrustDrop - Error")
		basicWindow.SetContent(widget.NewLabel(fmt.Sprintf("Failed to initialize: %v", err)))
		basicWindow.ShowAndRun()
		return
	}
	defer transferManager.Close()

	fmt.Printf("✅ Transfer manager ready\n")

	// Create and run GUI with bulletproof manager
	fmt.Printf("🎨 Creating GUI...\n")
	bulletproofApp := gui.NewAppWithBulletproofManager(transferManager, targetDataDir)
	if bulletproofApp == nil {
		fmt.Printf("❌ Failed to create GUI application\n")
		return
	}

	fmt.Printf("✅ GUI ready\n")

	// Display network status
	networkStatus := transferManager.GetNetworkStatus()
	fmt.Printf("🌐 Network Status:\n")
	if transportStatus, ok := networkStatus["transport_status"].(map[string]interface{}); ok {
		fmt.Printf("   Available Transports: %d\n", len(transportStatus))
	}

	fmt.Printf("🎉 TrustDrop is ready!\n")
	fmt.Printf("📋 Your downloads will be saved to:\n   %s\n", filepath.Join(targetDataDir, "received"))

	// Run the application
	bulletproofApp.Run()
}
