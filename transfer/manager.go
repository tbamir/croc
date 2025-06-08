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
	totalSize     int64
	totalFiles    int
	filesComplete int
	encryptionKey []byte
	fileHashes    map[string]string
	
	// Progress tracking
	lastProgressUpdate time.Time
	progressThrottle   time.Duration
}

type TransferProgress struct {
	CurrentFile      string
	FilesRemaining   int
	PercentComplete  float64
	BytesTransferred int64
	TotalBytes       int64
}

type FileManifest struct {
	Files        map[string]FileInfo `json:"files"`
	FolderName   string              `json:"folder_name,omitempty"`
	TotalFiles   int                 `json:"total_files"`
	TotalSize    int64               `json:"total_size"`
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
	localPeerID := utils.GetRandomName()
	
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

		tm.progressCb(TransferProgress{
			CurrentFile:      currentFile,
			FilesRemaining:   filesRemaining,
			PercentComplete:  percentComplete,
			BytesTransferred: current,
			TotalBytes:       total,
		})
	}
}

func (tm *TransferManager) StartReceive(peerSecret string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if tm.isReceiving || tm.isSending {
		return fmt.Errorf("transfer already in progress")
	}

	tm.currentSecret = peerSecret
	tm.isReceiving = true
	tm.transferStart = time.Now()
	tm.filesComplete = 0

	// Derive encryption key from the shared secret
	tm.deriveEncryptionKey(peerSecret)

	tm.updateStatus("Connecting to sender...")

	// Start receiving in background
	go tm.receiveFiles()

	return nil
}

func (tm *TransferManager) SendFiles(paths []string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if tm.isReceiving || tm.isSending {
		return fmt.Errorf("transfer already in progress")
	}

	tm.isSending = true
	tm.transferStart = time.Now()
	tm.filesComplete = 0

	// Derive encryption key from our local peer ID
	tm.deriveEncryptionKey(tm.localPeerID)

	tm.updateStatus("Preparing files for secure transfer...")

	// Start sending in background
	go tm.sendFiles(paths)

	return nil
}

func (tm *TransferManager) deriveEncryptionKey(secret string) {
	hash := sha256.Sum256([]byte(secret + "-trustdrop-aes"))
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
			fmt.Printf("TransferManager: Recovered from panic in receiveFiles: %v\n", r)
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
		tm.mutex.Unlock()
	}()

	// Create temporary directory for encrypted files
	tempDir := filepath.Join("data", "temp", fmt.Sprintf("receive-%d", time.Now().Unix()))
	if err := os.MkdirAll(tempDir, 0700); err != nil {
		tm.updateStatus(fmt.Sprintf("Failed to create temp directory: %v", err))
		return
	}
	defer os.RemoveAll(tempDir)

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
		Debug:          false,
		Overwrite:      true,
		Curve:          "p256",
		HashAlgorithm:  "xxhash",
	}

	// Initialize croc
	c, err := croc.New(options)
	if err != nil {
		tm.updateStatus(fmt.Sprintf("Failed to initialize: %v", err))
		return
	}

	tm.crocClient = c

	tm.updateStatus("Connected! Receiving files...")

	// Start receiving - this blocks until complete or error
	err = c.Receive()
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
		tm.fallbackDecryption(files, originalDir)
		return
	}

	// Create base directory if folder was sent
	baseDir := filepath.Join(originalDir, "data", "received")
	if manifest.FolderName != "" {
		baseDir = filepath.Join(baseDir, manifest.FolderName)
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			tm.updateStatus(fmt.Sprintf("Failed to create folder: %v", err))
			return
		}
	}

	// Decrypt files using manifest
	successCount := 0
	var totalDecrypted int64

	for _, file := range files {
		if file == "manifest.enc" {
			continue
		}

		// Read encrypted file
		encData, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		// Decrypt the file
		decData, err := security.DecryptAES256CBC(encData, tm.encryptionKey)
		if err != nil {
			continue
		}

		// Find original path from manifest
		fileInfo, exists := manifest.Files[file]
		if !exists {
			continue
		}

		// Construct final path
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

		// Update progress
		totalDecrypted += int64(len(decData))
		tm.updateProgress(totalDecrypted, tm.totalSize, fileInfo.RelativePath)

		// Update transfer info
		if tm.currentFile == "" {
			tm.currentFile = fileInfo.RelativePath
			tm.currentSize = int64(len(decData))
		}

		successCount++
		tm.filesComplete = successCount
	}

	if successCount > 0 {
		tm.updateStatus(fmt.Sprintf("Files received and decrypted successfully! (%d files)", successCount))
	} else {
		tm.updateStatus("Failed to decrypt received files")
	}
}

