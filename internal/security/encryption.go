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
	key   []byte
	block cipher.Block
	aead  cipher.AEAD
}

func decodeEncryptionKeyB64(keyB64 string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(keyB64)
	if err != nil {
		return nil, fmt.Errorf("must be base64 encoded: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("decoded key must be 32 bytes, got %d", len(key))
	}
	return key, nil
}

func newEncryptionFromKeyBytes(key []byte) (*Encryption, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	return &Encryption{key: key, block: block, aead: aead}, nil
}

// NewEncryption creates an Encryption instance using BI_ENCRYPTION_KEY env var.
// The key must be a 32-byte (256-bit) base64-encoded string.
func NewEncryption() (*Encryption, error) {
	keyB64 := os.Getenv("BI_ENCRYPTION_KEY")
	if keyB64 == "" {
		return nil, errors.New("BI_ENCRYPTION_KEY environment variable is not set")
	}
	return NewEncryptionFromBase64(keyB64)
}

// NewEncryptionFromBase64 creates an Encryption instance from a base64-encoded 32-byte key.
func NewEncryptionFromBase64(keyB64 string) (*Encryption, error) {
	if keyB64 == "" {
		return nil, errors.New("encryption key is empty")
	}
	key, err := decodeEncryptionKeyB64(keyB64)
	if err != nil {
		return nil, fmt.Errorf("invalid encryption key: %w", err)
	}
	return newEncryptionFromKeyBytes(key)
}

// NewEncryptionWithKey creates an Encryption instance with the provided key bytes.
// Useful for testing or when the key is loaded from a config file.
func NewEncryptionWithKey(key []byte) (*Encryption, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key: must be 32 bytes, got %d", len(key))
	}
	return newEncryptionFromKeyBytes(key)
}

// Encrypt encrypts plaintext and returns base64-encoded ciphertext.
// Format: base64(nonce + ciphertext) where nonce is 12 bytes.
func (e *Encryption) Encrypt(plaintext string) (string, error) {
	nonce := make([]byte, e.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := e.aead.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded ciphertext produced by Encrypt.
func (e *Encryption) Decrypt(encoded string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	nonceSize := e.aead.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := e.aead.Open(nil, nonce, ciphertext, nil)
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
