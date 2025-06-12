package transport

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"regexp"
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
	NetworkType        string   `json:"network_type"` // "home", "corporate", "university", "public", "mobile"
	ProxyDetected      bool     `json:"proxy_detected"`
	DPIDetected        bool     `json:"dpi_detected"` // Deep Packet Inspection
}

// NetworkRestriction represents detected network limitations
type NetworkRestriction struct {
	Type        string  `json:"type"` // "firewall", "proxy", "port_block", "domain_block", "dpi", "whitelist"
	Description string  `json:"description"`
	Severity    string  `json:"severity"` // "low", "medium", "high", "critical"
	Workaround  string  `json:"workaround"`
	Confidence  float64 `json:"confidence"` // 0.0 to 1.0
}

// MultiTransportManager manages multiple transport protocols with intelligent failover
type MultiTransportManager struct {
	transports          []Transport
	networkProfile      NetworkProfile
	networkRestrictions []NetworkRestriction
	config              TransportConfig
	mutex               sync.RWMutex
	currentTransport    Transport
	failedTransports    map[string]time.Time
	successHistory      map[string]int
	analysisComplete    bool
	detectionResults    map[string]bool
}

// NewMultiTransportManager creates a new multi-transport manager
func NewMultiTransportManager(config TransportConfig) (*MultiTransportManager, error) {
	mtm := &MultiTransportManager{
		config:              config,
		failedTransports:    make(map[string]time.Time),
		successHistory:      make(map[string]int),
		networkRestrictions: make([]NetworkRestriction, 0),
		detectionResults:    make(map[string]bool),
	}

	fmt.Printf("Initializing production-ready transport manager...\n")

	// Initialize with comprehensive defaults
	mtm.networkProfile = NetworkProfile{
		IsRestrictive:      false,
		AvailablePorts:     []int{80, 443},
		HasTorAccess:       false,
		HasWebRTC:          true,
		SupportsUDP:        true,
		PreferredTransport: "https-tunnel", // Default to most compatible
		NetworkType:        "unknown",
		ProxyDetected:      false,
		DPIDetected:        false,
	}

	// Initialize transports in production-ready priority order
	if err := mtm.initializeTransports(); err != nil {
		// Don't fail completely - some transports may work
		fmt.Printf("Some transports failed to initialize: %v\n", err)
	}

	// Start comprehensive network analysis
	go mtm.analyzeNetworkEnvironment()

	fmt.Printf("Transport manager ready with %d transports\n", len(mtm.transports))
	return mtm, nil
}

// initializeTransports sets up all available transport methods with production-ready priority ordering
func (mtm *MultiTransportManager) initializeTransports() error {
	var initErrors []string

	// TEMPORARILY DISABLE HTTPS LOCAL RELAY - it has cross-machine communication issues
	// Will re-enable once fixed to work across network
	fmt.Printf("Note: HTTPS local relay temporarily disabled due to cross-machine communication issues\n")

	// Initialize Enhanced Croc transport as PRIMARY (highest priority for now)
	crocTransport := NewCrocTransport(100) // Highest priority temporarily
	if err := crocTransport.Setup(mtm.config); err == nil {
		mtm.transports = append(mtm.transports, crocTransport)
		fmt.Printf("Enhanced CROC transport initialized as PRIMARY (priority: %d)\n", crocTransport.GetPriority())
	} else {
		initErrors = append(initErrors, fmt.Sprintf("Enhanced-CROC: %v", err))
		fmt.Printf("Warning: Enhanced CROC failed to initialize: %v\n", err)
	}

	// TEMPORARILY DISABLE unimplemented transports to avoid confusion
	fmt.Printf("Note: WebSocket and DirectP2P transports temporarily disabled (not implemented)\n")

	// Initialize only working fallback transports
	transportsToInit := []struct {
		name      string
		transport Transport
	}{
		{"Tor", &TorTransport{priority: 75}},
		// WebSocket and DirectP2P removed until implemented
	}

	for _, t := range transportsToInit {
		if err := t.transport.Setup(mtm.config); err == nil {
			mtm.transports = append(mtm.transports, t.transport)
			fmt.Printf("%s transport initialized (priority: %d)\n", t.name, t.transport.GetPriority())
		} else {
			initErrors = append(initErrors, fmt.Sprintf("%s: %v", t.name, err))
			fmt.Printf("Warning: %s transport failed to initialize: %v\n", t.name, err)
		}
	}

	// Ensure we have at least one working transport
	if len(mtm.transports) == 0 {
		return fmt.Errorf("CRITICAL: No transports available for file transfer. Errors: %s", strings.Join(initErrors, "; "))
	}

	// Log successful initialization
	fmt.Printf("Transport manager ready with %d/%d transports successfully initialized\n",
		len(mtm.transports), len(transportsToInit)+1) // +1 for CROC

	if len(initErrors) > 0 {
		fmt.Printf("Some transports failed to initialize (will continue with available ones): %s\n",
			strings.Join(initErrors, "; "))
	}

	return nil
}

