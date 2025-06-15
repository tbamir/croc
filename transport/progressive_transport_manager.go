package transport

import (
	"context"
	"fmt"
	"log"
	"net"
	"strings"
	"time"
)

// ProgressiveTransportManager implements intelligent transport learning and adaptation
type ProgressiveTransportManager struct {
	transports []TransportLayer
	analytics  *TransportAnalytics
}

// TransportLayer represents a transport with performance metrics
type TransportLayer struct {
	Name             string
	Transport        Transport
	SuccessRate      float64
	AvgLatency       time.Duration
	ReliabilityScore float64
}

// TransportAnalytics tracks transport performance for learning
type TransportAnalytics struct {
	NetworkType    string
	SuccessHistory map[string][]bool          // transport -> recent success/failures
	LatencyHistory map[string][]time.Duration // transport -> recent latencies
	ErrorPatterns  map[string]map[string]int  // transport -> error_type -> count
	LastUpdate     time.Time
}

// NewProgressiveTransportManager creates a new learning transport manager
func NewProgressiveTransportManager() *ProgressiveTransportManager {
	ptm := &ProgressiveTransportManager{
		analytics: &TransportAnalytics{
			SuccessHistory: make(map[string][]bool),
			LatencyHistory: make(map[string][]time.Duration),
			ErrorPatterns:  make(map[string]map[string]int),
			LastUpdate:     time.Now(),
		},
	}

	// 2024 optimal transport order for corporate networks based on research
	ptm.transports = []TransportLayer{
		{Name: "ice-webrtc", Transport: NewICETransport(95), SuccessRate: 0.95},          // WebRTC-style NAT traversal
		{Name: "https-443", Transport: NewHTTPSTunnelTransport(90), SuccessRate: 0.88},   // HTTPS on port 443
		{Name: "croc-relay", Transport: NewCrocTransport(85), SuccessRate: 0.82},         // CROC with relay servers
		{Name: "websocket-443", Transport: NewWebSocketTransport(75), SuccessRate: 0.75}, // WebSocket over HTTPS
		{Name: "tor-proxy", Transport: &TorTransport{priority: 60}, SuccessRate: 0.65},   // Tor as last resort
	}

	// Initialize analytics for each transport
	for _, layer := range ptm.transports {
		ptm.analytics.SuccessHistory[layer.Name] = make([]bool, 0)
		ptm.analytics.LatencyHistory[layer.Name] = make([]time.Duration, 0)
		ptm.analytics.ErrorPatterns[layer.Name] = make(map[string]int)
	}

	fmt.Printf("Progressive transport manager initialized with %d transports\n", len(ptm.transports))
	return ptm
}

func (ptm *ProgressiveTransportManager) SendWithIntelligentFallback(data []byte, metadata TransferMetadata) error {
	// INTERNATIONAL RELIABILITY STRATEGY
	orderedTransports := ptm.getInternationalOptimizedOrder()

	fmt.Printf("🌍 Starting international intelligent transport fallback with %d options\n", len(orderedTransports))

	// Pre-transfer network quality assessment
	networkQuality := ptm.assessNetworkQuality()
	fmt.Printf("📊 Network quality score: %.1f/10\n", networkQuality)

	for i, layer := range orderedTransports {
		startTime := time.Now()

		// Dynamic timeout based on network quality and attempt number
		timeoutMultiplier := 1.0 + float64(i)*0.5 // Increase timeout with attempts
		if networkQuality < 5.0 {
			timeoutMultiplier *= 2.0 // Double timeouts for poor networks
		}

		fmt.Printf("🚀 International attempt %d/%d: %s (quality-adjusted timeout: %.1fx)\n",
			i+1, len(orderedTransports), layer.Name, timeoutMultiplier)

		// Create context with quality-adjusted timeout
		baseTimeout := 60 * time.Second
		adjustedTimeout := time.Duration(float64(baseTimeout) * timeoutMultiplier)
		ctx, cancel := context.WithTimeout(context.Background(), adjustedTimeout)

		// Attempt transfer with timeout
		errChan := make(chan error, 1)
		go func() {
			errChan <- layer.Transport.Send(data, metadata)
		}()

		var err error
		select {
		case err = <-errChan:
			cancel()
		case <-ctx.Done():
			cancel()
			err = fmt.Errorf("international transfer timeout after %v", adjustedTimeout)
		}

		latency := time.Since(startTime)

		// Record analytics for international learning
		ptm.recordInternationalAttempt(layer.Name, err == nil, latency, err, networkQuality)

		if err == nil {
			fmt.Printf("✅ International success via %s in %v (quality: %.1f)\n", layer.Name, latency, networkQuality)
			ptm.updateSuccessRate(layer.Name, true)
			return nil
		}

		fmt.Printf("❌ %s failed in %v: %v\n", layer.Name, latency, err)
		ptm.updateSuccessRate(layer.Name, false)

		// International-aware progressive backoff
		if i < len(orderedTransports)-1 {
			delay := ptm.calculateInternationalBackoff(i, networkQuality, err)
			fmt.Printf("⏳ Waiting %v before next international attempt...\n", delay)
			time.Sleep(delay)
		}
	}

	return fmt.Errorf("all %d international transports failed after intelligent fallback", len(orderedTransports))
}

