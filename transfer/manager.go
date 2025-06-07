package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"trustdrop/internal"
	"trustdrop/logging"
	"trustdrop/security"

	"github.com/schollz/croc/v10/src/croc"
	"github.com/schollz/croc/v10/src/utils"
)

type TransferManager struct {
	localPeerID   string
	mutex         sync.RWMutex
	progressCb    func(progress TransferProgress)
	statusCb      func(status string)
	isReceiving   bool
	isSending     bool
	currentSecret string
	crocClient    *croc.Client
	logger        *logging.Logger
	transferStart time.Time
	currentFile   string
	currentSize   int64
	encryptionKey []byte            // AES key for current transfer
	fileHashes    map[string]string // Map of filename to SHA256 hash
}

type TransferProgress struct {
	CurrentFile      string
	FilesRemaining   int
	PercentComplete  float64
	BytesTransferred int64
	TotalBytes       int64
}

func NewTransferManager() (*TransferManager, error) {
	// Generate local peer ID using croc's method
	localPeerID := utils.GetRandomName()

	fmt.Printf("TransferManager: Generated local peer ID: %s\n", localPeerID)

	// Initialize logger with blockchain
	logger, err := logging.NewLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	return &TransferManager{
		localPeerID: localPeerID,
		isReceiving: false,
		isSending:   false,
		logger:      logger,
		fileHashes:  make(map[string]string),
	}, nil
}

func (tm *TransferManager) GetLocalPeerID() string {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	return tm.localPeerID
}

func (tm *TransferManager) SetProgressCallback(cb func(TransferProgress)) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	tm.progressCb = cb
}

func (tm *TransferManager) SetStatusCallback(cb func(string)) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()
	tm.statusCb = cb
}

// StartReceive prepares to receive files using the given secret
func (tm *TransferManager) StartReceive(peerSecret string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if tm.isReceiving || tm.isSending {
		return fmt.Errorf("transfer already in progress")
	}

	fmt.Printf("TransferManager: Starting receive with peer secret: %s\n", peerSecret)

	tm.currentSecret = peerSecret
	tm.isReceiving = true
	tm.transferStart = time.Now()

	// Derive encryption key from the shared secret
	tm.deriveEncryptionKey(peerSecret)

	if tm.statusCb != nil {
		tm.statusCb("Waiting for sender...")
	}

	// Start receiving in background
	go tm.receiveFiles()

	return nil
}

// SendFiles initiates sending files using the local peer ID
func (tm *TransferManager) SendFiles(paths []string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if tm.isReceiving || tm.isSending {
		return fmt.Errorf("transfer already in progress")
	}

	fmt.Printf("TransferManager: Starting send with paths: %v\n", paths)

	tm.isSending = true
	tm.transferStart = time.Now()

	// Derive encryption key from our local peer ID
	tm.deriveEncryptionKey(tm.localPeerID)

	// Store file info for logging
	if len(paths) > 0 {
		tm.currentFile = paths[0]
		// In production, calculate actual file size
		tm.currentSize = 0
	}

	if tm.statusCb != nil {
		tm.statusCb("Preparing files...")
	}

	// Start sending in background
	go tm.sendFiles(paths)

	return nil
}

// deriveEncryptionKey derives an AES key from the shared secret
func (tm *TransferManager) deriveEncryptionKey(secret string) {
	// Use SHA-256 to derive a 32-byte key from the secret
	hash := sha256.Sum256([]byte(secret + "-trustdrop-aes"))
	tm.encryptionKey = hash[:]
}

// calculateFileHash calculates SHA-256 hash of a file
func (tm *TransferManager) calculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