func (tm *TransferManager) fallbackDecryption(files []string, originalDir string) {
	// Fallback mode when no manifest is available
	successCount := 0
	baseDir := filepath.Join(originalDir, "data", "received")

	for _, encFile := range files {
		encData, err := os.ReadFile(encFile)
		if err != nil {
			continue
		}

		decData, err := security.DecryptAES256CBC(encData, tm.encryptionKey)
		if err != nil {
			continue
		}

		finalPath := filepath.Join(baseDir, encFile)
		os.MkdirAll(filepath.Dir(finalPath), 0755)

		if err := os.WriteFile(finalPath, decData, 0644); err != nil {
			continue
		}

		hash, err := tm.calculateFileHash(finalPath)
		if err == nil {
			tm.fileHashes[encFile] = hash
		}

		if tm.currentFile == "" {
			tm.currentFile = encFile
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
			fmt.Printf("TransferManager: Recovered from panic in sendFiles: %v\n", r)
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
		tm.mutex.Unlock()
	}()

	// Create temporary directory for encrypted files
	tempDir := filepath.Join("data", "temp", fmt.Sprintf("send-%d", time.Now().Unix()))
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

	// Check if we're sending a single folder
	if len(paths) == 1 {
		fileInfo, err := os.Stat(paths[0])
		if err == nil && fileInfo.IsDir() {
			baseDir = paths[0]
			folderName = filepath.Base(paths[0])
			manifest.FolderName = folderName
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
				}
				return nil
			})
			if err != nil {
				continue
			}
		} else {
			filesToEncrypt = append(filesToEncrypt, path)
		}
	}

	if len(filesToEncrypt) == 0 {
		tm.updateStatus("No files found to send")
		return
	}

	tm.totalFiles = len(filesToEncrypt)
	tm.updateStatus(fmt.Sprintf("Encrypting %d files...", len(filesToEncrypt)))

	// Calculate total size and encrypt files
	var encryptedPaths []string
	var totalEncrypted int64

	for i, filePath := range filesToEncrypt {
		// Calculate hash of original file
		hash, err := tm.calculateFileHash(filePath)
		if err != nil {
			continue
		}

		// Read original file
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		tm.totalSize += int64(len(data))

		// Update progress
		tm.updateProgress(totalEncrypted, tm.totalSize, filepath.Base(filePath))

		// Encrypt the file
		encData, err := security.EncryptAES256CBC(data, tm.encryptionKey)
		if err != nil {
			continue
		}

		// Generate anonymized filename
		hasher := sha256.New()
		hasher.Write([]byte(filePath + hash))
		anonymizedName := hex.EncodeToString(hasher.Sum(nil))[:32]

		// Create encrypted file
		encFile := filepath.Join(tempDir, anonymizedName)
		if err := os.WriteFile(encFile, encData, 0600); err != nil {
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

		// Store file info
		tm.fileHashes[relativePath] = hash
		if tm.currentFile == "" {
			tm.currentFile = relativePath
			tm.currentSize = int64(len(data))
		}

		totalEncrypted += int64(len(data))
		tm.filesComplete = i + 1
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

	// Add manifest to files to send
	encryptedPaths = append(encryptedPaths, manifestPath)

	// Get file info for encrypted files
	filesInfo, emptyFolders, totalFolders, err := croc.GetFilesInfo(encryptedPaths, false, false, []string{})
	if err != nil {
		tm.updateStatus(fmt.Sprintf("Failed to prepare files: %v", err))
		return
	}

	tm.updateStatus("Waiting for receiver to connect...")

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
		Debug:          false,
		Overwrite:      true,
		Curve:          "p256",
		HashAlgorithm:  "xxhash",
	}

	// Initialize croc
	c, err := croc.New(options)
	if err != nil {
		tm.updateStatus(fmt.Sprintf("Failed to initialize transfer: %v", err))
		return
	}

	tm.crocClient = c

	// Set transfer options
	c.FilesToTransfer = filesInfo
	c.EmptyFoldersToTransfer = emptyFolders
	c.TotalNumberFolders = totalFolders

	tm.updateStatus("Connected! Sending files...")

	// Start sending - this blocks until complete or error
	err = c.Send(filesInfo, emptyFolders, totalFolders)
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
		fmt.Printf("Failed to log transfer to blockchain: %v\n", err)
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

	if tm.statusCb != nil {
		tm.statusCb("Transfer cancelled")
	}
}

func (tm *TransferManager) GetLogger() *logging.Logger {
	return tm.logger
}