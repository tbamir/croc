package transfer

import (
	"fmt"
	"sync"
	"time"
	"trustdrop/logging"

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

func (tm *TransferManager) receiveFiles() {
	defer func() {
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
		
		tm.logTransfer(tm.currentSecret, tm.currentFile, tm.currentSize, "receive", status, errorMsg, duration)
		
		tm.isReceiving = false
		tm.currentSecret = ""
		tm.crocClient = nil
		tm.mutex.Unlock()
	}()

	fmt.Printf("TransferManager: Configuring croc for receiving...\n")

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
		Debug:          true, // Enable debug for troubleshooting
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
	} else {
		fmt.Printf("TransferManager: Receive successful!\n")
		if tm.statusCb != nil {
			tm.statusCb("Files received successfully!")
		}
		// Update file info from successful transfer
		if len(c.FilesToTransfer) > 0 {
			tm.currentFile = c.FilesToTransfer[0].Name
			tm.currentSize = c.FilesToTransfer[0].Size
		}
	}
}

func (tm *TransferManager) sendFiles(paths []string) {
	defer func() {
		tm.mutex.Lock()
		
		// Log the transfer to blockchain
		duration := time.Since(tm.transferStart).String()
		status := "success"
		errorMsg := ""
		
		if tm.crocClient == nil || !tm.crocClient.SuccessfulTransfer {
			status = "failed"
			errorMsg = "Transfer failed"
		}
		
		tm.logTransfer(tm.localPeerID, tm.currentFile, tm.currentSize, "send", status, errorMsg, duration)
		
		tm.isSending = false
		tm.crocClient = nil
		tm.mutex.Unlock()
	}()

	fmt.Printf("TransferManager: Getting file info for paths: %v\n", paths)

	// Get file info
	filesInfo, emptyFolders, totalFolders, err := croc.GetFilesInfo(paths, false, false, []string{})
	if err != nil {
		fmt.Printf("TransferManager: Failed to analyze files: %v\n", err)
		if tm.statusCb != nil {
			tm.statusCb(fmt.Sprintf("Failed to analyze files: %v", err))
		}
		return
	}

	// Update file info for logging
	if len(filesInfo) > 0 {
		tm.currentFile = filesInfo[0].Name
		tm.currentSize = filesInfo[0].Size
		fmt.Printf("TransferManager: Sending file: %s (size: %d bytes)\n", tm.currentFile, tm.currentSize)
	}

	// Create croc options for sending
	options := croc.Options{
		IsSender:       true,
		SharedSecret:   tm.localPeerID, // Use our local peer ID as the secret
		RelayAddress:   "croc.schollz.com:9009",
		RelayAddress6:  "croc6.schollz.com:9009",
		RelayPorts:     []string{"9009", "9010", "9011", "9012", "9013"},
		RelayPassword:  "pass123",
		NoPrompt:       true,
		NoMultiplexing: false,
		DisableLocal:   false,
		Ask:            false,
		Debug:          true, // Enable debug for troubleshooting
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
func (tm *TransferManager) logTransfer(peerID, fileName string, fileSize int64, direction, status, errorMsg, duration string) {
	log := logging.TransferLog{
		Timestamp: time.Now(),
		PeerID:    peerID,
		FileName:  fileName,
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
		tm.logTransfer(tm.localPeerID, tm.currentFile, tm.currentSize, direction, "cancelled", "User cancelled", duration)
	}

	tm.isReceiving = false
	tm.isSending = false
	tm.currentSecret = ""

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