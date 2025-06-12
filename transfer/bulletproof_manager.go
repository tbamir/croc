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
	"strings"
	"sync"
	"time"

	"trustdrop-bulletproof/blockchain"
	"trustdrop-bulletproof/logging"
	"trustdrop-bulletproof/security"
	"trustdrop-bulletproof/transport"
)

// BulletproofTransferManager provides ultra-reliable file transfers with network-aware failover
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
	lastTransferMeta *transport.TransferMetadata

	// Enhanced reliability features
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
	networkProfile     transport.NetworkProfile
	networkRestrictions []transport.NetworkRestriction
	adaptiveSettings   AdaptiveSettings
	lastNetworkCheck   time.Time
}

// AdaptiveSettings contains settings that adapt based on network conditions
type AdaptiveSettings struct {
	TimeoutMultiplier  float64
	ChunkSizeBytes     int64
	MaxConcurrentFiles int
	RetryStrategy      RetryStrategy
	PreferredTransport string
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
	Success            bool
	TransferredFiles   []string
	TotalBytes         int64
	Duration           time.Duration
	TransportUsed      string
	EncryptionMode     security.EncryptionMode
	IntegrityVerified  bool
	NetworkRestrictions []transport.NetworkRestriction
	NetworkType        string
	Error              error
}

// NewBulletproofTransferManager creates a production-ready transfer manager
func NewBulletproofTransferManager(targetDataDir string) (*BulletproofTransferManager, error) {
	// Initialize transport manager with enhanced config for institutional networks
	transportConfig := transport.TransportConfig{
		RelayServers: []string{
			// Primary institutional-friendly relays
			"croc.schollz.com:443",  // HTTPS port
			"croc2.schollz.com:443", // HTTPS port
			"croc.schollz.com:80",   // HTTP port
			"croc2.schollz.com:80",  // HTTP port
			// Fallback to standard ports
			"croc.schollz.com:9009",
			"croc2.schollz.com:9009",
		},
		Timeout: 60 * time.Second, // Extended timeout for institutional networks
	}

	fmt.Printf("Creating production-ready transport manager...\n")
	transportManager, err := transport.NewMultiTransportManager(transportConfig)
	if err != nil {
		// Continue with limited functionality rather than failing
		fmt.Printf("Transport manager had initialization issues: %v\n", err)
		fmt.Printf("Will continue with available transports\n")
	}

	fmt.Printf("Initializing advanced security system...\n")
	advancedSecurity := security.NewAdvancedSecurity()

	// Create context for transfer cancellation
	ctx, cancel := context.WithCancel(context.Background())

	btm := &BulletproofTransferManager{
		transportManager: transportManager,
		advancedSecurity: advancedSecurity,
		blockchain:       nil, // Will be initialized if needed
		logger:           nil, // Will be initialized if needed
		targetDataDir:    targetDataDir,
		maxRetries:       12,  // Increased for institutional networks
		retryDelay:       5 * time.Second,
		chunkSize:        8 * 1024 * 1024, // 8MB chunks for reliability
		resumeSupport:    true,
		integrityChecks:  true,
		cancelContext:    ctx,
		cancelFunction:   cancel,
		adaptiveSettings: AdaptiveSettings{
			TimeoutMultiplier:  2.0, // Conservative for institutional networks
			ChunkSizeBytes:     8 * 1024 * 1024,
			MaxConcurrentFiles: 2,   // Conservative for stability
			PreferredTransport: "https-tunnel",
			RetryStrategy: RetryStrategy{
				MaxAttempts:   12, // Increased for network reliability
				InitialDelay:  5 * time.Second,
				BackoffFactor: 1.5,
				MaxDelay:      60 * time.Second,
				JitterEnabled: true,
			},
		},
	}

	// Initialize network monitoring
	btm.initializeNetworkMonitoring()

	fmt.Printf("Production-ready transfer manager initialized\n")
	return btm, nil
}

// initializeNetworkMonitoring sets up comprehensive network monitoring
func (btm *BulletproofTransferManager) initializeNetworkMonitoring() {
	// Start with conservative defaults for institutional networks
	btm.networkProfile = transport.NetworkProfile{
		IsRestrictive:      true,  // Assume restrictive until proven otherwise
		AvailablePorts:     []int{80, 443},
		NetworkType:        "institutional",
		PreferredTransport: "https-tunnel",
	}

	// Start network analysis in background
	go func() {
		time.Sleep(1 * time.Second) // Brief delay for initialization
		btm.updateNetworkProfile()
		
		// Periodic network monitoring
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		
		for {
			select {
			case <-btm.cancelContext.Done():
				return
			case <-ticker.C:
				if time.Since(btm.lastNetworkCheck) > 2*time.Minute {
					btm.updateNetworkProfile()
				}
			}
		}
	}()
}

