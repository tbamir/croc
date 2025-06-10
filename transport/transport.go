package transport

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Transport defines the interface for different transport protocols
type Transport interface {
	Send(data []byte, metadata TransferMetadata) error
	Receive(metadata TransferMetadata) ([]byte, error)
	IsAvailable(ctx context.Context) bool
	GetPriority() int
	GetName() string
	Setup(config TransportConfig) error
	Close() error
}

// TransferMetadata contains information about the transfer
type TransferMetadata struct {
	TransferID  string `json:"transfer_id"`
	FileName    string `json:"file_name"`
	FileSize    int64  `json:"file_size"`
	Checksum    string `json:"checksum"`
	ChunkIndex  int    `json:"chunk_index,omitempty"`
	TotalChunks int    `json:"total_chunks,omitempty"`
}

// TransportConfig holds configuration for transports
type TransportConfig struct {
	RelayServers  []string      `json:"relay_servers"`
	TorProxy      string        `json:"tor_proxy,omitempty"`
	HTTPSProxy    string        `json:"https_proxy,omitempty"`
	WebRTCServers []string      `json:"webrtc_servers,omitempty"`
	EncryptionKey []byte        `json:"-"`
	Timeout       time.Duration `json:"timeout"`
}

// NetworkProfile describes the network environment characteristics
type NetworkProfile struct {
	IsRestrictive      bool     `json:"is_restrictive"`
	AvailablePorts     []int    `json:"available_ports"`
	Latency            int      `json:"latency_ms"`
	Bandwidth          int64    `json:"bandwidth_bps"`
	BlockedDomains     []string `json:"blocked_domains"`
	HasTorAccess       bool     `json:"has_tor_access"`
	HasWebRTC          bool     `json:"has_webrtc"`
	SupportsUDP        bool     `json:"supports_udp"`
	PreferredTransport string   `json:"preferred_transport"`
}

// MultiTransportManager manages multiple transport protocols with intelligent failover
type MultiTransportManager struct {
	transports       []Transport
	networkProfile   NetworkProfile
	config           TransportConfig
	mutex            sync.RWMutex
	currentTransport Transport
	failedTransports map[string]time.Time
	successHistory   map[string]int
}

// NewMultiTransportManager creates a new multi-transport manager
func NewMultiTransportManager(config TransportConfig) (*MultiTransportManager, error) {
	mtm := &MultiTransportManager{
		config:           config,
		failedTransports: make(map[string]time.Time),
		successHistory:   make(map[string]int),
	}

	// Set a basic network profile without complex testing
	fmt.Printf("Creating minimal transport manager (skipping complex initialization)...\n")
	mtm.networkProfile = NetworkProfile{
		IsRestrictive:      false,
		AvailablePorts:     []int{9009, 443, 80},
		HasTorAccess:       false,
		HasWebRTC:          true,
		SupportsUDP:        true,
		PreferredTransport: "croc",
	}

	// Initialize at least the croc transport for basic functionality
	fmt.Printf("Initializing core transports...\n")
	if err := mtm.initializeBasicTransports(); err != nil {
		return nil, fmt.Errorf("failed to initialize basic transports: %w", err)
	}

	fmt.Printf("Transport manager created successfully\n")
	return mtm, nil
}

// initializeBasicTransports sets up essential transports without complex network testing
func (mtm *MultiTransportManager) initializeBasicTransports() error {
	// Initialize Croc transport (highest priority for compatibility)
	fmt.Printf("Setting up croc transport...\n")
	crocTransport := NewCrocTransport(100)
	if err := crocTransport.Setup(mtm.config); err == nil {
		mtm.transports = append(mtm.transports, crocTransport)
		fmt.Printf("Croc transport initialized\n")
	} else {
		fmt.Printf("Failed to setup croc transport: %v\n", err)
	}

	if len(mtm.transports) == 0 {
		return fmt.Errorf("no transports available")
	}

	fmt.Printf("Initialized %d transports\n", len(mtm.transports))
	return nil
}

