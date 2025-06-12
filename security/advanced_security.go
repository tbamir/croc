package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
)

// EncryptionMode represents different encryption modes
type EncryptionMode int

const (
	ModeCBC EncryptionMode = iota
	ModeGCM
	ModeChaCha20
	ModeHybrid
)

func (m EncryptionMode) String() string {
	switch m {
	case ModeCBC:
		return "AES-256-CBC"
	case ModeGCM:
		return "AES-256-GCM"
	case ModeChaCha20:
		return "ChaCha20-Poly1305"
	case ModeHybrid:
		return "Hybrid-RSA-AES"
	default:
		return "Unknown"
	}
}

// AdvancedSecurity provides multiple encryption modes with automatic selection
type AdvancedSecurity struct {
	preferredMode EncryptionMode
}

// NewAdvancedSecurity creates a new advanced security instance
func NewAdvancedSecurity() *AdvancedSecurity {
	return &AdvancedSecurity{
		preferredMode: ModeGCM, // Default to GCM for better security
	}
}

// EncryptWithBestMode encrypts data using the best available mode based on data size and security requirements
func (as *AdvancedSecurity) EncryptWithBestMode(data, key []byte) ([]byte, EncryptionMode, error) {
	// For institutional networks, prioritize compatibility and reliability
	dataSize := len(data)
	
	// Choose encryption mode based on data size and requirements
	var mode EncryptionMode
	switch {
	case dataSize > 100*1024*1024: // Large files (>100MB) - use CBC for compatibility
		mode = ModeCBC
	case dataSize > 10*1024*1024: // Medium files (>10MB) - use GCM for balance
		mode = ModeGCM
	default: // Small files - use ChaCha20 for security
		mode = ModeChaCha20
	}

	encrypted, err := as.EncryptWithMode(data, key, mode)
	if err != nil {
		// Fallback to CBC if other modes fail
		encrypted, err = as.EncryptWithMode(data, key, ModeCBC)
		if err != nil {
			return nil, ModeCBC, fmt.Errorf("all encryption modes failed: %w", err)
		}
		mode = ModeCBC
	}

	return encrypted, mode, nil
}

// EncryptWithMode encrypts data using the specified encryption mode
func (as *AdvancedSecurity) EncryptWithMode(data []byte, key []byte, mode EncryptionMode) ([]byte, error) {
	switch mode {
	case ModeGCM:
		return as.encryptAESGCM(data, key)
	case ModeChaCha20:
		return as.encryptChaCha20Poly1305(data, key)
	case ModeCBC:
		return EncryptAES256CBC(data, key) // Use existing implementation
	case ModeHybrid:
		// For now, hybrid mode uses GCM with additional key derivation
		return as.encryptHybridMode(data, key)
	default:
		return nil, fmt.Errorf("unsupported encryption mode: %d", mode)
	}
}

// DecryptWithMode decrypts data using the specified mode
func (as *AdvancedSecurity) DecryptWithMode(data, key []byte, mode EncryptionMode) ([]byte, error) {
	switch mode {
	case ModeCBC:
		return DecryptAES256CBC(data, key)
	case ModeGCM:
		return as.decryptAESGCM(data, key)
	case ModeChaCha20:
		return as.decryptChaCha20Poly1305(data, key)
	case ModeHybrid:
		return as.decryptHybridMode(data, key)
	default:
		return nil, fmt.Errorf("unknown encryption mode: %v", mode)
	}
}

// encryptAESGCM encrypts data using AES-256-GCM (authenticated encryption)
func (as *AdvancedSecurity) encryptAESGCM(data []byte, key []byte) ([]byte, error) {
	// Derive a proper 32-byte key from input
	derivedKey := as.deriveKey(key, 32)

	// Create AES cipher
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and authenticate
	ciphertext := gcm.Seal(nil, nonce, data, nil)

	// Prepend nonce to ciphertext
	result := make([]byte, len(nonce)+len(ciphertext))
	copy(result[:len(nonce)], nonce)
	copy(result[len(nonce):], ciphertext)

	return result, nil
}

// decryptAESGCM decrypts data using AES-256-GCM
func (as *AdvancedSecurity) decryptAESGCM(data []byte, key []byte) ([]byte, error) {
	// Derive a proper 32-byte key from input
	derivedKey := as.deriveKey(key, 32)

	// Create AES cipher
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Check minimum length
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	// Extract nonce and ciphertext
	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]

	// Decrypt and verify
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}

