package transfer

import (
	"strings"
)

// TransferError provides structured error information with user guidance
type TransferError struct {
	Code       ErrorCode `json:"code"`
	Message    string    `json:"message"`
	UserAction string    `json:"user_action"`
	CanRetry   bool      `json:"can_retry"`
	Technical  string    `json:"technical_details"`
}

// ErrorCode represents different types of transfer errors
type ErrorCode int

const (
	ErrorNetworkBlocked ErrorCode = iota
	ErrorInvalidCode
	ErrorFileAccess
	ErrorDiskSpace
	ErrorTimeout
	ErrorEncryption
	ErrorTransportFailed
	ErrorUnknown
)

// Error implements the error interface
func (te TransferError) Error() string {
	return te.Message
}

// String returns a human-readable error code name
func (ec ErrorCode) String() string {
	switch ec {
	case ErrorNetworkBlocked:
		return "NetworkBlocked"
	case ErrorInvalidCode:
		return "InvalidCode"
	case ErrorFileAccess:
		return "FileAccess"
	case ErrorDiskSpace:
		return "DiskSpace"
	case ErrorTimeout:
		return "Timeout"
	case ErrorEncryption:
		return "Encryption"
	case ErrorTransportFailed:
		return "TransportFailed"
	default:
		return "Unknown"
	}
}

// HandleTransferError converts a generic error into a user-friendly TransferError
func HandleTransferError(err error, operation string) TransferError {
	if err == nil {
		return TransferError{}
	}

	errorStr := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errorStr, "connection refused") ||
		strings.Contains(errorStr, "network unreachable") ||
		strings.Contains(errorStr, "host unreachable") ||
		strings.Contains(errorStr, "no route to host"):
		return TransferError{
			Code:       ErrorNetworkBlocked,
			Message:    "Network is blocking the file transfer",
			UserAction: "Try using mobile hotspot or contact your IT department for firewall settings",
			CanRetry:   true,
			Technical:  err.Error(),
		}

	case strings.Contains(errorStr, "timeout") ||
		strings.Contains(errorStr, "deadline exceeded") ||
		strings.Contains(errorStr, "i/o timeout"):
		return TransferError{
			Code:       ErrorTimeout,
			Message:    "Transfer is taking too long",
			UserAction: "Check your internet connection, try again, or use a more stable network",
			CanRetry:   true,
			Technical:  err.Error(),
		}

	case strings.Contains(errorStr, "invalid") ||
		strings.Contains(errorStr, "not found") ||
		strings.Contains(errorStr, "transfer not found") ||
		strings.Contains(errorStr, "expired"):
		return TransferError{
			Code:       ErrorInvalidCode,
			Message:    "Transfer code is incorrect or expired",
			UserAction: "Double-check the transfer code and ask sender to generate a new one if needed",
			CanRetry:   false,
			Technical:  err.Error(),
		}

	case strings.Contains(errorStr, "permission denied") ||
		strings.Contains(errorStr, "access denied") ||
		strings.Contains(errorStr, "access is denied") ||
		strings.Contains(errorStr, "operation not permitted"):
		return TransferError{
			Code:       ErrorFileAccess,
			Message:    "Cannot access the file or folder",
			UserAction: "Check file permissions, close any programs using the file, or run as administrator",
			CanRetry:   true,
			Technical:  err.Error(),
		}

	case strings.Contains(errorStr, "no space") ||
		strings.Contains(errorStr, "disk full") ||
		strings.Contains(errorStr, "insufficient space") ||
		strings.Contains(errorStr, "storage space"):
		return TransferError{
			Code:       ErrorDiskSpace,
			Message:    "Not enough disk space for the transfer",
			UserAction: "Free up disk space by deleting unnecessary files, then try again",
			CanRetry:   true,
			Technical:  err.Error(),
		}

	case strings.Contains(errorStr, "decrypt") ||
		strings.Contains(errorStr, "encrypt") ||
		strings.Contains(errorStr, "cipher") ||
		strings.Contains(errorStr, "key") ||
		strings.Contains(errorStr, "authentication failed"):
		return TransferError{
			Code:       ErrorEncryption,
			Message:    "Encryption or decryption failed",
			UserAction: "Verify the transfer code is correct and ask sender to try again",
			CanRetry:   false,
			Technical:  err.Error(),
		}

	case strings.Contains(errorStr, "all transports failed") ||
		strings.Contains(errorStr, "transport") ||
		strings.Contains(errorStr, "relay") ||
		strings.Contains(errorStr, "service unavailable"):
		return TransferError{
			Code:       ErrorTransportFailed,
			Message:    "All transfer methods failed",
			UserAction: "Try again later, check internet connection, or use a different network (mobile hotspot)",
			CanRetry:   true,
			Technical:  err.Error(),
		}

	case strings.Contains(errorStr, "proxy") ||
		strings.Contains(errorStr, "firewall") ||
		strings.Contains(errorStr, "blocked"):
		return TransferError{
			Code:       ErrorNetworkBlocked,
			Message:    "Corporate firewall is blocking the transfer",
			UserAction: "Contact your IT department or try using mobile hotspot",
			CanRetry:   true,
			Technical:  err.Error(),
		}

	default:
		return TransferError{
			Code:       ErrorUnknown,
			Message:    "Transfer failed due to an unexpected error",
			UserAction: "Check your internet connection and try again. If problem persists, try a different network",
			CanRetry:   true,
			Technical:  err.Error(),
		}
	}
}