// analyzeNetworkEnvironment performs comprehensive network analysis
func (mtm *MultiTransportManager) analyzeNetworkEnvironment() {
	fmt.Printf("Starting comprehensive network environment analysis...\n")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	profile := NetworkProfile{
		AvailablePorts: []int{80, 443}, // Always assume HTTP/HTTPS work
	}

	// Comprehensive network detection
	mtm.detectInstitutionalNetwork(&profile)
	mtm.detectProxyEnvironment(&profile)
	mtm.detectFirewallRestrictions(&profile)
	mtm.detectDPIInterference(&profile)
	mtm.testPortConnectivity(&profile, ctx)
	mtm.testDNSFiltering(&profile, ctx)
	mtm.testP2PCapability(&profile, ctx)

	// Determine overall network type and restrictions
	mtm.classifyNetworkType(&profile)
	mtm.generateNetworkRestrictions(&profile)

	mtm.mutex.Lock()
	mtm.networkProfile = profile
	mtm.analysisComplete = true
	mtm.mutex.Unlock()

	restrictiveness := mtm.calculateRestrictiveness()
	fmt.Printf("Network analysis complete: %s network, restrictiveness: %.1f%%\n",
		profile.NetworkType, restrictiveness*100)

	if len(mtm.networkRestrictions) > 0 {
		fmt.Printf("Detected %d network restrictions\n", len(mtm.networkRestrictions))
		for _, restriction := range mtm.networkRestrictions {
			fmt.Printf("  - %s (%s): %s\n", restriction.Type, restriction.Severity, restriction.Description)
		}
	}

	// Log recommended transport
	fmt.Printf("Recommended transport: %s\n", profile.PreferredTransport)
}

// detectInstitutionalNetwork detects corporate/university networks
func (mtm *MultiTransportManager) detectInstitutionalNetwork(profile *NetworkProfile) {
	indicators := 0
	totalTests := 0

	// Test DNS servers (corporate networks often use internal DNS)
	if mtm.detectCorporateDNS() {
		indicators++
		mtm.detectionResults["corporate_dns"] = true
	}
	totalTests++

	// Test for proxy auto-config
	if mtm.detectProxyAutoConfig() {
		indicators++
		mtm.detectionResults["proxy_autoconfig"] = true
	}
	totalTests++

	// Test common corporate domains
	if mtm.detectCorporateDomains() {
		indicators++
		mtm.detectionResults["corporate_domains"] = true
	}
	totalTests++

	// Test network latency patterns (corporate networks often have higher latency)
	if mtm.detectHighLatency() {
		indicators++
		mtm.detectionResults["high_latency"] = true
	}
	totalTests++

	// Test for common institutional IP ranges
	if mtm.detectInstitutionalIPRanges() {
		indicators++
		mtm.detectionResults["institutional_ip"] = true
	}
	totalTests++

	// If 3 or more indicators, likely institutional
	if indicators >= 3 {
		profile.IsRestrictive = true
		if mtm.detectionResults["corporate_dns"] || mtm.detectionResults["corporate_domains"] {
			profile.NetworkType = "corporate"
		} else {
			profile.NetworkType = "university"
		}
	}
}

// detectCorporateDNS checks for corporate DNS servers
func (mtm *MultiTransportManager) detectCorporateDNS() bool {
	// Check if using common corporate DNS servers or internal DNS
	cmd := exec.Command("nslookup", "google.com")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	outputStr := string(output)
	corporateIndicators := []string{
		"10.0.0.", "192.168.", "172.16.", // Private IP ranges
		"corp", "internal", "local", "domain",
	}

	for _, indicator := range corporateIndicators {
		if strings.Contains(strings.ToLower(outputStr), indicator) {
			return true
		}
	}
	return false
}

