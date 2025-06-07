package logging

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"trustdrop/blockchain"
)

type TransferLog struct {
	Timestamp time.Time `json:"timestamp"`
	PeerID    string    `json:"peer_id"`
	FileName  string    `json:"file_name"`
	FileHash  string    `json:"file_hash"`  // SHA-256 hash of file content
	FileSize  int64     `json:"file_size"`
	Direction string    `json:"direction"` // "send" or "receive"
	Status    string    `json:"status"`    // "success" or "failed"
	Error     string    `json:"error,omitempty"`
	Duration  string    `json:"duration,omitempty"`
}

type Logger struct {
	logFile    string
	blockchain *blockchain.Blockchain
}

func NewLogger() (*Logger, error) {
	// Create logs directory if it doesn't exist
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	logFile := filepath.Join(logsDir, "transfers.log")
	
	// Initialize blockchain
	bc, err := blockchain.NewBlockchain()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize blockchain: %w", err)
	}

	return &Logger{
		logFile:    logFile,
		blockchain: bc,
	}, nil
}

func (l *Logger) LogTransfer(log TransferLog) error {
	// Generate transfer ID
	transferID := generateTransferID(log)
	
	// Create blockchain transfer data
	transferData := blockchain.TransferData{
		TransferID: transferID,
		PeerID:     log.PeerID,
		FileName:   log.FileName,
		FileSize:   log.FileSize,
		FileHash:   log.FileHash, // Now using real SHA-256 hash
		Direction:  log.Direction,
		Status:     log.Status,
		Error:      log.Error,
		Duration:   log.Duration,
		Timestamp:  log.Timestamp,
	}

	// Add to blockchain
	if err := l.blockchain.AddBlock(transferData); err != nil {
		return fmt.Errorf("failed to add block to blockchain: %w", err)
	}

	// Also maintain traditional log file for backward compatibility
	return l.appendToLogFile(log)
}

func (l *Logger) appendToLogFile(log TransferLog) error {
	// Read existing logs
	var logs []TransferLog
	if data, err := os.ReadFile(l.logFile); err == nil {
		json.Unmarshal(data, &logs)
	}

	// Add new log
	logs = append(logs, log)

	// Write back to file with secure permissions
	data, err := json.MarshalIndent(logs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal logs: %w", err)
	}

	return os.WriteFile(l.logFile, data, 0600)
}

func (l *Logger) GetLogs() ([]TransferLog, error) {
	// Get logs from blockchain
	transfers := l.blockchain.GetTransferHistory()
	
	logs := make([]TransferLog, len(transfers))
	for i, transfer := range transfers {
		logs[i] = TransferLog{
			Timestamp: transfer.Timestamp,
			PeerID:    transfer.PeerID,
			FileName:  transfer.FileName,
			FileHash:  transfer.FileHash,
			FileSize:  transfer.FileSize,
			Direction: transfer.Direction,
			Status:    transfer.Status,
			Error:     transfer.Error,
			Duration:  transfer.Duration,
		}
	}

	return logs, nil
}

// VerifyLedger verifies the integrity of the blockchain ledger
func (l *Logger) VerifyLedger() (bool, error) {
	return l.blockchain.VerifyChain()
}

// ExportLedger exports the blockchain to a JSON file
func (l *Logger) ExportLedger(filename string) error {
	return l.blockchain.ExportToJSON(filename)
}

// GetBlockchainInfo returns information about the blockchain
func (l *Logger) GetBlockchainInfo() map[string]interface{} {
	latestBlock := l.blockchain.GetLatestBlock()
	chainLength := l.blockchain.GetChainLength()
	
	info := map[string]interface{}{
		"chain_length":   chainLength,
		"latest_block":   nil,
		"ledger_healthy": false,
	}
	
	if latestBlock != nil {
		info["latest_block"] = map[string]interface{}{
			"index":     latestBlock.Index,
			"hash":      latestBlock.Hash,
			"timestamp": latestBlock.Timestamp,
		}
	}
	
	// Verify chain integrity
	healthy, _ := l.blockchain.VerifyChain()
	info["ledger_healthy"] = healthy
	
	return info
}

// generateTransferID creates a unique ID for a transfer
func generateTransferID(log TransferLog) string {
	data := fmt.Sprintf("%s-%s-%s-%d-%s",
		log.Timestamp.Format(time.RFC3339Nano),
		log.PeerID,
		log.FileName,
		log.FileSize,
		log.Direction)
	
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:16] // Use first 16 chars of hash
}