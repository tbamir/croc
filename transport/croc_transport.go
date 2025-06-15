package transport

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/schollz/croc/v10/src/croc"
)

// SimpleCrocTransport implements the Transport interface using the croc library
type SimpleCrocTransport struct {
	priority int
	config   TransportConfig
	options  croc.Options
}

// Setup configures the croc transport
func (t *SimpleCrocTransport) Setup(config TransportConfig) error {
	t.config = config

	// Configure for international lab-to-lab transfers with corporate firewall compatibility
	t.options = croc.Options{
		RelayAddress:  "croc.schollz.com", // Only use the working relay server
		RelayAddress6: "",                 // Disable IPv6 for corporate compatibility

		// CORPORATE FIREWALL-COMPATIBLE port progression
		RelayPorts: []string{
			"443",                  // HTTPS - highest success rate in corporate networks
			"80",                   // HTTP - second highest success rate
			"8080",                 // Alternative HTTP - common corporate allowlist
			"8443",                 // Alternative HTTPS - backup option
			"9009", "9010", "9011", // CROC standard ports
		},

		RelayPassword:  "pass123",
		NoPrompt:       true,
		NoMultiplexing: false, // Allow multiplexing for better performance
		DisableLocal:   true,  // FORCE relay usage for international transfers
		Ask:            false,
		Debug:          false,
		Overwrite:      true,
		Curve:          "p256",
		HashAlgorithm:  "xxhash",
	}

	fmt.Printf("International CROC transport setup completed\n")
	return nil
}

// Send transmits data using the croc protocol with international relay optimization
func (t *SimpleCrocTransport) Send(data []byte, metadata TransferMetadata) error {
	// Create temporary file for sending
	tempFile, err := os.CreateTemp("", "croc_send_*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Check file size limit (100MB for production stability)
	maxSize := int64(100 * 1024 * 1024) // 100MB limit
	if int64(len(data)) > maxSize {
		return fmt.Errorf("file too large for CROC transport (%d bytes, max %d)", len(data), maxSize)
	}

	// Write data to temp file
	if _, err := tempFile.Write(data); err != nil {
		return fmt.Errorf("failed to write data to temp file: %w", err)
	}
	tempFile.Close()

	// Create coordination file to signal readiness
	if err := t.createCoordinationFile(metadata.TransferID); err != nil {
		fmt.Printf("Warning: Could not create CROC coordination file: %v\n", err)
	}

	// Configure CROC client for sending
	t.options.IsSender = true
	t.options.SharedSecret = metadata.TransferID

	// International relay strategy with timeout management
	relayGroups := []struct {
		name    string
		servers []string
		timeout time.Duration
	}{
		{
			name:    "Primary Global Relays",
			servers: []string{"croc.schollz.com"}, // Only use working relay
			timeout: 30 * time.Second,             // Increased timeout for international
		},
	}

	var lastError error
	for groupIndex, group := range relayGroups {
		fmt.Printf("🌍 Trying %s (Group %d/%d)\n", group.name, groupIndex+1, len(relayGroups))

		for relayIndex, relayServer := range group.servers {
			fmt.Printf("🔄 Attempting relay %d/%d: %s\n", relayIndex+1, len(group.servers), relayServer)

			// Update relay configuration
			t.options.RelayAddress = relayServer

			// Create CROC client with timeout context
			ctx, cancel := context.WithTimeout(context.Background(), group.timeout)
			defer cancel()

			client, err := croc.New(t.options)
			if err != nil {
				lastError = fmt.Errorf("failed to create CROC client for relay %s: %w", relayServer, err)
				continue
			}

			// Test relay connectivity first
			if !t.testRelayConnectivity(ctx, relayServer, t.options.RelayPorts[0]) {
				lastError = fmt.Errorf("relay %s connectivity test failed", relayServer)
				continue
			}

			// Get file info and attempt send with timeout
			filesInfo, emptyFolders, totalFolders, err := croc.GetFilesInfo([]string{tempFile.Name()}, false, false, []string{})
			if err != nil {
				lastError = fmt.Errorf("failed to get file info for relay %s: %w", relayServer, err)
				continue
			}

			fmt.Printf("🚀 Initiating international CROC send via %s (timeout: %v)...\n", relayServer, group.timeout)

			// Send with context timeout
			sendErr := make(chan error, 1)
			go func() {
				sendErr <- client.Send(filesInfo, emptyFolders, totalFolders)
			}()

			select {
			case err = <-sendErr:
				if err == nil {
					fmt.Printf("✅ International CROC transfer successful via %s! Transfer code: %s\n", relayServer, metadata.TransferID)
					return nil
				}
				lastError = err

			case <-ctx.Done():
				lastError = fmt.Errorf("timeout sending via relay %s after %v", relayServer, group.timeout)
			}

			fmt.Printf("❌ Relay %s failed: %v\n", relayServer, lastError)
		}
	}

	// All relay groups failed
	return fmt.Errorf("all international CROC relay strategies failed, last error: %w", lastError)
}

// createCoordinationFile creates a file to signal CROC sender readiness
func (t *SimpleCrocTransport) createCoordinationFile(transferID string) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	coordDir := filepath.Join(homeDir, ".trustdrop")
	if err := os.MkdirAll(coordDir, 0755); err != nil {
		return err
	}

	// Create CROC coordination info
	coordInfo := fmt.Sprintf("croc_ready\ntransfer_id:%s\nrelay:croc.schollz.com\n", transferID)

	coordFile := filepath.Join(coordDir, fmt.Sprintf("croc_%s.coord", transferID))
	return os.WriteFile(coordFile, []byte(coordInfo), 0644)
}