// GetUserGuidanceByNetworkType returns specific guidance based on detected network type
func GetUserGuidanceByNetworkType(networkType string, err error) string {
	transferErr := HandleTransferError(err, "transfer")

	switch networkType {
	case "corporate":
		switch transferErr.Code {
		case ErrorNetworkBlocked:
			return "Corporate firewall detected. Try:\n1. Contact IT department\n2. Use mobile hotspot\n3. Try during off-peak hours"
		case ErrorTimeout:
			return "Corporate network timeout. Try:\n1. Wait for better network conditions\n2. Use mobile hotspot\n3. Try smaller files first"
		default:
			return transferErr.UserAction
		}

	case "university":
		switch transferErr.Code {
		case ErrorNetworkBlocked:
			return "University network restrictions detected. Try:\n1. Connect to guest WiFi\n2. Use mobile data\n3. Contact network support"
		default:
			return transferErr.UserAction
		}

	case "public":
		switch transferErr.Code {
		case ErrorTimeout:
			return "Public WiFi may be slow. Try:\n1. Move closer to router\n2. Use mobile data instead\n3. Try during less busy times"
		default:
			return transferErr.UserAction
		}

	default:
		return transferErr.UserAction
	}
}

// IsRetryableError returns true if the error can be retried
func IsRetryableError(err error) bool {
	transferErr := HandleTransferError(err, "")
	return transferErr.CanRetry
}

// GetErrorSeverity returns the severity level of the error
func GetErrorSeverity(err error) string {
	transferErr := HandleTransferError(err, "")

	switch transferErr.Code {
	case ErrorInvalidCode, ErrorEncryption:
		return "high" // User needs to take action, retry won't help
	case ErrorDiskSpace, ErrorFileAccess:
		return "medium" // User can fix, then retry
	case ErrorNetworkBlocked, ErrorTransportFailed:
		return "medium" // May resolve on retry or with network change
	case ErrorTimeout:
		return "low" // Often resolves on retry
	default:
		return "medium"
	}
}

// GetQuickFixes returns a list of quick fixes the user can try
func GetQuickFixes(err error) []string {
	transferErr := HandleTransferError(err, "")

	switch transferErr.Code {
	case ErrorNetworkBlocked:
		return []string{
			"Try using mobile hotspot",
			"Contact IT department",
			"Try again during off-peak hours",
			"Check if VPN is causing issues",
		}

	case ErrorTimeout:
		return []string{
			"Check internet connection speed",
			"Try again in a few minutes",
			"Use a more stable network",
			"Close other applications using internet",
		}

	case ErrorFileAccess:
		return []string{
			"Check if file is open in another program",
			"Try running as administrator",
			"Check file permissions",
			"Ensure antivirus isn't blocking",
		}

	case ErrorDiskSpace:
		return []string{
			"Delete unnecessary files",
			"Empty trash/recycle bin",
			"Choose a different save location",
			"Use external storage",
		}

	case ErrorInvalidCode:
		return []string{
			"Double-check the transfer code",
			"Ask sender for a new code",
			"Ensure code hasn't expired",
		}

	default:
		return []string{
			"Check internet connection",
			"Try again in a few minutes",
			"Restart the application",
		}
	}
}
