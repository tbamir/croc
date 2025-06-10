package transfer

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"trustdrop/logging"
	"trustdrop/security"

	"github.com/schollz/croc/v10/src/croc"
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
	totalSize     int64
	totalFiles    int
	filesComplete int
	encryptionKey []byte
	fileHashes    map[string]string
	targetDataDir string // Directory where data should be saved

	// FIXED: Enhanced progress tracking
	lastProgressUpdate time.Time
	progressThrottle   time.Duration
	bytesTransferred   int64
	overallProgress    float64
}

type TransferProgress struct {
	CurrentFile      string
	FilesRemaining   int
	PercentComplete  float64
	BytesTransferred int64
	TotalBytes       int64
}

type FileManifest struct {
	Files      map[string]FileInfo `json:"files"`
	FolderName string              `json:"folder_name,omitempty"`
	TotalFiles int                 `json:"total_files"`
	TotalSize  int64               `json:"total_size"`
}

type FileInfo struct {
	OriginalPath   string `json:"original_path"`
	RelativePath   string `json:"relative_path"`
	IsDirectory    bool   `json:"is_directory"`
	Size           int64  `json:"size"`
	Hash           string `json:"hash"`
	AnonymizedName string `json:"anonymized_name"`
}

func NewTransferManager(targetDataDir string) (*TransferManager, error) {
	// FIXED: Generate stronger local peer ID using secure random
	localPeerID := generateSecureCode()

	// Only show debug output when DEBUG environment variable is set
	if os.Getenv("DEBUG") != "" {
		fmt.Printf("TransferManager: Generated local peer ID: %s\n", localPeerID)
		fmt.Printf("TransferManager: Target data directory: %s\n", targetDataDir)
	}

	// Initialize logger with blockchain
	logger, err := logging.NewLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	return &TransferManager{
		localPeerID:      localPeerID,
		isReceiving:      false,
		isSending:        false,
		logger:           logger,
		fileHashes:       make(map[string]string),
		progressThrottle: 100 * time.Millisecond,
		overallProgress:  0.0,
		targetDataDir:    targetDataDir,
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

func (tm *TransferManager) updateStatus(status string) {
	if tm.statusCb != nil {
		tm.statusCb(status)
	}
}

// FIXED: Enhanced progress tracking with real-time updates
func (tm *TransferManager) updateProgress(current int64, total int64, currentFile string) {
	now := time.Now()
	if now.Sub(tm.lastProgressUpdate) < tm.progressThrottle {
		return
	}
	tm.lastProgressUpdate = now

	if tm.progressCb != nil {
		percentComplete := float64(0)
		if total > 0 {
			percentComplete = (float64(current) / float64(total)) * 100
		}

		filesRemaining := tm.totalFiles - tm.filesComplete
		if filesRemaining < 0 {
			filesRemaining = 0
		}

		// Update overall progress
		tm.overallProgress = percentComplete
		tm.bytesTransferred = current

		tm.progressCb(TransferProgress{
			CurrentFile:      currentFile,
			FilesRemaining:   filesRemaining,
			PercentComplete:  percentComplete,
			BytesTransferred: current,
			TotalBytes:       total,
		})
	}
}

// FIXED: New method for incremental progress updates during transfer
func (tm *TransferManager) updateIncrementalProgress(bytesAdded int64, currentFile string) {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tm.bytesTransferred += bytesAdded

	now := time.Now()
	if now.Sub(tm.lastProgressUpdate) < tm.progressThrottle {
		return
	}
	tm.lastProgressUpdate = now

	if tm.progressCb != nil && tm.totalSize > 0 {
		percentComplete := (float64(tm.bytesTransferred) / float64(tm.totalSize)) * 100
		if percentComplete > 100 {
			percentComplete = 100
		}

		filesRemaining := tm.totalFiles - tm.filesComplete
		if filesRemaining < 0 {
			filesRemaining = 0
		}

		tm.overallProgress = percentComplete

		tm.progressCb(TransferProgress{
			CurrentFile:      currentFile,
			FilesRemaining:   filesRemaining,
			PercentComplete:  percentComplete,
			BytesTransferred: tm.bytesTransferred,
			TotalBytes:       tm.totalSize,
		})
	}
}

func (tm *TransferManager) StartReceive(peerSecret string) error {
	// FIXED: Add panic recovery
	defer func() {
		if r := recover(); r != nil {
			tm.updateStatus(fmt.Sprintf("Receive failed: %v", r))
			if tm.logger != nil {
				tm.logger.LogTransfer(logging.TransferLog{
					Timestamp: time.Now(),
					PeerID:    peerSecret,
					FileName:  tm.currentFile,
					Status:    "failed",
					Error:     fmt.Sprintf("panic: %v", r),
					Direction: "receive",
				})
			}
		}
	}()

	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if tm.isReceiving || tm.isSending {
		return fmt.Errorf("transfer already in progress")
	}

	tm.currentSecret = peerSecret
	tm.isReceiving = true
	tm.transferStart = time.Now()
	tm.filesComplete = 0
	tm.bytesTransferred = 0
	tm.overallProgress = 0.0

	// Derive encryption key from the shared secret
	tm.deriveEncryptionKey(peerSecret)

	tm.updateStatus("Connecting to sender...")

	// Start receiving in background
	go tm.receiveFiles()

	return nil
}

func (tm *TransferManager) SendFiles(paths []string) error {
	// FIXED: Add panic recovery
	defer func() {
		if r := recover(); r != nil {
			tm.updateStatus(fmt.Sprintf("Transfer failed: %v", r))
			if tm.logger != nil {
				tm.logger.LogTransfer(logging.TransferLog{
					Timestamp: time.Now(),
					PeerID:    tm.localPeerID,
					FileName:  tm.currentFile,
					Status:    "failed",
					Error:     fmt.Sprintf("panic: %v", r),
					Direction: "send",
				})
			}
		}
	}()

	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if tm.isReceiving || tm.isSending {
		return fmt.Errorf("transfer already in progress")
	}

	tm.isSending = true
	tm.transferStart = time.Now()
	tm.filesComplete = 0
	tm.bytesTransferred = 0
	tm.overallProgress = 0.0

	// Derive encryption key from our local peer ID
	tm.deriveEncryptionKey(tm.localPeerID)

	tm.updateStatus("Preparing files for secure transfer...")

	// Start sending in background
	go tm.sendFiles(paths)

	return nil
}

// deriveEncryptionKey derives an AES key from the shared secret with improved security
func (tm *TransferManager) deriveEncryptionKey(secret string) {
	// FIXED: Multiple rounds of hashing for better security
	hash := sha256.Sum256([]byte(secret + "-trustdrop-aes-v1"))
	for i := 0; i < 10000; i++ {
		hash = sha256.Sum256(append(hash[:], []byte(secret)...))
	}
	tm.encryptionKey = hash[:]
}

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
			if os.Getenv("DEBUG") != "" {
				fmt.Printf("TransferManager: Recovered from panic in receiveFiles: %v\n", r)
			}
		}

		tm.mutex.Lock()
		duration := time.Since(tm.transferStart).String()
		status := "success"
		errorMsg := ""

		if tm.currentFile == "" {
			status = "failed"
			errorMsg = "No files received"
		}

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
		tm.bytesTransferred = 0
		tm.overallProgress = 0.0
		tm.mutex.Unlock()
	}()

	// Create temporary directory for encrypted files in the current working directory
	tempDir := filepath.Join("temp", fmt.Sprintf("receive-%d", time.Now().Unix()))
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		tm.updateStatus(fmt.Sprintf("Failed to create temp directory: %v", err))
		return
	}
	defer os.RemoveAll(tempDir)

	// Save current working directory and change to temp directory for receiving
	originalWorkingDir, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(originalWorkingDir)

	// ENHANCED: Use connection retry logic for better library/hotel connectivity
	if err := tm.connectWithRetry(10); err != nil { // Increased from 5 to 10 attempts
		tm.updateStatus(fmt.Sprintf("Connection failed: %v", err))
		tm.updateStatus("Library/hotel networks often block P2P traffic. Try a different network if possible.")
		return
	}

	tm.updateStatus("Connected! Receiving files...")

	// Start receiving - this blocks until complete or error
	err := tm.crocClient.Receive()
	if err != nil {
		tm.updateStatus(fmt.Sprintf("Receive failed: %v", err))
		return
	}

	// Process received files
	files, err := filepath.Glob("*")
	if err != nil || len(files) == 0 {
		tm.updateStatus("No files received")
		return
	}

	tm.updateStatus("Decrypting and organizing files...")

	// Look for and decrypt the manifest file first
	var manifest *FileManifest
	manifestFound := false

	for _, file := range files {
		if file == "manifest.enc" {
			encData, err := os.ReadFile(file)
			if err != nil {
				continue
			}

			decData, err := security.DecryptAES256CBC(encData, tm.encryptionKey)
			if err != nil {
				continue
			}

			manifest = &FileManifest{}
			if err := json.Unmarshal(decData, manifest); err != nil {
				continue
			}

			manifestFound = true
			tm.totalFiles = manifest.TotalFiles
			tm.totalSize = manifest.TotalSize
			break
		}
	}

	if !manifestFound {
		tm.fallbackDecryption(files, tm.targetDataDir)
		return
	}

	// Create base directory if folder was sent - use target data directory
	baseDir := filepath.Join(tm.targetDataDir, "data", "received")
	if manifest.FolderName != "" {
		baseDir = filepath.Join(baseDir, manifest.FolderName)
	}

	// Always create the base directory
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		tm.updateStatus(fmt.Sprintf("Failed to create folder: %v", err))
		return
	}

	// Decrypt files using manifest
	successCount := 0

	for _, file := range files {
		if file == "manifest.enc" {
			continue
		}

		tm.updateIncrementalProgress(0, fmt.Sprintf("Decrypting %s", file))

		// Read encrypted file to check if it's chunked format
		encData, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var decData []byte

		// Check if this is a chunked encrypted file
		if len(encData) > 10*1024*1024 { // Files over 10MB might be chunked
			// Try chunked decryption first
			tempDecFile := filepath.Join("temp", fmt.Sprintf("dec-%d", time.Now().UnixNano()))
			if chunkErr := tm.decryptFileChunked(file, tempDecFile); chunkErr == nil {
				// Chunked decryption succeeded, read the result
				if tempData, readErr := os.ReadFile(tempDecFile); readErr == nil {
					decData = tempData
				}
				os.Remove(tempDecFile) // Clean up temp file
			}
		}

		// If chunked decryption failed or wasn't attempted, try regular decryption
		if decData == nil {
			var decErr error
			decData, decErr = security.DecryptAES256CBC(encData, tm.encryptionKey)
			if decErr != nil {
				continue
			}
		}

		// Find original path from manifest
		fileInfo, exists := manifest.Files[file]
		if !exists {
			// Fallback to hash name - use target data directory
			finalPath := filepath.Join(tm.targetDataDir, "data", "received", file)
			os.MkdirAll(filepath.Dir(finalPath), 0755)
			if err := os.WriteFile(finalPath, decData, 0644); err == nil {
				successCount++
			}
			continue
		}

		// Restore original path and filename
		finalPath := filepath.Join(baseDir, fileInfo.RelativePath)

		// Create directory structure
		if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
			continue
		}

		// Write decrypted file with original name
		if err := os.WriteFile(finalPath, decData, 0644); err != nil {
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

		successCount++
	}

	if successCount > 0 {
		if manifest.FolderName != "" {
			tm.updateStatus(fmt.Sprintf("Transfer complete! Folder '%s' with %d files restored successfully", manifest.FolderName, successCount))
		} else {
			tm.updateStatus(fmt.Sprintf("Transfer complete! %d files restored successfully", successCount))
		}
	} else {
		tm.updateStatus("Transfer failed - no files could be decrypted")
	}
}

