package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
)

// EncryptAES256CBC encrypts data using AES-256-CBC (legacy compatibility)
func EncryptAES256CBC(data, key []byte) ([]byte, error) {
	// Derive a proper 32-byte key
	hasher := sha256.New()
	hasher.Write(key)
	derivedKey := hasher.Sum(nil)

	// Create AES cipher
	block, err := aes.NewCipher(derivedKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Generate random IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, fmt.Errorf("failed to generate IV: %w", err)
	}

	// Pad data to block size
	paddedData := pkcs7Pad(data, aes.BlockSize)

	// Encrypt
	mode := cipher.NewCBCEncrypter(block, iv)
	ciphertext := make([]byte, len(paddedData))
	mode.CryptBlocks(ciphertext, paddedData)

	// Prepend IV to ciphertext
	result := make([]byte, len(iv)+len(ciphertext))
	copy(result[:len(iv)], iv)
	copy(result[len(iv):], ciphertext)

	return result, nil
}

// DecryptAES256CBC decrypts data using AES-256-CBC (legacy compatibility)
func DecryptAES256CBC(data, key []byte) ([]byte, error) {
	// Derive a proper 32-byte key
	hasher := sha256.New()
	hasher.Write(key)
	derivedKey := hasher.Sum(nil)

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

	// Check if ciphertext is properly aligned
	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext is not a multiple of block size")
	}

	// Decrypt
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove padding
	return pkcs7Unpad(plaintext)
}

// pkcs7Pad pads data to the specified block size using PKCS#7
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := make([]byte, padding)
	for i := range padText {
		padText[i] = byte(padding)
	}
	return append(data, padText...)
}

// pkcs7Unpad removes PKCS#7 padding from data
func pkcs7Unpad(data []byte) ([]byte, error) {
	if len(data) == 0 {
		return nil, errors.New("data is empty")
	}

	padding := int(data[len(data)-1])
	if padding == 0 || padding > len(data) {
		return nil, errors.New("invalid padding")
	}

	// Verify padding
	for i := len(data) - padding; i < len(data); i++ {
		if data[i] != byte(padding) {
			return nil, errors.New("invalid padding")
		}
	}

	return data[:len(data)-padding], nil
}

// GenerateKey generates a random 256-bit key
func GenerateKey() ([]byte, error) {
	key := make([]byte, 32) // 256 bits
	_, err := rand.Read(key)
	return key, err
}

// HashFile calculates SHA-256 hash of file data for integrity verification
func HashFile(data []byte) []byte {
	hash := sha256.Sum256(data)
	return hash[:]
}

// VerifyFileIntegrity verifies file integrity using SHA-256 hash
func VerifyFileIntegrity(data []byte, expectedHash []byte) bool {
	actualHash := HashFile(data)
	if len(actualHash) != len(expectedHash) {
		return false
	}

	for i := range actualHash {
		if actualHash[i] != expectedHash[i] {
			return false
		}
	}

	return true
}
