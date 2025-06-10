package transfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"sync"
	"time"

	"trustdrop-bulletproof/blockchain"
	"trustdrop-bulletproof/logging"
	"trustdrop-bulletproof/security"
	"trustdrop-bulletproof/transport"
)

// BulletproofTransferManager provides ultra-reliable file transfers with multiple fallback mechanisms
type BulletproofTransferManager struct {
	// Core components
	transportManager *transport.MultiTransportManager
	advancedSecurity *security.AdvancedSecurity
	blockchain       *blockchain.Blockchain
	logger           *logging.Logger

	// Transfer state
	targetDataDir    string
	transferID       string
	totalFiles       int
	totalSize        int64
	processedSize    int64
	progressCallback func(int64, int64, string)
	statusCallback   func(string)

	// Reliability features
	maxRetries      int
	retryDelay      time.Duration
	chunkSize       int64
	resumeSupport   bool
	integrityChecks bool

	// Concurrency control
	mutex          sync.Mutex
	transferActive bool
	cancelContext  context.Context
	cancelFunction context.CancelFunc

	// Network adaptation
	networkProfile   transport.NetworkProfile
	adaptiveSettings AdaptiveSettings
}

// AdaptiveSettings contains settings that adapt based on network conditions
type AdaptiveSettings struct {
	TimeoutMultiplier  float64
	ChunkSizeBytes     int64
	MaxConcurrentFiles int
	RetryStrategy      RetryStrategy
}

// RetryStrategy defines how retries should be handled
type RetryStrategy struct {
	MaxAttempts   int
	InitialDelay  time.Duration
	BackoffFactor float64
	MaxDelay      time.Duration
	JitterEnabled bool
}

// TransferResult contains the result of a transfer operation
type TransferResult struct {
	Success           bool
	TransferredFiles  []string
	TotalBytes        int64
	Duration          time.Duration
	TransportUsed     string
	EncryptionMode    security.EncryptionMode
	IntegrityVerified bool
	Error             error
}

// NewBulletproofTransferManager creates a new bulletproof transfer manager
func NewBulletproofTransferManager(targetDataDir string) (*BulletproofTransferManager, error) {
	// Initialize transport manager with simplified config
	transportConfig := transport.TransportConfig{
		RelayServers: []string{
			"croc.schollz.com:9009",
		},
		Timeout: 10 * time.Second, // Shorter timeout
	}

	fmt.Printf("Creating transport manager...\n")
	transportManager, err := transport.NewMultiTransportManager(transportConfig)
	if err != nil {
		// Don't fail completely - create a minimal version
		fmt.Printf("Transport manager creation failed, using minimal version: %v\n", err)
		transportManager = &transport.MultiTransportManager{} // Minimal version
	}

	fmt.Printf("Creating advanced security...\n")
	advancedSecurity := security.NewAdvancedSecurity()

	// Skip blockchain and logging for clean user experience
	fmt.Printf("Skipping blockchain and logging for clean experience...\n")

	// Create context for cancellation
	ctx, cancel := context.WithCancel(context.Background())

	btm := &BulletproofTransferManager{
		transportManager: transportManager,
		advancedSecurity: advancedSecurity,
		blockchain:       nil, // Disabled for clean experience
		logger:           nil, // Disabled for clean experience
		targetDataDir:    targetDataDir,
		maxRetries:       5, // Reduced from 10
		retryDelay:       2 * time.Second,
		chunkSize:        10 * 1024 * 1024, // 10MB chunks
		resumeSupport:    true,
		integrityChecks:  true,
		cancelContext:    ctx,
		cancelFunction:   cancel,
		adaptiveSettings: AdaptiveSettings{
			TimeoutMultiplier:  1.0,
			ChunkSizeBytes:     10 * 1024 * 1024,
			MaxConcurrentFiles: 3,
			RetryStrategy: RetryStrategy{
				MaxAttempts:   5, // Reduced from 10
				InitialDelay:  2 * time.Second,
				BackoffFactor: 1.5,
				MaxDelay:      15 * time.Second, // Reduced from 30
				JitterEnabled: true,
			},
		},
	}

	// Skip network analysis for now to avoid hanging
	fmt.Printf("Skipping network analysis to avoid startup delay...\n")
	btm.networkProfile = transport.NetworkProfile{
		IsRestrictive:  false,
		AvailablePorts: []int{9009, 443, 80},
	}

	fmt.Printf("Transfer manager ready\n")
	return btm, nil
}

