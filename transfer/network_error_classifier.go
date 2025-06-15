package transfer

import (
	"strings"
	"time"
)

// NetworkErrorClassifier analyzes errors and provides actionable guidance
type NetworkErrorClassifier struct {
	patterns map[string]ErrorPattern
}

// ErrorPattern represents a recognized error pattern with guidance
type ErrorPattern struct {
	Keywords   []string // Keywords to match in error messages
	Category   string   // Category of the error
	Severity   string   // Severity level
	Suggested  []string // Suggested transports to try
	UserAction string   // User-friendly action guidance
}

// ErrorClassification provides detailed error analysis
type ErrorClassification struct {
	Category            string
	Severity            string
	SuggestedTransports []string
	UserAction          string
	TechnicalDetails    string
	OriginalTransport   string
	NetworkGuidance     string
}

// EnhancedTransferError combines error classification with additional context
type EnhancedTransferError struct {
	Classification ErrorClassification
	OriginalError  error
	Timestamp      time.Time
}

// Error implements the error interface
func (ete *EnhancedTransferError) Error() string {
	return ete.Classification.UserAction
}

// NewNetworkErrorClassifier creates a new error classifier with 2024 patterns
func NewNetworkErrorClassifier() *NetworkErrorClassifier {
	nec := &NetworkErrorClassifier{
		patterns: make(map[string]ErrorPattern),
	}

	// 2024 corporate firewall error patterns based on research
	nec.patterns["dpi_detected"] = ErrorPattern{
		Keywords:   []string{"connection reset", "connection interrupted", "unexpected eof", "broken pipe"},
		Category:   "deep_packet_inspection",
		Severity:   "high",
		Suggested:  []string{"ice-webrtc", "websocket-443", "tor-proxy"},
		UserAction: "Your network uses deep packet inspection (DPI). Switching to more secure protocols.",
	}

	nec.patterns["port_blocked"] = ErrorPattern{
		Keywords:   []string{"connection refused", "no route to host", "network unreachable", "port unreachable"},
		Category:   "firewall_port_block",
		Severity:   "medium",
		Suggested:  []string{"https-443", "websocket-443", "ice-webrtc"},
		UserAction: "Network blocks P2P ports. Using standard web ports (80/443) instead.",
	}

	nec.patterns["dns_filtered"] = ErrorPattern{
		Keywords:   []string{"no such host", "dns resolution failed", "server not found", "name resolution"},
		Category:   "dns_filtering",
		Severity:   "medium",
		Suggested:  []string{"ice-webrtc", "https-443"},
		UserAction: "DNS filtering detected. Using IP addresses and alternative connection methods.",
	}

	nec.patterns["proxy_required"] = ErrorPattern{
		Keywords:   []string{"proxy authentication", "proxy required", "407 proxy", "proxy error"},
		Category:   "corporate_proxy",
		Severity:   "high",
		Suggested:  []string{"websocket-443", "https-443"},
		UserAction: "Corporate proxy detected. Contact IT about file transfer permissions or use mobile hotspot.",
	}

	nec.patterns["ssl_inspection"] = ErrorPattern{
		Keywords:   []string{"certificate", "ssl error", "tls handshake", "certificate verification"},
		Category:   "ssl_mitm",
		Severity:   "high",
		Suggested:  []string{"ice-webrtc", "tor-proxy"},
		UserAction: "SSL inspection detected. Using end-to-end encrypted protocols that bypass inspection.",
	}

	nec.patterns["bandwidth_throttling"] = ErrorPattern{
		Keywords:   []string{"timeout", "deadline exceeded", "i/o timeout", "slow network"},
		Category:   "network_throttling",
		Severity:   "low",
		Suggested:  []string{"https-443", "croc-relay"},
		UserAction: "Network appears slow or throttled. Trying more efficient transfer methods.",
	}

	nec.patterns["geo_blocking"] = ErrorPattern{
		Keywords:   []string{"forbidden", "blocked", "geo", "region", "country"},
		Category:   "geographic_restriction",
		Severity:   "medium",
		Suggested:  []string{"tor-proxy", "ice-webrtc"},
		UserAction: "Geographic restrictions detected. Using privacy-focused protocols to bypass.",
	}

	nec.patterns["application_blocking"] = ErrorPattern{
		Keywords:   []string{"application blocked", "category blocked", "policy violation", "content filtered"},
		Category:   "application_firewall",
		Severity:   "high",
		Suggested:  []string{"websocket-443", "https-443"},
		UserAction: "Application firewall blocking file transfers. Using web-based protocols.",
	}

	return nec
}

// ClassifyError provides comprehensive error analysis
func (nec *NetworkErrorClassifier) ClassifyError(err error, attemptedTransport string) ErrorClassification {
	if err == nil {
		return ErrorClassification{Category: "success"}
	}

	errorText := strings.ToLower(err.Error())

	// Check against known patterns
	for _, pattern := range nec.patterns {
		for _, keyword := range pattern.Keywords {
			if strings.Contains(errorText, keyword) {
				return ErrorClassification{
					Category:            pattern.Category,
					Severity:            pattern.Severity,
					SuggestedTransports: pattern.Suggested,
					UserAction:          pattern.UserAction,
					TechnicalDetails:    err.Error(),
					OriginalTransport:   attemptedTransport,
					NetworkGuidance:     nec.getNetworkSpecificGuidance(pattern.Category),
				}
			}
		}
	}

	// Default classification for unknown errors
	return ErrorClassification{
		Category:            "unknown",
		Severity:            "medium",
		UserAction:          "Network issue detected. Trying alternative connection methods.",
		TechnicalDetails:    err.Error(),
		SuggestedTransports: []string{"ice-webrtc", "https-443", "croc-relay"},
		NetworkGuidance:     "If problems persist, try connecting from a different network.",
	}
}

