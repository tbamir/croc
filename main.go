package main

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"trustdrop/gui"
	"trustdrop/internal"
)

func main() {
	// Set up better logging
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	
	// Log system info for debugging
	fmt.Printf("TrustDrop - Secure Blockchain File Transfer\n")
	fmt.Printf("==========================================\n")
	fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Go Version: %s\n", runtime.Version())
	fmt.Printf("Working Directory: %s\n", getCurrentDir())
	fmt.Printf("\n")
	
	// Ensure data directories exist with secure permissions
	if err := internal.EnsureDataDirectory(); err != nil {
		log.Fatalf("Failed to create data directories: %v", err)
	}
	
	// Enable croc debug logging only if DEBUG env var is set
	if os.Getenv("DEBUG") != "" {
		os.Setenv("CROC_DEBUG", "1")
	}
	
	app, err := gui.NewTrustDropApp()
	if err != nil {
		log.Fatalf("Failed to initialize TrustDrop: %v", err)
	}
	
	fmt.Println("Starting application...")
	fmt.Println("")
	fmt.Println("=== IMPORTANT USAGE NOTES ===")
	fmt.Println("1. To SEND files: Click 'Send Files/Folders' and note your Peer ID")
	fmt.Println("2. To RECEIVE files: Enter the sender's Peer ID and click 'Start Receiving'")
	fmt.Println("3. All files are encrypted with AES-256 before transfer")
	fmt.Println("4. Received files are saved in: data/received/")
	fmt.Println("5. Transfer logs are immutably recorded on the blockchain")
	fmt.Println("=============================")
	fmt.Println("")
	
	app.Run()
}

func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return dir
}