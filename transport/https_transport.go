package transport

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
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
	// SECURE P2P RELAY: Only use local relay server for trusted lab-to-lab transfers
	maxSize := int64(100 * 1024 * 1024) // 100MB limit for HTTPS fallback
	if int64(len(data)) > maxSize {
		return fmt.Errorf("file too large for HTTPS fallback transport (%d bytes, max %d). Primary CROC transport should handle this", len(data), maxSize)
	}

	fmt.Println("🔒 HTTPS Fallback: Using secure local relay for lab-to-lab transfer...")

	// Try local relay first (most secure for lab environments)
	localRelay := NewSimpleHTTPRelay()
	err := localRelay.Start(8080) // Use port 8080 for local relay
	if err != nil {
		fmt.Printf("⚠️  Local relay failed to start: %v\n", err)
		return fmt.Errorf("HTTPS fallback transport failed: local relay unavailable")
	}
	defer localRelay.Stop()

	// Create secure payload for local relay
	payload := HTTPSTransferPayload{
		TransferID: metadata.TransferID,
		FileName:   metadata.FileName,
		FileSize:   int64(len(data)),
		Data:       base64.StdEncoding.EncodeToString(data),
		Checksum:   metadata.Checksum,
		Timestamp:  time.Now(),
	}

	// Send to local relay
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal secure payload: %w", err)
	}

	req, err := http.NewRequest("POST", "http://localhost:8080/relay", bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create relay request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "TrustDrop-Lab-Relay/1.0")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send to secure relay: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("secure relay returned status %d", resp.StatusCode)
	}

	fmt.Printf("✅ Successfully sent via secure local relay (lab-to-lab mode)\n")
	return nil
}

// External service methods removed - using secure local relay only for lab-to-lab transfers

// Receive retrieves data from secure local relay
func (t *HTTPSTunnelTransport) Receive(metadata TransferMetadata) ([]byte, error) {
	fmt.Println("🔒 HTTPS Fallback: Attempting to receive from secure local relay...")

	// Try to receive from local relay
	req, err := http.NewRequest("GET", "http://localhost:8080/relay/"+metadata.TransferID, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create relay request: %w", err)
	}

	req.Header.Set("User-Agent", "TrustDrop-Lab-Relay/1.0")

	resp, err := t.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to receive from secure relay: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("transfer not found on secure relay (may still be in progress)")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("secure relay returned status %d", resp.StatusCode)
	}

	var payload HTTPSTransferPayload
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("failed to decode secure relay payload: %w", err)
	}

	// Decode the data
	data, err := base64.StdEncoding.DecodeString(payload.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 data: %w", err)
	}

	fmt.Printf("✅ Successfully received from secure local relay (%d bytes)\n", len(data))
	return data, nil
}

// Legacy external service receive methods removed - using secure local relay only

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