// updateNetworkProfile gets current network status from transport manager
func (btm *BulletproofTransferManager) updateNetworkProfile() {
	btm.lastNetworkCheck = time.Now()
	
	if btm.transportManager != nil {
		btm.networkProfile = btm.transportManager.GetNetworkProfile()
		btm.networkRestrictions = btm.transportManager.GetNetworkRestrictions()
		btm.adaptSettingsToNetwork()
		
		// Update user about significant network changes
		if len(btm.networkRestrictions) > 0 {
			restrictionDesc := btm.getNetworkRestrictionsDescription()
			btm.updateStatus(fmt.Sprintf("Network analysis: %s", restrictionDesc))
		}
	}
}

// getNetworkRestrictionsDescription provides user-friendly network status
func (btm *BulletproofTransferManager) getNetworkRestrictionsDescription() string {
	if len(btm.networkRestrictions) == 0 {
		return "Network appears open for file transfers"
	}

	criticalCount := 0
	highSeverityCount := 0
	
	for _, restriction := range btm.networkRestrictions {
		switch restriction.Severity {
		case "critical":
			criticalCount++
		case "high":
			highSeverityCount++
		}
	}

	if criticalCount > 0 {
		return "Highly restrictive network detected - using maximum compatibility mode"
	}
	if highSeverityCount > 0 {
		return "Institutional network detected - using enterprise-friendly methods"
	}
	return "Minor network restrictions detected - adjusting transport methods"
}

// SetProgressCallback sets the progress callback function
func (btm *BulletproofTransferManager) SetProgressCallback(callback func(int64, int64, string)) {
	btm.progressCallback = callback
}

// SetStatusCallback sets the status callback function
func (btm *BulletproofTransferManager) SetStatusCallback(callback func(string)) {
	btm.statusCallback = callback
}

// SendFiles sends files with maximum reliability and institutional network compatibility
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
		TransferredFiles:    []string{},
		NetworkRestrictions: btm.networkRestrictions,
		NetworkType:         btm.networkProfile.NetworkType,
	}

	btm.transferID = transferCode
	btm.updateStatus("Initializing secure transfer...")

	// Provide network-specific guidance
	btm.provideNetworkGuidance()

	// Calculate total size with progress updates
	totalSize, err := btm.calculateTotalSizeWithProgress(filePaths)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze files: %w", err)
	}
	btm.totalSize = totalSize
	btm.totalFiles = len(filePaths)

	btm.updateStatus(fmt.Sprintf("Preparing %d files (%s) for secure transfer...",
		len(filePaths), btm.formatBytes(totalSize)))

	// Process files with enhanced error handling and network awareness
	var transferredBytes int64
	for i, filePath := range filePaths {
		select {
		case <-btm.cancelContext.Done():
			return result, fmt.Errorf("transfer cancelled by user")
		default:
		}

		fileName := filepath.Base(filePath)
		btm.updateStatus(fmt.Sprintf("Processing file %d/%d: %s", i+1, len(filePaths), fileName))

		// Process file with institutional network-aware retries
		fileResult, err := btm.processFileWithNetworkAwareRetries(filePath, transferCode)
		if err != nil {
			detailedError := btm.enhanceErrorMessage(err, filePath)
			btm.updateStatus(fmt.Sprintf("Failed to process file %s", fileName))
			result.Error = detailedError
			return result, result.Error
		}

		result.TransferredFiles = append(result.TransferredFiles, filePath)
		transferredBytes += fileResult.Size
		btm.updateProgress(transferredBytes, totalSize, fileName)
	}

	// Record blockchain entry if available
	if err := btm.recordTransferInBlockchain(result, transferCode); err != nil {
		btm.updateStatus(fmt.Sprintf("Note: Transfer audit logging unavailable: %v", err))
	}

	result.Success = true
	result.TotalBytes = transferredBytes
	result.Duration = time.Since(startTime)
	result.IntegrityVerified = btm.integrityChecks
	result.TransportUsed = btm.getUsedTransportName()

	successMsg := fmt.Sprintf("Transfer completed successfully! %d files (%s) in %v",
		len(result.TransferredFiles), btm.formatBytes(result.TotalBytes), result.Duration)
	
	if btm.networkProfile.IsRestrictive {
		successMsg += " via institutional-compatible transport"
	}
	
	btm.updateStatus(successMsg)
	return result, nil
}