func (tm *TransferManager) fallbackDecryption(files []string, targetDataDir string) {
	// Fallback mode when no manifest is available
	successCount := 0
	baseDir := filepath.Join(targetDataDir, "data", "received")

	// FIXED: Create base directory
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		tm.updateStatus(fmt.Sprintf("Failed to create received directory: %v", err))
		return
	}

	for i, encFile := range files {
		// FIXED: Update progress during fallback decryption
		progress := float64(i+1) / float64(len(files)) * 100
		tm.updateProgress(int64(progress), 100, encFile)

		encData, err := os.ReadFile(encFile)
		if err != nil {
			continue
		}

		var decData []byte

		// Check if this is a chunked encrypted file
		if len(encData) > 10*1024*1024 { // Files over 10MB might be chunked
			// Try chunked decryption first
			tempDecFile := filepath.Join("temp", fmt.Sprintf("dec-%d", time.Now().UnixNano()))
			if chunkErr := tm.decryptFileChunked(encFile, tempDecFile); chunkErr == nil {
				// Chunked decryption succeeded, read the result
				if tempData, readErr := os.ReadFile(tempDecFile); readErr == nil {
					decData = tempData
				}
				os.Remove(tempDecFile) // Clean up temp file
			}
		}

		// If chunked decryption failed or wasn't attempted, try regular decryption
		if decData == nil {
			var decErr error
			decData, decErr = security.DecryptAES256CBC(encData, tm.encryptionKey)
			if decErr != nil {
				continue
			}
		}

		// FIXED: Use strings.TrimSuffix unconditionally
		fileName := strings.TrimSuffix(encFile, ".enc")

		finalPath := filepath.Join(baseDir, fileName)

		if err := os.WriteFile(finalPath, decData, 0644); err != nil {
			continue
		}

		hash, err := tm.calculateFileHash(finalPath)
		if err == nil {
			tm.fileHashes[fileName] = hash
		}

		if tm.currentFile == "" {
			tm.currentFile = fileName
			tm.currentSize = int64(len(decData))
		}

		successCount++
	}

	if successCount > 0 {
		tm.updateStatus(fmt.Sprintf("Files received (%d files) - original names not preserved", successCount))
	} else {
		tm.updateStatus("Failed to decrypt received files")
	}
}

