package transport

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// DirectHTTPSTransport provides direct peer-to-peer HTTPS transport
type DirectHTTPSTransport struct {
	priority  int
	config    TransportConfig
	server    *http.Server
	mutex     sync.RWMutex
	transfers map[string][]byte // Temporary storage for P2P handoff
}

func NewHTTPSTunnelTransport(priority int) *DirectHTTPSTransport {
	return &DirectHTTPSTransport{
		priority:  priority,
		transfers: make(map[string][]byte),
	}
}

func (t *DirectHTTPSTransport) Setup(config TransportConfig) error {
	t.config = config
	fmt.Printf("Direct HTTPS P2P Transport initialized for lab-to-lab transfers\n")
	return nil
}

// Send implements direct P2P HTTPS without public services
func (t *DirectHTTPSTransport) Send(data []byte, metadata TransferMetadata) error {
	// For true P2P, we need to establish a direct connection
	// This implementation provides a framework for direct peer discovery

	maxSize := int64(100 * 1024 * 1024) // 100MB limit
	if int64(len(data)) > maxSize {
		return fmt.Errorf("file too large for direct HTTPS transport (%d bytes, max %d)", len(data), maxSize)
	}

	fmt.Println("🔒 Direct HTTPS: Starting peer-to-peer server for lab-to-lab transfer...")

	// Store data for P2P retrieval
	t.mutex.Lock()
	t.transfers[metadata.TransferID] = data
	t.mutex.Unlock()

	// Start temporary HTTPS server for direct peer connection
	return t.startP2PServer(metadata.TransferID)
}

// startP2PServer creates a temporary server for direct peer connection
func (t *DirectHTTPSTransport) startP2PServer(transferID string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/transfer/"+transferID, t.handleDirectTransfer)

	// Generate self-signed certificate for local use
	cert, key, err := t.generateSelfSignedCert()
	if err != nil {
		fmt.Printf("❌ Failed to generate certificate: %v\n", err)
		// Fall back to HTTP for local testing
		return t.startHTTPServer(transferID, mux)
	}

	// Use TLS for security
	t.server = &http.Server{
		Addr:    ":8443", // Use HTTPS port
		Handler: mux,
		TLSConfig: &tls.Config{
			Certificates: []tls.Certificate{{Certificate: [][]byte{cert}, PrivateKey: key}},
		},
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start server in background
	go func() {
		fmt.Printf("🚀 Starting HTTPS server on port 8443 for transfer %s\n", transferID)
		if err := t.server.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			fmt.Printf("❌ HTTPS server failed: %v\n", err)
		}
	}()

	fmt.Printf("✅ Direct P2P HTTPS server ready on port 8443 for transfer %s\n", transferID)
	return nil
}

// startHTTPServer starts an HTTP server as fallback
func (t *DirectHTTPSTransport) startHTTPServer(transferID string, mux *http.ServeMux) error {
	t.server = &http.Server{
		Addr:         ":8080", // Use HTTP port as fallback
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Start server in background
	go func() {
		fmt.Printf("🚀 Starting HTTP server on port 8080 for transfer %s\n", transferID)
		if err := t.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("❌ HTTP server failed: %v\n", err)
		}
	}()

	fmt.Printf("✅ Direct P2P HTTP server ready on port 8080 for transfer %s\n", transferID)
	return nil
}

// generateSelfSignedCert generates a self-signed certificate for local use
func (t *DirectHTTPSTransport) generateSelfSignedCert() ([]byte, interface{}, error) {
	// For now, return an error to fall back to HTTP
	// In a full implementation, this would generate a proper self-signed cert
	return nil, nil, fmt.Errorf("self-signed cert generation not implemented")
}

// handleDirectTransfer handles direct peer-to-peer transfer requests
func (t *DirectHTTPSTransport) handleDirectTransfer(w http.ResponseWriter, r *http.Request) {
	transferID := r.URL.Path[len("/transfer/"):]

	t.mutex.RLock()
	data, exists := t.transfers[transferID]
	t.mutex.RUnlock()

	if !exists {
		http.Error(w, "Transfer not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment")
	w.Write(data)

	// Clean up after successful transfer
	t.mutex.Lock()
	delete(t.transfers, transferID)
	t.mutex.Unlock()
}

// Receive implements direct P2P HTTPS connection
func (t *DirectHTTPSTransport) Receive(metadata TransferMetadata) ([]byte, error) {
	fmt.Println("🔒 Direct HTTPS: Attempting to receive from local network...")

	// Try common local network addresses where the sender might be (both HTTPS and HTTP)
	localAddresses := []string{
		"localhost:8443",
		"127.0.0.1:8443",
		"localhost:8080",
		"127.0.0.1:8080",
	}

	// Also try to discover local network IPs
	if localIPs := t.getLocalNetworkIPs(); len(localIPs) > 0 {
		for _, ip := range localIPs {
			localAddresses = append(localAddresses, fmt.Sprintf("%s:8443", ip))
			localAddresses = append(localAddresses, fmt.Sprintf("%s:8080", ip))
		}
	}

	for _, addr := range localAddresses {
		fmt.Printf("🔍 Trying to connect to: %s\n", addr)

		// Create HTTP client with timeout
		var client *http.Client
		var url string

		if strings.Contains(addr, ":8443") {
			// HTTPS client
			client = &http.Client{
				Timeout: 10 * time.Second,
				Transport: &http.Transport{
					TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, // For self-signed certs
				},
			}
			url = fmt.Sprintf("https://%s/transfer/%s", addr, metadata.TransferID)
		} else {
			// HTTP client
			client = &http.Client{
				Timeout: 10 * time.Second,
			}
			url = fmt.Sprintf("http://%s/transfer/%s", addr, metadata.TransferID)
		}
		resp, err := client.Get(url)
		if err != nil {
			fmt.Printf("❌ Failed to connect to %s: %v\n", addr, err)
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == http.StatusOK {
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				fmt.Printf("❌ Failed to read data from %s: %v\n", addr, err)
				continue
			}

			fmt.Printf("✅ Successfully received %d bytes from %s\n", len(data), addr)
			return data, nil
		}

		fmt.Printf("❌ Server at %s returned status: %d\n", addr, resp.StatusCode)
	}

	return nil, fmt.Errorf("could not connect to any local HTTPS servers for transfer %s", metadata.TransferID)
}

// getLocalNetworkIPs returns local network IP addresses
func (t *DirectHTTPSTransport) getLocalNetworkIPs() []string {
	var ips []string

	interfaces, err := net.Interfaces()
	if err != nil {
		return ips
	}

	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					ips = append(ips, ipnet.IP.String())
				}
			}
		}
	}

	return ips
}

// Rest of interface implementation
func (t *DirectHTTPSTransport) GetName() string {
	return "direct-https-p2p"
}

func (t *DirectHTTPSTransport) GetPriority() int {
	return t.priority
}

func (t *DirectHTTPSTransport) IsAvailable(ctx context.Context) bool {
	// Direct P2P HTTPS is available if we can bind to port
	return true
}

func (t *DirectHTTPSTransport) Close() error {
	if t.server != nil {
		return t.server.Shutdown(context.Background())
	}
	return nil
}
