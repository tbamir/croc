package transport

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if req.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if req.Method == "POST" {
		var payload HTTPSTransferPayload
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		r.mutex.Lock()
		r.storage[payload.TransferID] = payload
		r.mutex.Unlock()

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "stored"})
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRelayWithID handles GET requests to retrieve data
func (r *SimpleHTTPRelay) handleRelayWithID(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if req.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	transferID := req.URL.Path[len("/relay/"):]
	if transferID == "" {
		http.Error(w, "Transfer ID required", http.StatusBadRequest)
		return
	}

	if req.Method == "GET" {
		r.mutex.RLock()
		payload, exists := r.storage[transferID]
		r.mutex.RUnlock()

		if !exists {
			http.Error(w, "Transfer not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(payload)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// HTTPSRelayService represents a file relay service
type HTTPSRelayService struct {
	Name         string
	URL          string
	MaxSize      int64
	RequiresAuth bool
	Priority     int
}

// Available HTTPS relay services (ordered by reliability for corporate networks)
var relayServices = []HTTPSRelayService{
	{
		Name:         "Transfer.sh",
		URL:          "https://transfer.sh/",
		MaxSize:      10 * 1024 * 1024 * 1024, // 10GB
		RequiresAuth: false,
		Priority:     100,
	},
	{
		Name:         "File.io",
		URL:          "https://file.io/",
		MaxSize:      2 * 1024 * 1024 * 1024, // 2GB
		RequiresAuth: false,
		Priority:     90,
	},
	{
		Name:         "0x0.st",
		URL:          "https://0x0.st/",
		MaxSize:      512 * 1024 * 1024, // 512MB
		RequiresAuth: false,
		Priority:     80,
	},
	{
		Name:         "GitHub Gists",
		URL:          "https://api.github.com/gists",
		MaxSize:      25 * 1024 * 1024, // 25MB (base64 encoded)
		RequiresAuth: true,
		Priority:     70, // Lower priority due to auth requirement
	},
}

type HTTPSTunnelTransport struct {
	client      *http.Client
	maxFileSize int64
	githubToken string
	priority    int
	config      TransportConfig
}

func NewHTTPSTunnelTransport(priority int) *HTTPSTunnelTransport {
	// Create HTTP client with corporate proxy support
	client := &http.Client{
		Timeout: 120 * time.Second, // Extended timeout for international transfers
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment, // Corporate proxy support
			MaxIdleConns:        10,
			IdleConnTimeout:     90 * time.Second,
			DisableCompression:  false,
			TLSHandshakeTimeout: 30 * time.Second,
		},
	}

	// Check for GitHub token (optional)
	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		githubToken = os.Getenv("GITHUB_API_TOKEN")
	}

	return &HTTPSTunnelTransport{
		client:      client,
		maxFileSize: 50 * 1024 * 1024, // 50MB max for HTTPS transport
		githubToken: githubToken,
		priority:    priority,
	}
}

func (t *HTTPSTunnelTransport) Setup(config TransportConfig) error {
	t.config = config
	fmt.Printf("HTTPS International Transport initialized with GitHub Gists API and %d backup endpoints\n", len(relayServices)-1)
	return nil
}

// Send transmits data through multiple HTTPS relay services with automatic failover
func (t *HTTPSTunnelTransport) Send(data []byte, metadata TransferMetadata) error {
	// CRITICAL FIX: Validate size before any processing to prevent memory exhaustion
	maxRawSize := int64(25 * 1024 * 1024) // 25MB raw data limit (will be ~33MB base64)
	if int64(len(data)) > maxRawSize {
		return fmt.Errorf("file too large for HTTPS transport (%d bytes, max %d). Use CROC transport for large files", len(data), maxRawSize)
	}

	// Additional check for base64 expansion (4/3 ratio)
	estimatedBase64Size := int64(len(data)) * 4 / 3
	maxBase64Size := int64(50 * 1024 * 1024) // 50MB base64 limit
	if estimatedBase64Size > maxBase64Size {
		return fmt.Errorf("file too large after base64 encoding (%d bytes estimated, max %d). Use CROC transport for large files", estimatedBase64Size, maxBase64Size)
	}

	fmt.Println("🌍 Attempting international HTTPS transfer via multiple relay services...")

	// Try each service in priority order
	var lastError error
	for _, service := range relayServices {
		// Skip GitHub Gists if no token available
		if service.RequiresAuth && service.Name == "GitHub Gists" && t.githubToken == "" {
			fmt.Printf("⚠️  Skipping %s (no authentication token)\n", service.Name)
			continue
		}

		// Check size limits
		if int64(len(data)) > service.MaxSize {
			fmt.Printf("⚠️  Skipping %s (file too large: %d > %d)\n", service.Name, len(data), service.MaxSize)
			continue
		}

		fmt.Printf("🔄 Trying %s...\n", service.Name)

		err := t.sendViaService(service, data, metadata)
		if err == nil {
			fmt.Printf("✅ Successfully sent via %s\n", service.Name)
			return nil
		}

		fmt.Printf("❌ %s failed: %v\n", service.Name, err)
		lastError = err
	}

	return fmt.Errorf("all HTTPS relay services failed, last error: %w", lastError)
}

// sendViaService attempts to send data via a specific relay service
func (t *HTTPSTunnelTransport) sendViaService(service HTTPSRelayService, data []byte, metadata TransferMetadata) error {
	switch service.Name {
	case "Transfer.sh":
		return t.sendViaTransferSh(data, metadata)
	case "File.io":
		return t.sendViaFileIo(data, metadata)
	case "0x0.st":
		return t.sendVia0x0St(data, metadata)
	case "GitHub Gists":
		return t.sendViaGitHubGists(data, metadata)
	default:
		return fmt.Errorf("unknown service: %s", service.Name)
	}
}

// sendViaTransferSh sends data via transfer.sh
func (t *HTTPSTunnelTransport) sendViaTransferSh(data []byte, metadata TransferMetadata) error {
	url := "https://transfer.sh/" + metadata.TransferID

	req, err := http.NewRequest("PUT", url, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("User-Agent", "TrustDrop-Bulletproof/1.0")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("transfer.sh returned status %d", resp.StatusCode)
	}

	return nil
}

// sendViaFileIo sends data via file.io
func (t *HTTPSTunnelTransport) sendViaFileIo(data []byte, metadata TransferMetadata) error {
	// File.io requires multipart form data
	var buf bytes.Buffer
	boundary := "----TrustDropBoundary"

	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"" + metadata.TransferID + "\"\r\n")
	buf.WriteString("Content-Type: application/octet-stream\r\n\r\n")
	buf.Write(data)
	buf.WriteString("\r\n--" + boundary + "--\r\n")

	req, err := http.NewRequest("POST", "https://file.io/", &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	req.Header.Set("User-Agent", "TrustDrop-Bulletproof/1.0")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("file.io returned status %d", resp.StatusCode)
	}

	return nil
}

// sendVia0x0St sends data via 0x0.st
func (t *HTTPSTunnelTransport) sendVia0x0St(data []byte, metadata TransferMetadata) error {
	// 0x0.st requires multipart form data
	var buf bytes.Buffer
	boundary := "----TrustDropBoundary"

	buf.WriteString("--" + boundary + "\r\n")
	buf.WriteString("Content-Disposition: form-data; name=\"file\"; filename=\"" + metadata.TransferID + "\"\r\n")
	buf.WriteString("Content-Type: application/octet-stream\r\n\r\n")
	buf.Write(data)
	buf.WriteString("\r\n--" + boundary + "--\r\n")

	req, err := http.NewRequest("POST", "https://0x0.st/", &buf)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	req.Header.Set("User-Agent", "TrustDrop-Bulletproof/1.0")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("0x0.st returned status %d", resp.StatusCode)
	}

	return nil
}