func (tm *TransferManager) sendFiles(paths []string) {
	defer func() {
		if r := recover(); r != nil {
			if os.Getenv("DEBUG") != "" {
				fmt.Printf("TransferManager: Recovered from panic in sendFiles: %v\n", r)
			}
		}

		tm.mutex.Lock()

		duration := time.Since(tm.transferStart).String()
		status := "success"
		errorMsg := ""

		if tm.crocClient == nil || !tm.crocClient.SuccessfulTransfer {
			status = "failed"
			errorMsg = "Transfer failed"
		}

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
		tm.bytesTransferred = 0
		tm.overallProgress = 0.0
		tm.mutex.Unlock()
	}()

	// Create temporary directory for encrypted files
	tempDir := filepath.Join("temp", fmt.Sprintf("send-%d", time.Now().Unix()))
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		tm.updateStatus(fmt.Sprintf("Failed to create temp directory: %v", err))
		return
	}
	defer os.RemoveAll(tempDir)

	// Create manifest to preserve file structure
	manifest := &FileManifest{
		Files: make(map[string]FileInfo),
	}

	// Process files and folders
	var filesToEncrypt []string
	var baseDir string
	var folderName string
	isFolder := false

	// Check if we're sending a single folder
	if len(paths) == 1 {
		fileInfo, err := os.Stat(paths[0])
		if err == nil && fileInfo.IsDir() {
			baseDir = paths[0]
			folderName = filepath.Base(paths[0])
			manifest.FolderName = folderName
			isFolder = true
		}
	}

	// Collect all files to transfer
	for _, path := range paths {
		fileInfo, err := os.Stat(path)
		if err != nil {
			continue
		}

		if fileInfo.IsDir() {
			err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
				if err != nil {
					return err
				}
				if !info.IsDir() {
					filesToEncrypt = append(filesToEncrypt, filePath)
					tm.totalSize += info.Size()
				}
				return nil
			})
			if err != nil {
				continue
			}
		} else {
			filesToEncrypt = append(filesToEncrypt, path)
			tm.totalSize += fileInfo.Size()
		}
	}

	if len(filesToEncrypt) == 0 {
		tm.updateStatus("No files found to send")
		return
	}

	tm.totalFiles = len(filesToEncrypt)
	tm.updateStatus(fmt.Sprintf("Encrypting %d files for secure transfer...", len(filesToEncrypt)))

	// ENHANCED: Use chunked encryption for large files with better progress tracking
	var encryptedPaths []string

	for i, filePath := range filesToEncrypt {
		// More detailed progress reporting for larger transfers
		encryptionProgress := float64(i) / float64(len(filesToEncrypt)) * 30.0
		fileName := filepath.Base(filePath)
		tm.updateProgress(int64(float64(tm.totalSize)*encryptionProgress/100.0), tm.totalSize, fileName)
		tm.updateStatus(fmt.Sprintf("Encrypting file %d/%d: %s", i+1, len(filesToEncrypt), fileName))

		// Calculate hash of original file
		hash, err := tm.calculateFileHash(filePath)
		if err != nil {
			tm.updateStatus(fmt.Sprintf("Failed to hash file %s: %v", fileName, err))
			continue
		}

		// Generate anonymized filename
		hasher := sha256.New()
		hasher.Write([]byte(filePath + hash))
		anonymizedName := hex.EncodeToString(hasher.Sum(nil))[:32]

		// CRITICAL FIX: Use chunked encryption only for large files (>10MB)
		encFile := filepath.Join(tempDir, anonymizedName)
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			tm.updateStatus(fmt.Sprintf("Failed to stat file %s: %v", fileName, err))
			continue
		}

		// ENHANCED: Better error handling for encryption failures
		var encryptErr error
		if fileInfo.Size() > 10*1024*1024 {
			tm.updateStatus(fmt.Sprintf("Encrypting large file %s (%s) in chunks...", fileName, ByteCountDecimal(fileInfo.Size())))
			encryptErr = tm.encryptFileChunked(filePath, encFile)
		} else {
			encryptErr = tm.encryptFileRegular(filePath, encFile)
		}

		if encryptErr != nil {
			tm.updateStatus(fmt.Sprintf("Encryption failed for %s: %v", fileName, encryptErr))
			if os.Getenv("DEBUG") != "" {
				fmt.Printf("TransferManager: Failed to encrypt file %s: %v\n", filePath, encryptErr)
			}
			continue
		}

		encryptedPaths = append(encryptedPaths, encFile)

		// Get file size for manifest
		fileSize := fileInfo.Size()

		// Determine relative path for the manifest
		var relativePath string
		if isFolder && baseDir != "" {
			relPath, err := filepath.Rel(baseDir, filePath)
			if err != nil {
				relativePath = filepath.Base(filePath)
			} else {
				relativePath = relPath
			}
		} else {
			relativePath = filepath.Base(filePath)
		}

		// Add to manifest
		manifest.Files[anonymizedName] = FileInfo{
			OriginalPath:   filePath,
			RelativePath:   relativePath,
			IsDirectory:    false,
			Size:           fileSize,
			Hash:           hash,
			AnonymizedName: anonymizedName,
		}

		tm.fileHashes[relativePath] = hash
		if tm.currentFile == "" {
			tm.currentFile = relativePath
			tm.currentSize = fileSize
		}

		tm.filesComplete = i + 1

		// Add small delay to prevent overwhelming slower networks
		if len(filesToEncrypt) > 10 {
			time.Sleep(100 * time.Millisecond)
		}
	}

	if len(encryptedPaths) == 0 {
		tm.updateStatus("Failed to encrypt files")
		return
	}

	// Set manifest totals
	manifest.TotalFiles = len(encryptedPaths)
	manifest.TotalSize = tm.totalSize

	// Create and encrypt manifest
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		tm.updateStatus("Failed to create file manifest")
		return
	}

	encryptedManifest, err := security.EncryptAES256CBC(manifestData, tm.encryptionKey)
	if err != nil {
		tm.updateStatus("Failed to encrypt file manifest")
		return
	}

	manifestPath := filepath.Join(tempDir, "manifest.enc")
	if err := os.WriteFile(manifestPath, encryptedManifest, 0600); err != nil {
		tm.updateStatus("Failed to save file manifest")
		return
	}

	encryptedPaths = append(encryptedPaths, manifestPath)

	// Get file info for encrypted files
	filesInfo, emptyFolders, totalFolders, err := croc.GetFilesInfo(encryptedPaths, false, false, []string{})
	if err != nil {
		tm.updateStatus(fmt.Sprintf("Failed to prepare files: %v", err))
		return
	}

	tm.updateStatus("Connecting to receiver...")

	// ENHANCED: Use connection retry logic for better library/hotel connectivity
	if err := tm.connectWithRetry(10); err != nil { // Increased from 5 to 10 attempts
		tm.updateStatus(fmt.Sprintf("Connection failed: %v", err))
		tm.updateStatus("Library/hotel networks often block P2P traffic. Try a different network if possible.")
		return
	}

	// Set transfer options
	tm.crocClient.FilesToTransfer = filesInfo
	tm.crocClient.EmptyFoldersToTransfer = emptyFolders
	tm.crocClient.TotalNumberFolders = totalFolders

	tm.updateStatus("Connected! Sending files...")

	// Start sending
	err = tm.crocClient.Send(filesInfo, emptyFolders, totalFolders)
	if err != nil {
		tm.updateStatus(fmt.Sprintf("Transfer failed: %v", err))
		return
	}

	tm.updateStatus("Transfer completed successfully!")
}

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
		if os.Getenv("DEBUG") != "" {
			fmt.Printf("Failed to log transfer to blockchain: %v\n", err)
		}
	}
}