// initializeTransports sets up all available transport methods
func (mtm *MultiTransportManager) initializeTransports() error {
	// Initialize Croc transport (highest priority for compatibility)
	crocTransport := NewCrocTransport(100)
	if err := crocTransport.Setup(mtm.config); err == nil {
		mtm.transports = append(mtm.transports, crocTransport)
	}

	// Initialize other transports based on network capabilities
	if mtm.networkProfile.HasTorAccess {
		torTransport := &TorTransport{priority: 80}
		if err := torTransport.Setup(mtm.config); err == nil {
			mtm.transports = append(mtm.transports, torTransport)
		}
	}

	httpsTransport := &HTTPSTunnelTransport{priority: 60}
	if err := httpsTransport.Setup(mtm.config); err == nil {
		mtm.transports = append(mtm.transports, httpsTransport)
	}

	if mtm.networkProfile.HasWebRTC {
		wsTransport := &WebSocketTransport{priority: 40}
		if err := wsTransport.Setup(mtm.config); err == nil {
			mtm.transports = append(mtm.transports, wsTransport)
		}
	}

	if mtm.networkProfile.SupportsUDP {
		p2pTransport := &DirectP2PTransport{priority: 20}
		if err := p2pTransport.Setup(mtm.config); err == nil {
			mtm.transports = append(mtm.transports, p2pTransport)
		}
	}

	if len(mtm.transports) == 0 {
		return fmt.Errorf("no transports available")
	}

	return nil
}

// analyzeNetwork performs network analysis to optimize transport selection
func (mtm *MultiTransportManager) analyzeNetwork() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	profile := NetworkProfile{
		AvailablePorts: []int{80, 443, 8080},
	}

	// Test common ports
	testPorts := []int{9009, 9010, 9011, 9012, 9013, 80, 443, 8080, 1080}
	for _, port := range testPorts {
		if mtm.testPortConnectivity(ctx, port) {
			profile.AvailablePorts = append(profile.AvailablePorts, port)
		}
	}

	// Test Tor connectivity
	profile.HasTorAccess = mtm.testTorConnectivity(ctx)

	// Test UDP support
	profile.SupportsUDP = mtm.testUDPConnectivity(ctx)

	// Determine if network is restrictive
	profile.IsRestrictive = len(profile.AvailablePorts) < len(testPorts)

	// Set preferred transport based on analysis
	if profile.IsRestrictive {
		if profile.HasTorAccess {
			profile.PreferredTransport = "tor"
		} else {
			profile.PreferredTransport = "https"
		}
	} else {
		profile.PreferredTransport = "croc"
	}

	mtm.networkProfile = profile
	return nil
}

// testPortConnectivity tests if a specific port is accessible
func (mtm *MultiTransportManager) testPortConnectivity(ctx context.Context, port int) bool {
	addresses := []string{
		fmt.Sprintf("google.com:%d", port),
		fmt.Sprintf("cloudflare.com:%d", port),
		fmt.Sprintf("8.8.8.8:%d", port),
	}

	for _, addr := range addresses {
		conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
		if err == nil {
			conn.Close()
			return true
		}
	}
	return false
}

// testTorConnectivity tests if Tor network is accessible
func (mtm *MultiTransportManager) testTorConnectivity(ctx context.Context) bool {
	// Try to connect through common Tor proxy ports
	torProxies := []string{"127.0.0.1:9050", "127.0.0.1:9150", "127.0.0.1:8118"}

	for _, proxy := range torProxies {
		// Test if we can connect to a .onion address through the proxy
		if mtm.testTorProxy(ctx, proxy) {
			return true
		}
	}

	// Check if Tor is running by trying to start it
	if mtm.attemptTorStart() {
		return true
	}

	return false
}

// testTorProxy tests a specific Tor proxy
func (mtm *MultiTransportManager) testTorProxy(ctx context.Context, proxy string) bool {
	// Create a SOCKS5 dialer
	dialer, err := net.Dial("tcp", proxy)
	if err != nil {
		return false
	}
	dialer.Close()
	return true
}

// attemptTorStart tries to start Tor if it's installed
func (mtm *MultiTransportManager) attemptTorStart() bool {
	// Check if Tor is installed
	_, err := exec.LookPath("tor")
	if err != nil {
		return false
	}

	// Try to start Tor in the background (non-blocking)
	cmd := exec.Command("tor", "--quiet")
	err = cmd.Start()
	return err == nil
}