func (tm *TransferManager) receiveFiles() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("TransferManager: Recovered from panic in receiveFiles: %v\n", r)
		}

		tm.mutex.Lock()

		// Log the transfer to blockchain
		duration := time.Since(tm.transferStart).String()
		status := "success"
		errorMsg := ""

		if tm.currentFile == "" {
			status = "failed"
			errorMsg = "No files received"
		}

		if tm.crocClient != nil && !tm.crocClient.SuccessfulTransfer {
			status = "failed"
			if errorMsg == "" {
				errorMsg = "Transfer failed"
			}
		}

		// Get file hash if we have a file
		fileHash := ""
		if tm.currentFile != "" && status == "success" {
			if hash, ok := tm.fileHashes[tm.currentFile]; ok {
				fileHash = hash
			}
		}

		tm.logTransfer(tm.currentSecret, tm.currentFile, fileHash, tm.currentSize, "receive", status, errorMsg, duration)

		tm.isReceiving = false
		tm.currentSecret = ""
		tm.crocClient = nil
		tm.encryptionKey = nil
		tm.fileHashes = make(map[string]string)
		tm.mutex.Unlock()
	}()

	fmt.Printf("TransferManager: Configuring croc for receiving...\n")

	// Create temporary directory for encrypted files
	tempDir := filepath.Join("data", "temp", fmt.Sprintf("receive-%d", time.Now().Unix()))
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		fmt.Printf("TransferManager: Failed to create temp dir: %v\n", err)
		if tm.statusCb != nil {
			tm.statusCb(fmt.Sprintf("Failed to create temp directory: %v", err))
		}
		return
	}
	defer os.RemoveAll(tempDir) // Clean up temp directory

	// Change to temp directory for receiving
	originalDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(originalDir)

	// Create croc options for receiving
	options := croc.Options{
		IsSender:       false,
		SharedSecret:   tm.currentSecret,
		RelayAddress:   "croc.schollz.com:9009",
		RelayAddress6:  "croc6.schollz.com:9009",
		RelayPorts:     []string{"9009", "9010", "9011", "9012", "9013"},
		RelayPassword:  "pass123",
		NoPrompt:       true,
		NoMultiplexing: false,
		DisableLocal:   false,
		Ask:            false,
		Debug:          true,
		Overwrite:      true,
		Curve:          "p256",
		HashAlgorithm:  "xxhash",
	}

	// Initialize croc
	c, err := croc.New(options)
	if err != nil {
		fmt.Printf("TransferManager: Failed to initialize croc: %v\n", err)
		if tm.statusCb != nil {
			tm.statusCb(fmt.Sprintf("Failed to initialize: %v", err))
		}
		return
	}

	tm.crocClient = c

	if tm.statusCb != nil {
		tm.statusCb("Connecting to sender...")
	}

	fmt.Printf("TransferManager: Starting croc receive...\n")

	// Start receiving - this blocks until complete or error
	err = c.Receive()
	if err != nil {
		fmt.Printf("TransferManager: Receive failed: %v\n", err)
		if tm.statusCb != nil {
			tm.statusCb(fmt.Sprintf("Receive failed: %v", err))
		}
		return
	}

	fmt.Printf("TransferManager: Receive successful!\n")

	// Decrypt received files
	files, err := filepath.Glob("*")
	if err == nil && len(files) > 0 {
		for _, encFile := range files {
			fmt.Printf("TransferManager: Decrypting %s\n", encFile)

			// Read encrypted file
			encData, err := os.ReadFile(encFile)
			if err != nil {
				fmt.Printf("TransferManager: Failed to read encrypted file: %v\n", err)
				continue
			}

			// Decrypt the file
			decData, err := security.DecryptAES256CBC(encData, tm.encryptionKey)
			if err != nil {
				fmt.Printf("TransferManager: Failed to decrypt file: %v\n", err)
				continue
			}

			// Remove .enc extension if present
			decFile := encFile
			if filepath.Ext(encFile) == ".enc" {
				decFile = encFile[:len(encFile)-4]
			}

			// Write decrypted file to final location with secure permissions
			finalPath := filepath.Join(originalDir, "data", "received", decFile)
			os.MkdirAll(filepath.Dir(finalPath), 0700)

			if err := os.WriteFile(finalPath, decData, 0600); err != nil {
				fmt.Printf("TransferManager: Failed to write decrypted file: %v\n", err)
				continue
			}

			// Calculate hash of decrypted file
			hash, err := tm.calculateFileHash(finalPath)
			if err == nil {
				tm.fileHashes[decFile] = hash
			}

			// Update transfer info
			if tm.currentFile == "" {
				tm.currentFile = decFile
				tm.currentSize = int64(len(decData))
			}

			fmt.Printf("TransferManager: Successfully decrypted %s to %s\n", encFile, finalPath)
		}
	}

	if tm.statusCb != nil {
		tm.statusCb("Files received and decrypted successfully!")
	}
}

