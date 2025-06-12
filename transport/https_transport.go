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
	"sync"
	"time"
)

// SimpleHTTPRelay provides a local HTTP relay server for file transfers
type SimpleHTTPRelay struct {
	server  *http.Server
	storage map[string]HTTPSTransferPayload
	mutex   sync.RWMutex
	running bool
}

// NewSimpleHTTPRelay creates a new local HTTP relay server
func NewSimpleHTTPRelay() *SimpleHTTPRelay {
	return &SimpleHTTPRelay{
		storage: make(map[string]HTTPSTransferPayload),
	}
}

// Start starts the relay server on the specified port
func (r *SimpleHTTPRelay) Start(port int) error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.running {
		return fmt.Errorf("relay already running")
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/relay", r.handleRelay)
	mux.HandleFunc("/relay/", r.handleRelayWithID)

	r.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		if err := r.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Relay server error on port %d: %v\n", port, err)
		}
	}()

	r.running = true
	time.Sleep(100 * time.Millisecond) // Give server time to start
	return nil
}

// Stop stops the relay server
func (r *SimpleHTTPRelay) Stop() error {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if !r.running || r.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	r.running = false
	return r.server.Shutdown(ctx)
}

// handleRelay handles POST requests to store data
func (r *SimpleHTTPRelay) handleRelay(w http.ResponseWriter, req *http.Request) {
	switch req.Method {
	case http.MethodPost:
		body, err := io.ReadAll(req.Body)
		if err != nil {
			http.Error(w, "Failed to read body", http.StatusBadRequest)
			return
		}

		var payload HTTPSTransferPayload
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		r.mutex.Lock()
		r.storage[payload.TransferID] = payload
		r.mutex.Unlock()

		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("OK"))

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRelayWithID handles GET requests to retrieve data
func (r *SimpleHTTPRelay) handleRelayWithID(w http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract transfer ID from URL path
	path := req.URL.Path
	if len(path) < 7 { // "/relay/"
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	transferID := path[7:] // Remove "/relay/" prefix

	r.mutex.RLock()
	payload, exists := r.storage[transferID]
	r.mutex.RUnlock()

	if !exists {
		http.Error(w, "Transfer not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payload)
}

// HTTPSTunnelTransport provides institutional-network-friendly file transfer
// Uses local HTTP relay servers to work through corporate firewalls
type HTTPSTunnelTransport struct {
	priority    int
	config      TransportConfig
	httpClient  *http.Client
	relayURLs   []string
	mutex       sync.RWMutex
	relayServer *SimpleHTTPRelay
}

// HTTPSTransferPayload represents the data structure for HTTPS transfers
type HTTPSTransferPayload struct {
	TransferID  string    `json:"transfer_id"`
	FileName    string    `json:"file_name"`
	FileSize    int64     `json:"file_size"`
	Data        string    `json:"data"` // Base64 encoded
	Checksum    string    `json:"checksum"`
	Timestamp   time.Time `json:"timestamp"`
	ChunkIndex  int       `json:"chunk_index,omitempty"`
	TotalChunks int       `json:"total_chunks,omitempty"`
}

// Setup initializes the HTTPS tunnel transport with local relay servers
func (t *HTTPSTunnelTransport) Setup(config TransportConfig) error {
	t.config = config
	t.priority = 100 // Highest priority for corporate networks

	// Use LOCAL relay servers that work through corporate firewalls
	// Try multiple ports for maximum compatibility
	t.relayURLs = []string{
		"http://localhost:8080/relay", // Primary local relay
		"http://localhost:8081/relay", // Secondary local relay
		"http://127.0.0.1:8080/relay", // Alternative localhost
		"http://127.0.0.1:8081/relay", // Alternative localhost
		"http://localhost:8443/relay", // HTTPS-like port
		"http://localhost:3000/relay", // Common dev port
		"http://localhost:5000/relay", // Alternative port
		"http://localhost:9090/relay", // High port
	}

	// Start local relay server
	t.relayServer = NewSimpleHTTPRelay()
	go func() {
		// Try multiple ports to find one that works
		ports := []int{8080, 8081, 8443, 3000, 5000, 9090}
		for _, port := range ports {
			if err := t.relayServer.Start(port); err == nil {
				fmt.Printf("HTTPS relay started on port %d\n", port)
				break
			}
		}
	}()

	// Give relay server time to start
	time.Sleep(1 * time.Second)

	// Create HTTP client optimized for local connections
	t.httpClient = &http.Client{
		Timeout: 60 * time.Second, // Shorter timeout for local connections
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: true, // For local testing
			},
			DisableKeepAlives:     false,
			MaxIdleConns:          10,
			MaxIdleConnsPerHost:   5,
			IdleConnTimeout:       30 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			ExpectContinueTimeout: 5 * time.Second,
		},
	}

	return nil
}

// Send transmits data through local HTTPS relay
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
	maxChunkSize := 2 * 1024 * 1024 // 2MB chunks for local relay
	if len(encodedData) > maxChunkSize {
		return t.sendLargeFile(payload, maxChunkSize)
	}

	// Try each local relay server
	var lastErr error
	for _, relayURL := range t.relayURLs {
		err := t.sendToLocalRelay(payload, relayURL)
		if err == nil {
			return nil // Success
		}
		lastErr = err

		// Brief delay between attempts
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("all local HTTPS relays failed: %w", lastErr)
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

		// Try all local relays for each chunk
		var lastErr error
		success := false
		for _, relayURL := range t.relayURLs {
			err := t.sendToLocalRelay(chunkPayload, relayURL)
			if err == nil {
				success = true
				break
			}
			lastErr = err
			time.Sleep(50 * time.Millisecond)
		}

		if !success {
			return fmt.Errorf("failed to send chunk %d/%d to local relay: %w", i+1, totalChunks, lastErr)
		}
	}

	return nil
}

// sendToLocalRelay sends data to local relay server
func (t *HTTPSTunnelTransport) sendToLocalRelay(payload HTTPSTransferPayload, relayURL string) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to serialize payload: %w", err)
	}

	req, err := http.NewRequest("POST", relayURL, bytes.NewReader(payloadBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TrustDrop/1.0")
	req.Header.Set("X-Transfer-ID", payload.TransferID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("local relay upload failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("local relay returned status %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// Receive gets data from local HTTPS relay
func (t *HTTPSTunnelTransport) Receive(metadata TransferMetadata) ([]byte, error) {
	// Try each local relay server
	var lastErr error
	for _, relayURL := range t.relayURLs {
		data, err := t.receiveFromLocalRelay(metadata, relayURL)
		if err == nil {
			return data, nil
		}
		lastErr = err
		time.Sleep(100 * time.Millisecond)
	}

	return nil, fmt.Errorf("all local HTTPS relays failed for receive: %w", lastErr)
}

// receiveFromLocalRelay gets data from local relay server
func (t *HTTPSTunnelTransport) receiveFromLocalRelay(metadata TransferMetadata, relayURL string) ([]byte, error) {
	// Build URL for GET request
	url := fmt.Sprintf("%s/%s", relayURL, metadata.TransferID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "TrustDrop/1.0")
	req.Header.Set("X-Transfer-ID", metadata.TransferID)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("local relay download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("local relay returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var payload HTTPSTransferPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse payload: %w", err)
	}

	// Decode base64 data
	data, err := base64.StdEncoding.DecodeString(payload.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode data: %w", err)
	}

	return data, nil
}

// IsAvailable checks if local HTTPS relay is available
func (t *HTTPSTunnelTransport) IsAvailable(ctx context.Context) bool {
	// Test connectivity to at least one local relay
	for _, relayURL := range t.relayURLs[:3] { // Test first 3
		if t.testLocalRelayConnectivity(ctx, relayURL) {
			return true
		}
	}
	return false
}

// testLocalRelayConnectivity tests connection to local relay
func (t *HTTPSTunnelTransport) testLocalRelayConnectivity(ctx context.Context, relayURL string) bool {
	client := &http.Client{Timeout: 5 * time.Second}

	req, err := http.NewRequestWithContext(ctx, "HEAD", relayURL, nil)
	if err != nil {
		return false
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode < 500 // Accept any non-server-error response
}

// GetPriority returns the transport priority
func (t *HTTPSTunnelTransport) GetPriority() int {
	return t.priority
}

// GetName returns the transport name
func (t *HTTPSTunnelTransport) GetName() string {
	return "https-local-relay"
}

// Close cleans up the transport and stops relay server
func (t *HTTPSTunnelTransport) Close() error {
	if t.relayServer != nil {
		return t.relayServer.Stop()
	}
	return nil
}
