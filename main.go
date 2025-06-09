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

	// FIXED: Enhanced working directory setup for all platforms
	if err := setupWorkingDirectory(); err != nil {
		log.Printf("Warning: Could not set proper working directory: %v", err)
		// Continue anyway - use current directory as fallback
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

// FIXED: Cross-platform working directory setup
func setupWorkingDirectory() error {
	execPath, err := os.Executable()
	if err != nil {
		return err
	}

	var targetDir string

	switch runtime.GOOS {
	case "darwin":
		targetDir, err = setupMacOSWorkingDirectory(execPath)
	case "windows":
		targetDir, err = setupWindowsWorkingDirectory(execPath)
	case "linux":
		targetDir, err = setupLinuxWorkingDirectory(execPath)
	default:
		// Fallback: use directory containing executable
		targetDir = filepath.Dir(execPath)
	}

	if err != nil {
		return err
	}

	// Change to target directory
	if err := os.Chdir(targetDir); err != nil {
		return err
	}

	if os.Getenv("DEBUG") != "" {
		fmt.Printf("Set working directory to: %s\n", targetDir)
	}

	return nil
}

// FIXED: Enhanced macOS working directory setup
func setupMacOSWorkingDirectory(execPath string) (string, error) {
	// Check if we're running from a .app bundle
	if filepath.Base(filepath.Dir(execPath)) == "MacOS" {
		// We're in .app/Contents/MacOS/
		contentsDir := filepath.Dir(execPath)
		appDir := filepath.Dir(contentsDir)
		parentDir := filepath.Dir(appDir)
		
		// Strategy 1: Try to use the directory containing the .app bundle
		if isWritableDirectory(parentDir) {
			return parentDir, nil
		}
		
		// Strategy 2: Try the .app bundle's Contents directory
		if isWritableDirectory(contentsDir) {
			return contentsDir, nil
		}
		
		// Strategy 3: Use user's Documents directory for medical apps
		homeDir, err := os.UserHomeDir()
		if err == nil {
			documentsDir := filepath.Join(homeDir, "Documents", "TrustDrop")
			if err := os.MkdirAll(documentsDir, 0755); err == nil {
				if isWritableDirectory(documentsDir) {
					return documentsDir, nil
				}
			}
		}
		
		// Strategy 4: Fallback to user's home directory
		if err == nil {
			trustDropDir := filepath.Join(homeDir, "TrustDrop")
			if err := os.MkdirAll(trustDropDir, 0755); err == nil {
				if isWritableDirectory(trustDropDir) {
					return trustDropDir, nil
				}
			}
		}
		
		// Strategy 5: Last resort - use temp directory
		tempDir := filepath.Join(os.TempDir(), "TrustDrop")
		if err := os.MkdirAll(tempDir, 0755); err == nil {
			return tempDir, nil
		}
	}
	
	// If not in app bundle, use directory containing executable
	return filepath.Dir(execPath), nil
}

// FIXED: Windows working directory setup
func setupWindowsWorkingDirectory(execPath string) (string, error) {
	execDir := filepath.Dir(execPath)
	
	// Strategy 1: Try directory containing .exe
	if isWritableDirectory(execDir) {
		return execDir, nil
	}
	
	// Strategy 2: Use user's Documents directory
	homeDir, err := os.UserHomeDir()
	if err == nil {
		documentsDir := filepath.Join(homeDir, "Documents", "TrustDrop")
		if err := os.MkdirAll(documentsDir, 0755); err == nil {
			if isWritableDirectory(documentsDir) {
				return documentsDir, nil
			}
		}
	}
	
	// Strategy 3: Use AppData directory
	if err == nil {
		appDataDir := filepath.Join(homeDir, "AppData", "Local", "TrustDrop")
		if err := os.MkdirAll(appDataDir, 0755); err == nil {
			if isWritableDirectory(appDataDir) {
				return appDataDir, nil
			}
		}
	}
	
	// Fallback: use temp directory
	tempDir := filepath.Join(os.TempDir(), "TrustDrop")
	if err := os.MkdirAll(tempDir, 0755); err == nil {
		return tempDir, nil
	}
	
	return execDir, nil
}

// FIXED: Linux working directory setup
func setupLinuxWorkingDirectory(execPath string) (string, error) {
	execDir := filepath.Dir(execPath)
	
	// Strategy 1: Try directory containing executable
	if isWritableDirectory(execDir) {
		return execDir, nil
	}
	
	// Strategy 2: Use user's home directory
	homeDir, err := os.UserHomeDir()
	if err == nil {
		trustDropDir := filepath.Join(homeDir, ".trustdrop")
		if err := os.MkdirAll(trustDropDir, 0755); err == nil {
			if isWritableDirectory(trustDropDir) {
				return trustDropDir, nil
			}
		}
	}
	
	// Strategy 3: Use /tmp
	tempDir := filepath.Join("/tmp", "trustdrop")
	if err := os.MkdirAll(tempDir, 0755); err == nil {
		return tempDir, nil
	}
	
	return execDir, nil
}

// FIXED: Helper to test if directory is writable
func isWritableDirectory(dir string) bool {
	testFile := filepath.Join(dir, ".trustdrop_write_test")
	f, err := os.Create(testFile)
	if err != nil {
		return false
	}
	f.Close()
	os.Remove(testFile)
	return true
}

func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "unknown"
	}
	return dir
}