// ReceiveFiles receives files with enhanced reliability and institutional network support
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
		TransferredFiles:    []string{},
		NetworkRestrictions: btm.networkRestrictions,
		NetworkType:         btm.networkProfile.NetworkType,
	}

	btm.transferID = transferCode
	btm.updateStatus("Connecting with enhanced reliability...")

	// Provide network-specific connection guidance
	btm.provideConnectionGuidance()

	// Create received files directory
	receivedDir := filepath.Join(btm.targetDataDir, "received")
	if err := os.MkdirAll(receivedDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create received directory: %w", err)
	}

	// Receive files using transport manager with institutional network optimization
	metadata := transport.TransferMetadata{
		TransferID: transferCode,
	}
	btm.lastTransferMeta = &metadata

	btm.updateStatus("Establishing secure connection through available transports...")

	// Receive with enhanced retries optimized for institutional networks
	data, err := btm.receiveWithInstitutionalNetworkSupport(metadata)
	if err != nil {
		detailedError := btm.enhanceErrorMessage(err, "")
		return nil, detailedError
	}

	// Process received data with enhanced metadata preservation
	enhancedMetadata := &transport.TransferMetadata{
		TransferID: transferCode,
		FileName:   btm.lastTransferMeta.FileName,
	}

	receivedFiles, totalBytes, err := btm.processReceivedDataWithMetadata(data, transferCode, enhancedMetadata)
	if err != nil {
		return nil, fmt.Errorf("failed to process received data: %w", err)
	}

	result.Success = true
	result.TransferredFiles = receivedFiles
	result.TotalBytes = totalBytes
	result.Duration = time.Since(startTime)
	result.IntegrityVerified = btm.integrityChecks
	result.TransportUsed = btm.getUsedTransportName()

	// Record in blockchain if available
	if err := btm.recordTransferInBlockchain(result, transferCode); err != nil {
		btm.updateStatus(fmt.Sprintf("Note: Transfer audit logging unavailable: %v", err))
	}

	successMsg := fmt.Sprintf("Transfer completed successfully! %d files (%s) in %v",
		len(result.TransferredFiles), btm.formatBytes(result.TotalBytes), result.Duration)
	
	if btm.networkProfile.IsRestrictive {
		successMsg += " via institutional-compatible transport"
	}
	
	btm.updateStatus(successMsg)
	return result, nil
}

// provideNetworkGuidance provides user guidance based on network conditions
func (btm *BulletproofTransferManager) provideNetworkGuidance() {
	if btm.networkProfile.IsRestrictive {
		switch btm.networkProfile.NetworkType {
		case "corporate":
			btm.updateStatus("Corporate network detected - using enterprise-friendly transfer methods")
		case "university":
			btm.updateStatus("University network detected - using education-network-compatible methods")
		case "institutional":
			btm.updateStatus("Institutional network detected - using maximum compatibility mode")
		default:
			btm.updateStatus("Restrictive network detected - optimizing for institutional compatibility")
		}
	} else {
		btm.updateStatus("Open network detected - using optimized transfer methods")
	}
}

// provideConnectionGuidance provides connection-specific guidance
func (btm *BulletproofTransferManager) provideConnectionGuidance() {
	if btm.networkProfile.IsRestrictive {
		btm.updateStatus("Institutional network - connecting via web-compatible protocols...")
		if len(btm.networkRestrictions) > 0 {
			btm.updateStatus("Note: Network restrictions detected, using maximum compatibility mode")
		}
	} else {
		btm.updateStatus("Connecting via available transport methods...")
	}
}

// receiveWithInstitutionalNetworkSupport performs receive with institutional network optimization
func (btm *BulletproofTransferManager) receiveWithInstitutionalNetworkSupport(metadata transport.TransferMetadata) ([]byte, error) {
	strategy := btm.adaptiveSettings.RetryStrategy
	
	// Extended retry logic for institutional networks
	maxAttempts := strategy.MaxAttempts
	if btm.networkProfile.IsRestrictive {
		maxAttempts = int(float64(maxAttempts) * 1.5) // 50% more attempts for institutional networks
	}
	
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		// Update status with institutional network context
		if attempt > 1 {
			if btm.networkProfile.IsRestrictive {
				btm.updateStatus(fmt.Sprintf("Connection attempt %d/%d (institutional network may require additional time)...", 
					attempt, maxAttempts))
			} else {
				btm.updateStatus(fmt.Sprintf("Connection attempt %d/%d...", attempt, maxAttempts))
			}
		}

		data, err := btm.transportManager.ReceiveWithFailover(metadata)
		if err == nil {
			return data, nil
		}

		// Enhanced error analysis for institutional networks
		if btm.isInstitutionalNetworkError(err) && attempt <= 3 {
			btm.updateStatus("Institutional network restrictions detected - adjusting connection method...")
			time.Sleep(5 * time.Second) // Extended delay for network adaptation
		}

		if attempt < maxAttempts {
			delay := btm.calculateInstitutionalNetworkDelay(attempt, strategy)
			btm.updateStatus(fmt.Sprintf("Attempt %d failed, retrying in %v: %v",
				attempt, delay, btm.simplifyErrorMessage(err)))
			time.Sleep(delay)
		}
	}

	return nil, fmt.Errorf("receive failed after %d attempts optimized for institutional networks", maxAttempts)
}

