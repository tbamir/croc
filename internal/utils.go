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

// FIXED: Enhanced data directory creation with better error handling
func EnsureDataDirectory() error {
	// Get current working directory with fallback
	workingDir, err := os.Getwd()
	if err != nil {
		// Fallback to executable directory
		if execPath, execErr := os.Executable(); execErr == nil {
			workingDir = filepath.Dir(execPath)
		} else {
			return fmt.Errorf("failed to determine working directory: %w", err)
		}
	}

	// FIXED: Create directories with absolute paths and better error handling
	dirs := []string{
		filepath.Join(workingDir, "data"),
		filepath.Join(workingDir, "data", "received"),
		filepath.Join(workingDir, "data", "temp"),
		filepath.Join(workingDir, "logs"),
		filepath.Join(workingDir, "blockchain_data"),
	}

	for _, dir := range dirs {
		// Create directory with proper permissions
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
		
		// FIXED: Enhanced verification that directory exists and is writable
		if !FileExists(dir) {
			return fmt.Errorf("directory %s was not created successfully", dir)
		}
		
		// Test write permissions with better error handling
		if err := testWritePermissions(dir); err != nil {
			return fmt.Errorf("directory %s is not writable: %w", dir, err)
		}
	}

	return nil
}

// FIXED: Enhanced write permission testing
func testWritePermissions(dir string) error {
	testFile := filepath.Join(dir, fmt.Sprintf(".write_test_%d", time.Now().UnixNano()))
	f, err := os.Create(testFile)
	if err != nil {
		return err
	}
	
	// Write a small test
	if _, err := f.WriteString("test"); err != nil {
		f.Close()
		os.Remove(testFile)
		return err
	}
	
	f.Close()
	
	// Verify we can read it back
	if data, err := os.ReadFile(testFile); err != nil || string(data) != "test" {
		os.Remove(testFile)
		return fmt.Errorf("read test failed")
	}
	
	// Clean up
	return os.Remove(testFile)
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

// FIXED: Enhanced path resolution functions
func GetReceivedFilesPath() string {
	workingDir, err := os.Getwd()
	if err != nil {
		// Fallback to executable directory
		if execPath, execErr := os.Executable(); execErr == nil {
			workingDir = filepath.Dir(execPath)
		} else {
			return filepath.Join("data", "received") // last resort fallback
		}
	}
	return filepath.Join(workingDir, "data", "received")
}

func GetTempPath() string {
	workingDir, err := os.Getwd()
	if err != nil {
		// Fallback to executable directory
		if execPath, execErr := os.Executable(); execErr == nil {
			workingDir = filepath.Dir(execPath)
		} else {
			return filepath.Join("data", "temp") // last resort fallback
		}
	}
	return filepath.Join(workingDir, "data", "temp")
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// FIXED: Enhanced cross-platform path resolution with better error handling
func ResolvePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path provided")
	}

	// Handle different path formats across platforms
	switch runtime.GOOS {
	case "windows":
		// Fix Windows paths that start with / but should be drive letters
		if strings.HasPrefix(path, "/") && len(path) > 1 && path[2] == ':' {
			path = path[1:] // Remove leading slash
		}
		// Handle UNC paths
		if strings.HasPrefix(path, "//") {
			return filepath.Abs(path)
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
		// Handle relative paths starting with ./
		if strings.HasPrefix(path, "./") {
			workingDir, err := os.Getwd()
			if err != nil {
				return "", fmt.Errorf("failed to get working directory: %w", err)
			}
			path = filepath.Join(workingDir, path[2:])
		}
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Clean the path to remove any .. or . components
	cleanPath := filepath.Clean(absPath)

	// Verify the path exists and is accessible
	if _, err := os.Stat(cleanPath); err != nil {
		return "", fmt.Errorf("path does not exist or is not accessible: %w", err)
	}

	return cleanPath, nil
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

// FIXED: Enhanced file operations with better error handling
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
	return testWritePermissions(path)
}

// FIXED: Enhanced absolute path resolution
func GetAbsolutePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("empty path provided")
	}

	// First try to resolve any platform-specific path issues
	resolvedPath, err := ResolvePath(path)
	if err != nil {
		// If resolution fails, try to get absolute path directly
		absPath, absErr := filepath.Abs(path)
		if absErr != nil {
			return "", fmt.Errorf("failed to resolve path: %w (original error: %v)", absErr, err)
		}
		return absPath, nil
	}
	return resolvedPath, nil
}

// ValidateTransferPath validates that a path is safe for file transfer operations
func ValidateTransferPath(path string) error {
	if path == "" {
		return fmt.Errorf("empty path provided")
	}

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

// FIXED: Add helper function to detect if running as built application
func IsBuiltApplication() bool {
	execPath, err := os.Executable()
	if err != nil {
		return false
	}

	switch runtime.GOOS {
	case "darwin":
		// Check if running from .app bundle
		return strings.Contains(execPath, ".app/Contents/MacOS/")
	case "windows":
		// Check if executable name suggests it's a built binary
		return strings.HasSuffix(strings.ToLower(execPath), ".exe")
	case "linux":
		// Check if running from a typical binary location
		return !strings.Contains(execPath, "go-build")
	}

	return false
}

// FIXED: Get appropriate working directory for built applications
func GetWorkingDirectory() (string, error) {
	if IsBuiltApplication() {
		execPath, err := os.Executable()
		if err != nil {
			return "", err
		}

		switch runtime.GOOS {
		case "darwin":
			// For .app bundles, try to use a user-accessible directory
			if strings.Contains(execPath, ".app/Contents/MacOS/") {
				homeDir, err := os.UserHomeDir()
				if err == nil {
					trustDropDir := filepath.Join(homeDir, "Documents", "TrustDrop")
					if err := os.MkdirAll(trustDropDir, 0755); err == nil {
						return trustDropDir, nil
					}
				}
			}
			return filepath.Dir(execPath), nil
		case "windows":
			// For Windows .exe, try to use executable directory
			execDir := filepath.Dir(execPath)
			if testWritePermissions(execDir) == nil {
				return execDir, nil
			}
			// Fallback to user documents
			homeDir, err := os.UserHomeDir()
			if err == nil {
				trustDropDir := filepath.Join(homeDir, "Documents", "TrustDrop")
				if err := os.MkdirAll(trustDropDir, 0755); err == nil {
					return trustDropDir, nil
				}
			}
			return execDir, nil
		default:
			return filepath.Dir(execPath), nil
		}
	}

	// For development, use current directory
	return os.Getwd()
}