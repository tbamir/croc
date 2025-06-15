package security

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/chacha20poly1305"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/pbkdf2"
)

// EncryptionMode represents different encryption algorithms
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
		return "Hybrid-Mode"
	default:
		return "Unknown"
	}
}

// AdvancedSecurity provides institutional-grade encryption with multiple modes
type AdvancedSecurity struct {
	preferredMode EncryptionMode
}

func NewAdvancedSecurity() *AdvancedSecurity {
	return &AdvancedSecurity{
		preferredMode: ModeGCM, // Default to GCM for institutional networks
	}
}

// EncryptWithBestMode automatically selects the best encryption mode based on data characteristics
func (as *AdvancedSecurity) EncryptWithBestMode(data, key []byte) ([]byte, EncryptionMode, error) {
	// Analyze data characteristics to choose optimal mode
	dataSize := int64(len(data))

	var mode EncryptionMode
	if dataSize > 100*1024*1024 { // Large files > 100MB
		mode = ModeGCM // Good balance for large files
	} else if dataSize > 10*1024*1024 { // Medium files > 10MB
		mode = ModeChaCha20 // Faster for medium files
	} else {
		mode = ModeGCM // Default for small files
	}

	// Strengthen the key before encryption
	strengthenedKey, _, err := as.StrengthenTransferCode(string(key), "encryption")
	if err != nil {
		return nil, mode, fmt.Errorf("key strengthening failed: %w", err)
	}

	encrypted, err := as.EncryptWithMode(data, strengthenedKey, mode)
	if err != nil {
		return nil, mode, err
	}

	return encrypted, mode, nil
}

// EncryptWithMode encrypts data using the specified mode
func (as *AdvancedSecurity) EncryptWithMode(data []byte, key []byte, mode EncryptionMode) ([]byte, error) {
	switch mode {
	case ModeGCM:
		return as.encryptAESGCM(data, key)
	case ModeChaCha20:
		return as.encryptChaCha20Poly1305(data, key)
	case ModeCBC:
		return as.encryptAESCBC(data, key)
	case ModeHybrid:
		return as.encryptHybridMode(data, key)
	default:
		return nil, fmt.Errorf("unsupported encryption mode: %d", mode)
	}
}

// DecryptWithMode decrypts data using the specified mode
func (as *AdvancedSecurity) DecryptWithMode(data, key []byte, mode EncryptionMode) ([]byte, error) {
	switch mode {
	case ModeCBC:
		return as.decryptAESCBC(data, key)
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

// StrengthenTransferCode converts a weak transfer code into a cryptographically strong key
// This addresses the critical vulnerability of using transfer codes directly as encryption keys
func (as *AdvancedSecurity) StrengthenTransferCode(transferCode string, context string) ([]byte, []byte, error) {
	if len(transferCode) < 8 {
		return nil, nil, fmt.Errorf("transfer code too short: minimum 8 characters required")
	}

	// Generate deterministic salt based on transfer code for peer synchronization
	saltSource := append([]byte(transferCode), []byte(context+"trustdrop-v3-2024")...)
	saltHash := sha256.Sum256(saltSource)
	salt := saltHash[:]

	// Use high-iteration PBKDF2 to strengthen the weak transfer code
	strengthenedKey := pbkdf2.Key(
		[]byte(transferCode+context),
		salt,
		100000, // High iteration count for security
		32,     // 256-bit key
		sha256.New,
	)

	return strengthenedKey, salt, nil
}

// StrengthenTransferCodeWithSalt recreates a strengthened key using existing salt
func (as *AdvancedSecurity) StrengthenTransferCodeWithSalt(transferCode string, context string, salt []byte) []byte {
	return pbkdf2.Key(
		[]byte(transferCode+context),
		salt,
		100000,
		32,
		sha256.New,
	)
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

// encryptAESCBC encrypts data using AES-256-CBC
func (as *AdvancedSecurity) encryptAESCBC(data []byte, key []byte) ([]byte, error) {
	// Derive a proper 32-byte key from input
	derivedKey := as.deriveKey(key, 32)

	// Create AES cipher
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Add PKCS7 padding
	padding := aes.BlockSize - len(data)%aes.BlockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	data = append(data, padtext...)

	// Generate random IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	// Encrypt
	mode := cipher.NewCBCEncrypter(block, iv)
	ciphertext := make([]byte, len(data))
	mode.CryptBlocks(ciphertext, data)

	// Prepend IV to ciphertext
	result := make([]byte, len(iv)+len(ciphertext))
	copy(result[:len(iv)], iv)
	copy(result[len(iv):], ciphertext)

	return result, nil
}

// decryptAESCBC decrypts data using AES-256-CBC
func (as *AdvancedSecurity) decryptAESCBC(data []byte, key []byte) ([]byte, error) {
	// Derive a proper 32-byte key from input
	derivedKey := as.deriveKey(key, 32)

	// Create AES cipher
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Check minimum length
	if len(data) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	// Extract IV and ciphertext
	iv := data[:aes.BlockSize]
	ciphertext := data[aes.BlockSize:]

	// Check if ciphertext is multiple of block size
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext is not a multiple of the block size")
	}

	// Decrypt
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove PKCS7 padding
	if len(plaintext) == 0 {
		return nil, errors.New("invalid padding")
	}
	padding := int(plaintext[len(plaintext)-1])
	if padding > aes.BlockSize || padding == 0 {
		return nil, errors.New("invalid padding")
	}
	for i := len(plaintext) - padding; i < len(plaintext); i++ {
		if plaintext[i] != byte(padding) {
			return nil, errors.New("invalid padding")
		}
	}
	plaintext = plaintext[:len(plaintext)-padding]

	return plaintext, nil
}

// deriveKey derives a key with deterministic salt for peer synchronization
func (as *AdvancedSecurity) deriveKey(inputKey []byte, length int) []byte {
	// Use deterministic salt based on input key for peer synchronization
	saltSource := append(inputKey, []byte("trustdrop-v3-salt-2024")...)
	salt := sha256.Sum256(saltSource)

	info := []byte("trustdrop-v3-secure-key-derivation-2024")
	hkdf := hkdf.New(sha256.New, inputKey, salt[:], info)

	derivedKey := make([]byte, length)
	if _, err := io.ReadFull(hkdf, derivedKey); err != nil {
		// Secure fallback using PBKDF2 with deterministic salt
		derivedKey = pbkdf2.Key(inputKey, salt[:], 100000, length, sha256.New)
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

// GetRecommendedMode returns the recommended encryption mode based on threat level and data characteristics
func (as *AdvancedSecurity) GetRecommendedMode(dataSize int64, networkType string) EncryptionMode {
	// Recommendations based on institutional network requirements
	switch networkType {
	case "corporate", "university", "institutional":
		// For institutional networks, prioritize compatibility
		if dataSize > 50*1024*1024 { // Large files
			return ModeGCM // Good balance of security and compatibility
		} else {
			return ModeGCM // Default to GCM for institutional networks
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

// SetPreferredMode sets the preferred encryption mode
func (as *AdvancedSecurity) SetPreferredMode(mode EncryptionMode) {
	as.preferredMode = mode
}

// GetPreferredMode returns the preferred encryption mode
func (as *AdvancedSecurity) GetPreferredMode() EncryptionMode {
	return as.preferredMode
}