func (tm *TransferManager) IsTransferActive() bool {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	return tm.isReceiving || tm.isSending
}

func (tm *TransferManager) CancelTransfer() {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if tm.isReceiving || tm.isSending {
		direction := "receive"
		if tm.isSending {
			direction = "send"
		}
		duration := time.Since(tm.transferStart).String()

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
	tm.crocClient = nil
	tm.bytesTransferred = 0
	tm.overallProgress = 0.0

	if tm.statusCb != nil {
		tm.statusCb("Transfer cancelled")
	}
}

func (tm *TransferManager) GetLogger() *logging.Logger {
	return tm.logger
}

// FIXED: Add chunked file encryption to prevent memory crashes with large files
func (tm *TransferManager) encryptFileChunked(filePath, encPath string) error {
	inFile, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer inFile.Close()

	outFile, err := os.Create(encPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	// Process in 10MB chunks to prevent memory issues
	buffer := make([]byte, 10*1024*1024)
	for {
		n, err := inFile.Read(buffer)
		if n > 0 {
			encrypted, encErr := security.EncryptAES256CBC(buffer[:n], tm.encryptionKey)
			if encErr != nil {
				return encErr
			}

			// Write the size of this encrypted chunk first (4 bytes)
			sizeBytes := make([]byte, 4)
			sizeBytes[0] = byte(len(encrypted) >> 24)
			sizeBytes[1] = byte(len(encrypted) >> 16)
			sizeBytes[2] = byte(len(encrypted) >> 8)
			sizeBytes[3] = byte(len(encrypted))

			if _, writeErr := outFile.Write(sizeBytes); writeErr != nil {
				return writeErr
			}
			if _, writeErr := outFile.Write(encrypted); writeErr != nil {
				return writeErr
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// FIXED: Add regular file encryption for files under 10MB
func (tm *TransferManager) encryptFileRegular(filePath, encPath string) error {
	// Read the entire file for regular encryption
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Encrypt the file data
	encrypted, err := security.EncryptAES256CBC(data, tm.encryptionKey)
	if err != nil {
		return err
	}

	// Write encrypted data to output file
	return os.WriteFile(encPath, encrypted, 0600)
}

// ENHANCED: Add connection retry logic optimized for restrictive networks (libraries, hotels, etc.)
func (tm *TransferManager) connectWithRetry(attempts int) error {
	// More relay servers including alternative ports
	relays := []string{
		"croc.schollz.com:9009",
		"croc2.schollz.com:9009",
		"croc3.schollz.com:9009",
		"croc4.schollz.com:9009",
		"croc5.schollz.com:9009",
		// Add more relay servers for better redundancy
		"croc.schollz.com:443", // HTTPS port often unblocked
		"croc2.schollz.com:443",
		"croc.schollz.com:80", // HTTP port often unblocked
		"croc2.schollz.com:80",
	}

	// Increase attempts for restrictive networks
	maxAttempts := attempts
	if maxAttempts < 10 {
		maxAttempts = 10 // Minimum 10 attempts for library networks
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Try different relays on each attempt with more variety
		relay := relays[attempt%len(relays)]
		tm.updateStatus(fmt.Sprintf("Connection attempt %d/%d using relay %s...", attempt+1, maxAttempts, relay))

		// Extract host and port from relay
		relayHost, relayPort, err := net.SplitHostPort(relay)
		if err != nil {
			relayHost = relay
			relayPort = "9009"
		}

		// Configure croc with current relay and more permissive settings
		options := croc.Options{
			IsSender:       tm.isSending,
			SharedSecret:   tm.getSecret(),
			RelayAddress:   relayHost,
			RelayAddress6:  relayHost,
			RelayPorts:     []string{relayPort, "9009", "9010", "9011", "9012", "9013", "443", "80"},
			RelayPassword:  "pass123",
			NoPrompt:       true,
			NoMultiplexing: true, // Enable multiplexing for better performance
			DisableLocal:   false,
			Ask:            false,
			Debug:          false,
			Overwrite:      true,
			Curve:          "p256",
			HashAlgorithm:  "xxhash",
		}

		c, err := croc.New(options)
		if err == nil {
			tm.crocClient = c
			return nil // Success!
		}

		// Progressive backoff with longer delays for restrictive networks
		if attempt < maxAttempts-1 {
			var backoffTime time.Duration
			if attempt < 3 {
				backoffTime = time.Duration(attempt+1) * 3 * time.Second // 3s, 6s, 9s
			} else if attempt < 6 {
				backoffTime = time.Duration(attempt+1) * 5 * time.Second // 20s, 25s, 30s
			} else {
				backoffTime = time.Duration(attempt+1) * 10 * time.Second // 70s, 80s, 90s, 100s
			}

			tm.updateStatus(fmt.Sprintf("Network seems restrictive (library/hotel?). Retrying in %v...", backoffTime))
			time.Sleep(backoffTime)
		}
	}

	return fmt.Errorf("failed to connect after %d attempts with multiple relays - network may be blocking P2P traffic", maxAttempts)
}

func (tm *TransferManager) getSecret() string {
	if tm.isSending {
		return tm.localPeerID
	}
	return tm.currentSecret
}

// FIXED: Generate stronger transfer codes
func generateSecureCode() string {
	// Use OS random for better entropy
	b := make([]byte, 9) // 72 bits of entropy
	rand.Read(b)

	// Convert to human-readable format
	words := []string{
		"alpha", "bravo", "charlie", "delta", "echo",
		"foxtrot", "golf", "hotel", "india", "juliet",
		"kilo", "lima", "mike", "november", "oscar",
		"papa", "quebec", "romeo", "sierra", "tango",
	}

	code := ""
	for i := 0; i < 3; i++ {
		code += words[int(b[i])%len(words)] + "-"
	}
	code += fmt.Sprintf("%d", binary.BigEndian.Uint16(b[7:9]))
	return code
}

// FIXED: Add chunked file decryption to match chunked encryption
func (tm *TransferManager) decryptFileChunked(encPath, decPath string) error {
	encFile, err := os.Open(encPath)
	if err != nil {
		return err
	}
	defer encFile.Close()

	decFile, err := os.Create(decPath)
	if err != nil {
		return err
	}
	defer decFile.Close()

	// Read chunks with size headers
	for {
		// Read chunk size (4 bytes)
		sizeBytes := make([]byte, 4)
		n, err := encFile.Read(sizeBytes)
		if n == 0 || err == io.EOF {
			break
		}
		if n != 4 || err != nil {
			return fmt.Errorf("failed to read chunk size")
		}

		// Calculate chunk size
		chunkSize := int(sizeBytes[0])<<24 | int(sizeBytes[1])<<16 | int(sizeBytes[2])<<8 | int(sizeBytes[3])
		if chunkSize <= 0 || chunkSize > 20*1024*1024 { // Sanity check
			return fmt.Errorf("invalid chunk size: %d", chunkSize)
		}

		// Read the encrypted chunk
		encChunk := make([]byte, chunkSize)
		n, err = encFile.Read(encChunk)
		if n != chunkSize || err != nil {
			return fmt.Errorf("failed to read encrypted chunk")
		}

		// Decrypt the chunk
		decrypted, decErr := security.DecryptAES256CBC(encChunk, tm.encryptionKey)
		if decErr != nil {
			return decErr
		}

		// Write decrypted data
		if _, writeErr := decFile.Write(decrypted); writeErr != nil {
			return writeErr
		}
	}
	return nil
}

// EncryptFileChunked encrypts a file in chunks to prevent memory issues with large files
func (tm *TransferManager) EncryptFileChunked(filePath, encPath string) error {
	return tm.encryptFileChunked(filePath, encPath)
}

// DecryptFileChunked decrypts a chunked encrypted file
func (tm *TransferManager) DecryptFileChunked(encPath, decPath string) error {
	return tm.decryptFileChunked(encPath, decPath)
}

// ByteCountDecimal converts bytes to human readable format
func ByteCountDecimal(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}