// ReceiveWithIntelligentFallback attempts to receive data using learned transport preferences
func (ptm *ProgressiveTransportManager) ReceiveWithIntelligentFallback(metadata TransferMetadata) ([]byte, error) {
	orderedTransports := ptm.getNetworkOptimizedOrder()

	fmt.Printf("🎯 Starting intelligent receive with %d options\n", len(orderedTransports))

	for i, layer := range orderedTransports {
		startTime := time.Now()

		fmt.Printf("🔄 Receive attempt %d/%d: %s\n", i+1, len(orderedTransports), layer.Name)

		data, err := layer.Transport.Receive(metadata)
		latency := time.Since(startTime)

		// Record analytics
		ptm.recordAttempt(layer.Name, err == nil && len(data) > 0, latency, err)

		if err == nil && len(data) > 0 {
			fmt.Printf("✅ Received %d bytes via %s in %v\n", len(data), layer.Name, latency)
			ptm.updateSuccessRate(layer.Name, true)
			return data, nil
		}

		if err != nil {
			fmt.Printf("❌ %s receive failed in %v: %v\n", layer.Name, latency, err)
		}
		ptm.updateSuccessRate(layer.Name, false)

		// Quick retry delay for receive operations
		if i < len(orderedTransports)-1 {
			delay := time.Duration(1+i) * time.Second // 1s, 2s, 3s, 4s...
			time.Sleep(delay)
		}
	}

	return nil, fmt.Errorf("all %d transports failed to receive data", len(orderedTransports))
}

// getNetworkOptimizedOrder adapts transport order based on learned performance
func (ptm *ProgressiveTransportManager) getNetworkOptimizedOrder() []TransportLayer {
	// Create a copy to avoid modifying original
	optimized := make([]TransportLayer, len(ptm.transports))
	copy(optimized, ptm.transports)

	// Update reliability scores based on recent performance
	for i := range optimized {
		name := optimized[i].Name

		// Calculate reliability score based on recent history
		recentSuccess := ptm.getRecentSuccessRate(name)
		avgLatency := ptm.getAverageLatency(name)

		// Weighted scoring: 70% success rate, 30% speed
		reliabilityScore := recentSuccess*0.7 + (1.0-ptm.normalizeLatency(avgLatency))*0.3
		optimized[i].ReliabilityScore = reliabilityScore
		optimized[i].SuccessRate = recentSuccess
		optimized[i].AvgLatency = avgLatency

		fmt.Printf("📊 %s: Success=%.1f%%, Latency=%v, Score=%.2f\n",
			name, recentSuccess*100, avgLatency, reliabilityScore)
	}

	// Sort by reliability score (higher is better)
	for i := 0; i < len(optimized)-1; i++ {
		for j := i + 1; j < len(optimized); j++ {
			if optimized[j].ReliabilityScore > optimized[i].ReliabilityScore {
				optimized[i], optimized[j] = optimized[j], optimized[i]
			}
		}
	}

	return optimized
}

