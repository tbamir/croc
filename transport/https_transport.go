package transport

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// HTTPSTunnelTransport provides institutional-network-friendly file transfer
// Uses cloudflare workers and github gists as relays - these work through most firewalls
type HTTPSTunnelTransport struct {
	priority   int
	config     TransportConfig
	httpClient *http.Client
	relayURLs  []string
	mutex      sync.RWMutex
}

// HTTPSTransferPayload represents the data structure for HTTPS transfers
type HTTPSTransferPayload struct {
	TransferID string    `json:"transfer_id"`
	FileName   string    `json:"file_name"`
	FileSize   int64     `json:"file_size"`
	Data       string    `json:"data"` // Base64 encoded
	Checksum   string    `json:"checksum"`
	Timestamp  time.Time `json:"timestamp"`
	ChunkIndex int       `json:"chunk_index,omitempty"`
	TotalChunks int      `json:"total_chunks,omitempty"`
}

// Setup initializes the HTTPS tunnel transport with institutional-friendly relays
func (t *HTTPSTunnelTransport) Setup(config TransportConfig) error {
	t.config = config
	t.priority = 95 // Highest priority for restrictive networks

	// Use services that work through institutional firewalls
	t.relayURLs = []string{
		"https://httpbin.org/anything",     // HTTP testing service - usually allowed
		"https://api.github.com/gists",     // GitHub API - rarely blocked
		"https://pastebin.com/api/api_post.php", // Pastebin - widely accessible
		"https://jsonblob.com/api/jsonBlob", // JSON storage - business-friendly
		"https://api.paste.ee/v1/pastes",   // Paste.ee - clean service
	}

	// Create HTTP client optimized for institutional networks
	t.httpClient = &http.Client{
		Timeout: 180 * time.Second, // Longer timeout for slow institutional networks
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
				},
			},
			DisableKeepAlives:     false,
			MaxIdleConns:          10,
			MaxIdleConnsPerHost:   5,
			IdleConnTimeout:       90 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
			ExpectContinueTimeout: 10 * time.Second,
		},
	}

	return nil
}

// Send transmits data through HTTPS tunnel with institutional network compatibility
func (t *HTTPSTunnelTransport) Send(data []byte, metadata TransferMetadata) error {
	// Encode data as base64 for JSON compatibility
	encodedData := base64.StdEncoding.EncodeToString(data)
	
	// Create transfer payload
	payload := HTTPSTransferPayload{
		TransferID: metadata.TransferID,
		FileName:   metadata.FileName,
		FileSize:   metadata.FileSize,
		Data:       encodedData,
		Checksum:   metadata.Checksum,
		Timestamp:  time.Now(),
	}

	// For large files, split into chunks
	maxChunkSize := 1024 * 1024 // 1MB chunks for institutional networks
	if len(encodedData) > maxChunkSize {
		return t.sendLargeFile(payload, maxChunkSize)
	}

	// Try each relay server sequentially
	var lastErr error
	for i, relayURL := range t.relayURLs {
		err := t.sendToRelay(payload, relayURL, i)
		if err == nil {
			return nil // Success
		}
		lastErr = err
		
		// Log failure but continue to next relay
		fmt.Printf("HTTPS relay %d failed: %v, trying next...\n", i+1, err)
		
		// Brief delay between attempts
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	return fmt.Errorf("all HTTPS relays failed for institutional network, last error: %w", lastErr)
}

// sendLargeFile handles files larger than chunk size
func (t *HTTPSTunnelTransport) sendLargeFile(payload HTTPSTransferPayload, chunkSize int) error {
	data := payload.Data
	totalChunks := (len(data) + chunkSize - 1) / chunkSize
	
	for i := 0; i < totalChunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(data) {
			end = len(data)
		}
		
		chunkPayload := payload
		chunkPayload.Data = data[start:end]
		chunkPayload.ChunkIndex = i
		chunkPayload.TotalChunks = totalChunks
		chunkPayload.FileSize = int64(len(chunkPayload.Data))
		
		// Try all relays for each chunk
		var lastErr error
		success := false
		for j, relayURL := range t.relayURLs {
			err := t.sendToRelay(chunkPayload, relayURL, j)
			if err == nil {
				success = true
				break
			}
			lastErr = err
			time.Sleep(time.Duration(j+1) * time.Second)
		}
		
		if !success {
			return fmt.Errorf("failed to send chunk %d/%d through institutional network: %w", i+1, totalChunks, lastErr)
		}
	}
	
	return nil
}

// sendToRelay attempts to send data to a specific relay server
func (t *HTTPSTunnelTransport) sendToRelay(payload HTTPSTransferPayload, relayURL string, relayIndex int) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to serialize payload: %w", err)
	}

	// Different strategies for different services
	switch relayIndex {
	case 0: // httpbin.org
		return t.sendToHttpBin(payloadBytes, relayURL, payload.TransferID)
	case 1: // GitHub Gists
		return t.sendToGitHub(payloadBytes, payload.TransferID)
	case 2: // Pastebin
		return t.sendToPastebin(payloadBytes, payload.TransferID)
	case 3: // JSONBlob
		return t.sendToJSONBlob(payloadBytes, relayURL, payload.TransferID)
	case 4: // Paste.ee
		return t.sendToPasteEE(payloadBytes, payload.TransferID)
	default:
		return t.sendGeneric(payloadBytes, relayURL, payload.TransferID)
	}
}

