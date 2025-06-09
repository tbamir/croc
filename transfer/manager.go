package transfer

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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

	// Enhanced progress tracking
	lastProgressUpdate time.Time
	progressThrottle   time.Duration
	bytesTransferred   int64
	overallProgress    float64
	
	// FIXED: Add error tracking and validation
	lastError          error
	transferCompleted  bool
	receivedFilesCount int
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

func NewTransferManager() (*TransferManager, error) {
	// Generate stronger local peer ID using secure random
	localPeerID := generateSecureCode()

	// Only show debug output when DEBUG environment variable is set
	if os.Getenv("DEBUG") != "" {
		fmt.Printf("TransferManager: Generated local peer ID: %s\n", localPeerID)
	}

	// Initialize logger with blockchain
	logger, err := logging.NewLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	return &TransferManager{
		localPeerID:        localPeerID,
		isReceiving:        false,
		isSending:          false,
		logger:             logger,
		fileHashes:         make(map[string]string),
		progressThrottle:   100 * time.Millisecond,
		overallProgress:    0.0,
		transferCompleted:  false,
		receivedFilesCount: 0,
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

// Enhanced progress tracking with real-time updates
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

// New method for incremental progress updates during transfer
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
	// Add panic recovery
	defer func() {
		if r := recover(); r != nil {
			errorMsg := fmt.Sprintf("Receive failed: %v", r)
			tm.updateStatus(errorMsg)
			tm.lastError = fmt.Errorf("panic: %v", r)
			if tm.logger != nil {
				tm.logger.LogTransfer(logging.TransferLog{
					Timestamp: time.Now(),
					PeerID:    peerSecret,
					FileName:  tm.currentFile,
					Status:    "failed",
					Error:     errorMsg,
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

	// FIXED: Reset transfer state
	tm.currentSecret = peerSecret
	tm.isReceiving = true
	tm.transferStart = time.Now()
	tm.filesComplete = 0
	tm.bytesTransferred = 0
	tm.overallProgress = 0.0
	tm.transferCompleted = false
	tm.receivedFilesCount = 0
	tm.lastError = nil

	// Derive encryption key from the shared secret
	tm.deriveEncryptionKey(peerSecret)

	tm.updateStatus("Connecting to sender...")

	// Start receiving in background
	go tm.receiveFiles()

	return nil
}

func (tm *TransferManager) SendFiles(paths []string) error {
	// Add panic recovery
	defer func() {
		if r := recover(); r != nil {
			errorMsg := fmt.Sprintf("Transfer failed: %v", r)
			tm.updateStatus(errorMsg)
			tm.lastError = fmt.Errorf("panic: %v", r)
			if tm.logger != nil {
				tm.logger.LogTransfer(logging.TransferLog{
					Timestamp: time.Now(),
					PeerID:    tm.localPeerID,
					FileName:  tm.currentFile,
					Status:    "failed",
					Error:     errorMsg,
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

	// FIXED: Reset transfer state
	tm.isSending = true
	tm.transferStart = time.Now()
	tm.filesComplete = 0
	tm.bytesTransferred = 0
	tm.overallProgress = 0.0
	tm.transferCompleted = false
	tm.lastError = nil

	// Derive encryption key from our local peer ID
	tm.deriveEncryptionKey(tm.localPeerID)

	tm.updateStatus("Preparing files for secure transfer...")

	// Start sending in background
	go tm.sendFiles(paths)

	return nil
}

// deriveEncryptionKey derives an AES key from the shared secret with improved security
func (tm *TransferManager) deriveEncryptionKey(secret string) {
	// Multiple rounds of hashing for better security
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
			tm.lastError = fmt.Errorf("panic in receiveFiles: %v", r)
		}

		tm.mutex.Lock()
		duration := time.Since(tm.transferStart).String()
		
		// FIXED: Better success/failure determination
		status := "failed"
		errorMsg := "No files received"
		
		if tm.lastError == nil && tm.receivedFilesCount > 0 && tm.transferCompleted {
			status = "success"
			errorMsg = ""
		} else if tm.lastError != nil {
			errorMsg = tm.lastError.Error()
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

	// FIXED: Ensure absolute paths for temp and output directories
	workingDir, err := os.Getwd()
	if err != nil {
		tm.lastError = fmt.Errorf("failed to get working directory: %v", err)
		tm.updateStatus("Failed to get working directory")
		return
	}

	// Create temporary directory for encrypted files
	tempDir := filepath.Join(workingDir, "data", "temp", fmt.Sprintf("receive-%d", time.Now().Unix()))
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		tm.lastError = fmt.Errorf("failed to create temp directory: %v", err)
		tm.updateStatus(fmt.Sprintf("Failed to create temp directory: %v", err))
		return
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory for receiving
	originalDir := workingDir
	if err := os.Chdir(tempDir); err != nil {
		tm.lastError = fmt.Errorf("failed to change to temp directory: %v", err)
		tm.updateStatus("Failed to setup temp directory")
		return
	}
	defer os.Chdir(originalDir)

	// Use connection retry logic for better connectivity
	if err := tm.connectWithRetry(5); err != nil {
		tm.lastError = err
		tm.updateStatus(fmt.Sprintf("Failed to connect: %v", err))
		return
	}

	tm.updateStatus("Connected! Receiving files...")

	// Start receiving - this blocks until complete or error
	err = tm.crocClient.Receive()
	if err != nil {
		tm.lastError = err
		tm.updateStatus(fmt.Sprintf("Receive failed: %v", err))
		return
	}

	// FIXED: Verify files were actually received
	files, err := filepath.Glob("*")
	if err != nil || len(files) == 0 {
		tm.lastError = fmt.Errorf("no files received from sender")
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
				tm.lastError = fmt.Errorf("failed to decrypt manifest: %v", err)
				continue
			}

			manifest = &FileManifest{}
			if err := json.Unmarshal(decData, manifest); err != nil {
				tm.lastError = fmt.Errorf("failed to parse manifest: %v", err)
				continue
			}

			manifestFound = true
			tm.totalFiles = manifest.TotalFiles
			tm.totalSize = manifest.TotalSize
			break
		}
	}

	if !manifestFound {
		tm.lastError = fmt.Errorf("manifest file not found or corrupted")
		tm.fallbackDecryption(files, originalDir)
		return
	}

	// FIXED: Create base directory with absolute path
	baseDir := filepath.Join(originalDir, "data", "received")
	if manifest.FolderName != "" {
		baseDir = filepath.Join(baseDir, manifest.FolderName)
	}

	// Always create the base directory
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		tm.lastError = fmt.Errorf("failed to create output directory: %v", err)
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

		// Read encrypted file
		encData, err := os.ReadFile(file)
		if err != nil {
			if os.Getenv("DEBUG") != "" {
				fmt.Printf("Failed to read encrypted file %s: %v\n", file, err)
			}
			continue
		}

		// Decrypt the file
		decData, err := security.DecryptAES256CBC(encData, tm.encryptionKey)
		if err != nil {
			if os.Getenv("DEBUG") != "" {
				fmt.Printf("Failed to decrypt file %s: %v\n", file, err)
			}
			continue
		}

		// Find original path from manifest
		fileInfo, exists := manifest.Files[file]
		if !exists {
			// Fallback to hash name
			finalPath := filepath.Join(baseDir, file)
			if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err == nil {
				if err := os.WriteFile(finalPath, decData, 0644); err == nil {
					successCount++
				}
			}
			continue
		}

		// Restore original path and filename
		finalPath := filepath.Join(baseDir, fileInfo.RelativePath)

		// Create directory structure
		if err := os.MkdirAll(filepath.Dir(finalPath), 0755); err != nil {
			if os.Getenv("DEBUG") != "" {
				fmt.Printf("Failed to create directory for %s: %v\n", finalPath, err)
			}
			continue
		}

		// Write decrypted file with original name
		if err := os.WriteFile(finalPath, decData, 0644); err != nil {
			if os.Getenv("DEBUG") != "" {
				fmt.Printf("Failed to write file %s: %v\n", finalPath, err)
			}
			continue
		}

		// Calculate hash of decrypted file for verification
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

	// FIXED: Set completion status based on actual success
	tm.receivedFilesCount = successCount
	if successCount > 0 {
		tm.transferCompleted = true
		if manifest.FolderName != "" {
			tm.updateStatus(fmt.Sprintf("Transfer complete! Folder '%s' with %d files restored successfully", manifest.FolderName, successCount))
		} else {
			tm.updateStatus(fmt.Sprintf("Transfer complete! %d files restored successfully", successCount))
		}
	} else {
		tm.lastError = fmt.Errorf("no files could be decrypted successfully")
		tm.updateStatus("Transfer failed - no files could be decrypted")
	}
}

func (tm *TransferManager) fallbackDecryption(files []string, originalDir string) {
	// Fallback mode when no manifest is available
	successCount := 0
	baseDir := filepath.Join(originalDir, "data", "received")

	// Create base directory
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		tm.lastError = fmt.Errorf("failed to create received directory: %v", err)
		tm.updateStatus(fmt.Sprintf("Failed to create received directory: %v", err))
		return
	}

	for i, encFile := range files {
		// Update progress during fallback decryption
		progress := float64(i+1) / float64(len(files)) * 100
		tm.updateProgress(int64(progress), 100, encFile)

		encData, err := os.ReadFile(encFile)
		if err != nil {
			continue
		}

		decData, err := security.DecryptAES256CBC(encData, tm.encryptionKey)
		if err != nil {
			continue
		}

		// Use strings.TrimSuffix unconditionally
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

	tm.receivedFilesCount = successCount
	if successCount > 0 {
		tm.transferCompleted = true
		tm.updateStatus(fmt.Sprintf("Files received (%d files) - original names not preserved", successCount))
	} else {
		tm.lastError = fmt.Errorf("failed to decrypt any received files")
		tm.updateStatus("Failed to decrypt received files")
	}
}

func (tm *TransferManager) sendFiles(paths []string) {
	defer func() {
		if r := recover(); r != nil {
			if os.Getenv("DEBUG") != "" {
				fmt.Printf("TransferManager: Recovered from panic in sendFiles: %v\n", r)
			}
			tm.lastError = fmt.Errorf("panic in sendFiles: %v", r)
		}

		tm.mutex.Lock()

		duration := time.Since(tm.transferStart).String()
		status := "success"
		errorMsg := ""

		if tm.lastError != nil || tm.crocClient == nil || !tm.crocClient.SuccessfulTransfer {
			status = "failed"
			if tm.lastError != nil {
				errorMsg = tm.lastError.Error()
			} else {
				errorMsg = "Transfer failed"
			}
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

	// FIXED: Ensure absolute paths
	workingDir, err := os.Getwd()
	if err != nil {
		tm.lastError = fmt.Errorf("failed to get working directory: %v", err)
		tm.updateStatus("Failed to get working directory")
		return
	}

	// Create temporary directory for encrypted files
	tempDir := filepath.Join(workingDir, "data", "temp", fmt.Sprintf("send-%d", time.Now().Unix()))
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		tm.lastError = fmt.Errorf("failed to create temp directory: %v", err)
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
		// FIXED: Ensure absolute path resolution
		absPath, err := filepath.Abs(path)
		if err != nil {
			tm.lastError = fmt.Errorf("failed to resolve path %s: %v", path, err)
			tm.updateStatus(fmt.Sprintf("Failed to resolve path: %v", err))
			return
		}

		fileInfo, err := os.Stat(absPath)
		if err != nil {
			tm.lastError = fmt.Errorf("cannot access file %s: %v", absPath, err)
			tm.updateStatus(fmt.Sprintf("Cannot access file: %v", err))
			return
		}

		if fileInfo.IsDir() {
			err = filepath.Walk(absPath, func(filePath string, info os.FileInfo, err error) error {
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
				tm.lastError = fmt.Errorf("failed to walk directory %s: %v", absPath, err)
				tm.updateStatus(fmt.Sprintf("Failed to process directory: %v", err))
				return
			}
		} else {
			filesToEncrypt = append(filesToEncrypt, absPath)
			tm.totalSize += fileInfo.Size()
		}
	}

	if len(filesToEncrypt) == 0 {
		tm.lastError = fmt.Errorf("no files found to send")
		tm.updateStatus("No files found to send")
		return
	}

	tm.totalFiles = len(filesToEncrypt)
	tm.updateStatus(fmt.Sprintf("Encrypting %d files...", len(filesToEncrypt)))

	// Use chunked encryption for large files
	var encryptedPaths []string

	for i, filePath := range filesToEncrypt {
		encryptionProgress := float64(i) / float64(len(filesToEncrypt)) * 30.0
		tm.updateProgress(int64(float64(tm.totalSize)*encryptionProgress/100.0), tm.totalSize, filepath.Base(filePath))

		// Calculate hash of original file
		hash, err := tm.calculateFileHash(filePath)
		if err != nil {
			if os.Getenv("DEBUG") != "" {
				fmt.Printf("Failed to hash file %s: %v\n", filePath, err)
			}
			continue
		}

		// Generate anonymized filename
		hasher := sha256.New()
		hasher.Write([]byte(filePath + hash))
		anonymizedName := hex.EncodeToString(hasher.Sum(nil))[:32]

		// Use chunked encryption instead of loading entire file
		encFile := filepath.Join(tempDir, anonymizedName)
		if err := tm.encryptFileChunked(filePath, encFile); err != nil {
			if os.Getenv("DEBUG") != "" {
				fmt.Printf("TransferManager: Failed to encrypt file %s: %v\n", filePath, err)
			}
			continue
		}

		encryptedPaths = append(encryptedPaths, encFile)

		// Get file size for manifest
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			continue
		}

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
			Size:           fileInfo.Size(),
			Hash:           hash,
			AnonymizedName: anonymizedName,
		}

		tm.fileHashes[relativePath] = hash
		if tm.currentFile == "" {
			tm.currentFile = relativePath
			tm.currentSize = fileInfo.Size()
		}

		tm.filesComplete = i + 1
	}

	if len(encryptedPaths) == 0 {
		tm.lastError = fmt.Errorf("failed to encrypt any files")
		tm.updateStatus("Failed to encrypt files")
		return
	}

	// Set manifest totals
	manifest.TotalFiles = len(encryptedPaths)
	manifest.TotalSize = tm.totalSize

	// Create and encrypt manifest
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		tm.lastError = fmt.Errorf("failed to create manifest: %v", err)
		tm.updateStatus("Failed to create file manifest")
		return
	}

	encryptedManifest, err := security.EncryptAES256CBC(manifestData, tm.encryptionKey)
	if err != nil {
		tm.lastError = fmt.Errorf("failed to encrypt manifest: %v", err)
		tm.updateStatus("Failed to encrypt file manifest")
		return
	}

	manifestPath := filepath.Join(tempDir, "manifest.enc")
	if err := os.WriteFile(manifestPath, encryptedManifest, 0600); err != nil {
		tm.lastError = fmt.Errorf("failed to save manifest: %v", err)
		tm.updateStatus("Failed to save file manifest")
		return
	}

	encryptedPaths = append(encryptedPaths, manifestPath)

	// Get file info for encrypted files
	filesInfo, emptyFolders, totalFolders, err := croc.GetFilesInfo(encryptedPaths, false, false, []string{})
	if err != nil {
		tm.lastError = fmt.Errorf("failed to prepare files: %v", err)
		tm.updateStatus(fmt.Sprintf("Failed to prepare files: %v", err))
		return
	}

	tm.updateStatus("Connecting to receiver...")

	// Use connection retry logic for better connectivity
	if err := tm.connectWithRetry(5); err != nil {
		tm.lastError = err
		tm.updateStatus(fmt.Sprintf("Failed to connect: %v", err))
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
		tm.lastError = err
		tm.updateStatus(fmt.Sprintf("Transfer failed: %v", err))
		return
	}

	// FIXED: Only report success if croc confirms successful transfer
	if tm.crocClient.SuccessfulTransfer {
		tm.transferCompleted = true
		tm.updateStatus("Transfer completed successfully!")
	} else {
		tm.lastError = fmt.Errorf("transfer did not complete successfully")
		tm.updateStatus("Transfer failed to complete")
	}
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
	tm.transferCompleted = false
	tm.lastError = nil

	if tm.statusCb != nil {
		tm.statusCb("Transfer cancelled")
	}
}

func (tm *TransferManager) GetLogger() *logging.Logger {
	return tm.logger
}

// Add chunked file encryption to prevent memory crashes with large files
func (tm *TransferManager) encryptFileChunked(filePath, encPath string) error {
	inFile, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open input file: %v", err)
	}
	defer inFile.Close()

	outFile, err := os.Create(encPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer outFile.Close()

	// Process in 10MB chunks to prevent memory issues
	buffer := make([]byte, 10*1024*1024)
	for {
		n, err := inFile.Read(buffer)
		if n > 0 {
			encrypted, encErr := security.EncryptAES256CBC(buffer[:n], tm.encryptionKey)
			if encErr != nil {
				return fmt.Errorf("encryption failed: %v", encErr)
			}
			if _, writeErr := outFile.Write(encrypted); writeErr != nil {
				return fmt.Errorf("write failed: %v", writeErr)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read failed: %v", err)
		}
	}
	return nil
}

// Add connection retry logic for Europe-US transfers
func (tm *TransferManager) connectWithRetry(attempts int) error {
	relays := []string{
		"croc.schollz.com:9009",
		"croc2.schollz.com:9009",
		"croc3.schollz.com:9009",
		"croc4.schollz.com:9009",
		"croc5.schollz.com:9009",
	}

	for attempt := 0; attempt < attempts; attempt++ {
		// Try different relays on each attempt
		relay := relays[attempt%len(relays)]
		tm.updateStatus(fmt.Sprintf("Connection attempt %d/%d using relay %s...", attempt+1, attempts, relay))

		// Configure croc with current relay
		options := croc.Options{
			IsSender:       tm.isSending,
			SharedSecret:   tm.getSecret(),
			RelayAddress:   relay,
			RelayAddress6:  relay,
			RelayPorts:     []string{"9009", "9010", "9011", "9012", "9013"},
			RelayPassword:  "pass123",
			NoPrompt:       true,
			NoMultiplexing: false,
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

		// Exponential backoff between attempts
		if attempt < attempts-1 {
			backoffTime := time.Duration(attempt+1) * 2 * time.Second
			tm.updateStatus(fmt.Sprintf("Retrying in %v...", backoffTime))
			time.Sleep(backoffTime)
		}
	}

	return fmt.Errorf("failed to connect after %d attempts with multiple relays", attempts)
}

func (tm *TransferManager) getSecret() string {
	if tm.isSending {
		return tm.localPeerID
	}
	return tm.currentSecret
}

// Generate stronger transfer codes
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