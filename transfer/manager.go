package transfer

import (
	"fmt"
	"sync"

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
}

type TransferProgress struct {
	CurrentFile      string
	FilesRemaining   int
	PercentComplete  float64
	BytesTransferred int64
	TotalBytes       int64
}

func NewTransferManager() *TransferManager {
	// Generate local peer ID using croc's method
	localPeerID := utils.GetRandomName()

	return &TransferManager{
		localPeerID: localPeerID,
		isReceiving: false,
		isSending:   false,
	}
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

	tm.currentSecret = peerSecret
	tm.isReceiving = true

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

	tm.isSending = true

	if tm.statusCb != nil {
		tm.statusCb("Connecting to receiver...")
	}

	// Start sending in background
	go tm.sendFiles(paths)

	return nil
}

func (tm *TransferManager) receiveFiles() {
	defer func() {
		tm.mutex.Lock()
		tm.isReceiving = false
		tm.currentSecret = ""
		tm.mutex.Unlock()
	}()

	// Create croc options for receiving
	options := croc.Options{
		IsSender:     false,
		SharedSecret: tm.currentSecret,
		RelayAddress: "croc.schollz.com:9009",
		RelayPorts:   []string{"9009", "9010", "9011", "9012", "9013"},
		NoPrompt:     true,
		Debug:        false,
	}

	// Initialize croc
	c, err := croc.New(options)
	if err != nil {
		if tm.statusCb != nil {
			tm.statusCb(fmt.Sprintf("Failed to initialize: %v", err))
		}
		return
	}

	if tm.statusCb != nil {
		tm.statusCb("Connected! Receiving files...")
	}

	// Start receiving - this blocks until complete or error
	err = c.Receive()
	if err != nil {
		if tm.statusCb != nil {
			tm.statusCb(fmt.Sprintf("Receive failed: %v", err))
		}
	} else {
		if tm.statusCb != nil {
			tm.statusCb("Files received successfully!")
		}
	}
}

func (tm *TransferManager) sendFiles(paths []string) {
	defer func() {
		tm.mutex.Lock()
		tm.isSending = false
		tm.mutex.Unlock()
	}()

	// Create croc options for sending
	options := croc.Options{
		IsSender:     true,
		SharedSecret: tm.localPeerID, // Use our local peer ID as the secret
		RelayAddress: "croc.schollz.com:9009",
		RelayPorts:   []string{"9009", "9010", "9011", "9012", "9013"},
		NoPrompt:     true,
		Debug:        false,
	}

	// Get file info
	filesInfo, emptyFolders, totalFolders, err := croc.GetFilesInfo(paths, false, false, []string{})
	if err != nil {
		if tm.statusCb != nil {
			tm.statusCb(fmt.Sprintf("Failed to analyze files: %v", err))
		}
		return
	}

	// Initialize croc
	c, err := croc.New(options)
	if err != nil {
		if tm.statusCb != nil {
			tm.statusCb(fmt.Sprintf("Failed to initialize: %v", err))
		}
		return
	}

	if tm.statusCb != nil {
		tm.statusCb("Connected! Sending files...")
	}

	// Start sending - this blocks until complete or error
	err = c.Send(filesInfo, emptyFolders, totalFolders)
	if err != nil {
		if tm.statusCb != nil {
			tm.statusCb(fmt.Sprintf("Send failed: %v", err))
		}
	} else {
		if tm.statusCb != nil {
			tm.statusCb("Files sent successfully!")
		}
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

	tm.isReceiving = false
	tm.isSending = false
	tm.currentSecret = ""

	if tm.statusCb != nil {
		tm.statusCb("Transfer cancelled")
	}
}