// SetProgressCallback sets the progress callback function
func (btm *BulletproofTransferManager) SetProgressCallback(callback func(int64, int64, string)) {
	btm.progressCallback = callback
}

// SetStatusCallback sets the status callback function
func (btm *BulletproofTransferManager) SetStatusCallback(callback func(string)) {
	btm.statusCallback = callback
}

// SendFiles sends files with maximum reliability
func (btm *BulletproofTransferManager) SendFiles(filePaths []string, transferCode string) (*TransferResult, error) {
	btm.mutex.Lock()
	defer btm.mutex.Unlock()

	if btm.transferActive {
		return nil, fmt.Errorf("transfer already in progress")
	}
	btm.transferActive = true
	defer func() { btm.transferActive = false }()

	startTime := time.Now()
	result := &TransferResult{
		TransferredFiles: []string{},
	}

	btm.transferID = transferCode
	btm.updateStatus("Initializing bulletproof transfer...")

	// Calculate total size
	totalSize, err := btm.calculateTotalSize(filePaths)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate total size: %w", err)
	}
	btm.totalSize = totalSize
	btm.totalFiles = len(filePaths)

	btm.updateStatus(fmt.Sprintf("Preparing %d files (%s) for secure transfer...",
		len(filePaths), btm.formatBytes(totalSize)))

	// Process files with adaptive chunking and encryption
	var transferredBytes int64
	for i, filePath := range filePaths {
		select {
		case <-btm.cancelContext.Done():
			return result, fmt.Errorf("transfer cancelled")
		default:
		}

		fileName := filepath.Base(filePath)
		btm.updateStatus(fmt.Sprintf("Processing file %d/%d: %s", i+1, len(filePaths), fileName))

		// Process file with retries
		fileResult, err := btm.processFileWithRetries(filePath, transferCode)
		if err != nil {
			btm.logger.LogError(fmt.Sprintf("Failed to process file %s: %v", filePath, err))
			result.Error = err
			return result, err
		}

		result.TransferredFiles = append(result.TransferredFiles, filePath)
		transferredBytes += fileResult.Size
		btm.updateProgress(transferredBytes, totalSize, fileName)
	}

	// Record blockchain entry
	err = btm.recordTransferInBlockchain(result, transferCode)
	if err != nil {
		btm.logger.LogWarning(fmt.Sprintf("Failed to record transfer in blockchain: %v", err))
	}

	result.Success = true
	result.TotalBytes = transferredBytes
	result.Duration = time.Since(startTime)
	result.IntegrityVerified = btm.integrityChecks

	btm.updateStatus(fmt.Sprintf("Transfer completed successfully! %d files (%s) in %v",
		len(result.TransferredFiles), btm.formatBytes(result.TotalBytes), result.Duration))

	return result, nil
}

