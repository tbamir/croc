package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Block represents a single block in the blockchain
type Block struct {
	Index        int64        `json:"index"`
	Timestamp    time.Time    `json:"timestamp"`
	Data         TransferData `json:"data"`
	PreviousHash string       `json:"previous_hash"`
	Hash         string       `json:"hash"`
	Nonce        int          `json:"nonce"`
}

// TransferData contains the transfer information to be stored in a block
type TransferData struct {
	TransferID string    `json:"transfer_id"`
	PeerID     string    `json:"peer_id"`
	FileName   string    `json:"file_name"`
	FileSize   int64     `json:"file_size"`
	FileHash   string    `json:"file_hash"` // SHA-256 hash of file content
	Direction  string    `json:"direction"` // "send" or "receive"
	Status     string    `json:"status"`    // "success" or "failed"
	Error      string    `json:"error,omitempty"`
	Duration   string    `json:"duration,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// Blockchain represents the entire chain
type Blockchain struct {
	blocks      []Block
	currentHash string
	mutex       sync.RWMutex
	dbPath      string
}

// NewBlockchain creates a new blockchain or loads existing one
func NewBlockchain() (*Blockchain, error) {
	// Create blockchain directory if it doesn't exist with secure permissions
	blockchainDir := "blockchain_data"
	if err := os.MkdirAll(blockchainDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create blockchain directory: %w", err)
	}

	dbPath := filepath.Join(blockchainDir, "ledger.json")
	bc := &Blockchain{
		dbPath: dbPath,
		blocks: make([]Block, 0),
	}

	// Try to load existing blockchain
	if err := bc.loadFromDisk(); err != nil {
		// If no blockchain exists, create genesis block
		if os.IsNotExist(err) {
			bc.createGenesisBlock()
			if err := bc.saveToDisk(); err != nil {
				return nil, fmt.Errorf("failed to save genesis block: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to load blockchain: %w", err)
		}
	}

	return bc, nil
}

// createGenesisBlock creates the first block in the blockchain
func (bc *Blockchain) createGenesisBlock() {
	genesis := Block{
		Index:     0,
		Timestamp: time.Now(),
		Data: TransferData{
			TransferID: "genesis",
			PeerID:     "system",
			FileName:   "Genesis Block",
			FileSize:   0,
			FileHash:   "0000000000000000000000000000000000000000000000000000000000000000",
			Direction:  "system",
			Status:     "genesis",
			Timestamp:  time.Now(),
		},
		PreviousHash: "0",
		Nonce:        0,
	}
	genesis.Hash = bc.calculateHash(genesis)
	bc.blocks = append(bc.blocks, genesis)
	bc.currentHash = genesis.Hash
}

// calculateHash calculates the SHA-256 hash of a block
func (bc *Blockchain) calculateHash(block Block) string {
	data := fmt.Sprintf("%d%s%s%s%d",
		block.Index,
		block.Timestamp.String(),
		block.PreviousHash,
		block.Data.TransferID,
		block.Nonce)

	// Include all transfer data in hash
	transferJSON, _ := json.Marshal(block.Data)
	data += string(transferJSON)

	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// AddBlock adds a new transfer record to the blockchain
func (bc *Blockchain) AddBlock(data TransferData) error {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()

	// Create new block
	newBlock := Block{
		Index:        int64(len(bc.blocks)),
		Timestamp:    time.Now(),
		Data:         data,
		PreviousHash: bc.currentHash,
		Nonce:        0,
	}

	// Simple proof of work (find hash with leading zeros)
	for {
		newBlock.Hash = bc.calculateHash(newBlock)
		if newBlock.Hash[:2] == "00" { // Require 2 leading zeros
			break
		}
		newBlock.Nonce++
	}

	// Add block to chain
	bc.blocks = append(bc.blocks, newBlock)
	bc.currentHash = newBlock.Hash

	// Save to disk
	return bc.saveToDisk()
}

// VerifyChain verifies the integrity of the entire blockchain
func (bc *Blockchain) VerifyChain() (bool, error) {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()

	for i := 1; i < len(bc.blocks); i++ {
		currentBlock := bc.blocks[i]
		previousBlock := bc.blocks[i-1]

		// Verify current block's hash
		calculatedHash := bc.calculateHash(currentBlock)
		if currentBlock.Hash != calculatedHash {
			return false, fmt.Errorf("block %d has invalid hash", i)
		}

		// Verify link to previous block
		if currentBlock.PreviousHash != previousBlock.Hash {
			return false, fmt.Errorf("block %d has invalid previous hash link", i)
		}

		// Verify proof of work
		if currentBlock.Hash[:2] != "00" {
			return false, fmt.Errorf("block %d has invalid proof of work", i)
		}
	}

	return true, nil
}

// GetBlocks returns all blocks in the chain
func (bc *Blockchain) GetBlocks() []Block {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()

	// Return a copy to prevent external modification
	blocksCopy := make([]Block, len(bc.blocks))
	copy(blocksCopy, bc.blocks)
	return blocksCopy
}

// GetTransferHistory returns all transfer records
func (bc *Blockchain) GetTransferHistory() []TransferData {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()

	transfers := make([]TransferData, 0, len(bc.blocks)-1)
	for i := 1; i < len(bc.blocks); i++ { // Skip genesis block
		transfers = append(transfers, bc.blocks[i].Data)
	}
	return transfers
}

// saveToDisk saves the blockchain to disk with secure permissions
func (bc *Blockchain) saveToDisk() error {
	data, err := json.MarshalIndent(bc.blocks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal blockchain: %w", err)
	}

	// Write to temporary file first with secure permissions
	tempFile := bc.dbPath + ".tmp"
	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		return fmt.Errorf("failed to write blockchain to disk: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tempFile, bc.dbPath); err != nil {
		// If rename fails, try to clean up temp file
		os.Remove(tempFile)
		return fmt.Errorf("failed to save blockchain: %w", err)
	}

	// Ensure file has correct permissions
	os.Chmod(bc.dbPath, 0600)

	return nil
}

// loadFromDisk loads the blockchain from disk
func (bc *Blockchain) loadFromDisk() error {
	data, err := os.ReadFile(bc.dbPath)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, &bc.blocks); err != nil {
		return fmt.Errorf("failed to unmarshal blockchain: %w", err)
	}

	if len(bc.blocks) > 0 {
		bc.currentHash = bc.blocks[len(bc.blocks)-1].Hash
	}

	return nil
}

// ExportToJSON exports the blockchain to a JSON file with secure permissions
func (bc *Blockchain) ExportToJSON(filename string) error {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()

	data, err := json.MarshalIndent(bc.blocks, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal blockchain: %w", err)
	}

	return os.WriteFile(filename, data, 0600)
}

// GetBlockByTransferID finds a block by transfer ID
func (bc *Blockchain) GetBlockByTransferID(transferID string) (*Block, error) {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()

	for _, block := range bc.blocks {
		if block.Data.TransferID == transferID {
			blockCopy := block
			return &blockCopy, nil
		}
	}

	return nil, fmt.Errorf("block with transfer ID %s not found", transferID)
}

// GetLatestBlock returns the most recent block
func (bc *Blockchain) GetLatestBlock() *Block {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()

	if len(bc.blocks) == 0 {
		return nil
	}

	latestBlock := bc.blocks[len(bc.blocks)-1]
	return &latestBlock
}

// GetChainLength returns the number of blocks in the chain
func (bc *Blockchain) GetChainLength() int {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	return len(bc.blocks)
}

// TransferEntry represents a transfer record (compatibility adapter)
type TransferEntry struct {
	TransferCode string    `json:"transfer_code"`
	Timestamp    time.Time `json:"timestamp"`
	FileCount    int       `json:"file_count"`
	TotalSize    int64     `json:"total_size"`
	Success      bool      `json:"success"`
	Transport    string    `json:"transport"`
}

// AddTransferEntry adds a transfer entry (adapter for bulletproof manager)
func (bc *Blockchain) AddTransferEntry(entry TransferEntry) error {
	// Convert TransferEntry to TransferData
	data := TransferData{
		TransferID: entry.TransferCode,
		PeerID:     "bulletproof-system",
		FileName:   fmt.Sprintf("%d files", entry.FileCount),
		FileSize:   entry.TotalSize,
		FileHash:   "bulletproof-entry",
		Direction:  "bulletproof",
		Status:     map[bool]string{true: "success", false: "failed"}[entry.Success],
		Duration:   "",
		Timestamp:  entry.Timestamp,
	}

	return bc.AddBlock(data)
}