// recordAttempt records the outcome of a transport attempt for learning
func (ptm *ProgressiveTransportManager) recordAttempt(transportName string, success bool, latency time.Duration, err error) {
	// Record success/failure
	history := ptm.analytics.SuccessHistory[transportName]
	history = append(history, success)

	// Keep only last 20 attempts for recent performance
	if len(history) > 20 {
		history = history[len(history)-20:]
	}
	ptm.analytics.SuccessHistory[transportName] = history

	// Record latency
	latencyHistory := ptm.analytics.LatencyHistory[transportName]
	latencyHistory = append(latencyHistory, latency)

	// Keep only last 10 latency measurements
	if len(latencyHistory) > 10 {
		latencyHistory = latencyHistory[len(latencyHistory)-10:]
	}
	ptm.analytics.LatencyHistory[transportName] = latencyHistory

	// Record error patterns
	if err != nil {
		errorType := ptm.classifyError(err)
		if ptm.analytics.ErrorPatterns[transportName] == nil {
			ptm.analytics.ErrorPatterns[transportName] = make(map[string]int)
		}
		ptm.analytics.ErrorPatterns[transportName][errorType]++
	}

	ptm.analytics.LastUpdate = time.Now()
}

// getRecentSuccessRate calculates success rate from recent attempts
func (ptm *ProgressiveTransportManager) getRecentSuccessRate(transportName string) float64 {
	history := ptm.analytics.SuccessHistory[transportName]
	if len(history) == 0 {
		return 0.5 // Neutral starting point
	}

	successCount := 0
	for _, success := range history {
		if success {
			successCount++
		}
	}

	return float64(successCount) / float64(len(history))
}

// getAverageLatency calculates average latency from recent attempts
func (ptm *ProgressiveTransportManager) getAverageLatency(transportName string) time.Duration {
	history := ptm.analytics.LatencyHistory[transportName]
	if len(history) == 0 {
		return 10 * time.Second // Default assumption
	}

	var total time.Duration
	for _, latency := range history {
		total += latency
	}

	return total / time.Duration(len(history))
}

// normalizeLatency converts latency to a 0-1 score (lower latency = higher score)
func (ptm *ProgressiveTransportManager) normalizeLatency(latency time.Duration) float64 {
	// Normalize latency: 0-5s = 1.0, 5-15s = 0.5, >15s = 0.0
	seconds := latency.Seconds()
	switch {
	case seconds <= 5:
		return 1.0 - (seconds / 10) // 0-5s maps to 1.0-0.5
	case seconds <= 15:
		return 0.5 - ((seconds - 5) / 20) // 5-15s maps to 0.5-0.0
	default:
		return 0.0 // >15s is very slow
	}
}

// updateSuccessRate updates the base success rate for a transport
func (ptm *ProgressiveTransportManager) updateSuccessRate(transportName string, success bool) {
	for i := range ptm.transports {
		if ptm.transports[i].Name == transportName {
			currentRate := ptm.transports[i].SuccessRate

			// Exponential moving average with recent bias
			if success {
				ptm.transports[i].SuccessRate = currentRate*0.9 + 0.1
			} else {
				ptm.transports[i].SuccessRate = currentRate * 0.9
			}

			// Ensure bounds
			if ptm.transports[i].SuccessRate > 1.0 {
				ptm.transports[i].SuccessRate = 1.0
			}
			if ptm.transports[i].SuccessRate < 0.0 {
				ptm.transports[i].SuccessRate = 0.0
			}
			break
		}
	}
}

// classifyError categorizes errors for pattern learning
func (ptm *ProgressiveTransportManager) classifyError(err error) string {
	if err == nil {
		return "success"
	}

	errorStr := err.Error()
	switch {
	case contains(errorStr, "connection refused", "network unreachable"):
		return "network_blocked"
	case contains(errorStr, "timeout", "deadline exceeded"):
		return "timeout"
	case contains(errorStr, "dns", "no such host"):
		return "dns_failure"
	case contains(errorStr, "proxy", "authentication"):
		return "proxy_issue"
	case contains(errorStr, "encrypt", "decrypt", "security"):
		return "security_error"
	default:
		return "unknown"
	}
}

// contains checks if any of the patterns exist in the string
func contains(str string, patterns ...string) bool {
	for _, pattern := range patterns {
		if len(str) >= len(pattern) {
			for i := 0; i <= len(str)-len(pattern); i++ {
				if str[i:i+len(pattern)] == pattern {
					return true
				}
			}
		}
	}
	return false
}

// GetAnalytics returns current transport analytics
func (ptm *ProgressiveTransportManager) GetAnalytics() *TransportAnalytics {
	return ptm.analytics
}

