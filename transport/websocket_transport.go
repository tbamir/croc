package transport

import (
	"context"
	"fmt"
)

// WebSocketTransport provides WebSocket-based file transfer
type WebSocketTransport struct {
	priority int
	config   TransportConfig
}

// Setup initializes the WebSocket transport
func (t *WebSocketTransport) Setup(config TransportConfig) error {
	t.config = config
	t.priority = 40
	return nil
}

// Send transmits data through WebSocket
func (t *WebSocketTransport) Send(data []byte, metadata TransferMetadata) error {
	return fmt.Errorf("WebSocket send not implemented yet")
}

// Receive gets data through WebSocket
func (t *WebSocketTransport) Receive(metadata TransferMetadata) ([]byte, error) {
	return nil, fmt.Errorf("WebSocket receive not implemented yet")
}

// IsAvailable checks if WebSocket transport is available
func (t *WebSocketTransport) IsAvailable(ctx context.Context) bool {
	return false // Not implemented yet
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