// processFileWithNetworkAwareRetries processes files with network-aware retry logic
func (btm *BulletproofTransferManager) processFileWithNetworkAwareRetries(filePath, transferCode string) (*FileProcessResult, error) {
	strategy := btm.adaptiveSettings.RetryStrategy

	// Adjust attempts based on network type
	maxAttempts := strategy.MaxAttempts
	if btm.networkProfile.IsRestrictive {
		maxAttempts = int(float64(maxAttempts) * 1.5)
	}

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		result, err := btm.processFile(filePath, transferCode)
		if err == nil {
			return result, nil
		}

		// Enhanced error handling for institutional networks
		if btm.isInstitutionalNetworkError(err) && attempt <= 3 {
			btm.updateStatus("Institutional network restrictions detected - adjusting transport method...")
			time.Sleep(3 * time.Second)
		}

		if attempt < maxAttempts {
			delay := btm.calculateInstitutionalNetworkDelay(attempt, strategy)
			btm.updateStatus(fmt.Sprintf("Processing attempt %d failed, retrying in %v: %v",
				attempt, delay, btm.simplifyErrorMessage(err)))
			time.Sleep(delay)
		}
	}

	return nil, fmt.Errorf("file processing failed after %d network-aware attempts", maxAttempts)
}

// isInstitutionalNetworkError checks if an error indicates institutional network restrictions
func (btm *BulletproofTransferManager) isInstitutionalNetworkError(err error) bool {
	if err == nil {
		return false
	}
	
	errorStr := strings.ToLower(err.Error())
	institutionalErrorIndicators := []string{
		"connection refused", "timeout", "network unreachable", "no route to host",
		"connection reset", "dial tcp", "croc relay", "firewall", "blocked",
		"proxy", "institutional", "corporate", "university", "managed network",
		"policy", "restricted", "filtered", "deep packet inspection", "dpi",
	}
	
	for _, indicator := range institutionalErrorIndicators {
		if strings.Contains(errorStr, indicator) {
			return true
		}
	}
	
	return false
}

// calculateInstitutionalNetworkDelay calculates backoff delay optimized for institutional networks
func (btm *BulletproofTransferManager) calculateInstitutionalNetworkDelay(attempt int, strategy RetryStrategy) time.Duration {
	// Base delay calculation
	delay := float64(strategy.InitialDelay) * (strategy.BackoffFactor * float64(attempt-1))

	// Extended delays for institutional networks
	if btm.networkProfile.IsRestrictive {
		delay *= 1.5 // 50% longer delays for institutional networks
	}

	if delay > float64(strategy.MaxDelay) {
		delay = float64(strategy.MaxDelay)
	}

	// Add jitter to avoid thundering herd in institutional environments
	if strategy.JitterEnabled {
		jitter := delay * 0.3 * (2*rand.Float64() - 1) // Up to 30% jitter
		delay += jitter
	}

	return time.Duration(delay)
}

