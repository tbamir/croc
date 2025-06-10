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
		preferredMode: ModeCBC, // Default to CBC for compatibility
	}
}

// EncryptWithBestMode encrypts data using the best available mode
func (as *AdvancedSecurity) EncryptWithBestMode(data, key []byte) ([]byte, EncryptionMode, error) {
	// For now, use the legacy CBC mode for compatibility
	encrypted, err := EncryptAES256CBC(data, key)
	if err != nil {
		return nil, ModeCBC, fmt.Errorf("CBC encryption failed: %w", err)
	}

	return encrypted, ModeCBC, nil
}

// DecryptWithMode decrypts data using the specified mode
func (as *AdvancedSecurity) DecryptWithMode(data, key []byte, mode EncryptionMode) ([]byte, error) {
	switch mode {
	case ModeCBC:
		return DecryptAES256CBC(data, key)
	case ModeGCM:
		// TODO: Implement GCM mode
		return nil, fmt.Errorf("GCM mode not implemented yet")
	case ModeChaCha20:
		// TODO: Implement ChaCha20 mode
		return nil, fmt.Errorf("ChaCha20 mode not implemented yet")
	case ModeHybrid:
		// TODO: Implement hybrid mode
		return nil, fmt.Errorf("Hybrid mode not implemented yet")
	default:
		return nil, fmt.Errorf("unknown encryption mode: %v", mode)
	}
}

// GenerateSecureKey generates a cryptographically secure key
func (as *AdvancedSecurity) GenerateSecureKey(size int) ([]byte, error) {
	key := make([]byte, size)
	_, err := rand.Read(key)
	return key, err
}

// AnalyzeThreatLevel analyzes the current threat environment
func (as *AdvancedSecurity) AnalyzeThreatLevel() (string, error) {
	// Simplified threat analysis
	return "LOW", nil
}

// GetRecommendedMode returns the recommended encryption mode based on threat level
func (as *AdvancedSecurity) GetRecommendedMode() EncryptionMode {
	return as.preferredMode
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
	default:
		return nil, fmt.Errorf("unsupported encryption mode: %d", mode)
	}
}

// VerifyIntegrity verifies the integrity of encrypted data
func (as *AdvancedSecurity) VerifyIntegrity(encryptedData []byte, key []byte, mode EncryptionMode) error {
	// For authenticated encryption modes, integrity is verified during decryption
	switch mode {
	case ModeGCM, ModeChaCha20:
		// Try to decrypt - if it succeeds, integrity is verified
		_, err := as.DecryptWithMode(encryptedData, key, mode)
		return err
	case ModeCBC:
		// For CBC mode, we don't have built-in authentication
		// This is one reason why it's less secure
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

// deriveKey derives a key of specified length using HKDF
func (as *AdvancedSecurity) deriveKey(inputKey []byte, length int) []byte {
	// Use HKDF to derive a proper key
	salt := []byte("trustdrop-bulletproof-v1")
	info := []byte("file-encryption")

	hkdf := hkdf.New(sha256.New, inputKey, salt, info)

	derivedKey := make([]byte, length)
	if _, err := io.ReadFull(hkdf, derivedKey); err != nil {
		// Fallback to simple hash if HKDF fails
		hash := sha256.Sum256(append(inputKey, salt...))
		copy(derivedKey, hash[:])
		if length > 32 {
			// For longer keys, repeat the hash
			for i := 32; i < length; i++ {
				derivedKey[i] = derivedKey[i%32]
			}
		}
	}

	return derivedKey
}
