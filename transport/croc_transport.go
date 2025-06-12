package transport

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/schollz/croc/v10/src/croc"
)

// SimpleCrocTransport provides a straightforward wrapper around the CROC library
type SimpleCrocTransport struct {
	priority int
	config   TransportConfig
}

// Setup initializes the simple croc transport
func (t *SimpleCrocTransport) Setup(config TransportConfig) error {
	t.config = config
	fmt.Printf("Simple CROC transport setup completed\n")
	return nil
}

// Send transmits data using the croc protocol
func (t *SimpleCrocTransport) Send(data []byte, metadata TransferMetadata) error {
	// Create temporary file for the data
	tempFile, err := ioutil.TempFile("", "croc_send_*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Write data to temp file
	if _, err := tempFile.Write(data); err != nil {
		return fmt.Errorf("failed to write to temp file: %w", err)
	}
	tempFile.Close()

	// Create CROC client for sending
	options := croc.Options{
		IsSender:       true,
		SharedSecret:   metadata.TransferID,
		RelayAddress:   "croc.schollz.com", // Use default relay
		RelayAddress6:  "croc6.schollz.com",
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

	client, err := croc.New(options)
	if err != nil {
		return fmt.Errorf("failed to create croc client: %w", err)
	}

	// Get file info and send
	filesInfo, emptyFolders, totalFolders, err := croc.GetFilesInfo([]string{tempFile.Name()}, false, false, []string{})
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	err = client.Send(filesInfo, emptyFolders, totalFolders)
	if err != nil {
		return fmt.Errorf("croc send failed: %w", err)
	}

	fmt.Printf("CROC send successful with transfer code: %s\n", metadata.TransferID)
	return nil
}

// Receive gets data using the croc protocol
func (t *SimpleCrocTransport) Receive(metadata TransferMetadata) ([]byte, error) {
	// Create temporary directory for receiving
	tempDir, err := ioutil.TempDir("", "croc_receive_")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Change to temp directory for receiving
	oldDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(tempDir); err != nil {
		return nil, fmt.Errorf("failed to change to temp directory: %w", err)
	}

	// Create CROC client for receiving
	options := croc.Options{
		IsSender:       false,
		SharedSecret:   metadata.TransferID,
		RelayAddress:   "croc.schollz.com", // Use default relay
		RelayAddress6:  "croc6.schollz.com",
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

	client, err := croc.New(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create croc client: %w", err)
	}

	// Receive files
	err = client.Receive()
	if err != nil {
		return nil, fmt.Errorf("croc receive failed: %w", err)
	}

	// Find the received file
	var receivedFile string
	err = filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && path != tempDir {
			receivedFile = path
			return filepath.SkipDir // Found a file, stop walking
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to find received file: %w", err)
	}

	if receivedFile == "" {
		return nil, fmt.Errorf("no file received")
	}

	// Read the received file
	data, err := ioutil.ReadFile(receivedFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read received file: %w", err)
	}

	fmt.Printf("CROC receive successful, got %d bytes\n", len(data))
	return data, nil
}

// IsAvailable checks if the croc transport is available
func (t *SimpleCrocTransport) IsAvailable(ctx context.Context) bool {
	// Simple CROC transport is always available if properly configured
	fmt.Printf("Simple CROC transport reporting as available\n")
	return true
}

// GetPriority returns the transport priority
func (t *SimpleCrocTransport) GetPriority() int {
	return t.priority
}

// GetName returns the transport name
func (t *SimpleCrocTransport) GetName() string {
	return "simple-croc"
}

// Close cleans up the transport
func (t *SimpleCrocTransport) Close() error {
	return nil
}

// NewCrocTransport creates a new simple croc transport with specified priority
func NewCrocTransport(priority int) *SimpleCrocTransport {
	transport := &SimpleCrocTransport{
		priority: priority,
	}

	// Setup with default config
	transport.Setup(TransportConfig{})

	return transport
}
