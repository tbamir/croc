package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type TransferLog struct {
	Timestamp time.Time `json:"timestamp"`
	PeerID    string    `json:"peer_id"`
	FileName  string    `json:"file_name"`
	FileSize  int64     `json:"file_size"`
	Direction string    `json:"direction"` // "send" or "receive"
	Status    string    `json:"status"`    // "success" or "failed"
	Error     string    `json:"error,omitempty"`
	Duration  string    `json:"duration,omitempty"`
}

type Logger struct {
	logFile string
}

func NewLogger() (*Logger, error) {
	// Create logs directory if it doesn't exist
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create logs directory: %w", err)
	}

	logFile := filepath.Join(logsDir, "transfers.log")
	return &Logger{logFile: logFile}, nil
}

func (l *Logger) LogTransfer(log TransferLog) error {
	// Read existing logs
	var logs []TransferLog
	if data, err := os.ReadFile(l.logFile); err == nil {
		json.Unmarshal(data, &logs)
	}

	// Add new log
	logs = append(logs, log)

	// Write back to file
	data, err := json.MarshalIndent(logs, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal logs: %w", err)
	}

	return os.WriteFile(l.logFile, data, 0644)
}

func (l *Logger) GetLogs() ([]TransferLog, error) {
	var logs []TransferLog
	data, err := os.ReadFile(l.logFile)
	if err != nil {
		if os.IsNotExist(err) {
			return logs, nil // Return empty slice if file doesn't exist
		}
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	err = json.Unmarshal(data, &logs)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal logs: %w", err)
	}

	return logs, nil
}
