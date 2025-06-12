package transport

import (
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/schollz/croc/v10/src/croc"
)

// EnhancedCrocTransport provides an improved version of the croc protocol with better error handling
type EnhancedCrocTransport struct {
	priority int
	config   TransportConfig
	relays   []string
	client   *croc.Client
}

// Setup initializes the enhanced croc transport
func (t *EnhancedCrocTransport) Setup(config TransportConfig) error {
	t.config = config

	// CORPORATE NETWORK FRIENDLY: Prioritize standard web ports
	t.relays = []string{
		// Direct IP addresses to avoid DNS issues in corporate networks
		"165.232.162.250:443",  // HTTPS port - most likely to work
		"165.232.162.250:80",   // HTTP port - second most likely
		"165.232.162.250:9009", // Standard CROC port
		"165.232.162.250:9010", // Alternative CROC port

		// Fallback to domain names (may fail in corporate networks)
		"croc.schollz.com:443",
		"croc2.schollz.com:443",
		"croc.schollz.com:80",
		"croc2.schollz.com:80",
		"croc.schollz.com:9009",
		"croc2.schollz.com:9009",
		"croc3.schollz.com:9009",
		"croc4.schollz.com:9009",
		"croc5.schollz.com:9009",
	}

	if len(config.RelayServers) > 0 {
		// Prepend configured relays but keep corporate-friendly ones as fallback
		corporateFriendlyRelays := t.relays[:4] // Keep the IP-based ones
		t.relays = append(config.RelayServers, corporateFriendlyRelays...)
	}

	return nil
}

// Send transmits data using the croc protocol with enhanced reliability
func (t *EnhancedCrocTransport) Send(data []byte, metadata TransferMetadata) error {
	// Create temporary file for the data
	tempFile, err := t.createTempFile(data, metadata.FileName)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer t.cleanupTempFile(tempFile)

	// Try each relay with progressive timeouts
	timeouts := []time.Duration{30 * time.Second, 2 * time.Minute, 5 * time.Minute}

	for _, timeout := range timeouts {
		for _, relay := range t.relays {
			if err := t.attemptSendWithRelay(tempFile, metadata.TransferID, relay, timeout); err == nil {
				return nil
			}
		}
	}

	return fmt.Errorf("all croc relay attempts failed")
}

