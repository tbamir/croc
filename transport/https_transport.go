package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HTTPSTunnelTransport provides file transfer through HTTPS tunneling
type HTTPSTunnelTransport struct {
	priority   int
	config     TransportConfig
	httpClient *http.Client
	serverURL  string
}

// Setup initializes the HTTPS tunnel transport
func (t *HTTPSTunnelTransport) Setup(config TransportConfig) error {
	t.config = config
	t.priority = 9

	// Use a relay server that accepts HTTPS connections
	t.serverURL = "https://transfer-relay.herokuapp.com" // Example relay

	// Create HTTP client with proper TLS configuration
	t.httpClient = &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
			DisableKeepAlives: false,
			MaxIdleConns:      10,
			IdleConnTimeout:   30 * time.Second,
		},
	}

	return nil
}

// Send transmits data through HTTPS tunnel
func (t *HTTPSTunnelTransport) Send(data []byte, metadata TransferMetadata) error {
	// Create transfer payload
	payload := HTTPSTransferPayload{
		TransferID: metadata.TransferID,
		FileName:   metadata.FileName,
		FileSize:   metadata.FileSize,
		Data:       data,
		Checksum:   metadata.Checksum,
		Timestamp:  time.Now(),
	}

	// Serialize payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to serialize payload: %w", err)
	}

	// Send via HTTPS POST
	resp, err := t.httpClient.Post(
		t.serverURL+"/upload",
		"application/json",
		bytes.NewReader(payloadBytes),
	)
	if err != nil {
		return fmt.Errorf("HTTPS upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("HTTPS upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Receive gets data through HTTPS tunnel
func (t *HTTPSTunnelTransport) Receive(metadata TransferMetadata) ([]byte, error) {
	// Poll for the file using the transfer ID
	maxRetries := 30 // 5 minutes with 10-second intervals

	for retry := 0; retry < maxRetries; retry++ {
		// Request file from server
		resp, err := t.httpClient.Get(
			fmt.Sprintf("%s/download/%s", t.serverURL, metadata.TransferID),
		)
		if err != nil {
			return nil, fmt.Errorf("HTTPS download failed: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			// File found, read response
			var payload HTTPSTransferPayload
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				return nil, fmt.Errorf("failed to decode HTTPS response: %w", err)
			}

			// Verify checksum
			if payload.Checksum != metadata.Checksum {
				return nil, fmt.Errorf("checksum mismatch in HTTPS transfer")
			}

			return payload.Data, nil
		} else if resp.StatusCode == http.StatusNotFound {
			// File not ready yet, wait and retry
			time.Sleep(10 * time.Second)
			continue
		} else {
			// Other error
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("HTTPS download failed with status %d: %s", resp.StatusCode, string(body))
		}
	}

	return nil, fmt.Errorf("HTTPS transfer timeout - file not found after %d retries", maxRetries)
}

// IsAvailable checks if HTTPS transport is available
func (t *HTTPSTunnelTransport) IsAvailable(ctx context.Context) bool {
	// Test connectivity to relay server
	req, err := http.NewRequestWithContext(ctx, "GET", t.serverURL+"/ping", nil)
	if err != nil {
		return false
	}

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// GetPriority returns the transport priority
func (t *HTTPSTunnelTransport) GetPriority() int {
	return t.priority
}

// GetName returns the transport name
func (t *HTTPSTunnelTransport) GetName() string {
	return "https-tunnel"
}

// Close cleans up the HTTPS transport
func (t *HTTPSTunnelTransport) Close() error {
	// Close HTTP client connections
	if transport, ok := t.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
	return nil
}

// HTTPSTransferPayload represents the data structure for HTTPS transfers
type HTTPSTransferPayload struct {
	TransferID string    `json:"transfer_id"`
	FileName   string    `json:"file_name"`
	FileSize   int64     `json:"file_size"`
	Data       []byte    `json:"data"`
	Checksum   string    `json:"checksum"`
	Timestamp  time.Time `json:"timestamp"`
}
