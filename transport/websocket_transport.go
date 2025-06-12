package transport

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketTransport provides firewall-friendly WebSocket-based file transfer
type WebSocketTransport struct {
	priority        int
	config          TransportConfig
	echoServiceURLs []string
	dialer          *websocket.Dialer
}

// WebSocketMessage represents a message sent over WebSocket
type WebSocketMessage struct {
	Type       string `json:"type"` // "transfer_data", "transfer_request", "ack"
	TransferID string `json:"transfer_id"`
	Data       string `json:"data"` // Base64 encoded
	FileName   string `json:"file_name"`
	FileSize   int64  `json:"file_size"`
	Checksum   string `json:"checksum"`
	Timestamp  int64  `json:"timestamp"`
}

// Setup initializes the WebSocket transport with public echo services
func (t *WebSocketTransport) Setup(config TransportConfig) error {
	t.config = config
	t.priority = 70 // Good priority for corporate networks

	// Use public WebSocket echo services that work through most firewalls
	t.echoServiceURLs = []string{
		"wss://ws.postman-echo.com/raw",         // Postman WebSocket echo
		"wss://echo.websocket.org/",             // WebSocket.org echo service
		"ws://echo.websocket.org/",              // Fallback to non-SSL
		"wss://ws.ifelse.io/",                   // Alternative echo service
		"wss://socketsbay.com/wss/v2/1/demo/",   // Another public service
		"wss://demo.piesocket.com/v3/channel_1", // PieSocket demo
	}

	// Create WebSocket dialer optimized for corporate networks
	t.dialer = &websocket.Dialer{
		HandshakeTimeout:  45 * time.Second,
		Proxy:             http.ProxyFromEnvironment, // Use system proxy settings
		EnableCompression: true,
	}

	fmt.Printf("WebSocket transport initialized with %d echo services\n", len(t.echoServiceURLs))
	return nil
}

// Send transmits data using WebSocket echo services
func (t *WebSocketTransport) Send(data []byte, metadata TransferMetadata) error {
	// Encode data as base64
	encodedData := base64.StdEncoding.EncodeToString(data)

	// Check size limit for WebSocket (most services have limits)
	maxSize := 512 * 1024 // 512KB limit for WebSocket messages
	if len(encodedData) > maxSize {
		return fmt.Errorf("file too large for WebSocket transport (%d bytes, max %d)", len(encodedData), maxSize)
	}

	// Create WebSocket message
	message := WebSocketMessage{
		Type:       "transfer_data",
		TransferID: metadata.TransferID,
		Data:       encodedData,
		FileName:   metadata.FileName,
		FileSize:   metadata.FileSize,
		Checksum:   metadata.Checksum,
		Timestamp:  time.Now().Unix(),
	}

	// Try each WebSocket echo service
	var lastErr error
	for i, wsURL := range t.echoServiceURLs {
		fmt.Printf("Attempting WebSocket service %d/%d: %s\n", i+1, len(t.echoServiceURLs), wsURL)

		err := t.sendViaWebSocket(message, wsURL)
		if err == nil {
			fmt.Printf("✅ WebSocket send successful via: %s\n", wsURL)
			return nil
		}

		lastErr = err
		fmt.Printf("❌ WebSocket service failed: %v\n", err)

		// Brief delay between attempts
		if i < len(t.echoServiceURLs)-1 {
			time.Sleep(2 * time.Second)
		}
	}

	return fmt.Errorf("all WebSocket services failed: %w", lastErr)
}