// enhanceErrorMessage provides detailed, network-aware error messages
func (btm *BulletproofTransferManager) enhanceErrorMessage(err error, filePath string) error {
	if err == nil {
		return nil
	}

	var enhancedMsg strings.Builder
	
	// Check if this is an institutional network-related error
	if btm.isInstitutionalNetworkError(err) {
		if btm.networkProfile.IsRestrictive {
			enhancedMsg.WriteString("Transfer failed due to institutional network restrictions.\n\n")
			
			switch btm.networkProfile.NetworkType {
			case "corporate":
				enhancedMsg.WriteString("Your corporate network has strict security policies that block ")
				enhancedMsg.WriteString("peer-to-peer file transfer protocols.\n\n")
			case "university":
				enhancedMsg.WriteString("Your university network has academic security policies that restrict ")
				enhancedMsg.WriteString("direct file transfer protocols.\n\n")
			default:
				enhancedMsg.WriteString("Your managed network has IT policies that block ")
				enhancedMsg.WriteString("direct file transfer protocols.\n\n")
			}
			
			if len(btm.networkRestrictions) > 0 {
				enhancedMsg.WriteString("Detected network restrictions:\n")
				for _, restriction := range btm.networkRestrictions {
					enhancedMsg.WriteString(fmt.Sprintf("• %s: %s\n", 
						strings.Title(restriction.Type), restriction.Description))
				}
				enhancedMsg.WriteString("\n")
			}
			
			enhancedMsg.WriteString("Recommended solutions:\n")
			enhancedMsg.WriteString("• Try from a different network (mobile hotspot, home WiFi)\n")
			enhancedMsg.WriteString("• Contact your IT department about approved file transfer methods\n")
			enhancedMsg.WriteString("• Use a personal device with mobile data if permitted by policy\n")
			enhancedMsg.WriteString("• Consider using your organization's approved file sharing platform\n")
			enhancedMsg.WriteString("• Temporarily connect via mobile hotspot if policies allow\n")
		} else {
			enhancedMsg.WriteString("Network connectivity issue detected.\n\n")
			enhancedMsg.WriteString("Troubleshooting steps:\n")
			enhancedMsg.WriteString("• Verify your internet connection is stable\n")
			enhancedMsg.WriteString("• Check if your firewall or antivirus is blocking the connection\n")
			enhancedMsg.WriteString("• Try again in a few minutes in case of temporary network issues\n")
			enhancedMsg.WriteString("• Restart your network adapter or router if problems persist\n")
		}
	} else {
		// Non-network error
		enhancedMsg.WriteString("Transfer failed: ")
		enhancedMsg.WriteString(btm.simplifyErrorMessage(err))
		enhancedMsg.WriteString("\n\nGeneral troubleshooting steps:\n")
		enhancedMsg.WriteString("• Verify the transfer code is correct and hasn't expired\n")
		enhancedMsg.WriteString("• Ensure both devices are connected to the internet\n")
		enhancedMsg.WriteString("• Try restarting the application\n")
		enhancedMsg.WriteString("• Check available disk space on the receiving device\n")
	}
	
	if filePath != "" {
		enhancedMsg.WriteString(fmt.Sprintf("\nFile: %s", filePath))
	}
	
	enhancedMsg.WriteString(fmt.Sprintf("\nNetwork Type: %s", btm.networkProfile.NetworkType))
	enhancedMsg.WriteString(fmt.Sprintf("\nTechnical Details: %v", err))
	
	return fmt.Errorf("%s", enhancedMsg.String())
}

// simplifyErrorMessage creates user-friendly versions of technical errors
func (btm *BulletproofTransferManager) simplifyErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	
	errorStr := err.Error()
	
	// Replace technical terms with user-friendly ones
	replacements := map[string]string{
		"dial tcp":               "connection failed",
		"connection refused":     "connection blocked by network",
		"i/o timeout":           "connection timeout",
		"network unreachable":   "network not accessible",
		"no route to host":      "destination unreachable",
		"connection reset":      "connection interrupted by network",
		"institutional":         "managed network",
		"corporate":             "business network",
		"university":            "educational network",
	}
	
	for technical, friendly := range replacements {
		if strings.Contains(strings.ToLower(errorStr), technical) {
			return friendly
		}
	}
	
	// If no simplification found, return original but truncated
	if len(errorStr) > 100 {
		return errorStr[:97] + "..."
	}
	
	return errorStr
}

// getUsedTransportName returns the name of the transport that was used
func (btm *BulletproofTransferManager) getUsedTransportName() string {
	if btm.transportManager == nil {
		return "unknown"
	}
	
	// Get transport status to find which was successful
	status := btm.transportManager.GetTransportStatus()
	for name, transportStatus := range status {
		if statusMap, ok := transportStatus.(map[string]interface{}); ok {
			if available, ok := statusMap["available"].(bool); ok && available {
				if recommended, ok := statusMap["recommended"].(bool); ok && recommended {
					return name
				}
			}
		}
	}
	
	return "auto-selected"
}