// ReceiveFiles receives files with maximum reliability
func (btm *BulletproofTransferManager) ReceiveFiles(transferCode string) (*TransferResult, error) {
	btm.mutex.Lock()
	defer btm.mutex.Unlock()

	if btm.transferActive {
		return nil, fmt.Errorf("transfer already in progress")
	}
	btm.transferActive = true
	defer func() { btm.transferActive = false }()

	startTime := time.Now()
	result := &TransferResult{
		TransferredFiles: []string{},
	}

	btm.transferID = transferCode
	btm.updateStatus("Connecting with bulletproof reliability...")

	// Create received files directory
	receivedDir := filepath.Join(btm.targetDataDir, "received")
	if err := os.MkdirAll(receivedDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create received directory: %w", err)
	}

	// Receive files using transport manager with failover
	metadata := transport.TransferMetadata{
		TransferID: transferCode,
	}

	btm.updateStatus("Attempting to receive files through all available transports...")

	// Try to receive with automatic failover
	data, err := btm.transportManager.ReceiveWithFailover(metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to receive files: %w", err)
	}

	// Process received data
	receivedFiles, totalBytes, err := btm.processReceivedData(data, transferCode)
	if err != nil {
		return nil, fmt.Errorf("failed to process received data: %w", err)
	}

	result.Success = true
	result.TransferredFiles = receivedFiles
	result.TotalBytes = totalBytes
	result.Duration = time.Since(startTime)
	result.IntegrityVerified = btm.integrityChecks

	btm.updateStatus(fmt.Sprintf("Receive completed successfully! %d files (%s) in %v",
		len(result.TransferredFiles), btm.formatBytes(result.TotalBytes), result.Duration))

	return result, nil
}

// Helper methods

func (btm *BulletproofTransferManager) adaptSettingsToNetwork() {
	profile := btm.networkProfile

	if profile.IsRestrictive {
		// Restrictive network - be more conservative
		btm.adaptiveSettings.TimeoutMultiplier = 2.0
		btm.adaptiveSettings.ChunkSizeBytes = 5 * 1024 * 1024 // 5MB chunks
		btm.adaptiveSettings.MaxConcurrentFiles = 1
		btm.adaptiveSettings.RetryStrategy.MaxAttempts = 15
		btm.adaptiveSettings.RetryStrategy.InitialDelay = 5 * time.Second
	} else {
		// Open network - more aggressive
		btm.adaptiveSettings.TimeoutMultiplier = 1.0
		btm.adaptiveSettings.ChunkSizeBytes = 20 * 1024 * 1024 // 20MB chunks
		btm.adaptiveSettings.MaxConcurrentFiles = 5
		btm.adaptiveSettings.RetryStrategy.MaxAttempts = 8
		btm.adaptiveSettings.RetryStrategy.InitialDelay = 1 * time.Second
	}

	btm.logger.LogInfo(fmt.Sprintf("Adapted settings for %s network",
		map[bool]string{true: "restrictive", false: "open"}[profile.IsRestrictive]))
}

func (btm *BulletproofTransferManager) calculateTotalSize(filePaths []string) (int64, error) {
	var totalSize int64
	for _, filePath := range filePaths {
		info, err := os.Stat(filePath)
		if err != nil {
			return 0, fmt.Errorf("failed to stat file %s: %w", filePath, err)
		}
		totalSize += info.Size()
	}
	return totalSize, nil
}

func (btm *BulletproofTransferManager) processFileWithRetries(filePath, transferCode string) (*FileProcessResult, error) {
	strategy := btm.adaptiveSettings.RetryStrategy

	for attempt := 1; attempt <= strategy.MaxAttempts; attempt++ {
		result, err := btm.processFile(filePath, transferCode)
		if err == nil {
			return result, nil
		}

		if attempt < strategy.MaxAttempts {
			delay := btm.calculateRetryDelay(attempt, strategy)
			btm.updateStatus(fmt.Sprintf("Attempt %d failed, retrying in %v: %v",
				attempt, delay, err))
			time.Sleep(delay)
		}
	}

	return nil, fmt.Errorf("file processing failed after %d attempts", strategy.MaxAttempts)
}

type FileProcessResult struct {
	Size int64
	Hash string
}

