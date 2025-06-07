package transfer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

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

// FileManifest stores the mapping between anonymized filenames and original paths
type FileManifest struct {
	Files map[string]FileInfo `json:"files"` // hash -> original info
}

type FileInfo struct {
	OriginalPath   string `json:"original_path"`
	RelativePath   string `json:"relative_path"`
	IsDirectory    bool   `json:"is_directory"`
	Size           int64  `json:"size"`
	Hash           string `json:"hash"`
	AnonymizedName string `json:"anonymized_name"`
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

	fmt.Printf("TransferManager: Receive successful! Processing files...\n")

	// Process received files - decrypt and restore original structure
	files, err := filepath.Glob("*")
	if err != nil || len(files) == 0 {
		fmt.Printf("TransferManager: No files received\n")
		if tm.statusCb != nil {
			tm.statusCb("No files received")
		}
		return
	}

	if tm.statusCb != nil {
		tm.statusCb("Decrypting and restoring files...")
	}

	// Look for and decrypt the manifest file first
	var manifest *FileManifest
	manifestFound := false

	for _, file := range files {
		if file == "manifest.enc" {
			fmt.Printf("TransferManager: Found manifest file, decrypting...\n")

			encData, err := os.ReadFile(file)
			if err != nil {
				fmt.Printf("TransferManager: Failed to read manifest: %v\n", err)
				continue
			}

			decData, err := security.DecryptAES256CBC(encData, tm.encryptionKey)
			if err != nil {
				fmt.Printf("TransferManager: Failed to decrypt manifest: %v\n", err)
				continue
			}

			manifest = &FileManifest{}
			if err := json.Unmarshal(decData, manifest); err != nil {
				fmt.Printf("TransferManager: Failed to parse manifest: %v\n", err)
				continue
			}

			manifestFound = true
			fmt.Printf("TransferManager: Manifest loaded with %d file entries\n", len(manifest.Files))
			break
		}
	}

	if !manifestFound {
		fmt.Printf("TransferManager: No manifest found, using fallback decryption\n")
		tm.fallbackDecryption(files, originalDir)
		return
	}

	// Decrypt files using manifest
	successCount := 0
	for _, file := range files {
		if file == "manifest.enc" {
			continue // Skip the manifest file itself
		}

		fmt.Printf("TransferManager: Processing file: %s\n", file)

		// Read encrypted file
		encData, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("TransferManager: Failed to read %s: %v\n", file, err)
			continue
		}

		// Decrypt the file
		decData, err := security.DecryptAES256CBC(encData, tm.encryptionKey)
		if err != nil {
			fmt.Printf("TransferManager: Failed to decrypt %s: %v\n", file, err)
			continue
		}

		// Find original path from manifest
		fileInfo, exists := manifest.Files[file]
		if !exists {
			fmt.Printf("TransferManager: File %s not found in manifest, using fallback name\n", file)
			// Fallback to hash name
			finalPath := filepath.Join(originalDir, "data", "received", file)
			os.MkdirAll(filepath.Dir(finalPath), 0755)
			if err := os.WriteFile(finalPath, decData, 0644); err != nil {
				fmt.Printf("TransferManager: Failed to write fallback file: %v\n", err)
			} else {
				successCount++
			}
			continue
		}

		// Restore original path and filename
		finalPath := filepath.Join(originalDir, "data", "received", fileInfo.RelativePath)

		// Create directory structure
		if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
			fmt.Printf("TransferManager: Failed to create directory for %s: %v\n", finalPath, err)
			continue
		}

		// Write decrypted file with original name
		if err := os.WriteFile(finalPath, decData, 0644); err != nil {
			fmt.Printf("TransferManager: Failed to write %s: %v\n", finalPath, err)
			continue
		}

		// Calculate hash of decrypted file
		hash, err := tm.calculateFileHash(finalPath)
		if err == nil {
			tm.fileHashes[fileInfo.RelativePath] = hash
		}

		// Update transfer info
		if tm.currentFile == "" {
			tm.currentFile = fileInfo.RelativePath
			tm.currentSize = int64(len(decData))
		}

		fmt.Printf("TransferManager: ✅ Restored: %s (%.2f KB)\n", fileInfo.RelativePath, float64(len(decData))/1024)
		successCount++
	}

	fmt.Printf("TransferManager: Successfully restored %d files with original names and structure!\n", successCount)

	if tm.statusCb != nil {
		tm.statusCb(fmt.Sprintf("Transfer complete! %d files restored successfully", successCount))
	}
}