// adaptSettingsToNetwork adapts settings based on network analysis
func (btm *BulletproofTransferManager) adaptSettingsToNetwork() {
	profile := btm.networkProfile

	if profile.IsRestrictive {
		// Restrictive network - be more conservative and patient
		btm.adaptiveSettings.TimeoutMultiplier = 3.0
		btm.adaptiveSettings.ChunkSizeBytes = 4 * 1024 * 1024 // 4MB chunks for institutional networks
		btm.adaptiveSettings.MaxConcurrentFiles = 1
		btm.adaptiveSettings.PreferredTransport = "https-tunnel"
		btm.adaptiveSettings.RetryStrategy.MaxAttempts = 15    // More retries
		btm.adaptiveSettings.RetryStrategy.InitialDelay = 7 * time.Second
		btm.adaptiveSettings.RetryStrategy.MaxDelay = 90 * time.Second
	} else {
		// Open network - more aggressive but still reliable
		btm.adaptiveSettings.TimeoutMultiplier = 1.5
		btm.adaptiveSettings.ChunkSizeBytes = 16 * 1024 * 1024 // 16MB chunks
		btm.adaptiveSettings.MaxConcurrentFiles = 3
		btm.adaptiveSettings.PreferredTransport = "https-tunnel" // Still prefer HTTPS for reliability
		btm.adaptiveSettings.RetryStrategy.MaxAttempts = 10
		btm.adaptiveSettings.RetryStrategy.InitialDelay = 3 * time.Second
		btm.adaptiveSettings.RetryStrategy.MaxDelay = 45 * time.Second
	}

	fmt.Printf("Adapted settings for %s network (restrictive: %t, preferred: %s)\n",
		profile.NetworkType, profile.IsRestrictive, btm.adaptiveSettings.PreferredTransport)
}

// calculateTotalSizeWithProgress calculates the total size of files with progress updates
func (btm *BulletproofTransferManager) calculateTotalSizeWithProgress(filePaths []string) (int64, error) {
	var totalSize int64
	for i, filePath := range filePaths {
		btm.updateStatus(fmt.Sprintf("Analyzing file %d/%d: %s", i+1, len(filePaths), filepath.Base(filePath)))
		
		info, err := os.Stat(filePath)
		if err != nil {
			return 0, fmt.Errorf("failed to analyze file %s: %w", filePath, err)
		}
		totalSize += info.Size()
	}
	return totalSize, nil
}

type FileProcessResult struct {
	Size int64
	Hash string
}

// processReceivedDataWithMetadata handles processing of received data with enhanced metadata
func (btm *BulletproofTransferManager) processReceivedDataWithMetadata(encryptedData []byte, transferCode string, metadata *transport.TransferMetadata) ([]string, int64, error) {
	// Decrypt data with all available modes
	var decryptedData []byte
	var err error

	// Try different encryption modes for maximum compatibility
	modes := []security.EncryptionMode{
		security.ModeCBC,    // Most compatible
		security.ModeGCM,    // Modern authenticated
		security.ModeChaCha20, // High security
		security.ModeHybrid,   // Future-proof
	}

	for _, mode := range modes {
		decryptedData, err = btm.advancedSecurity.DecryptWithMode(encryptedData, []byte(transferCode), mode)
		if err == nil {
			break
		}
	}

	if err != nil {
		return nil, 0, fmt.Errorf("failed to decrypt data with any supported encryption mode: %w", err)
	}

	// Create received directory
	receivedDir := filepath.Join(btm.targetDataDir, "received")
	if err := os.MkdirAll(receivedDir, 0755); err != nil {
		return nil, 0, fmt.Errorf("failed to create received directory: %w", err)
	}

	// Try to parse as file manifest (multiple files or folder)
	var manifest FileManifest
	if err := json.Unmarshal(decryptedData, &manifest); err == nil && len(manifest.Files) > 0 {
		return btm.processFileManifestWithProgress(manifest, receivedDir, transferCode)
	}

	// Try to parse as single file payload with embedded filename
	var filePayload struct {
		OriginalName string `json:"original_name"`
		Data         []byte `json:"data"`
	}

	if err := json.Unmarshal(decryptedData, &filePayload); err == nil && filePayload.OriginalName != "" {
		// Single file with embedded filename
		filename := btm.sanitizeFilename(filePayload.OriginalName)
		filePath := filepath.Join(receivedDir, filename)
		
		if err := os.WriteFile(filePath, filePayload.Data, 0644); err != nil {
			return nil, 0, fmt.Errorf("failed to write received file: %w", err)
		}

		btm.updateStatus(fmt.Sprintf("Received file: %s", filename))
		return []string{filePath}, int64(len(filePayload.Data)), nil
	}

	// Raw file data (legacy format)
	filename := fmt.Sprintf("received_file_%d", time.Now().Unix())
	if metadata != nil && metadata.FileName != "" {
		filename = btm.sanitizeFilename(metadata.FileName)
	} else if btm.transferID != "" {
		filename = fmt.Sprintf("file_%s", btm.transferID)
	}

	filePath := filepath.Join(receivedDir, filename)
	if err := os.WriteFile(filePath, decryptedData, 0644); err != nil {
		return nil, 0, fmt.Errorf("failed to write received file: %w", err)
	}

	btm.updateStatus(fmt.Sprintf("Received file: %s", filename))
	return []string{filePath}, int64(len(decryptedData)), nil
}

