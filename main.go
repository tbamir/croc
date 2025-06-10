package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/widget"

	"trustdrop-bulletproof/gui"
	"trustdrop-bulletproof/internal"
	"trustdrop-bulletproof/transfer"
)

func main() {
	// Determine target data directory based on platform and context
	var targetDataDir string
	var err error

	if runtime.GOOS == "darwin" {
		// macOS: Check if running from app bundle
		executablePath, err := os.Executable()
		if err != nil {
			fmt.Printf("Error getting executable path: %v\n", err)
			targetDataDir = "."
		} else {
			// Check if running from within app bundle
			if filepath.Dir(executablePath) == "/Applications/TrustDrop.app/Contents/MacOS" ||
				filepath.Base(filepath.Dir(filepath.Dir(executablePath))) == "Contents" {
				// Running from app bundle - use Desktop
				homeDir, err := os.UserHomeDir()
				if err != nil {
					targetDataDir = "."
				} else {
					targetDataDir = filepath.Join(homeDir, "Desktop", "TrustDrop")
				}
			} else {
				// Running from development or terminal - use current directory
				targetDataDir = "."
			}
		}
	} else {
		// Windows and other platforms - use current directory
		targetDataDir = "."
	}

	// Ensure the data directory exists
	err = internal.EnsureDataDirectoryAtPath(targetDataDir)
	if err != nil {
		fmt.Printf("Error creating data directory: %v\n", err)
		targetDataDir = "." // Fallback to current directory
	}

	fmt.Printf("TrustDrop Bulletproof Edition starting...\n")
	fmt.Printf("Data directory: %s\n", targetDataDir)

	// Initialize bulletproof transfer manager
	fmt.Printf("Initializing bulletproof transfer manager...\n")
	transferManager, err := transfer.NewBulletproofTransferManager(targetDataDir)
	if err != nil {
		fmt.Printf("Failed to initialize bulletproof transfer manager: %v\n", err)
		// Fallback to basic GUI without bulletproof features
		basicApp := app.New()
		basicWindow := basicApp.NewWindow("TrustDrop - Error")
		basicWindow.SetContent(widget.NewLabel(fmt.Sprintf("Failed to initialize: %v", err)))
		basicWindow.ShowAndRun()
		return
	}
	defer transferManager.Close()

	fmt.Printf("Transfer manager initialized successfully\n")

	// Create and run GUI with bulletproof manager
	fmt.Printf("Creating GUI...\n")
	bulletproofApp := gui.NewAppWithBulletproofManager(transferManager, targetDataDir)
	if bulletproofApp == nil {
		fmt.Printf("Failed to create GUI application\n")
		return
	}

	fmt.Printf("GUI created successfully\n")

	// Display network status
	fmt.Printf("Getting network status...\n")
	networkStatus := transferManager.GetNetworkStatus()
	fmt.Printf("Network Analysis Complete:\n")
	fmt.Printf("- Network Profile: %+v\n", networkStatus["network_profile"])
	fmt.Printf("- Available Transports: %+v\n", networkStatus["transport_status"])

	// Run the application
	fmt.Printf("Starting GUI...\n")
	bulletproofApp.Run()
}