func (tm *TransferManager) sendFiles(paths []string) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("TransferManager: Recovered from panic in sendFiles: %v\n", r)
		}

		tm.mutex.Lock()

		// Log the transfer to blockchain
		duration := time.Since(tm.transferStart).String()
		status := "success"
		errorMsg := ""

		if tm.crocClient == nil || !tm.crocClient.SuccessfulTransfer {
			status = "failed"
			errorMsg = "Transfer failed"
		}

		// Get file hash
		fileHash := ""
		if tm.currentFile != "" && status == "success" {
			if hash, ok := tm.fileHashes[tm.currentFile]; ok {
				fileHash = hash
			}
		}

		tm.logTransfer(tm.localPeerID, tm.currentFile, fileHash, tm.currentSize, "send", status, errorMsg, duration)

		tm.isSending = false
		tm.crocClient = nil
		tm.encryptionKey = nil
		tm.fileHashes = make(map[string]string)
		tm.mutex.Unlock()
	}()

	fmt.Printf("TransferManager: Getting file info for paths: %v\n", paths)

	// Create temporary directory for encrypted files
	tempDir := filepath.Join("data", "temp", fmt.Sprintf("send-%d", time.Now().Unix()))
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		fmt.Printf("TransferManager: Failed to create temp dir: %v\n", err)
		if tm.statusCb != nil {
			tm.statusCb(fmt.Sprintf("Failed to create temp directory: %v", err))
		}
		return
	}
	defer os.RemoveAll(tempDir) // Clean up temp directory

	// Process files and folders
	var filesToEncrypt []string
	for _, path := range paths {
		fileInfo, err := os.Stat(path)
		if err != nil {
			fmt.Printf("TransferManager: Failed to stat path %s: %v\n", path, err)
			continue
		}

		if fileInfo.IsDir() {
			// Walk through directory and collect all files
			err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() {
					filesToEncrypt = append(filesToEncrypt, filePath)
				}
				return nil
			})
			if err != nil {
				fmt.Printf("TransferManager: Failed to walk directory %s: %v\n", path, err)
				continue
			}
		} else {
			// Single file
			filesToEncrypt = append(filesToEncrypt, path)
		}
	}

	// Encrypt files before sending
	var encryptedPaths []string
	for _, filePath := range filesToEncrypt {
		// Calculate hash of original file
		hash, err := tm.calculateFileHash(filePath)
		if err != nil {
			fmt.Printf("TransferManager: Failed to calculate hash for %s: %v\n", filePath, err)
			continue
		}

		// Read original file
		data, err := os.ReadFile(filePath)
		if err != nil {
			fmt.Printf("TransferManager: Failed to read file %s: %v\n", filePath, err)
			continue
		}

		// Encrypt the file
		encData, err := security.EncryptAES256CBC(data, tm.encryptionKey)
		if err != nil {
			fmt.Printf("TransferManager: Failed to encrypt file %s: %v\n", filePath, err)
			continue
		}

		// Preserve relative path structure for folders, but sanitize filenames
		var encFile string
		if len(paths) == 1 && len(filesToEncrypt) > 1 {
			// This is a folder transfer, preserve structure
			baseDir := paths[0]
			relPath, _ := filepath.Rel(baseDir, filePath)
			// Sanitize the relative path to handle problematic Unicode characters
			sanitizedRelPath := internal.SanitizePath(relPath)
			encFile = filepath.Join(tempDir, sanitizedRelPath+".enc")
			// Create subdirectories if needed
			encDir := filepath.Dir(encFile)
			if err := os.MkdirAll(encDir, 0700); err != nil {
				fmt.Printf("TransferManager: Failed to create directory %s: %v\n", encDir, err)
				continue
			}
		} else {
			// Single file or multiple individual files
			// Sanitize the filename to handle problematic Unicode characters
			sanitizedBaseName := internal.SanitizeFilename(filepath.Base(filePath))
			encFile = filepath.Join(tempDir, sanitizedBaseName+".enc")
		}

		if err := os.WriteFile(encFile, encData, 0600); err != nil {
			fmt.Printf("TransferManager: Failed to write encrypted file: %v\n", err)
			continue
		}

		encryptedPaths = append(encryptedPaths, encFile)

		// Store file info using sanitized basename to match what gets transferred
		baseName := filepath.Base(encFile)
		if filepath.Ext(baseName) == ".enc" {
			baseName = baseName[:len(baseName)-4] // Remove .enc extension
		}
		tm.fileHashes[baseName] = hash
		if tm.currentFile == "" {
			tm.currentFile = baseName
			tm.currentSize = int64(len(data))
		}

		fmt.Printf("TransferManager: Encrypted %s -> %s (hash: %s)\n", filePath, encFile, hash)
	}

	if len(encryptedPaths) == 0 {
		fmt.Printf("TransferManager: No files to send after encryption\n")
		if tm.statusCb != nil {
			tm.statusCb("Failed to encrypt files")
		}
		return
	}

	// Get file info for encrypted files
	filesInfo, emptyFolders, totalFolders, err := croc.GetFilesInfo(encryptedPaths, false, false, []string{})
	if err != nil {
		fmt.Printf("TransferManager: Failed to analyze files: %v\n", err)
		if tm.statusCb != nil {
			tm.statusCb(fmt.Sprintf("Failed to analyze files: %v", err))
		}
		return
	}

	// Create croc options for sending
	options := croc.Options{
		IsSender:       true,
		SharedSecret:   tm.localPeerID,
		RelayAddress:   "croc.schollz.com:9009",
		RelayAddress6:  "croc6.schollz.com:9009",
		RelayPorts:     []string{"9009", "9010", "9011", "9012", "9013"},
		RelayPassword:  "pass123",
		NoPrompt:       true,
		NoMultiplexing: false,
		DisableLocal:   false,
		Ask:            false,
		Debug:          true,
		Overwrite:      true,
		Curve:          "p256",
		HashAlgorithm:  "xxhash",
		SendingText:    false,
		NoCompress:     false,
	}

	// Initialize croc
	c, err := croc.New(options)
	if err != nil {
		fmt.Printf("TransferManager: Failed to initialize croc: %v\n", err)
		if tm.statusCb != nil {
			tm.statusCb(fmt.Sprintf("Failed to initialize: %v", err))
		}
		return
	}

	tm.crocClient = c

	if tm.statusCb != nil {
		tm.statusCb("Waiting for receiver to connect...")
	}

	fmt.Printf("TransferManager: Starting croc send...\n")
	fmt.Printf("TransferManager: Receiver should use code: %s\n", tm.localPeerID)

	// Start sending - this blocks until complete or error
	err = c.Send(filesInfo, emptyFolders, totalFolders)
	if err != nil {
		fmt.Printf("TransferManager: Send failed: %v\n", err)
		if tm.statusCb != nil {
			tm.statusCb(fmt.Sprintf("Send failed: %v", err))
		}
	} else {
		fmt.Printf("TransferManager: Send successful!\n")
		if tm.statusCb != nil {
			tm.statusCb("Files sent successfully!")
		}
	}
}