// encryptChaCha20Poly1305 encrypts data using ChaCha20-Poly1305 (authenticated encryption)
func (as *AdvancedSecurity) encryptChaCha20Poly1305(data []byte, key []byte) ([]byte, error) {
	// Derive a proper 32-byte key from input
	derivedKey := as.deriveKey(key, 32)

	// Create ChaCha20-Poly1305 cipher
	aead, err := chacha20poly1305.New(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create ChaCha20-Poly1305 cipher: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and authenticate
	ciphertext := aead.Seal(nil, nonce, data, nil)

	// Prepend nonce to ciphertext
	result := make([]byte, len(nonce)+len(ciphertext))
	copy(result[:len(nonce)], nonce)
	copy(result[len(nonce):], ciphertext)

	return result, nil
}

// decryptChaCha20Poly1305 decrypts data using ChaCha20-Poly1305
func (as *AdvancedSecurity) decryptChaCha20Poly1305(data []byte, key []byte) ([]byte, error) {
	// Derive a proper 32-byte key from input
	derivedKey := as.deriveKey(key, 32)

	// Create ChaCha20-Poly1305 cipher
	aead, err := chacha20poly1305.New(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create ChaCha20-Poly1305 cipher: %w", err)
	}

	// Check minimum length
	nonceSize := aead.NonceSize()
	if len(data) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	// Extract nonce and ciphertext
	nonce := data[:nonceSize]
	ciphertext := data[nonceSize:]

	// Decrypt and verify
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}

// encryptHybridMode encrypts using hybrid approach with enhanced key derivation
func (as *AdvancedSecurity) encryptHybridMode(data []byte, key []byte) ([]byte, error) {
	// Use multiple rounds of key derivation for enhanced security
	derivedKey1 := as.deriveKey(key, 32)
	derivedKey2 := as.deriveKey(derivedKey1, 32)

	// First encryption layer with ChaCha20
	firstLayer, err := as.encryptChaCha20Poly1305(data, derivedKey1)
	if err != nil {
		return nil, fmt.Errorf("first encryption layer failed: %w", err)
	}

	// Second encryption layer with AES-GCM
	secondLayer, err := as.encryptAESGCM(firstLayer, derivedKey2)
	if err != nil {
		return nil, fmt.Errorf("second encryption layer failed: %w", err)
	}

	return secondLayer, nil
}

// decryptHybridMode decrypts using hybrid approach
func (as *AdvancedSecurity) decryptHybridMode(data []byte, key []byte) ([]byte, error) {
	// Use same key derivation as encryption
	derivedKey1 := as.deriveKey(key, 32)
	derivedKey2 := as.deriveKey(derivedKey1, 32)

	// First decryption layer (AES-GCM)
	firstLayer, err := as.decryptAESGCM(data, derivedKey2)
	if err != nil {
		return nil, fmt.Errorf("first decryption layer failed: %w", err)
	}

	// Second decryption layer (ChaCha20)
	plaintext, err := as.decryptChaCha20Poly1305(firstLayer, derivedKey1)
	if err != nil {
		return nil, fmt.Errorf("second decryption layer failed: %w", err)
	}

	return plaintext, nil
}

// deriveKey derives a key of specified length using HKDF with enhanced security
func (as *AdvancedSecurity) deriveKey(inputKey []byte, length int) []byte {
	// Use application-specific salt and info for better security
	salt := []byte("trustdrop-bulletproof-v2.0-institutional-grade")
	info := []byte("institutional-network-file-encryption-2024")

	hkdf := hkdf.New(sha256.New, inputKey, salt, info)

	derivedKey := make([]byte, length)
	if _, err := io.ReadFull(hkdf, derivedKey); err != nil {
		// Fallback to enhanced hash-based derivation if HKDF fails
		hash := sha256.Sum256(append(append(inputKey, salt...), info...))
		copy(derivedKey, hash[:])
		
		// For longer keys, use iterative hashing
		if length > 32 {
			for i := 32; i < length; i++ {
				// Use previous hash as input for next round
				prevHash := derivedKey[i-32 : i]
				nextHash := sha256.Sum256(append(prevHash, byte(i/32)))
				if i < length {
					derivedKey[i] = nextHash[i%32]
				}
			}
		}
	}

	return derivedKey
}

// GenerateSecureKey generates a cryptographically secure key
func (as *AdvancedSecurity) GenerateSecureKey(size int) ([]byte, error) {
	if size <= 0 {
		return nil, fmt.Errorf("invalid key size: %d", size)
	}

	key := make([]byte, size)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate secure key: %w", err)
	}

	return key, nil
}

// AnalyzeThreatLevel analyzes the current threat environment
func (as *AdvancedSecurity) AnalyzeThreatLevel() (string, error) {
	// Enhanced threat analysis based on multiple factors
	
	// Check system entropy
	entropyTest := make([]byte, 32)
	if _, err := rand.Read(entropyTest); err != nil {
		return "HIGH", fmt.Errorf("insufficient system entropy")
	}

	// Basic threat assessment - in production this would be more sophisticated
	// For now, return conservative assessment for institutional environments
	return "MEDIUM", nil
}