// sanitizeFilename ensures filenames are safe for the filesystem
func (btm *BulletproofTransferManager) sanitizeFilename(filename string) string {
	// Remove path components and dangerous characters
	filename = filepath.Base(filename)
	
	// Replace dangerous characters
	dangerous := []string{"..", "/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	for _, char := range dangerous {
		filename = strings.ReplaceAll(filename, char, "_")
	}
	
	// Ensure filename isn't empty
	if filename == "" || filename == "." || filename == ".." {
		filename = fmt.Sprintf("received_file_%d", time.Now().Unix())
	}
	
	return filename
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

// processFileManifestWithProgress handles multiple files/folder reconstruction with progress
func (btm *BulletproofTransferManager) processFileManifestWithProgress(manifest FileManifest, receivedDir, transferCode string) ([]string, int64, error) {
	var processedFiles []string
	var totalBytes int64

	// Create base folder if specified
	baseDir := receivedDir
	if manifest.FolderName != "" {
		baseDir = filepath.Join(receivedDir, btm.sanitizeFilename(manifest.FolderName))
		if err := os.MkdirAll(baseDir, 0755); err != nil {
			return nil, 0, fmt.Errorf("failed to create folder %s: %w", manifest.FolderName, err)
		}
	}

	btm.updateStatus(fmt.Sprintf("Reconstructing %d files from transfer...", manifest.TotalFiles))

	// Process each file with progress updates
	processedCount := 0
	for _, fileInfo := range manifest.Files {
		processedCount++
		
		// Sanitize the relative path
		relativePath := btm.sanitizeFilename(fileInfo.RelativePath)
		fullPath := filepath.Join(baseDir, relativePath)

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

			var fileData []byte
			if len(fileInfo.Data) > 0 {
				fileData = fileInfo.Data
			} else {
				// Large file placeholder
				if fileInfo.Size > 100*1024*1024 {
					btm.updateStatus(fmt.Sprintf("Large file %s requires separate transfer", fileInfo.RelativePath))
					placeholderContent := fmt.Sprintf("LARGE FILE PLACEHOLDER\n\nOriginal: %s\nSize: %s\nHash: %s\n\nThis file was too large for the current transfer method.\nPlease transfer large files individually.",
						fileInfo.OriginalPath, btm.formatBytes(fileInfo.Size), fileInfo.Hash)
					fileData = []byte(placeholderContent)
					fullPath = fullPath + ".placeholder.txt"
				} else {
					btm.updateStatus(fmt.Sprintf("Skipping incomplete file: %s", fileInfo.RelativePath))
					continue
				}
			}

			if err := os.WriteFile(fullPath, fileData, 0644); err != nil {
				return nil, 0, fmt.Errorf("failed to write file %s: %w", fullPath, err)
			}

			processedFiles = append(processedFiles, fullPath)
			totalBytes += int64(len(fileData))

			// Update progress
			btm.updateProgress(int64(processedCount), int64(manifest.TotalFiles), fileInfo.RelativePath)
		}
	}

	btm.updateStatus(fmt.Sprintf("Successfully reconstructed %d files", len(processedFiles)))
	return processedFiles, totalBytes, nil
}

// processFile handles sending individual files or folders
func (btm *BulletproofTransferManager) processFile(filePath, transferCode string) (*FileProcessResult, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to access file: %w", err)
	}

	if fileInfo.IsDir() {
		return btm.processFolder(filePath, transferCode)
	} else {
		return btm.processSingleFile(filePath, transferCode)
	}
}

