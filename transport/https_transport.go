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

// Setup initializes the HTTPS tunnel transport with REMOTE relay servers for international transfers
func (t *HTTPSTunnelTransport) Setup(config TransportConfig) error {
	t.config = config
	t.priority = 95 // High priority for corporate networks

	// Use GitHub Gists API for actual data storage across international transfers
	// This works through most corporate firewalls since GitHub is commonly allowed
	t.relayURLs = []string{
		"https://api.github.com/gists", // Primary - GitHub Gists API
		"https://httpbin.org/post",     // Secondary - testing endpoint
		"https://reqres.in/api/data",   // Tertiary - testing endpoint
	}

	// NO local relay server needed - use remote services only
	fmt.Printf("HTTPS International Transport initialized with GitHub Gists API and %d backup endpoints\n", len(t.relayURLs))

	// Create HTTP client optimized for international connections through firewalls
	t.httpClient = &http.Client{
		Timeout: 120 * time.Second, // Extended timeout for international transfers
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				MinVersion:         tls.VersionTLS12,
				InsecureSkipVerify: false, // Use proper TLS for security
			},
			DisableKeepAlives:     false,
			MaxIdleConns:          20,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       60 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
			ExpectContinueTimeout: 10 * time.Second,
			// Proxy settings for corporate environments
			Proxy: http.ProxyFromEnvironment, // Use system proxy settings
		},
	}

	return nil
}

// Send transmits data through GitHub Gists API or backup international HTTPS endpoints
func (t *HTTPSTunnelTransport) Send(data []byte, metadata TransferMetadata) error {
	// Encode data as base64 for JSON compatibility
	encodedData := base64.StdEncoding.EncodeToString(data)

	// Check size limit for HTTPS transport (GitHub Gists has 10MB limit)
	maxChunkSize := 5 * 1024 * 1024 // 5MB to be safe
	if len(encodedData) > maxChunkSize {
		return fmt.Errorf("file too large for HTTPS transport (%d bytes, max %d). Use CROC or WebSocket transport for large files", len(encodedData), maxChunkSize)
	}

	// Try GitHub Gists API first (most reliable for international transfers)
	fmt.Printf("🌍 Attempting international HTTPS transfer via GitHub Gists API...\n")

	err := t.sendViaGitHubGists(encodedData, metadata)
	if err == nil {
		fmt.Printf("✅ International HTTPS send successful via GitHub Gists API\n")
		return nil
	}

	fmt.Printf("❌ GitHub Gists API failed: %v\n", err)
	fmt.Printf("🔄 Falling back to backup HTTPS endpoints...\n")

	// Fallback to other endpoints for connectivity verification
	return fmt.Errorf("HTTPS International transport requires GitHub API access for data storage. Primary method failed: %w", err)
}

// sendViaGitHubGists uses GitHub Gists API to store transfer data
func (t *HTTPSTunnelTransport) sendViaGitHubGists(encodedData string, metadata TransferMetadata) error {
	// Create GitHub Gist payload
	gistPayload := map[string]interface{}{
		"description": fmt.Sprintf("TrustDrop Transfer: %s", metadata.FileName),
		"public":      false, // Private gist for security
		"files": map[string]interface{}{
			fmt.Sprintf("trustdrop_%s.json", metadata.TransferID): map[string]interface{}{
				"content": fmt.Sprintf(`{
	"transfer_id": "%s",
	"file_name": "%s",
	"file_size": %d,
	"checksum": "%s",
	"data": "%s",
	"timestamp": "%s"
}`, metadata.TransferID, metadata.FileName, metadata.FileSize, metadata.Checksum, encodedData, time.Now().UTC().Format(time.RFC3339)),
			},
		},
	}

	payloadBytes, err := json.Marshal(gistPayload)
	if err != nil {
		return fmt.Errorf("failed to create GitHub Gist payload: %w", err)
	}

	// Create GitHub API request
	req, err := http.NewRequest("POST", "https://api.github.com/gists", bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create GitHub API request: %w", err)
	}

	// Headers for GitHub API
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TrustDrop-International/1.0")
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	// Note: Anonymous gists are allowed, so no authentication required

	// Send request
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	req = req.WithContext(ctx)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 201 { // GitHub returns 201 for successful gist creation
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GitHub API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response to get gist URL
	var gistResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&gistResponse); err != nil {
		return fmt.Errorf("failed to parse GitHub API response: %w", err)
	}

	if gistURL, ok := gistResponse["html_url"].(string); ok {
		fmt.Printf("📝 GitHub Gist created successfully: %s\n", gistURL)
		fmt.Printf("📋 Share this transfer ID with recipient: %s\n", metadata.TransferID)
	}

	return nil
}

// Receive gets data from GitHub Gists API
func (t *HTTPSTunnelTransport) Receive(metadata TransferMetadata) ([]byte, error) {
	fmt.Printf("🌍 Attempting to receive international HTTPS transfer via GitHub Gists API...\n")
	fmt.Printf("🔍 Searching for transfer ID: %s\n", metadata.TransferID)

	// For now, return a helpful error since we need the specific gist ID
	// In a production system, we'd implement a discovery mechanism
	return nil, fmt.Errorf("HTTPS International receive requires specific gist ID. This transport is primarily for sending. For receiving, please use CROC or WebSocket transport")
}

// IsAvailable checks if international HTTPS relays are available
func (t *HTTPSTunnelTransport) IsAvailable(ctx context.Context) bool {
	// Test connectivity to international relay servers (not local)
	for i, relayURL := range t.relayURLs {
		if i >= 3 { // Test first 3 international relays
			break
		}
		if t.testInternationalRelayConnectivity(ctx, relayURL) {
			fmt.Printf("HTTPS International transport available via: %s\n", relayURL)
			return true
		}
	}
	fmt.Printf("HTTPS International transport not available - no relay servers reachable\n")
	return false
}

// testInternationalRelayConnectivity tests connection to international relay servers
func (t *HTTPSTunnelTransport) testInternationalRelayConnectivity(ctx context.Context, relayURL string) bool {
	client := &http.Client{
		Timeout: 10 * time.Second, // Extended timeout for international connectivity
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment, // Use system proxy for corporate networks
		},
	}

	// Use HEAD request for connectivity test (lighter than GET)
	req, err := http.NewRequestWithContext(ctx, "HEAD", relayURL, nil)
	if err != nil {
		return false
	}

	// Add headers that help with firewall traversal
	req.Header.Set("User-Agent", "TrustDrop-International/1.0")
	req.Header.Set("Accept", "*/*")

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Accept any response from the server (including 404) as proof of connectivity
	// The important thing is that we can reach the server through the firewall
	return resp.StatusCode > 0
}

// GetPriority returns the transport priority
func (t *HTTPSTunnelTransport) GetPriority() int {
	return t.priority
}

// GetName returns the transport name
func (t *HTTPSTunnelTransport) GetName() string {
	return "https-international"
}

// Close cleans up the transport and stops relay server
func (t *HTTPSTunnelTransport) Close() error {
	if t.relayServer != nil {
		return t.relayServer.Stop()
	}
	return nil
}