// testUDPConnectivity tests UDP hole punching capability
func (mtm *MultiTransportManager) testUDPConnectivity(ctx context.Context) bool {
	// Test UDP connectivity to known servers
	servers := []string{
		"8.8.8.8:53",
		"1.1.1.1:53",
		"stun.l.google.com:19302",
	}

	for _, server := range servers {
		conn, err := net.DialTimeout("udp", server, 3*time.Second)
		if err == nil {
			conn.Close()
			return true
		}
	}
	return false
}

// SendWithFailover attempts to send data using the best available transport with automatic failover
func (mtm *MultiTransportManager) SendWithFailover(data []byte, metadata TransferMetadata) error {
	mtm.mutex.Lock()
	defer mtm.mutex.Unlock()

	// Sort transports by priority and success history
	orderedTransports := mtm.getOrderedTransports()

	var lastErr error
	for _, transport := range orderedTransports {
		// Skip recently failed transports
		if failTime, exists := mtm.failedTransports[transport.GetName()]; exists {
			if time.Since(failTime) < 5*time.Minute {
				continue
			}
		}

		// Test transport availability
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if !transport.IsAvailable(ctx) {
			cancel()
			continue
		}
		cancel()

		// Attempt transfer
		err := transport.Send(data, metadata)
		if err == nil {
			// Success - update statistics
			mtm.successHistory[transport.GetName()]++
			delete(mtm.failedTransports, transport.GetName())
			mtm.currentTransport = transport
			return nil
		}

		// Mark as failed
		mtm.failedTransports[transport.GetName()] = time.Now()
		lastErr = err
	}

	return fmt.Errorf("all transports failed, last error: %w", lastErr)
}

// ReceiveWithFailover attempts to receive data using available transports
func (mtm *MultiTransportManager) ReceiveWithFailover(metadata TransferMetadata) ([]byte, error) {
	mtm.mutex.Lock()
	defer mtm.mutex.Unlock()

	orderedTransports := mtm.getOrderedTransports()

	var lastErr error
	for _, transport := range orderedTransports {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if !transport.IsAvailable(ctx) {
			cancel()
			continue
		}
		cancel()

		data, err := transport.Receive(metadata)
		if err == nil {
			mtm.successHistory[transport.GetName()]++
			delete(mtm.failedTransports, transport.GetName())
			return data, nil
		}

		mtm.failedTransports[transport.GetName()] = time.Now()
		lastErr = err
	}

	return nil, fmt.Errorf("all transports failed, last error: %w", lastErr)
}

// getOrderedTransports returns transports ordered by priority and success rate
func (mtm *MultiTransportManager) getOrderedTransports() []Transport {
	transports := make([]Transport, len(mtm.transports))
	copy(transports, mtm.transports)

	// Sort by success rate and priority
	for i := 0; i < len(transports)-1; i++ {
		for j := i + 1; j < len(transports); j++ {
			iSuccess := mtm.successHistory[transports[i].GetName()]
			jSuccess := mtm.successHistory[transports[j].GetName()]

			// Prefer transports with higher success rate, then higher priority
			if jSuccess > iSuccess || (jSuccess == iSuccess && transports[j].GetPriority() > transports[i].GetPriority()) {
				transports[i], transports[j] = transports[j], transports[i]
			}
		}
	}

	return transports
}

// GetNetworkProfile returns the analyzed network profile
func (mtm *MultiTransportManager) GetNetworkProfile() NetworkProfile {
	mtm.mutex.RLock()
	defer mtm.mutex.RUnlock()
	return mtm.networkProfile
}

// GetTransportStatus returns status of all transports
func (mtm *MultiTransportManager) GetTransportStatus() map[string]interface{} {
	mtm.mutex.RLock()
	defer mtm.mutex.RUnlock()

	status := make(map[string]interface{})

	for _, transport := range mtm.transports {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		available := transport.IsAvailable(ctx)
		cancel()

		status[transport.GetName()] = map[string]interface{}{
			"available":     available,
			"priority":      transport.GetPriority(),
			"success_count": mtm.successHistory[transport.GetName()],
			"last_failure":  mtm.failedTransports[transport.GetName()],
		}
	}

	return status
}

// Close closes all transports
func (mtm *MultiTransportManager) Close() error {
	mtm.mutex.Lock()
	defer mtm.mutex.Unlock()

	var errors []string
	for _, transport := range mtm.transports {
		if err := transport.Close(); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", transport.GetName(), err))
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to close transports: %s", strings.Join(errors, ", "))
	}

	return nil
}