// getNetworkSpecificGuidance provides detailed guidance based on error category
func (nec *NetworkErrorClassifier) getNetworkSpecificGuidance(category string) string {
	switch category {
	case "deep_packet_inspection":
		return `Your network inspects all data packets. Solutions:
1. Use TrustDrop's WebRTC mode (bypasses DPI)
2. Try mobile hotspot
3. Contact IT about whitelist exceptions`

	case "firewall_port_block":
		return `Standard P2P ports are blocked. Solutions:
1. TrustDrop will use ports 80/443 (web traffic)
2. If still blocked, try mobile hotspot
3. Contact network admin about port exceptions`

	case "dns_filtering":
		return `DNS queries are being filtered. Solutions:
1. TrustDrop will use direct IP connections
2. Try changing DNS settings to 8.8.8.8
3. Use mobile hotspot if available`

	case "corporate_proxy":
		return `All traffic goes through corporate proxy. Solutions:
1. Configure TrustDrop to use proxy settings
2. Contact IT for proxy exceptions
3. Use mobile hotspot for important transfers`

	case "ssl_mitm":
		return `SSL traffic is being intercepted and inspected. Solutions:
1. Use TrustDrop's encrypted P2P mode
2. Try Tor mode for maximum privacy
3. Mobile hotspot bypasses SSL inspection`

	case "network_throttling":
		return `Network is slow or throttled. Solutions:
1. Try during off-peak hours (lunch/evening)
2. Use TrustDrop's compression features
3. Break large transfers into smaller chunks`

	case "geographic_restriction":
		return `Your location or destination is geo-blocked. Solutions:
1. Use TrustDrop's privacy mode
2. Try Tor transport if available
3. Contact network admin about restrictions`

	case "application_firewall":
		return `Application-level firewall blocking file transfers. Solutions:
1. TrustDrop will disguise as web traffic
2. Try during different times of day
3. Contact IT about business justification`

	default:
		return "Try alternative connection methods or contact your network administrator."
	}
}

// GetUserFriendlyMessage converts technical errors to user-friendly messages
func (nec *NetworkErrorClassifier) GetUserFriendlyMessage(err error, networkType string) string {
	classification := nec.ClassifyError(err, "unknown")

	baseMessage := classification.UserAction

	// Add network-specific context
	switch networkType {
	case "corporate":
		return baseMessage + "\n\nCorporate networks often have strict security policies. TrustDrop is designed to work around these restrictions."

	case "university":
		return baseMessage + "\n\nUniversity networks typically block P2P traffic. TrustDrop uses web-compatible protocols to bypass these restrictions."

	case "public":
		return baseMessage + "\n\nPublic WiFi networks may have limitations. Consider using mobile data for important transfers."

	case "home":
		return baseMessage + "\n\nHome network issue detected. Check your internet connection or try restarting your router."

	default:
		return baseMessage
	}
}

// GetRecommendedActions returns a prioritized list of actions to try
func (nec *NetworkErrorClassifier) GetRecommendedActions(err error, networkType string) []string {
	classification := nec.ClassifyError(err, "unknown")

	var actions []string

	// Add error-specific actions
	switch classification.Category {
	case "deep_packet_inspection":
		actions = append(actions, "Switch to WebRTC mode", "Use mobile hotspot", "Contact IT for whitelist")

	case "firewall_port_block":
		actions = append(actions, "Try standard web ports", "Use mobile hotspot", "Request port exceptions from IT")

	case "dns_filtering":
		actions = append(actions, "Use direct IP connections", "Change DNS to 8.8.8.8", "Try mobile hotspot")

	case "corporate_proxy":
		actions = append(actions, "Configure proxy settings", "Contact IT", "Use mobile hotspot")

	case "network_throttling":
		actions = append(actions, "Try during off-peak hours", "Use compression", "Break into smaller transfers")

	default:
		actions = append(actions, "Try alternative protocols", "Check internet connection", "Use mobile hotspot")
	}

	// Add network-type specific actions
	switch networkType {
	case "corporate":
		actions = append(actions, "Contact IT department", "Try guest WiFi", "Use mobile data")

	case "university":
		actions = append(actions, "Try guest network", "Use mobile data", "Contact network support")

	case "public":
		actions = append(actions, "Move closer to router", "Try mobile data", "Find different WiFi")
	}

	return actions
}

// IsTransientError determines if the error is likely to resolve on retry
func (nec *NetworkErrorClassifier) IsTransientError(err error) bool {
	classification := nec.ClassifyError(err, "unknown")

	switch classification.Category {
	case "network_throttling", "unknown":
		return true // These often resolve on retry

	case "deep_packet_inspection", "firewall_port_block", "corporate_proxy":
		return false // These require different transports, not retries

	default:
		return true // Default to allowing retries
	}
}

// GetRetryStrategy returns appropriate retry strategy for the error
func (nec *NetworkErrorClassifier) GetRetryStrategy(err error) (maxRetries int, delay time.Duration) {
	classification := nec.ClassifyError(err, "unknown")

	switch classification.Category {
	case "network_throttling":
		return 5, 10 * time.Second // More retries with longer delays

	case "dns_filtering":
		return 3, 5 * time.Second // Quick retries

	case "deep_packet_inspection", "firewall_port_block":
		return 1, 1 * time.Second // Don't retry much, switch transports

	default:
		return 3, 3 * time.Second // Default strategy
	}
}