// GetTransportStatus returns detailed status of all transports
func (ptm *ProgressiveTransportManager) GetTransportStatus() map[string]interface{} {
	status := make(map[string]interface{})

	for _, layer := range ptm.transports {
		transportStatus := map[string]interface{}{
			"name":              layer.Name,
			"success_rate":      layer.SuccessRate,
			"avg_latency":       layer.AvgLatency.String(),
			"reliability_score": layer.ReliabilityScore,
			"recent_attempts":   len(ptm.analytics.SuccessHistory[layer.Name]),
			"error_patterns":    ptm.analytics.ErrorPatterns[layer.Name],
		}
		status[layer.Name] = transportStatus
	}

	status["last_update"] = ptm.analytics.LastUpdate
	status["network_type"] = ptm.analytics.NetworkType

	return status
}

// Close cleans up the progressive transport manager
func (ptm *ProgressiveTransportManager) Close() error {
	for _, layer := range ptm.transports {
		if err := layer.Transport.Close(); err != nil {
			log.Printf("Error closing transport %s: %v", layer.Name, err)
		}
	}
	return nil
}

// getInternationalOptimizedOrder adapts transport order for international transfers
func (ptm *ProgressiveTransportManager) getInternationalOptimizedOrder() []TransportLayer {
	// Use existing network optimization as base
	return ptm.getNetworkOptimizedOrder()
}

// assessNetworkQuality provides a 0-10 score for international network quality
func (ptm *ProgressiveTransportManager) assessNetworkQuality() float64 {
	score := 10.0 // Start optimistic

	// Test multiple international endpoints
	endpoints := []string{
		"8.8.8.8:53",
		"1.1.1.1:53",
		"croc.schollz.com:443",
		"google.com:443",
	}

	var totalLatency time.Duration
	var successCount int

	for _, endpoint := range endpoints {
		start := time.Now()
		conn, err := net.DialTimeout("tcp", endpoint, 10*time.Second)
		latency := time.Since(start)

		if err == nil {
			conn.Close()
			totalLatency += latency
			successCount++
		} else {
			score -= 1.5 // Penalty for failed connections
		}
	}

	if successCount > 0 {
		avgLatency := totalLatency / time.Duration(successCount)

		// Latency scoring (international focus)
		if avgLatency > 1000*time.Millisecond {
			score -= 4.0 // Very high latency
		} else if avgLatency > 500*time.Millisecond {
			score -= 2.0 // High latency
		} else if avgLatency > 200*time.Millisecond {
			score -= 1.0 // Moderate latency
		}
	} else {
		score = 1.0 // Minimum score if no connections work
	}

	// Ensure score is in valid range
	if score < 0 {
		score = 0
	}
	if score > 10 {
		score = 10
	}

	return score
}

// calculateInternationalBackoff calculates delays for international context
func (ptm *ProgressiveTransportManager) calculateInternationalBackoff(attempt int, quality float64, err error) time.Duration {
	// Base delay increases with attempts
	baseDelay := time.Duration(5+attempt*3) * time.Second

	// Quality adjustments
	if quality < 3.0 {
		baseDelay *= 3 // Much longer delays for very poor networks
	} else if quality < 6.0 {
		baseDelay *= 2 // Longer delays for poor networks
	}

	// Error-specific adjustments
	if err != nil {
		errorStr := strings.ToLower(err.Error())
		if strings.Contains(errorStr, "timeout") {
			baseDelay *= 2 // Longer delays after timeouts
		} else if strings.Contains(errorStr, "connection refused") {
			baseDelay = time.Duration(float64(baseDelay) * 1.5) // Moderate delays after connection failures
		}
	}

	// Cap maximum delay
	maxDelay := 60 * time.Second
	if baseDelay > maxDelay {
		baseDelay = maxDelay
	}

	return baseDelay
}

// recordInternationalAttempt records attempts with international context
func (ptm *ProgressiveTransportManager) recordInternationalAttempt(transportName string, success bool, latency time.Duration, err error, _ float64) {
	// Enhanced recording for international transfers
	ptm.recordAttempt(transportName, success, latency, err)

	// Additional international-specific metrics could be stored here
	// For example: quality scores, geographic routing information, etc.
}
