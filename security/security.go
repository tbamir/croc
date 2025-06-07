package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"fmt"
	"io"
)

// EncryptAES256CBC encrypts data using AES-256-CBC
func EncryptAES256CBC(data []byte, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes for AES-256")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Add PKCS7 padding
	padding := aes.BlockSize - len(data)%aes.BlockSize
	padtext := make([]byte, padding)
	for i := range padtext {
		padtext[i] = byte(padding)
	}
	data = append(data, padtext...)

	// Generate random IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, err
	}

	// Encrypt
	mode := cipher.NewCBCEncrypter(block, iv)
	ciphertext := make([]byte, len(data))
	mode.CryptBlocks(ciphertext, data)

	// Prepend IV to ciphertext
	return append(iv, ciphertext...), nil
}

// DecryptAES256CBC decrypts data using AES-256-CBC
func DecryptAES256CBC(data []byte, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("key must be 32 bytes for AES-256")
	}

	if len(data) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	// Extract IV and ciphertext
	iv := data[:aes.BlockSize]
	ciphertext := data[aes.BlockSize:]

	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, fmt.Errorf("ciphertext is not a multiple of the block size")
	}

	// Decrypt
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	// Remove PKCS7 padding
	padding := int(plaintext[len(plaintext)-1])
	if padding > aes.BlockSize || padding == 0 {
		return nil, fmt.Errorf("invalid padding")
	}

	for i := len(plaintext) - padding; i < len(plaintext); i++ {
		if plaintext[i] != byte(padding) {
			return nil, fmt.Errorf("invalid padding")
		}
	}

	return plaintext[:len(plaintext)-padding], nil
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
