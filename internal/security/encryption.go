package security

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
)

// Encryption provides AES-256-GCM encryption/decryption for sensitive fields.
type Encryption struct {
	key []byte
}

// NewEncryption creates an Encryption instance using BI_ENCRYPTION_KEY env var.
// The key must be a 32-byte (256-bit) base64-encoded string.
func NewEncryption() (*Encryption, error) {
	keyB64 := os.Getenv("BI_ENCRYPTION_KEY")
	if keyB64 == "" {
		return nil, errors.New("BI_ENCRYPTION_KEY environment variable is not set")
	}

	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, fmt.Errorf("invalid BI_ENCRYPTION_KEY: must be base64 encoded: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid BI_ENCRYPTION_KEY: decoded key must be 32 bytes, got %d", len(key))
	}

	return &Encryption{key: key}, nil
}

// NewEncryptionWithKey creates an Encryption instance with the provided key bytes.
// Useful for testing or when the key is loaded from a config file.
func NewEncryptionWithKey(key []byte) (*Encryption, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key: must be 32 bytes, got %d", len(key))
	}
	return &Encryption{key: key}, nil
}

// Encrypt encrypts plaintext and returns base64-encoded ciphertext.
// Format: base64(nonce + ciphertext) where nonce is 12 bytes.
func (e *Encryption) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded ciphertext produced by Encrypt.
func (e *Encryption) Decrypt(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(plaintext), nil
}

// IsEncrypted checks if a value looks like our encrypted format (base64, sufficiently long).
// This is a heuristic used during migration to detect plaintext DSNs.
func (e *Encryption) IsEncrypted(value string) bool {
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return false
	}
	// Encrypted value = 12-byte nonce + at least some ciphertext
	return len(decoded) > 12
}