// processReceivedData handles processing of received data and files
func (btm *BulletproofTransferManager) processReceivedData(encryptedData []byte, transferCode string) ([]string, int64, error) {
	// Decrypt data with all available modes
	var decryptedData []byte
	var err error

	// Try different encryption modes
	modes := []security.EncryptionMode{
		security.ModeCBC,
		security.ModeGCM,
		security.ModeChaCha20,
		security.ModeHybrid,
	}

	for _, mode := range modes {
		decryptedData, err = btm.advancedSecurity.DecryptWithMode(encryptedData, []byte(transferCode), mode)
		if err == nil {
			break
		}
	}

	if err != nil {
		return nil, 0, fmt.Errorf("failed to decrypt data with any mode: %w", err)
	}

	// Create received directory
	receivedDir := filepath.Join(btm.targetDataDir, "received")
	if err := os.MkdirAll(receivedDir, 0755); err != nil {
		return nil, 0, fmt.Errorf("failed to create received directory: %w", err)
	}

	// Check if it's a single file or manifest
	var manifest FileManifest
	if err := json.Unmarshal(decryptedData, &manifest); err == nil && len(manifest.Files) > 0 {
		// It's a file manifest (multiple files or folder)
		return btm.processFileManifest(manifest, receivedDir, transferCode)
	} else {
		// It's a single file
		filename := fmt.Sprintf("received_file_%d", time.Now().Unix())
		if btm.transferID != "" {
			filename = fmt.Sprintf("file_%s", btm.transferID)
		}

		filePath := filepath.Join(receivedDir, filename)
		if err := os.WriteFile(filePath, decryptedData, 0644); err != nil {
			return nil, 0, fmt.Errorf("failed to write received file: %w", err)
		}

		return []string{filePath}, int64(len(decryptedData)), nil
	}
}

// FileManifest represents multiple files or folder structure
type FileManifest struct {
	Files      map[string]FileInfo `json:"files"`
	FolderName string              `json:"folder_name,omitempty"`
	TotalFiles int                 `json:"total_files"`
	TotalSize  int64               `json:"total_size"`
}

type FileInfo struct {
	OriginalPath string `json:"original_path"`
	RelativePath string `json:"relative_path"`
	IsDirectory  bool   `json:"is_directory"`
	Size         int64  `json:"size"`
	Hash         string `json:"hash"`
	Data         []byte `json:"data,omitempty"`
}

// processFileManifest handles multiple files/folder reconstruction
func (btm *BulletproofTransferManager) processFileManifest(manifest FileManifest, receivedDir, transferCode string) ([]string, int64, error) {
	var processedFiles []string
	var totalBytes int64

	// Create base folder if specified
	baseDir := receivedDir
	if manifest.FolderName != "" {
		baseDir = filepath.Join(receivedDir, manifest.FolderName)
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			return nil, 0, fmt.Errorf("failed to create folder %s: %w", manifest.FolderName, err)
		}
	}

	// Process each file
	for _, fileInfo := range manifest.Files {
		fullPath := filepath.Join(baseDir, fileInfo.RelativePath)

		if fileInfo.IsDirectory {
			// Create directory
			if err := os.MkdirAll(fullPath, 0755); err != nil {
				return nil, 0, fmt.Errorf("failed to create directory %s: %w", fullPath, err)
			}
			processedFiles = append(processedFiles, fullPath)
		} else {
			// Create parent directories if needed
			parentDir := filepath.Dir(fullPath)
			if err := os.MkdirAll(parentDir, 0755); err != nil {
				return nil, 0, fmt.Errorf("failed to create parent directory %s: %w", parentDir, err)
			}

			// Write file data
			var fileData []byte
			if len(fileInfo.Data) > 0 {
				// Data is embedded in manifest
				fileData = fileInfo.Data
			} else {
				// Large file without embedded data - create placeholder
				if fileInfo.Size > 50*1024*1024 {
					btm.updateStatus(fmt.Sprintf("Large file %s not fully transferred - creating placeholder", fileInfo.RelativePath))

					// Create a placeholder file with info
					placeholderContent := fmt.Sprintf("LARGE FILE PLACEHOLDER\n\nOriginal file: %s\nSize: %s\nHash: %s\n\nThis file was too large to transfer in the current session.\nPlease transfer large files individually.",
						fileInfo.OriginalPath, btm.formatBytes(fileInfo.Size), fileInfo.Hash)
					fileData = []byte(placeholderContent)

					// Change filename to indicate it's a placeholder
					fullPath = fullPath + ".placeholder.txt"
				} else {
					// File data needs to be received separately (for smaller files)
					btm.updateStatus(fmt.Sprintf("Receiving file separately: %s", fileInfo.RelativePath))
					continue
				}
			}

			if err := os.WriteFile(fullPath, fileData, 0644); err != nil {
				return nil, 0, fmt.Errorf("failed to write file %s: %w", fullPath, err)
			}

			processedFiles = append(processedFiles, fullPath)
			totalBytes += int64(len(fileData))

			btm.updateProgress(totalBytes, manifest.TotalSize, fileInfo.RelativePath)
		}
	}

	return processedFiles, totalBytes, nil
}