// sendViaGitHubGists sends data via GitHub Gists API (requires authentication)
func (t *HTTPSTunnelTransport) sendViaGitHubGists(data []byte, metadata TransferMetadata) error {
	// Now safe to encode - we've validated size limits
	encodedData := base64.StdEncoding.EncodeToString(data)

	// Create gist payload
	gistPayload := map[string]interface{}{
		"description": fmt.Sprintf("TrustDrop transfer: %s", metadata.TransferID),
		"public":      false,
		"files": map[string]interface{}{
			metadata.TransferID + ".txt": map[string]string{
				"content": encodedData,
			},
		},
	}

	payloadBytes, err := json.Marshal(gistPayload)
	if err != nil {
		return fmt.Errorf("failed to marshal gist payload: %w", err)
	}

	// Create request with authentication
	req, err := http.NewRequest("POST", "https://api.github.com/gists", bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create gist request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "token "+t.githubToken)
	req.Header.Set("User-Agent", "TrustDrop-Bulletproof/1.0")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create gist: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github API returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return nil
}

// Receive retrieves data from HTTPS relay services
func (t *HTTPSTunnelTransport) Receive(metadata TransferMetadata) ([]byte, error) {
	fmt.Println("🌍 Attempting to receive via international HTTPS relay services...")

	// Try each service to find the data
	var lastError error
	for _, service := range relayServices {
		fmt.Printf("🔄 Trying to receive from %s...\n", service.Name)

		data, err := t.receiveViaService(service, metadata.TransferID)
		if err == nil {
			fmt.Printf("✅ Successfully received from %s\n", service.Name)
			return data, nil
		}

		fmt.Printf("❌ %s failed: %v\n", service.Name, err)
		lastError = err
	}

	return nil, fmt.Errorf("failed to receive from all HTTPS relay services, last error: %w", lastError)
}

// receiveViaService attempts to receive data from a specific relay service
func (t *HTTPSTunnelTransport) receiveViaService(service HTTPSRelayService, transferID string) ([]byte, error) {
	switch service.Name {
	case "GitHub Gists":
		return t.receiveViaGitHubGists(transferID)
	default:
		// For other services, we'd need to implement specific retrieval methods
		// For now, focus on GitHub Gists as the primary method
		return nil, fmt.Errorf("receive not implemented for %s", service.Name)
	}
}

// receiveViaGitHubGists receives data from GitHub Gists
func (t *HTTPSTunnelTransport) receiveViaGitHubGists(transferID string) ([]byte, error) {
	// This would require storing the gist ID somewhere accessible
	// For now, return an error indicating this needs implementation
	return nil, fmt.Errorf("GitHub Gists receive requires gist ID storage implementation")
}

func (t *HTTPSTunnelTransport) GetName() string {
	return "HTTPS International"
}

func (t *HTTPSTunnelTransport) GetPriority() int {
	return t.priority
}

func (t *HTTPSTunnelTransport) IsAvailable(ctx context.Context) bool {
	// Test connectivity to at least one service
	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	for _, service := range relayServices[:2] { // Test first 2 services
		if service.RequiresAuth && service.Name == "GitHub Gists" && t.githubToken == "" {
			continue
		}

		req, err := http.NewRequestWithContext(testCtx, "HEAD", service.URL, nil)
		if err != nil {
			continue
		}

		resp, err := t.client.Do(req)
		if err != nil {
			continue
		}
		resp.Body.Close()

		if resp.StatusCode < 500 { // Accept any non-server-error response
			return true
		}
	}

	return false
}

func (t *HTTPSTunnelTransport) Close() error {
	// Nothing to close for HTTP client
	return nil
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