// sendViaWebSocket sends data through a specific WebSocket service
func (t *WebSocketTransport) sendViaWebSocket(message WebSocketMessage, wsURL string) error {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Parse URL to validate
	_, err := url.Parse(wsURL)
	if err != nil {
		return fmt.Errorf("invalid WebSocket URL: %w", err)
	}

	// Connect to WebSocket
	conn, _, err := t.dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}
	defer conn.Close()

	// Set write deadline
	conn.SetWriteDeadline(time.Now().Add(30 * time.Second))

	// Send message
	if err := conn.WriteJSON(message); err != nil {
		return fmt.Errorf("failed to send WebSocket message: %w", err)
	}

	// Wait for echo response to confirm delivery
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	var response WebSocketMessage
	if err := conn.ReadJSON(&response); err != nil {
		return fmt.Errorf("failed to read WebSocket response: %w", err)
	}

	// Verify echo response
	if response.TransferID != message.TransferID {
		return fmt.Errorf("WebSocket echo response mismatch")
	}

	fmt.Printf("WebSocket echo confirmed for transfer %s\n", message.TransferID)
	return nil
}

// Receive gets data using WebSocket echo services
func (t *WebSocketTransport) Receive(metadata TransferMetadata) ([]byte, error) {
	// Try each WebSocket service to find the echoed data
	var lastErr error

	for i, wsURL := range t.echoServiceURLs {
		fmt.Printf("Checking WebSocket service %d/%d for transfer %s\n", i+1, len(t.echoServiceURLs), metadata.TransferID)

		data, err := t.receiveViaWebSocket(metadata, wsURL)
		if err == nil && len(data) > 0 {
			fmt.Printf("✅ WebSocket receive successful via: %s (%d bytes)\n", wsURL, len(data))
			return data, nil
		}

		if err != nil {
			lastErr = err
			fmt.Printf("❌ WebSocket service check failed: %v\n", err)
		}

		// Brief delay between checks
		if i < len(t.echoServiceURLs)-1 {
			time.Sleep(1 * time.Second)
		}
	}

	return nil, fmt.Errorf("transfer not found on any WebSocket service: %w", lastErr)
}

// receiveViaWebSocket retrieves data from a WebSocket service
func (t *WebSocketTransport) receiveViaWebSocket(metadata TransferMetadata, wsURL string) ([]byte, error) {
	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	// Connect to WebSocket
	conn, _, err := t.dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}
	defer conn.Close()

	// Send request for transfer
	request := WebSocketMessage{
		Type:       "transfer_request",
		TransferID: metadata.TransferID,
		Timestamp:  time.Now().Unix(),
	}

	conn.SetWriteDeadline(time.Now().Add(20 * time.Second))
	if err := conn.WriteJSON(request); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Wait for response
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	var response WebSocketMessage
	if err := conn.ReadJSON(&response); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check if we got the right transfer
	if response.TransferID != metadata.TransferID || response.Type != "transfer_data" {
		return nil, fmt.Errorf("transfer not found or wrong type")
	}

	// Decode base64 data
	data, err := base64.StdEncoding.DecodeString(response.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to decode data: %w", err)
	}

	return data, nil
}

// IsAvailable checks if WebSocket transport is available
func (t *WebSocketTransport) IsAvailable(ctx context.Context) bool {
	// Test connectivity to first WebSocket service
	if len(t.echoServiceURLs) == 0 {
		return false
	}

	// Quick connectivity test
	testURL := t.echoServiceURLs[0]
	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, _, err := t.dialer.DialContext(testCtx, testURL, nil)
	if err != nil {
		fmt.Printf("WebSocket availability test failed: %v\n", err)
		return false
	}
	defer conn.Close()

	fmt.Printf("WebSocket transport available via: %s\n", testURL)
	return true
}

// GetPriority returns the transport priority
func (t *WebSocketTransport) GetPriority() int {
	return t.priority
}

// GetName returns the transport name
func (t *WebSocketTransport) GetName() string {
	return "websocket"
}

// Close cleans up the WebSocket transport
func (t *WebSocketTransport) Close() error {
	return nil
}

// NewWebSocketTransport creates a new WebSocket transport
func NewWebSocketTransport(priority int) *WebSocketTransport {
	transport := &WebSocketTransport{
		priority: priority,
	}

	// Setup with default config
	transport.Setup(TransportConfig{})

	return transport
}