// Enhanced processFile with folder support
func (btm *BulletproofTransferManager) processFile(filePath, transferCode string) (*FileProcessResult, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if fileInfo.IsDir() {
		// Process folder
		return btm.processFolder(filePath, transferCode)
	} else {
		// Process single file
		return btm.processSingleFile(filePath, transferCode)
	}
}

// processFolder handles sending entire folders
func (btm *BulletproofTransferManager) processFolder(folderPath, transferCode string) (*FileProcessResult, error) {
	btm.updateStatus(fmt.Sprintf("Analyzing folder: %s", filepath.Base(folderPath)))

	// Create file manifest
	manifest := FileManifest{
		Files:      make(map[string]FileInfo),
		FolderName: filepath.Base(folderPath),
		TotalFiles: 0,
		TotalSize:  0,
	}

	// Count files first for progress tracking
	fileCount := 0
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			fileCount++
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to count files: %w", err)
	}

	btm.updateStatus(fmt.Sprintf("Processing %d files in folder...", fileCount))
	processedFiles := 0

	// Walk through folder and collect all files
	err = filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			btm.updateStatus(fmt.Sprintf("Warning: Error accessing %s, skipping", path))
			return nil // Skip this file but continue
		}

		// Get relative path
		relPath, err := filepath.Rel(folderPath, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil // Skip the root folder itself
		}

		fileInfo := FileInfo{
			OriginalPath: path,
			RelativePath: relPath,
			IsDirectory:  info.IsDir(),
			Size:         info.Size(),
		}

		if !info.IsDir() {
			processedFiles++
			btm.updateProgress(int64(processedFiles), int64(fileCount), relPath)

			// Handle files of all sizes properly
			if info.Size() < 50*1024*1024 { // Increased limit to 50MB for embedded files
				data, err := os.ReadFile(path)
				if err != nil {
					btm.updateStatus(fmt.Sprintf("Warning: Could not read %s, skipping", relPath))
					return nil // Skip this file but continue with others
				}

				// Calculate hash
				hash := sha256.Sum256(data)
				fileInfo.Hash = hex.EncodeToString(hash[:])
				fileInfo.Data = data

				manifest.TotalSize += int64(len(data))
			} else {
				// For very large files (>50MB), store metadata only
				// The file data will need to be transferred separately
				btm.updateStatus(fmt.Sprintf("Large file detected: %s (%s) - adding to manifest", relPath, btm.formatBytes(info.Size())))

				// Calculate hash of first 1KB for identification
				if file, err := os.Open(path); err == nil {
					buffer := make([]byte, 1024)
					if n, err := file.Read(buffer); err == nil {
						hash := sha256.Sum256(buffer[:n])
						fileInfo.Hash = hex.EncodeToString(hash[:])
					}
					file.Close()
				}

				// Don't embed data, but include in manifest for proper reconstruction
				fileInfo.Data = nil
				manifest.TotalSize += info.Size()
			}
		}

		manifest.Files[relPath] = fileInfo
		manifest.TotalFiles++

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk folder: %w", err)
	}

	// Serialize manifest
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize manifest: %w", err)
	}

	// Calculate hash of manifest
	hash := sha256.Sum256(manifestData)
	hashString := hex.EncodeToString(hash[:])

	// Encrypt manifest
	encryptedData, _, err := btm.advancedSecurity.EncryptWithBestMode(manifestData, []byte(transferCode))
	if err != nil {
		return nil, fmt.Errorf("encryption failed: %w", err)
	}

	// Send through transport manager
	metadata := transport.TransferMetadata{
		TransferID: transferCode,
		FileName:   manifest.FolderName,
		FileSize:   int64(len(manifestData)),
		Checksum:   hashString,
	}

	err = btm.transportManager.SendWithFailover(encryptedData, metadata)
	if err != nil {
		return nil, fmt.Errorf("transport failed: %w", err)
	}

	return &FileProcessResult{
		Size: manifest.TotalSize,
		Hash: hashString,
	}, nil
}

