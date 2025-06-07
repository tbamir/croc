package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// EnsureDataDirectory creates the data directory structure if it doesn't exist
func EnsureDataDirectory() error {
	dirs := []string{
		"data",
		"data/received",
		"data/temp",
		"logs",
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// FormatFileSize formats file size in human readable format
func FormatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatDuration formats duration in human readable format
func FormatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	} else if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	} else {
		return fmt.Sprintf("%.1fh", d.Hours())
	}
}

// GetReceivedFilesPath returns the path where received files are stored
func GetReceivedFilesPath() string {
	return filepath.Join("data", "received")
}

// GetTempPath returns the path for temporary files
func GetTempPath() string {
	return filepath.Join("data", "temp")
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// SanitizeFilename removes or replaces problematic characters from filenames
// This prevents issues with non-printable Unicode characters and cross-platform compatibility
func SanitizeFilename(filename string) string {
	// Remove or replace problematic Unicode characters
	var result strings.Builder

	for _, r := range filename {
		switch {
		case r < 32: // Control characters
			result.WriteRune('_')
		case r == 127: // DEL character
			result.WriteRune('_')
		case r > 127: // Non-ASCII characters (for maximum compatibility)
			result.WriteRune('_')
		case strings.ContainsRune(`<>:"/\\|?*`, r): // Windows forbidden characters
			result.WriteRune('_')
		default:
			result.WriteRune(r)
		}
	}

	sanitized := result.String()

	// Remove multiple consecutive underscores
	re := regexp.MustCompile(`_{2,}`)
	sanitized = re.ReplaceAllString(sanitized, "_")

	// Remove leading/trailing underscores and dots
	sanitized = strings.Trim(sanitized, "_.")

	// Ensure filename isn't empty
	if sanitized == "" {
		sanitized = "unnamed_file"
	}

	// Limit length (most filesystems support 255 characters)
	if len(sanitized) > 200 {
		ext := filepath.Ext(sanitized)
		base := sanitized[:200-len(ext)]
		sanitized = base + ext
	}

	return sanitized
}

// SanitizePath sanitizes an entire file path while preserving directory structure
func SanitizePath(path string) string {
	dir := filepath.Dir(path)
	filename := filepath.Base(path)

	sanitizedFilename := SanitizeFilename(filename)

	if dir == "." {
		return sanitizedFilename
	}

	return filepath.Join(dir, sanitizedFilename)
}