// GetRecommendedMode returns the recommended encryption mode based on threat level and data characteristics
func (as *AdvancedSecurity) GetRecommendedMode(dataSize int64, networkType string) EncryptionMode {
	// Recommendations based on institutional network requirements
	switch networkType {
	case "corporate", "university", "institutional":
		// For institutional networks, prioritize compatibility
		if dataSize > 50*1024*1024 { // Large files
			return ModeCBC // Most compatible
		} else {
			return ModeGCM // Good balance of security and compatibility
		}
	case "mobile":
		// For mobile networks, prioritize efficiency
		if dataSize > 10*1024*1024 {
			return ModeGCM
		} else {
			return ModeChaCha20
		}
	default:
		// For open networks, prioritize security
		if dataSize > 100*1024*1024 {
			return ModeGCM
		} else {
			return ModeChaCha20
		}
	}
}

// VerifyIntegrity verifies the integrity of encrypted data
func (as *AdvancedSecurity) VerifyIntegrity(encryptedData []byte, key []byte, mode EncryptionMode) error {
	// For authenticated encryption modes, integrity is verified during decryption
	switch mode {
	case ModeGCM, ModeChaCha20, ModeHybrid:
		// Try to decrypt - if it succeeds, integrity is verified
		_, err := as.DecryptWithMode(encryptedData, key, mode)
		if err != nil {
			return fmt.Errorf("integrity verification failed: %w", err)
		}
		return nil
	case ModeCBC:
		// For CBC mode, we don't have built-in authentication
		// Perform basic checks
		if len(encryptedData) < aes.BlockSize {
			return fmt.Errorf("encrypted data too short for CBC mode")
		}
		if len(encryptedData)%aes.BlockSize != 0 {
			return fmt.Errorf("encrypted data not properly aligned for CBC mode")
		}
		return nil
	default:
		return fmt.Errorf("unsupported encryption mode for integrity verification: %d", mode)
	}
}

// SecureCompareKeys performs constant-time comparison of keys to prevent timing attacks
func (as *AdvancedSecurity) SecureCompareKeys(key1, key2 []byte) bool {
	if len(key1) != len(key2) {
		return false
	}

	var result byte
	for i := 0; i < len(key1); i++ {
		result |= key1[i] ^ key2[i]
	}

	return result == 0
}

// SecureWipeMemory securely wipes sensitive data from memory
func (as *AdvancedSecurity) SecureWipeMemory(data []byte) {
	// Overwrite with random data first
	if _, err := rand.Read(data); err == nil {
		// Then overwrite with zeros
		for i := range data {
			data[i] = 0
		}
	} else {
		// Fallback to zero-only wipe
		for i := range data {
			data[i] = 0
		}
	}
}

// CreateSecureHash creates a secure hash of data with salt
func (as *AdvancedSecurity) CreateSecureHash(data []byte, salt []byte) []byte {
	hasher := sha256.New()
	
	// Write salt first
	if len(salt) > 0 {
		hasher.Write(salt)
	} else {
		// Use default salt if none provided
		defaultSalt := []byte("trustdrop-secure-hash-salt-v1")
		hasher.Write(defaultSalt)
	}
	
	// Write data
	hasher.Write(data)
	
	return hasher.Sum(nil)
}

// ValidateEncryptionKey validates that an encryption key meets security requirements
func (as *AdvancedSecurity) ValidateEncryptionKey(key []byte, minLength int) error {
	if len(key) < minLength {
		return fmt.Errorf("key too short: %d bytes, minimum required: %d", len(key), minLength)
	}

	// Check for all-zero key
	allZero := true
	for _, b := range key {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return fmt.Errorf("key cannot be all zeros")
	}

	// Basic entropy check - count unique bytes
	uniqueBytes := make(map[byte]bool)
	for _, b := range key {
		uniqueBytes[b] = true
	}

	// Require at least 1/4 of possible byte values for reasonable entropy
	minUniqueBytes := len(key) / 4
	if minUniqueBytes < 4 {
		minUniqueBytes = 4
	}
	if len(uniqueBytes) < minUniqueBytes {
		return fmt.Errorf("key has insufficient entropy: %d unique bytes in %d total", len(uniqueBytes), len(key))
	}

	return nil
}

// GetSecurityMetrics returns security-related metrics
func (as *AdvancedSecurity) GetSecurityMetrics() map[string]interface{} {
	metrics := make(map[string]interface{})
	
	// Test random number generation
	testEntropy := make([]byte, 32)
	entropyAvailable := true
	if _, err := rand.Read(testEntropy); err != nil {
		entropyAvailable = false
	}
	
	metrics["entropy_available"] = entropyAvailable
	metrics["supported_modes"] = []string{"AES-256-CBC", "AES-256-GCM", "ChaCha20-Poly1305", "Hybrid"}
	metrics["default_mode"] = as.preferredMode.String()
	metrics["key_derivation"] = "HKDF-SHA256"
	metrics["authenticated_encryption"] = true
	
	return metrics
}

// SetPreferredMode sets the preferred encryption mode
func (as *AdvancedSecurity) SetPreferredMode(mode EncryptionMode) {
	as.preferredMode = mode
}

// GetPreferredMode returns the current preferred encryption mode
func (as *AdvancedSecurity) GetPreferredMode() EncryptionMode {
	return as.preferredMode
}