// detectProxyAutoConfig checks for PAC files
func (mtm *MultiTransportManager) detectProxyAutoConfig() bool {
	// Common PAC file locations
	pacURLs := []string{
		"http://wpad/wpad.dat",
		"http://wpad.local/wpad.dat",
		"http://proxy.local/proxy.pac",
	}

	client := &http.Client{Timeout: 5 * time.Second}
	for _, url := range pacURLs {
		resp, err := client.Head(url)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			return true
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
	return false
}

// detectCorporateDomains checks reverse DNS for corporate domains
func (mtm *MultiTransportManager) detectCorporateDomains() bool {
	// Get local IP and do reverse DNS lookup
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return false
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	names, err := net.LookupAddr(localAddr.IP.String())
	if err != nil {
		return false
	}

	corporateIndicators := []string{
		".corp", ".local", ".internal", ".company", ".org", ".edu",
		"vpn", "firewall", "proxy", "gateway",
	}

	for _, name := range names {
		nameLower := strings.ToLower(name)
		for _, indicator := range corporateIndicators {
			if strings.Contains(nameLower, indicator) {
				return true
			}
		}
	}
	return false
}

// detectHighLatency checks for unusually high latency
func (mtm *MultiTransportManager) detectHighLatency() bool {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", "8.8.8.8:53", 5*time.Second)
	if err != nil {
		return true // If we can't connect, assume restrictive
	}
	defer conn.Close()

	latency := time.Since(start)
	return latency > 100*time.Millisecond // High latency indicator
}

// detectInstitutionalIPRanges checks if we're in known institutional IP ranges
func (mtm *MultiTransportManager) detectInstitutionalIPRanges() bool {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return false
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	ip := localAddr.IP.String()

	// Common institutional IP patterns
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^10\.`),                          // Class A private
		regexp.MustCompile(`^172\.(1[6-9]|2[0-9]|3[0-1])\.`), // Class B private
		regexp.MustCompile(`^192\.168\.`),                    // Class C private
		regexp.MustCompile(`^169\.254\.`),                    // Link-local
	}

	for _, pattern := range patterns {
		if pattern.MatchString(ip) {
			return true
		}
	}
	return false
}

// detectProxyEnvironment checks for proxy servers
func (mtm *MultiTransportManager) detectProxyEnvironment(profile *NetworkProfile) {
	// Check environment variables
	proxyVars := []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy"}
	for _, envVar := range proxyVars {
		if value := os.Getenv(envVar); value != "" {
			profile.ProxyDetected = true
			mtm.detectionResults["env_proxy"] = true
			break
		}
	}

	// Test for transparent proxy
	if mtm.detectTransparentProxy() {
		profile.ProxyDetected = true
		mtm.detectionResults["transparent_proxy"] = true
	}
}

// detectTransparentProxy tests for transparent HTTP proxy
func (mtm *MultiTransportManager) detectTransparentProxy() bool {
	// Make a request that should normally fail but might succeed through a proxy
	client := &http.Client{Timeout: 5 * time.Second}

	// Test with a request that includes proxy detection headers
	req, err := http.NewRequest("GET", "http://httpbin.org/headers", nil)
	if err != nil {
		return false
	}

	req.Header.Set("X-Proxy-Test", "transparent-detection")

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	// Check response headers for proxy indicators
	proxyHeaders := []string{"Via", "X-Forwarded-For", "X-Proxy-Authorization"}
	for _, header := range proxyHeaders {
		if resp.Header.Get(header) != "" {
			return true
		}
	}
	return false
}

// detectFirewallRestrictions tests for firewall restrictions
func (mtm *MultiTransportManager) detectFirewallRestrictions(profile *NetworkProfile) {
	// Test common P2P and non-standard ports
	restrictivePorts := []int{1080, 8080, 9009, 9010, 9011, 6881, 6969}
	blockedPorts := 0

	for _, port := range restrictivePorts {
		if !mtm.testSinglePortConnectivity(port, 3*time.Second) {
			blockedPorts++
		}
	}

	restrictiveness := float64(blockedPorts) / float64(len(restrictivePorts))
	if restrictiveness > 0.5 {
		profile.IsRestrictive = true
		mtm.detectionResults["port_blocking"] = true
	}
}

// detectDPIInterference tests for deep packet inspection
func (mtm *MultiTransportManager) detectDPIInterference(profile *NetworkProfile) {
	// Test for DPI by sending suspicious patterns
	if mtm.testDPIDetection() {
		profile.DPIDetected = true
		profile.IsRestrictive = true
		mtm.detectionResults["dpi_detected"] = true
	}
}

// testDPIDetection tests for deep packet inspection
func (mtm *MultiTransportManager) testDPIDetection() bool {
	// Create a connection with patterns that DPI might flag
	conn, err := net.DialTimeout("tcp", "8.8.8.8:80", 5*time.Second)
	if err != nil {
		return false
	}
	defer conn.Close()

	// Send HTTP request with BitTorrent-like patterns
	suspiciousRequest := "GET /announce?info_hash=test HTTP/1.1\r\nHost: 8.8.8.8\r\n\r\n"

	_, err = conn.Write([]byte(suspiciousRequest))
	if err != nil {
		return true // Connection terminated, might be DPI
	}

	// Set a short read timeout
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buffer := make([]byte, 1024)
	_, err = conn.Read(buffer)

	// If connection was reset or timed out suspiciously, might be DPI
	return err != nil
}

// testPortConnectivity tests connectivity to various ports
func (mtm *MultiTransportManager) testPortConnectivity(profile *NetworkProfile, ctx context.Context) {
	standardPorts := []int{21, 22, 23, 25, 53, 80, 110, 143, 443, 993, 995}
	p2pPorts := []int{1080, 6881, 6969, 8080, 9009, 9010}

	var availablePorts []int

	// Test standard ports
	for _, port := range standardPorts {
		if mtm.testSinglePortConnectivity(port, 3*time.Second) {
			availablePorts = append(availablePorts, port)
		}
	}

	// Test P2P ports
	p2pBlocked := 0
	for _, port := range p2pPorts {
		if !mtm.testSinglePortConnectivity(port, 3*time.Second) {
			p2pBlocked++
		} else {
			availablePorts = append(availablePorts, port)
		}
	}

	profile.AvailablePorts = availablePorts

	// If most P2P ports are blocked, likely restrictive
	if float64(p2pBlocked)/float64(len(p2pPorts)) > 0.7 {
		profile.IsRestrictive = true
		mtm.detectionResults["p2p_blocking"] = true
	}
}

// testSinglePortConnectivity tests connectivity to a single port
func (mtm *MultiTransportManager) testSinglePortConnectivity(port int, timeout time.Duration) bool {
	testHosts := []string{"8.8.8.8", "1.1.1.1", "google.com"}

	for _, host := range testHosts {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), timeout)
		if err == nil {
			conn.Close()
			return true
		}
	}
	return false
}

// testDNSFiltering tests for DNS filtering
func (mtm *MultiTransportManager) testDNSFiltering(profile *NetworkProfile, ctx context.Context) {
	// Test resolution of various domain types
	testDomains := []string{
		"google.com",       // Should always work
		"thepiratebay.org", // Often blocked
		"torproject.org",   // Sometimes blocked
		"blockchain.info",  // Crypto-related, sometimes blocked
	}

	blockedDomains := 0
	for _, domain := range testDomains[1:] { // Skip google.com
		if !mtm.testDNSResolution(domain) {
			blockedDomains++
			profile.BlockedDomains = append(profile.BlockedDomains, domain)
		}
	}

	if blockedDomains > 0 {
		profile.IsRestrictive = true
		mtm.detectionResults["dns_filtering"] = true
	}
}

// testDNSResolution tests if a domain resolves
func (mtm *MultiTransportManager) testDNSResolution(domain string) bool {
	_, err := net.LookupHost(domain)
	return err == nil
}

// testP2PCapability tests UDP hole punching and P2P capability
func (mtm *MultiTransportManager) testP2PCapability(profile *NetworkProfile, ctx context.Context) {
	// Test UDP connectivity
	conn, err := net.DialTimeout("udp", "8.8.8.8:53", 5*time.Second)
	if err != nil {
		profile.SupportsUDP = false
		mtm.detectionResults["udp_blocked"] = true
	} else {
		conn.Close()
		profile.SupportsUDP = true
	}

	// Test STUN servers for NAT type detection
	profile.HasWebRTC = mtm.testSTUNConnectivity()
}

// testSTUNConnectivity tests STUN server connectivity
func (mtm *MultiTransportManager) testSTUNConnectivity() bool {
	stunServers := []string{
		"stun.l.google.com:19302",
		"stun1.l.google.com:19302",
		"stun.cloudflare.com:3478",
	}

	for _, server := range stunServers {
		conn, err := net.DialTimeout("udp", server, 5*time.Second)
		if err == nil {
			conn.Close()
			return true
		}
	}
	return false
}

// classifyNetworkType determines the overall network type
func (mtm *MultiTransportManager) classifyNetworkType(profile *NetworkProfile) {
	// Start with default
	if profile.NetworkType == "unknown" {
		profile.NetworkType = "home"
	}

	// Check for mobile network indicators
	if mtm.detectMobileNetwork() {
		profile.NetworkType = "mobile"
		mtm.detectionResults["mobile_network"] = true
	}

	// Determine preferred transport based on network type
	if profile.IsRestrictive {
		profile.PreferredTransport = "https-tunnel"
	} else {
		// Even for open networks, prefer HTTPS for reliability
		profile.PreferredTransport = "https-tunnel"
	}
}

// detectMobileNetwork checks for mobile network characteristics
func (mtm *MultiTransportManager) detectMobileNetwork() bool {
	// Check for mobile network interface names (platform-specific)
	interfaces, err := net.Interfaces()
	if err != nil {
		return false
	}

	mobileIndicators := []string{
		"cellular", "mobile", "wwan", "usb", "ppp", "3g", "4g", "lte",
	}

	for _, iface := range interfaces {
		nameLower := strings.ToLower(iface.Name)
		for _, indicator := range mobileIndicators {
			if strings.Contains(nameLower, indicator) {
				return true
			}
		}
	}
	return false
}

// generateNetworkRestrictions creates restriction descriptions
func (mtm *MultiTransportManager) generateNetworkRestrictions(profile *NetworkProfile) {
	restrictions := make([]NetworkRestriction, 0)

	if mtm.detectionResults["port_blocking"] {
		restrictions = append(restrictions, NetworkRestriction{
			Type:        "firewall",
			Description: "P2P ports blocked by institutional firewall",
			Severity:    "high",
			Workaround:  "Using HTTPS transport on standard web ports",
			Confidence:  0.9,
		})
	}

	if mtm.detectionResults["dns_filtering"] {
		restrictions = append(restrictions, NetworkRestriction{
			Type:        "domain_block",
			Description: "DNS filtering detected - some domains blocked",
			Severity:    "medium",
			Workaround:  "Using IP addresses where possible",
			Confidence:  0.8,
		})
	}

	if mtm.detectionResults["dpi_detected"] {
		restrictions = append(restrictions, NetworkRestriction{
			Type:        "dpi",
			Description: "Deep Packet Inspection detected",
			Severity:    "critical",
			Workaround:  "Using encrypted HTTPS transport with standard patterns",
			Confidence:  0.7,
		})
	}

	if mtm.detectionResults["transparent_proxy"] || mtm.detectionResults["env_proxy"] {
		restrictions = append(restrictions, NetworkRestriction{
			Type:        "proxy",
			Description: "HTTP proxy detected in network path",
			Severity:    "medium",
			Workaround:  "Routing through proxy-compatible transport",
			Confidence:  0.9,
		})
	}

	if profile.IsRestrictive && len(restrictions) == 0 {
		restrictions = append(restrictions, NetworkRestriction{
			Type:        "whitelist",
			Description: "Institutional network with restrictive policies",
			Severity:    "high",
			Workaround:  "Using institutional-friendly HTTPS transport",
			Confidence:  0.8,
		})
	}

	mtm.networkRestrictions = restrictions
}

// calculateRestrictiveness calculates overall network restrictiveness
func (mtm *MultiTransportManager) calculateRestrictiveness() float64 {
	restrictiveFactors := 0
	totalFactors := 0

	factors := []string{
		"port_blocking", "dns_filtering", "dpi_detected", "transparent_proxy",
		"corporate_dns", "proxy_autoconfig", "udp_blocked", "p2p_blocking",
	}

	for _, factor := range factors {
		totalFactors++
		if mtm.detectionResults[factor] {
			restrictiveFactors++
		}
	}

	if totalFactors == 0 {
		return 0.0
	}
	return float64(restrictiveFactors) / float64(totalFactors)
}

// Rest of the methods remain the same but with enhanced error handling...

// SendWithFailover attempts to send data using the best available transport
func (mtm *MultiTransportManager) SendWithFailover(data []byte, metadata TransferMetadata) error {
	mtm.mutex.Lock()
	defer mtm.mutex.Unlock()

	// Wait for network analysis with timeout
	analysisTimeout := time.After(15 * time.Second)
	analysisTicker := time.NewTicker(200 * time.Millisecond)
	defer analysisTicker.Stop()

	for !mtm.analysisComplete {
		select {
		case <-analysisTimeout:
			fmt.Printf("Network analysis timeout, proceeding with HTTPS-first strategy\n")
			goto proceed
		case <-analysisTicker.C:
			continue
		}
	}

proceed:
	// Get ordered transports with HTTPS prioritized for institutional networks
	orderedTransports := mtm.getOrderedTransports()

	fmt.Printf("Attempting send with %d transports (network: %s, restrictive: %t)\n",
		len(orderedTransports), mtm.networkProfile.NetworkType, mtm.networkProfile.IsRestrictive)

	var lastErr error
	for transportIndex, transport := range orderedTransports {
		transportName := transport.GetName()

		// Skip recently failed transports (except first attempt always tries HTTPS)
		if transportIndex > 0 && transportName != "https-tunnel" {
			if failTime, exists := mtm.failedTransports[transportName]; exists {
				cooldownPeriod := 3 * time.Minute
				if time.Since(failTime) < cooldownPeriod {
					fmt.Printf("Skipping %s (cooldown period)\n", transportName)
					continue
				}
			}
		}

		// Test transport availability
		fmt.Printf("Trying transport: %s (priority: %d)\n", transportName, transport.GetPriority())
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		if !transport.IsAvailable(ctx) {
			cancel()
			fmt.Printf("Transport %s not available\n", transportName)
			continue
		}
		cancel()

		// Attempt transfer
		fmt.Printf("Sending via %s...\n", transportName)
		err := transport.Send(data, metadata)
		if err == nil {
			// Success
			mtm.successHistory[transportName]++
			delete(mtm.failedTransports, transportName)
			mtm.currentTransport = transport
			fmt.Printf("Send successful via %s\n", transportName)
			return nil
		}

		// Mark as failed and continue
		mtm.failedTransports[transportName] = time.Now()
		lastErr = err
		fmt.Printf("Transport %s failed: %v\n", transportName, err)
	}

	// All transports failed
	errorMsg := mtm.buildFailureErrorMessage(lastErr)
	return fmt.Errorf("%s", errorMsg)
}

// ReceiveWithFailover attempts to receive data using available transports
func (mtm *MultiTransportManager) ReceiveWithFailover(metadata TransferMetadata) ([]byte, error) {
	mtm.mutex.Lock()
	defer mtm.mutex.Unlock()

	orderedTransports := mtm.getOrderedTransports()

	fmt.Printf("Attempting receive with %d transports\n", len(orderedTransports))

	var lastErr error
	for _, transport := range orderedTransports {
		transportName := transport.GetName()

		// Test availability
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		if !transport.IsAvailable(ctx) {
			cancel()
			continue
		}
		cancel()

		fmt.Printf("Receiving via %s...\n", transportName)
		data, err := transport.Receive(metadata)
		if err == nil {
			mtm.successHistory[transportName]++
			delete(mtm.failedTransports, transportName)
			fmt.Printf("Receive successful via %s\n", transportName)
			return data, nil
		}

		mtm.failedTransports[transportName] = time.Now()
		lastErr = err
		fmt.Printf("Transport %s receive failed: %v\n", transportName, err)
	}

	errorMsg := mtm.buildFailureErrorMessage(lastErr)
	return nil, fmt.Errorf("%s", errorMsg)
}

// buildFailureErrorMessage creates helpful error messages
func (mtm *MultiTransportManager) buildFailureErrorMessage(lastErr error) string {
	var errorMsg strings.Builder

	if mtm.networkProfile.IsRestrictive {
		errorMsg.WriteString("Transfer failed in institutional network environment.\n\n")

		if len(mtm.networkRestrictions) > 0 {
			errorMsg.WriteString("Detected network restrictions:\n")
			for _, restriction := range mtm.networkRestrictions {
				errorMsg.WriteString(fmt.Sprintf("• %s: %s (confidence: %.0f%%)\n",
					restriction.Type, restriction.Description, restriction.Confidence*100))
			}
			errorMsg.WriteString("\n")
		}

		errorMsg.WriteString("This appears to be a managed network with strict policies.\n")
		errorMsg.WriteString("Troubleshooting steps:\n")
		errorMsg.WriteString("• Try from a different network (mobile hotspot, home WiFi)\n")
		errorMsg.WriteString("• Contact IT department about file transfer policies\n")
		errorMsg.WriteString("• Use a personal device with mobile data if permitted\n")
		errorMsg.WriteString("• Consider using approved enterprise file sharing solutions\n")
	} else {
		errorMsg.WriteString("Transfer failed despite open network environment.\n\n")
		errorMsg.WriteString("Troubleshooting steps:\n")
		errorMsg.WriteString("• Check internet connection stability\n")
		errorMsg.WriteString("• Verify transfer code is correct\n")
		errorMsg.WriteString("• Try again in a few minutes\n")
		errorMsg.WriteString("• Restart the application\n")
		errorMsg.WriteString("• Check if antivirus software is blocking connections\n")
	}

	if lastErr != nil {
		errorMsg.WriteString(fmt.Sprintf("\nTechnical details: %v", lastErr))
	}

	return errorMsg.String()
}

// getOrderedTransports returns transports ordered by effectiveness for current network
func (mtm *MultiTransportManager) getOrderedTransports() []Transport {
	transports := make([]Transport, len(mtm.transports))
	copy(transports, mtm.transports)

	// Sort by network-aware priority
	for i := 0; i < len(transports)-1; i++ {
		for j := i + 1; j < len(transports); j++ {
			iPriority := mtm.getEffectivePriority(transports[i])
			jPriority := mtm.getEffectivePriority(transports[j])

			if jPriority > iPriority {
				transports[i], transports[j] = transports[j], transports[i]
			}
		}
	}

	return transports
}

// getEffectivePriority calculates priority based on network conditions
func (mtm *MultiTransportManager) getEffectivePriority(transport Transport) int {
	basePriority := transport.GetPriority()
	transportName := transport.GetName()

	// Simple CROC gets highest priority (HTTPS local relay temporarily disabled)
	if transportName == "simple-croc" {
		return basePriority + 20 // High boost for CROC
	}

	// Reduce priorities for other transports to ensure CROC is preferred
	if transportName == "https-tunnel" || transportName == "https-local-relay" {
		return basePriority - 50 // Lower priority until HTTPS cross-machine issues are fixed
	}

	// Apply success/failure history
	if successCount, exists := mtm.successHistory[transportName]; exists {
		if successCount > 0 {
			return basePriority + (successCount * 5) // Boost based on success history
		}
	}

	return basePriority
}

// GetNetworkProfile returns the analyzed network profile
func (mtm *MultiTransportManager) GetNetworkProfile() NetworkProfile {
	mtm.mutex.RLock()
	defer mtm.mutex.RUnlock()
	return mtm.networkProfile
}

// GetNetworkRestrictions returns detected network restrictions
func (mtm *MultiTransportManager) GetNetworkRestrictions() []NetworkRestriction {
	mtm.mutex.RLock()
	defer mtm.mutex.RUnlock()
	return mtm.networkRestrictions
}

// GetTransportStatus returns status of all transports
func (mtm *MultiTransportManager) GetTransportStatus() map[string]interface{} {
	mtm.mutex.RLock()
	defer mtm.mutex.RUnlock()

	status := make(map[string]interface{})

	for _, transport := range mtm.transports {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		available := transport.IsAvailable(ctx)
		cancel()

		transportName := transport.GetName()
		status[transportName] = map[string]interface{}{
			"available":          available,
			"priority":           transport.GetPriority(),
			"effective_priority": mtm.getEffectivePriority(transport),
			"success_count":      mtm.successHistory[transportName],
			"last_failure":       mtm.failedTransports[transportName],
			"recommended":        transportName == mtm.networkProfile.PreferredTransport,
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
