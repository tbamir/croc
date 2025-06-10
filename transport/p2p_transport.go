package transport

import (
	"context"
	"fmt"
)

// DirectP2PTransport provides direct peer-to-peer file transfer
type DirectP2PTransport struct {
	priority int
	config   TransportConfig
}

// Setup initializes the direct P2P transport
func (t *DirectP2PTransport) Setup(config TransportConfig) error {
	t.config = config
	t.priority = 6
	return nil
}

// Send transmits data through direct P2P
func (t *DirectP2PTransport) Send(data []byte, metadata TransferMetadata) error {
	return fmt.Errorf("Direct P2P send not implemented yet")
}

// Receive gets data through direct P2P
func (t *DirectP2PTransport) Receive(metadata TransferMetadata) ([]byte, error) {
	return nil, fmt.Errorf("Direct P2P receive not implemented yet")
}

// IsAvailable checks if direct P2P transport is available
func (t *DirectP2PTransport) IsAvailable(ctx context.Context) bool {
	return false // Not implemented yet
}

// GetPriority returns the transport priority
func (t *DirectP2PTransport) GetPriority() int {
	return t.priority
}

// GetName returns the transport name
func (t *DirectP2PTransport) GetName() string {
	return "direct-p2p"
}

// Close cleans up the direct P2P transport
func (t *DirectP2PTransport) Close() error {
	return nil
}