// Receive gets data using the croc protocol
func (t *EnhancedCrocTransport) Receive(metadata TransferMetadata) ([]byte, error) {
	// Create temporary directory for receiving
	tempDir, err := ioutil.TempDir("", "croc_receive_")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Try each relay with progressive timeouts
	timeouts := []time.Duration{30 * time.Second, 2 * time.Minute, 5 * time.Minute}

	var receivedFilename string
	for _, timeout := range timeouts {
		for _, relay := range t.relays {
			if err := t.attemptReceiveWithRelay(metadata.TransferID, tempDir, relay, timeout); err == nil {
				// Get the received filename and data
				data, filename, err := t.readReceivedFileWithName(tempDir)
				if err == nil {
					receivedFilename = filename
					// Update the metadata with the actual received filename
					metadata.FileName = receivedFilename
					return data, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("all croc relay attempts failed")
}

// IsAvailable checks if the croc transport is available
func (t *EnhancedCrocTransport) IsAvailable(ctx context.Context) bool {
	// Test connectivity to at least one relay server
	// Safely determine how many relays to test (max 3, but don't exceed actual length)
	numRelaysToTest := len(t.relays)
	if numRelaysToTest > 3 {
		numRelaysToTest = 3
	}

	for i := 0; i < numRelaysToTest; i++ {
		if t.testRelayConnectivity(ctx, t.relays[i]) {
			return true
		}
	}
	return false
}

// GetPriority returns the transport priority
func (t *EnhancedCrocTransport) GetPriority() int {
	return t.priority
}

// GetName returns the transport name
func (t *EnhancedCrocTransport) GetName() string {
	return "enhanced-croc"
}

// Close cleans up the transport
func (t *EnhancedCrocTransport) Close() error {
	return nil
}

// Helper methods

func (t *EnhancedCrocTransport) createTempFile(data []byte, filename string) (string, error) {
	// Create a temporary file with the data
	tempFile, err := ioutil.TempFile("", "croc_send_*.tmp")
	if err != nil {
		return "", err
	}
	defer tempFile.Close()

	// Write data to temp file
	if _, err := tempFile.Write(data); err != nil {
		os.Remove(tempFile.Name())
		return "", err
	}

	return tempFile.Name(), nil
}

func (t *EnhancedCrocTransport) cleanupTempFile(filepath string) {
	os.Remove(filepath)
}

func (t *EnhancedCrocTransport) attemptSendWithRelay(filepath, transferCode, relay string, timeout time.Duration) error {
	// Extract host and port from relay
	relayHost, relayPort, err := net.SplitHostPort(relay)
	if err != nil {
		relayHost = relay
		relayPort = "9009"
	}

	// Configure croc options (using the correct API)
	options := croc.Options{
		IsSender:       true,
		SharedSecret:   transferCode,
		RelayAddress:   relayHost,
		RelayAddress6:  relayHost,
		RelayPorts:     []string{relayPort, "9009", "9010", "9011", "9012", "9013", "443", "80"},
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

	// Create croc client
	client, err := croc.New(options)
	if err != nil {
		return fmt.Errorf("failed to create croc client: %w", err)
	}

	// Store client for potential cleanup
	t.client = client

	// Get file info for the temporary file using the croc library's method
	filesInfo, emptyFolders, totalFolders, err := croc.GetFilesInfo([]string{filepath}, false, false, []string{})
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Use the correct croc library Send method
	err = client.Send(filesInfo, emptyFolders, totalFolders)
	if err != nil {
		return fmt.Errorf("croc send failed: %w", err)
	}

	return nil
}

func (t *EnhancedCrocTransport) attemptReceiveWithRelay(transferCode, targetDir, relay string, timeout time.Duration) error {
	// Extract host and port from relay
	relayHost, relayPort, err := net.SplitHostPort(relay)
	if err != nil {
		relayHost = relay
		relayPort = "9009"
	}

	// Configure croc options for receiving
	options := croc.Options{
		IsSender:       false,
		SharedSecret:   transferCode,
		RelayAddress:   relayHost,
		RelayAddress6:  relayHost,
		RelayPorts:     []string{relayPort, "9009", "9010", "9011", "9012", "9013", "443", "80"},
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

	// Create croc client for receiving
	client, err := croc.New(options)
	if err != nil {
		return fmt.Errorf("failed to create croc client: %w", err)
	}

	// Store client for potential cleanup
	t.client = client

	// Change to target directory for receiving
	oldDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current directory: %w", err)
	}
	defer os.Chdir(oldDir)

	if err := os.Chdir(targetDir); err != nil {
		return fmt.Errorf("failed to change to target directory: %w", err)
	}

	// Use the correct croc library Receive method
	err = client.Receive()
	if err != nil {
		return fmt.Errorf("croc receive failed: %w", err)
	}

	return nil
}

func (t *EnhancedCrocTransport) readReceivedFile(tempDir, expectedFileName string) ([]byte, error) {
	// Try to find the received file
	var receivedFile string

	err := filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
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

	// Read the file
	data, err := ioutil.ReadFile(receivedFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read received file: %w", err)
	}

	return data, nil
}

func (t *EnhancedCrocTransport) readReceivedFileWithName(tempDir string) ([]byte, string, error) {
	// Try to find the received file
	var receivedFile string
	var receivedFilename string

	err := filepath.Walk(tempDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			receivedFile = path
			receivedFilename = info.Name()
			return filepath.SkipDir // Found a file, stop walking
		}
		return nil
	})

	if err != nil {
		return nil, "", fmt.Errorf("failed to find received file: %w", err)
	}

	if receivedFile == "" {
		return nil, "", fmt.Errorf("no file received")
	}

	// Read the file
	data, err := ioutil.ReadFile(receivedFile)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read received file: %w", err)
	}

	return data, receivedFilename, nil
}

func (t *EnhancedCrocTransport) testRelayConnectivity(ctx context.Context, relay string) bool {
	// Test if we can connect to the relay server
	host, port, err := net.SplitHostPort(relay)
	if err != nil {
		return false
	}

	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, port), 3*time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// NewCrocTransport creates a new croc transport with specified priority
func NewCrocTransport(priority int) *EnhancedCrocTransport {
	transport := &EnhancedCrocTransport{
		priority: priority,
	}

	// Setup with default config
	transport.Setup(TransportConfig{})

	return transport
}