// fallbackDecryption handles files when no manifest is available
func (tm *TransferManager) fallbackDecryption(files []string, originalDir string) {
	fmt.Printf("TransferManager: Using fallback decryption mode\n")

	successCount := 0
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

		// Use hash name as filename for fallback
		finalPath := filepath.Join(originalDir, "data", "received", encFile)
		os.MkdirAll(filepath.Dir(finalPath), 0755)

		if err := os.WriteFile(finalPath, decData, 0644); err != nil {
			fmt.Printf("TransferManager: Failed to write decrypted file: %v\n", err)
			continue
		}

		// Calculate hash of decrypted file
		hash, err := tm.calculateFileHash(finalPath)
		if err == nil {
			tm.fileHashes[encFile] = hash
		}

		// Update transfer info
		if tm.currentFile == "" {
			tm.currentFile = encFile
			tm.currentSize = int64(len(decData))
		}

		fmt.Printf("TransferManager: Successfully decrypted %s\n", encFile)
		successCount++
	}

	if tm.statusCb != nil {
		if successCount > 0 {
			tm.statusCb(fmt.Sprintf("Files received and decrypted (%d files) - original names not preserved", successCount))
		} else {
			tm.statusCb("Failed to decrypt received files")
		}
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

	fmt.Printf("TransferManager: Preparing files for secure transfer...\n")

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

	// Create manifest to preserve file structure
	manifest := &FileManifest{
		Files: make(map[string]FileInfo),
	}

	// Process files and folders
	var filesToEncrypt []string
	var baseDir string

	// Determine if we're sending a single folder or multiple items
	if len(paths) == 1 {
		fileInfo, err := os.Stat(paths[0])
		if err == nil && fileInfo.IsDir() {
			baseDir = paths[0]
		}
	}

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

	if len(filesToEncrypt) == 0 {
		fmt.Printf("TransferManager: No files to send\n")
		if tm.statusCb != nil {
			tm.statusCb("No files found to send")
		}
		return
	}

	fmt.Printf("TransferManager: Encrypting %d files...\n", len(filesToEncrypt))

	// Encrypt files and build manifest
	var encryptedPaths []string
	for i, filePath := range filesToEncrypt {
		if tm.statusCb != nil {
			tm.statusCb(fmt.Sprintf("Encrypting file %d of %d...", i+1, len(filesToEncrypt)))
		}

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

		// Generate anonymized filename
		anonymizedName := hex.EncodeToString(sha256.New().Sum([]byte(filePath + hash)))[:32]

		// Create encrypted file path
		encFile := filepath.Join(tempDir, anonymizedName)
		if err := os.WriteFile(encFile, encData, 0600); err != nil {
			fmt.Printf("TransferManager: Failed to write encrypted file: %v\n", err)
			continue
		}

		encryptedPaths = append(encryptedPaths, encFile)

		// Determine relative path for the manifest
		var relativePath string
		if baseDir != "" {
			// We're sending a folder, preserve structure
			relPath, err := filepath.Rel(baseDir, filePath)
			if err != nil {
				relativePath = filepath.Base(filePath)
			} else {
				relativePath = relPath
			}
		} else {
			// Multiple files or single file, use base name
			relativePath = filepath.Base(filePath)
		}

		// Add to manifest
		manifest.Files[anonymizedName] = FileInfo{
			OriginalPath:   filePath,
			RelativePath:   relativePath,
			IsDirectory:    false,
			Size:           int64(len(data)),
			Hash:           hash,
			AnonymizedName: anonymizedName,
		}

		// Store file info for logging
		tm.fileHashes[relativePath] = hash
		if tm.currentFile == "" {
			tm.currentFile = relativePath
			tm.currentSize = int64(len(data))
		}

		fmt.Printf("TransferManager: ✅ Encrypted: %s -> %s\n", relativePath, anonymizedName)
	}

	if len(encryptedPaths) == 0 {
		fmt.Printf("TransferManager: No files encrypted successfully\n")
		if tm.statusCb != nil {
			tm.statusCb("Failed to encrypt files")
		}
		return
	}

	// Create and encrypt manifest
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		fmt.Printf("TransferManager: Failed to marshal manifest: %v\n", err)
		if tm.statusCb != nil {
			tm.statusCb("Failed to create file manifest")
		}
		return
	}

	encryptedManifest, err := security.EncryptAES256CBC(manifestData, tm.encryptionKey)
	if err != nil {
		fmt.Printf("TransferManager: Failed to encrypt manifest: %v\n", err)
		if tm.statusCb != nil {
			tm.statusCb("Failed to encrypt file manifest")
		}
		return
	}

	manifestPath := filepath.Join(tempDir, "manifest.enc")
	if err := os.WriteFile(manifestPath, encryptedManifest, 0600); err != nil {
		fmt.Printf("TransferManager: Failed to write manifest: %v\n", err)
		if tm.statusCb != nil {
			tm.statusCb("Failed to save file manifest")
		}
		return
	}

	// Add manifest to files to send
	encryptedPaths = append(encryptedPaths, manifestPath)

	fmt.Printf("TransferManager: Created manifest with %d file entries\n", len(manifest.Files))

	// Get file info for encrypted files
	filesInfo, emptyFolders, totalFolders, err := croc.GetFilesInfo(encryptedPaths, false, false, []string{})
	if err != nil {
		fmt.Printf("TransferManager: Failed to get files info: %v\n", err)
		if tm.statusCb != nil {
			tm.statusCb(fmt.Sprintf("Failed to prepare files: %v", err))
		}
		return
	}

	fmt.Printf("TransferManager: Prepared %d encrypted files for transfer\n", len(filesInfo))

	if tm.statusCb != nil {
		tm.statusCb("Starting secure transfer...")
	}

	// Configure croc for sending
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
	}

	// Initialize croc
	c, err := croc.New(options)
	if err != nil {
		fmt.Printf("TransferManager: Failed to initialize croc: %v\n", err)
		if tm.statusCb != nil {
			tm.statusCb(fmt.Sprintf("Failed to initialize transfer: %v", err))
		}
		return
	}

	tm.crocClient = c

	// Set transfer options
	c.FilesToTransfer = filesInfo
	c.EmptyFoldersToTransfer = emptyFolders
	c.TotalNumberFolders = totalFolders

	fmt.Printf("TransferManager: Starting encrypted transfer...\n")

	// Start sending - this blocks until complete or error
	err = c.Send(filesInfo, emptyFolders, totalFolders)
	if err != nil {
		fmt.Printf("TransferManager: Send failed: %v\n", err)
		if tm.statusCb != nil {
			tm.statusCb(fmt.Sprintf("Transfer failed: %v", err))
		}
		return
	}

	fmt.Printf("TransferManager: Transfer completed successfully!\n")
	if tm.statusCb != nil {
		tm.statusCb("Transfer completed successfully!")
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