// logTransfer logs the transfer to the blockchain
func (tm *TransferManager) logTransfer(peerID, fileName, fileHash string, fileSize int64, direction, status, errorMsg, duration string) {
	log := logging.TransferLog{
		Timestamp: time.Now(),
		PeerID:    peerID,
		FileName:  fileName,
		FileHash:  fileHash,
		FileSize:  fileSize,
		Direction: direction,
		Status:    status,
		Error:     errorMsg,
		Duration:  duration,
	}

	if err := tm.logger.LogTransfer(log); err != nil {
		// Log error but don't fail the transfer
		fmt.Printf("Failed to log transfer to blockchain: %v\n", err)
	} else {
		fmt.Printf("Transfer logged to blockchain: %s %s (%s)\n", direction, fileName, status)
	}
}

// IsTransferActive returns true if any transfer is in progress
func (tm *TransferManager) IsTransferActive() bool {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	return tm.isReceiving || tm.isSending
}

// CancelTransfer cancels any active transfer
func (tm *TransferManager) CancelTransfer() {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	fmt.Printf("TransferManager: Cancelling transfer...\n")

	// Log cancelled transfer
	if tm.isReceiving || tm.isSending {
		direction := "receive"
		if tm.isSending {
			direction = "send"
		}
		duration := time.Since(tm.transferStart).String()

		// Get file hash if available
		fileHash := ""
		if hash, ok := tm.fileHashes[tm.currentFile]; ok {
			fileHash = hash
		}

		tm.logTransfer(tm.localPeerID, tm.currentFile, fileHash, tm.currentSize, direction, "cancelled", "User cancelled", duration)
	}

	tm.isReceiving = false
	tm.isSending = false
	tm.currentSecret = ""
	tm.encryptionKey = nil
	tm.fileHashes = make(map[string]string)

	// Note: We can't actually cancel the croc transfer once started
	// This is a limitation of the croc library
	tm.crocClient = nil

	if tm.statusCb != nil {
		tm.statusCb("Transfer cancelled")
	}
}

// GetLogger returns the logger instance for external use
func (tm *TransferManager) GetLogger() *logging.Logger {
	return tm.logger
}