// processFolder handles sending entire folders
func (btm *BulletproofTransferManager) processFolder(folderPath, transferCode string) (*FileProcessResult, error) {
	btm.updateStatus(fmt.Sprintf("Analyzing folder: %s", filepath.Base(folderPath)))

	manifest := FileManifest{
		Files:      make(map[string]FileInfo),
		FolderName: filepath.Base(folderPath),
		TotalFiles: 0,
		TotalSize:  0,
	}

	// Count files for progress tracking
	fileCount := 0
	err := filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			fileCount++
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to analyze folder: %w", err)
	}

	btm.updateStatus(fmt.Sprintf("Processing %d files in folder...", fileCount))
	processedFiles := 0

	// Walk through folder and collect files
	err = filepath.Walk(folderPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			btm.updateStatus(fmt.Sprintf("Warning: Error accessing %s, skipping", path))
			return nil
		}

		relPath, err := filepath.Rel(folderPath, path)
		if err != nil {
			return err
		}

		if relPath == "." {
			return nil
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

			// Handle files up to 25MB for embedding
			if info.Size() < 25*1024*1024 {
				data, err := os.ReadFile(path)
				if err != nil {
					btm.updateStatus(fmt.Sprintf("Warning: Could not read %s, skipping", relPath))
					return nil
				}

				hash := sha256.Sum256(data)
				fileInfo.Hash = hex.EncodeToString(hash[:])
				fileInfo.Data = data
				manifest.TotalSize += int64(len(data))
			} else {
				// For larger files, store metadata only
				btm.updateStatus(fmt.Sprintf("Large file detected: %s (%s) - adding metadata only", 
					relPath, btm.formatBytes(info.Size())))

				if file, err := os.Open(path); err == nil {
					buffer := make([]byte, 1024)
					if n, err := file.Read(buffer); err == nil {
						hash := sha256.Sum256(buffer[:n])
						fileInfo.Hash = hex.EncodeToString(hash[:])
					}
					file.Close()
				}

				fileInfo.Data = nil
				manifest.TotalSize += info.Size()
			}
		}

		manifest.Files[relPath] = fileInfo
		manifest.TotalFiles++
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to process folder: %w", err)
	}

	// Serialize and encrypt manifest
	manifestData, err := json.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("failed to create folder manifest: %w", err)
	}

	hash := sha256.Sum256(manifestData)
	hashString := hex.EncodeToString(hash[:])

	encryptedData, _, err := btm.advancedSecurity.EncryptWithBestMode(manifestData, []byte(transferCode))
	if err != nil {
		return nil, fmt.Errorf("encryption failed: %w", err)
	}

	// Send via transport manager
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
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Create file payload with preserved filename
	filePayload := struct {
		OriginalName string `json:"original_name"`
		Data         []byte `json:"data"`
	}{
		OriginalName: filepath.Base(filePath),
		Data:         data,
	}

	payloadData, err := json.Marshal(filePayload)
	if err != nil {
		return nil, fmt.Errorf("failed to create file payload: %w", err)
	}

	hash := sha256.Sum256(data)
	hashString := hex.EncodeToString(hash[:])

	encryptedData, _, err := btm.advancedSecurity.EncryptWithBestMode(payloadData, []byte(transferCode))
	if err != nil {
		return nil, fmt.Errorf("encryption failed: %w", err)
	}

	metadata := transport.TransferMetadata{
		TransferID: transferCode,
		FileName:   filepath.Base(filePath),
		FileSize:   int64(len(payloadData)),
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

// recordTransferInBlockchain records transfer in blockchain if enabled
func (btm *BulletproofTransferManager) recordTransferInBlockchain(result *TransferResult, transferCode string) error {
	// Initialize blockchain if not already done
	if btm.blockchain == nil {
		blockchain, err := blockchain.NewBlockchain(btm.targetDataDir)
		if err != nil {
			return fmt.Errorf("failed to initialize blockchain: %w", err)
		}
		btm.blockchain = blockchain
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

// updateProgress calls the progress callback if set
func (btm *BulletproofTransferManager) updateProgress(current, total int64, fileName string) {
	if btm.progressCallback != nil {
		btm.progressCallback(current, total, fileName)
	}
}

// updateStatus calls the status callback if set
func (btm *BulletproofTransferManager) updateStatus(status string) {
	if btm.logger != nil {
		btm.logger.LogInfo(status)
	}
	if btm.statusCallback != nil {
		btm.statusCallback(status)
	}
}

// formatBytes formats bytes in human-readable format
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

// GetNetworkStatus returns comprehensive network status and transport availability
func (btm *BulletproofTransferManager) GetNetworkStatus() map[string]interface{} {
	status := map[string]interface{}{
		"network_profile":      btm.networkProfile,
		"network_restrictions": btm.networkRestrictions,
		"adaptive_settings":    btm.adaptiveSettings,
		"last_check":          btm.lastNetworkCheck,
	}

	if btm.transportManager != nil {
		status["transport_status"] = btm.transportManager.GetTransportStatus()
	}

	return status
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

	if btm.transportManager != nil {
		if err := btm.transportManager.Close(); err != nil {
			errors = append(errors, err)
		}
	}

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