package transfer

import (
	"fmt"
	"sync"
	"time"

	"github.com/schollz/croc/v10/src/croc"
	"github.com/schollz/croc/v10/src/utils"
)

type TransferManager struct {
	isConnected   bool
	currentPeerID string
	localPeerID   string
	mutex         sync.RWMutex
	progressCb    func(progress TransferProgress)
	statusCb      func(status string)
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
		isConnected: false,
		localPeerID: localPeerID,
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

func (tm *TransferManager) ConnectToPeer(peerID string) error {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	if tm.statusCb != nil {
		tm.statusCb("Connecting...")
	}

	// TODO: Implement actual peer connection logic
	// For now, simulate connection
	time.Sleep(2 * time.Second)

	tm.isConnected = true
	tm.currentPeerID = peerID

	if tm.statusCb != nil {
		tm.statusCb(fmt.Sprintf("Connected to peer: %s", peerID))
	}

	return nil
}

func (tm *TransferManager) IsConnected() bool {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	return tm.isConnected
}

func (tm *TransferManager) GetConnectedPeerID() string {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()
	return tm.currentPeerID
}

func (tm *TransferManager) Disconnect() {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	tm.isConnected = false
	tm.currentPeerID = ""

	if tm.statusCb != nil {
		tm.statusCb("Not Connected")
	}
}

func (tm *TransferManager) SendFiles(paths []string) error {
	tm.mutex.RLock()
	if !tm.isConnected {
		tm.mutex.RUnlock()
		return fmt.Errorf("not connected to peer")
	}
	peerID := tm.currentPeerID
	tm.mutex.RUnlock()

	// Create croc options
	options := croc.Options{
		IsSender:     true,
		SharedSecret: peerID,                  // Use peer ID as shared secret for now
		RelayAddress: "croc.schollz.com:9009", // Default relay
		NoPrompt:     true,
		// TODO: Add more configuration options
	}

	// Get file info
	filesInfo, emptyFolders, totalFolders, err := croc.GetFilesInfo(paths, false, false, []string{})
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Initialize croc
	c, err := croc.New(options)
	if err != nil {
		return fmt.Errorf("failed to initialize transfer: %w", err)
	}

	// Start transfer in background
	go func() {
		err := c.Send(filesInfo, emptyFolders, totalFolders)
		if err != nil {
			if tm.statusCb != nil {
				tm.statusCb(fmt.Sprintf("Transfer failed: %v", err))
			}
		} else {
			if tm.statusCb != nil {
				tm.statusCb("Transfer completed successfully")
			}
		}
	}()

	return nil
}

func (tm *TransferManager) ReceiveFiles() error {
	tm.mutex.RLock()
	if !tm.isConnected {
		tm.mutex.RUnlock()
		return fmt.Errorf("not connected to peer")
	}
	peerID := tm.currentPeerID
	tm.mutex.RUnlock()

	// Create croc options for receiving
	options := croc.Options{
		IsSender:     false,
		SharedSecret: peerID,
		RelayAddress: "croc.schollz.com:9009",
		NoPrompt:     true,
	}

	// Initialize croc
	c, err := croc.New(options)
	if err != nil {
		return fmt.Errorf("failed to initialize transfer: %w", err)
	}

	// Start receive in background
	go func() {
		err := c.Receive()
		if err != nil {
			if tm.statusCb != nil {
				tm.statusCb(fmt.Sprintf("Receive failed: %v", err))
			}
		} else {
			if tm.statusCb != nil {
				tm.statusCb("Files received successfully")
			}
		}
	}()

	return nil
}