// testRelayConnectivity tests if relay is reachable before attempting transfer
func (t *SimpleCrocTransport) testRelayConnectivity(ctx context.Context, relayServer, port string) bool {
	testTimeout := 5 * time.Second
	testCtx, cancel := context.WithTimeout(ctx, testTimeout)
	defer cancel()

	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(testCtx, "tcp", net.JoinHostPort(relayServer, port))
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Receive gets data using the croc protocol
func (t *SimpleCrocTransport) Receive(metadata TransferMetadata) ([]byte, error) {
	// Wait for sender coordination file (CROC sender ready signal)
	if err := t.waitForSenderReady(metadata.TransferID, 45*time.Second); err != nil {
		fmt.Printf("⏰ CROC sender not ready yet, proceeding anyway: %v\n", err)
	}

	// Create temporary directory for receiving
	tempDir, err := os.MkdirTemp("", "croc_receive_")
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

	// Try multiple relay servers for maximum firewall compatibility
	relayServers := []string{
		"croc.schollz.com", // Only use the main working relay server
	}

	var lastError error
	for i, relayServer := range relayServers {
		fmt.Printf("🔄 Attempting CROC receive from relay %d/%d: %s\n", i+1, len(relayServers), relayServer)

		options := croc.Options{
			IsSender:     false,
			SharedSecret: metadata.TransferID,

			// LAB-OPTIMIZED RELAY CONFIGURATION with multiple fallbacks
			RelayAddress:  relayServer,
			RelayAddress6: "", // Disable IPv6 for corporate compatibility

			// CORPORATE FIREWALL-COMPATIBLE port progression
			RelayPorts: []string{
				"443",                  // HTTPS - highest success rate in corporate networks
				"80",                   // HTTP - second highest success rate
				"8080",                 // Alternative HTTP - common corporate allowlist
				"8443",                 // Alternative HTTPS - backup option
				"9009", "9010", "9011", // CROC standard ports
			},

			RelayPassword:  "pass123",
			NoPrompt:       true,
			NoMultiplexing: false, // Allow multiplexing for better performance
			DisableLocal:   true,  // FORCE relay usage for international transfers
			Ask:            false,
			Debug:          false,
			Overwrite:      true,
			Curve:          "p256",
			HashAlgorithm:  "xxhash",
		}

		client, err := croc.New(options)
		if err != nil {
			lastError = fmt.Errorf("failed to create CROC client for relay %s: %w", relayServer, err)
			continue
		}

		fmt.Printf("📡 Connecting to lab relay server: %s...\n", relayServer)

		// Add timeout for receive operation
		receiveErr := make(chan error, 1)
		go func() {
			receiveErr <- client.Receive()
		}()

		// Wait for receive with timeout
		select {
		case err = <-receiveErr:
			if err == nil {
				fmt.Printf("✅ CROC lab receive successful from %s! Got file data\n", relayServer)
				break
			}
			fmt.Printf("❌ Relay %s failed: %v\n", relayServer, err)
			lastError = err

		case <-time.After(60 * time.Second): // Extended timeout for international
			lastError = fmt.Errorf("timeout receiving from relay %s after 60s", relayServer)
			fmt.Printf("❌ Relay %s timed out after 60s\n", relayServer)
		}
	}

	if lastError != nil {
		return nil, fmt.Errorf("all CROC relay servers failed for lab receive, last error: %w", lastError)
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
		return nil, fmt.Errorf("no file received via CROC international transfer")
	}

	// Read the received file
	data, err := os.ReadFile(receivedFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read received file: %w", err)
	}

	fmt.Printf("✅ CROC international receive successful! Got %d bytes\n", len(data))
	return data, nil
}

// waitForSenderReady waits for the CROC coordination file to appear
func (t *SimpleCrocTransport) waitForSenderReady(transferID string, timeout time.Duration) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	coordFile := filepath.Join(homeDir, ".trustdrop", fmt.Sprintf("croc_%s.coord", transferID))

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(coordFile); err == nil {
			fmt.Printf("📡 CROC sender ready signal detected\n")
			return nil
		}
		time.Sleep(2 * time.Second) // Check every 2 seconds
	}

	return fmt.Errorf("CROC sender ready signal not found within %v", timeout)
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