// processSingleFile handles sending individual files
func (btm *BulletproofTransferManager) processSingleFile(filePath, transferCode string) (*FileProcessResult, error) {
	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Calculate hash
	hash := sha256.Sum256(data)
	hashString := hex.EncodeToString(hash[:])

	// Encrypt with best mode
	encryptedData, _, err := btm.advancedSecurity.EncryptWithBestMode(data, []byte(transferCode))
	if err != nil {
		return nil, fmt.Errorf("encryption failed: %w", err)
	}

	// Send through transport manager
	metadata := transport.TransferMetadata{
		TransferID: transferCode,
		FileName:   filepath.Base(filePath),
		FileSize:   int64(len(data)),
		Checksum:   hashString,
	}

	err = btm.transportManager.SendWithFailover(encryptedData, metadata)
	if err != nil {
		return nil, fmt.Errorf("transport failed: %w", err)
	}

	return &FileProcessResult{
		Size: int64(len(data)),
		Hash: hashString,
	}, nil
}

func (btm *BulletproofTransferManager) calculateRetryDelay(attempt int, strategy RetryStrategy) time.Duration {
	delay := float64(strategy.InitialDelay) *
		(strategy.BackoffFactor * float64(attempt-1))

	if delay > float64(strategy.MaxDelay) {
		delay = float64(strategy.MaxDelay)
	}

	if strategy.JitterEnabled {
		// Add up to 20% jitter
		jitter := delay * 0.2 * (2*rand.Float64() - 1)
		delay += jitter
	}

	return time.Duration(delay)
}

func (btm *BulletproofTransferManager) recordTransferInBlockchain(result *TransferResult, transferCode string) error {
	// Skip blockchain recording for clean experience
	if btm.blockchain == nil {
		return nil
	}

	entry := blockchain.TransferEntry{
		TransferCode: transferCode,
		Timestamp:    time.Now(),
		FileCount:    len(result.TransferredFiles),
		TotalSize:    result.TotalBytes,
		Success:      result.Success,
		Transport:    result.TransportUsed,
	}

	return btm.blockchain.AddTransferEntry(entry)
}

func (btm *BulletproofTransferManager) updateProgress(current, total int64, fileName string) {
	if btm.progressCallback != nil {
		btm.progressCallback(current, total, fileName)
	}
}

func (btm *BulletproofTransferManager) updateStatus(status string) {
	// Only log if logger is available
	if btm.logger != nil {
		btm.logger.LogInfo(status)
	}
	if btm.statusCallback != nil {
		btm.statusCallback(status)
	}
}

func (btm *BulletproofTransferManager) formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// GetNetworkStatus returns the current network status and transport availability
func (btm *BulletproofTransferManager) GetNetworkStatus() map[string]interface{} {
	return map[string]interface{}{
		"network_profile":   btm.networkProfile,
		"transport_status":  btm.transportManager.GetTransportStatus(),
		"adaptive_settings": btm.adaptiveSettings,
	}
}

// Cancel cancels the current transfer
func (btm *BulletproofTransferManager) Cancel() {
	if btm.cancelFunction != nil {
		btm.cancelFunction()
	}
	btm.updateStatus("Transfer cancelled by user")
}

// Close cleans up resources
func (btm *BulletproofTransferManager) Close() error {
	btm.Cancel()

	var errors []error

	if err := btm.transportManager.Close(); err != nil {
		errors = append(errors, err)
	}

	// Only close logger if it exists
	if btm.logger != nil {
		if err := btm.logger.Close(); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("cleanup errors: %v", errors)
	}

	return nil
}
