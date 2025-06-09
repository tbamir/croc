package internal

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// EnsureDataDirectory creates the data directory structure if it doesn't exist
func EnsureDataDirectory() error {
	// FIXED: Get current working directory and create absolute paths
	workingDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}

	dirs := []string{
		filepath.Join(workingDir, "data"),
		filepath.Join(workingDir, "data", "received"),
		filepath.Join(workingDir, "data", "temp"),
		filepath.Join(workingDir, "logs"),
		filepath.Join(workingDir, "blockchain_data"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		
		// FIXED: Verify directory was actually created and is writable
		if !FileExists(dir) {
			return fmt.Errorf("directory %s was not created successfully", dir)
		}
		
		// Test write permissions
		testFile := filepath.Join(dir, ".write_test")
		if f, err := os.Create(testFile); err != nil {
			return fmt.Errorf("directory %s is not writable: %w", dir, err)
		} else {
			f.Close()
			os.Remove(testFile)
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

// GetReceivedFilesPath returns the absolute path where received files are stored
func GetReceivedFilesPath() string {
	workingDir, err := os.Getwd()
	if err != nil {
		return filepath.Join("data", "received") // fallback to relative
	}
	return filepath.Join(workingDir, "data", "received")
}

// GetTempPath returns the absolute path for temporary files
func GetTempPath() string {
	workingDir, err := os.Getwd()
	if err != nil {
		return filepath.Join("data", "temp") // fallback to relative
	}
	return filepath.Join(workingDir, "data", "temp")
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// FIXED: Enhanced cross-platform path resolution
func ResolvePath(path string) (string, error) {
	// Handle different path formats across platforms
	switch runtime.GOOS {
	case "windows":
		// Fix Windows paths that start with / but should be drive letters
		if strings.HasPrefix(path, "/") && len(path) > 1 && path[2] == ':' {
			path = path[1:] // Remove leading slash
		}
	case "darwin", "linux":
		// Expand ~ to home directory on Unix systems
		if strings.HasPrefix(path, "~/") {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("failed to get home directory: %w", err)
			}
			path = filepath.Join(homeDir, path[2:])
		}
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Verify the path exists and is accessible
	if _, err := os.Stat(absPath); err != nil {
		return "", fmt.Errorf("path does not exist or is not accessible: %w", err)
	}

	return absPath, nil
}

// FIXED: Enhanced filename sanitization with better Unicode handling
func SanitizeFilename(filename string) string {
	if filename == "" {
		return "unnamed_file"
	}

	// Remove or replace problematic Unicode characters
	var result strings.Builder
	
	for _, r := range filename {
		switch {
		case r < 32: // Control characters
			result.WriteRune('_')
		case r == 127: // DEL character
			result.WriteRune('_')
		case r > 127 && r < 160: // Extended control characters
			result.WriteRune('_')
		case strings.ContainsRune(`<>:"/\\|?*`, r): // Windows forbidden characters
			result.WriteRune('_')
		case r == 0xFEFF: // Byte order mark
			// Skip BOM
		default:
			// Allow most Unicode characters for international filename support
			// but replace some problematic ones
			switch r {
			case 0x202E, 0x202D: // Right-to-left/left-to-right override (security issue)
				result.WriteRune('_')
			default:
				result.WriteRune(r)
			}
		}
	}

	sanitized := result.String()

	// Remove multiple consecutive underscores
	re := regexp.MustCompile(`_{2,}`)
	sanitized = re.ReplaceAllString(sanitized, "_")

	// Remove leading/trailing underscores and dots
	sanitized = strings.Trim(sanitized, "_.")

	// Ensure filename isn't empty after sanitization
	if sanitized == "" {
		sanitized = "unnamed_file"
	}

	// Limit length (most filesystems support 255 characters)
	if len(sanitized) > 200 {
		ext := filepath.Ext(sanitized)
		base := sanitized[:200-len(ext)]
		sanitized = base + ext
	}

	// Check for reserved names on Windows
	if runtime.GOOS == "windows" {
		reserved := []string{
			"CON", "PRN", "AUX", "NUL",
			"COM1", "COM2", "COM3", "COM4", "COM5", "COM6", "COM7", "COM8", "COM9",
			"LPT1", "LPT2", "LPT3", "LPT4", "LPT5", "LPT6", "LPT7", "LPT8", "LPT9",
		}
		
		baseWithoutExt := strings.TrimSuffix(sanitized, filepath.Ext(sanitized))
		for _, res := range reserved {
			if strings.EqualFold(baseWithoutExt, res) {
				sanitized = "file_" + sanitized
				break
			}
		}
	}

	return sanitized
}

// SanitizePath sanitizes an entire file path while preserving directory structure
func SanitizePath(path string) string {
	if path == "" {
		return ""
	}

	// Split path into directory and filename
	dir := filepath.Dir(path)
	filename := filepath.Base(path)

	// Sanitize the filename
	sanitizedFilename := SanitizeFilename(filename)

	// For root directory
	if dir == "." || dir == "/" {
		return sanitizedFilename
	}

	// Reconstruct path with sanitized filename
	return filepath.Join(dir, sanitizedFilename)
}

// FIXED: Add cross-platform file operations helpers
func CopyFile(src, dst string) error {
	// Resolve source path
	srcPath, err := ResolvePath(src)
	if err != nil {
		return fmt.Errorf("invalid source path: %w", err)
	}

	// Ensure destination directory exists
	dstDir := filepath.Dir(dst)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Open source file
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	// Create destination file
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dstFile.Close()

	// Copy file contents
	if _, err := srcFile.WriteTo(dstFile); err != nil {
		return fmt.Errorf("failed to copy file contents: %w", err)
	}

	// Copy file permissions
	srcInfo, err := srcFile.Stat()
	if err == nil {
		dstFile.Chmod(srcInfo.Mode())
	}

	return nil
}

// EnsureWritableDirectory ensures a directory exists and is writable
func EnsureWritableDirectory(path string) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	// Test write permissions
	testFile := filepath.Join(path, ".write_test_"+fmt.Sprintf("%d", time.Now().Unix()))
	f, err := os.Create(testFile)
	if err != nil {
		return fmt.Errorf("directory %s is not writable: %w", path, err)
	}
	f.Close()
	os.Remove(testFile)

	return nil
}

// GetAbsolutePath returns the absolute path, handling cross-platform issues
func GetAbsolutePath(path string) (string, error) {
	// First try to resolve any platform-specific path issues
	resolvedPath, err := ResolvePath(path)
	if err != nil {
		// If resolution fails, try to get absolute path directly
		return filepath.Abs(path)
	}
	return resolvedPath, nil
}

// ValidateTransferPath validates that a path is safe for file transfer operations
func ValidateTransferPath(path string) error {
	// Check for null bytes (security issue)
	if strings.Contains(path, "\x00") {
		return fmt.Errorf("path contains null bytes")
	}

	// Check for path traversal attempts
	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("path traversal detected")
	}

	// Check path length
	if len(path) > 4096 {
		return fmt.Errorf("path too long")
	}

	// Try to resolve the path
	if _, err := ResolvePath(path); err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	return nil
}