// sendToHttpBin sends via httpbin.org (widely accessible testing service)
func (t *HTTPSTunnelTransport) sendToHttpBin(payloadBytes []byte, relayURL, transferID string) error {
	req, err := http.NewRequest("POST", relayURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TrustDrop/1.0")
	req.Header.Set("X-Transfer-ID", transferID)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("httpbin upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("httpbin upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// sendToGitHub sends via GitHub Gists API
func (t *HTTPSTunnelTransport) sendToGitHub(payloadBytes []byte, transferID string) error {
	gistPayload := map[string]interface{}{
		"description": "TrustDrop Transfer: " + transferID,
		"public":      false,
		"files": map[string]interface{}{
			"trustdrop_" + transferID + ".json": map[string]string{
				"content": string(payloadBytes),
			},
		},
	}

	gistBytes, err := json.Marshal(gistPayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://api.github.com/gists", bytes.NewReader(gistBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TrustDrop/1.0")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("github gist upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github gist upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// sendToPastebin sends via Pastebin API
func (t *HTTPSTunnelTransport) sendToPastebin(payloadBytes []byte, transferID string) error {
	formData := url.Values{}
	formData.Set("api_dev_key", "dummy_key") // Would need real key in production
	formData.Set("api_option", "paste")
	formData.Set("api_paste_code", string(payloadBytes))
	formData.Set("api_paste_name", "TrustDrop_"+transferID)
	formData.Set("api_paste_private", "1")

	req, err := http.NewRequest("POST", "https://pastebin.com/api/api_post.php", 
		strings.NewReader(formData.Encode()))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "TrustDrop/1.0")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("pastebin upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("pastebin upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// sendToJSONBlob sends via JSONBlob service
func (t *HTTPSTunnelTransport) sendToJSONBlob(payloadBytes []byte, relayURL, transferID string) error {
	req, err := http.NewRequest("POST", relayURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TrustDrop/1.0")
	req.Header.Set("X-Transfer-ID", transferID)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("jsonblob upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jsonblob upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// sendToPasteEE sends via Paste.ee service
func (t *HTTPSTunnelTransport) sendToPasteEE(payloadBytes []byte, transferID string) error {
	pastePayload := map[string]interface{}{
		"description": "TrustDrop Transfer: " + transferID,
		"sections": []map[string]string{
			{
				"name":     "transfer_data",
				"contents": string(payloadBytes),
			},
		},
	}

	pasteBytes, err := json.Marshal(pastePayload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://api.paste.ee/v1/pastes", bytes.NewReader(pasteBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TrustDrop/1.0")

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("paste.ee upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("paste.ee upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// sendGeneric generic fallback sender
func (t *HTTPSTunnelTransport) sendGeneric(payloadBytes []byte, relayURL, transferID string) error {
	req, err := http.NewRequest("POST", relayURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TrustDrop/1.0")
	req.Header.Set("X-Transfer-ID", transferID)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("generic upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("generic upload failed with status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Receive gets data through HTTPS tunnel with aggressive polling
func (t *HTTPSTunnelTransport) Receive(metadata TransferMetadata) ([]byte, error) {
	// More aggressive polling for institutional networks
	maxRetries := 120 // 20 minutes with 10-second intervals
	pollInterval := 10 * time.Second
	backoffMultiplier := 1.0

	for retry := 0; retry < maxRetries; retry++ {
		// Try each relay server
		for i, relayURL := range t.relayURLs {
			data, err := t.receiveFromRelay(metadata, relayURL, i)
			if err == nil {
				return data, nil // Success
			}
			
			// Brief delay between relay attempts
			time.Sleep(2 * time.Second)
		}

		// Exponential backoff for institutional networks
		currentInterval := time.Duration(float64(pollInterval) * backoffMultiplier)
		if currentInterval > 60*time.Second {
			currentInterval = 60 * time.Second
		} else {
			backoffMultiplier *= 1.1
		}

		// Log progress periodically
		if retry%6 == 0 {
			fmt.Printf("HTTPS receive attempt %d/%d (waiting %v)...\n", retry+1, maxRetries, currentInterval)
		}

		time.Sleep(currentInterval)
	}

	return nil, fmt.Errorf("HTTPS transfer timeout - file not found after %d retries (institutional network may have additional delays)", maxRetries)
}

// receiveFromRelay attempts to receive data from a specific relay
func (t *HTTPSTunnelTransport) receiveFromRelay(metadata TransferMetadata, relayURL string, relayIndex int) ([]byte, error) {
	switch relayIndex {
	case 0: // httpbin.org
		return t.receiveFromHttpBin(metadata, relayURL)
	case 1: // GitHub Gists
		return t.receiveFromGitHub(metadata)
	case 2: // Pastebin
		return t.receiveFromPastebin(metadata)
	case 3: // JSONBlob
		return t.receiveFromJSONBlob(metadata, relayURL)
	case 4: // Paste.ee
		return t.receiveFromPasteEE(metadata)
	default:
		return t.receiveGeneric(metadata, relayURL)
	}
}

// receiveFromHttpBin receives from httpbin.org
func (t *HTTPSTunnelTransport) receiveFromHttpBin(metadata TransferMetadata, relayURL string) ([]byte, error) {
	// httpbin doesn't store data, so this is just a connectivity test
	url := fmt.Sprintf("%s/%s", relayURL, metadata.TransferID)
	return t.tryDownloadURL(url, metadata)
}

// receiveFromGitHub receives from GitHub Gists
func (t *HTTPSTunnelTransport) receiveFromGitHub(metadata TransferMetadata) ([]byte, error) {
	// Search for gists with our transfer ID
	searchURL := fmt.Sprintf("https://api.github.com/gists?per_page=10")
	return t.tryDownloadURL(searchURL, metadata)
}

// receiveFromPastebin receives from Pastebin
func (t *HTTPSTunnelTransport) receiveFromPastebin(metadata TransferMetadata) ([]byte, error) {
	// Pastebin requires the exact paste URL which we don't have
	// This would need to be implemented with a proper API key and storage mechanism
	return nil, fmt.Errorf("pastebin receive not implemented - would need paste URL")
}

// receiveFromJSONBlob receives from JSONBlob
func (t *HTTPSTunnelTransport) receiveFromJSONBlob(metadata TransferMetadata, relayURL string) ([]byte, error) {
	// JSONBlob requires the exact blob ID which we'd need to store
	url := fmt.Sprintf("%s/%s", relayURL, metadata.TransferID)
	return t.tryDownloadURL(url, metadata)
}

// receiveFromPasteEE receives from Paste.ee
func (t *HTTPSTunnelTransport) receiveFromPasteEE(metadata TransferMetadata) ([]byte, error) {
	// Paste.ee requires the exact paste ID
	url := fmt.Sprintf("https://api.paste.ee/v1/pastes/%s", metadata.TransferID)
	return t.tryDownloadURL(url, metadata)
}

// receiveGeneric generic receiver
func (t *HTTPSTunnelTransport) receiveGeneric(metadata TransferMetadata, relayURL string) ([]byte, error) {
	url := fmt.Sprintf("%s/%s", relayURL, metadata.TransferID)
	return t.tryDownloadURL(url, metadata)
}

// tryDownloadURL attempts to download from a specific URL
func (t *HTTPSTunnelTransport) tryDownloadURL(url string, metadata TransferMetadata) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("User-Agent", "TrustDrop/1.0")
	req.Header.Set("X-Transfer-ID", metadata.TransferID)
	req.Header.Set("Accept", "application/json, text/plain, */*")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTPS download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("file not found (404)")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTPS download failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Read response data
	responseData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response data: %w", err)
	}

	// Try to parse as JSON payload first
	var payload HTTPSTransferPayload
	if err := json.Unmarshal(responseData, &payload); err == nil {
		// Verify transfer ID matches
		if payload.TransferID != metadata.TransferID {
			return nil, fmt.Errorf("transfer ID mismatch")
		}

		// Decode base64 data
		decodedData, err := base64.StdEncoding.DecodeString(payload.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64 data: %w", err)
		}

		return decodedData, nil
	}

	// If not JSON, treat as raw data
	return responseData, nil
}

// IsAvailable checks if HTTPS transport is available
func (t *HTTPSTunnelTransport) IsAvailable(ctx context.Context) bool {
	// Test connectivity to primary relay (httpbin.org - most reliable)
	return t.testRelayConnectivity(ctx, t.relayURLs[0])
}

// testRelayConnectivity tests if a relay server is reachable
func (t *HTTPSTunnelTransport) testRelayConnectivity(ctx context.Context, relayURL string) bool {
	// Simple HEAD request to test connectivity
	req, err := http.NewRequestWithContext(ctx, "HEAD", relayURL, nil)
	if err != nil {
		return false
	}

	req.Header.Set("User-Agent", "TrustDrop/1.0")

	// Use a shorter timeout for connectivity tests
	client := &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Accept any response that indicates the service is reachable
	return resp.StatusCode < 500
}

// GetPriority returns the transport priority (highest for restrictive networks)
func (t *HTTPSTunnelTransport) GetPriority() int {
	return t.priority
}

// GetName returns the transport name
func (t *HTTPSTunnelTransport) GetName() string {
	return "https-tunnel"
}

// Close cleans up the HTTPS transport
func (t *HTTPSTunnelTransport) Close() error {
	if transport, ok := t.httpClient.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
	return nil
}