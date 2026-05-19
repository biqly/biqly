package security

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEncryptionRoundTrip(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	enc, err := NewEncryptionWithKey(key)
	require.NoError(t, err)

	plaintext := "postgres://user:password@localhost:5432/mydb?sslmode=disable" //nolint:gosec // test fixture DSN, not a real credential

	encrypted, err := enc.Encrypt(plaintext)
	require.NoError(t, err)
	assert.NotEqual(t, plaintext, encrypted, "encrypted value should differ from plaintext")

	decrypted, err := enc.Decrypt(encrypted)
	require.NoError(t, err)
	assert.Equal(t, plaintext, decrypted, "decrypted value should match original")
}

func TestEncryptionDifferentNonce(t *testing.T) {
	key := make([]byte, 32)
	enc, _ := NewEncryptionWithKey(key)

	encrypted1, _ := enc.Encrypt("same")
	encrypted2, _ := enc.Encrypt("same")

	assert.NotEqual(t, encrypted1, encrypted2, "two encryptions of same value should differ (random nonce)")
}

func TestEncryptionInvalidKey(t *testing.T) {
	short := make([]byte, 16)
	_, err := NewEncryptionWithKey(short)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "32 bytes")
}

func TestEncryptionDecryptInvalid(t *testing.T) {
	key := make([]byte, 32)
	enc, _ := NewEncryptionWithKey(key)

	_, err := enc.Decrypt("not-base64!!!")
	require.Error(t, err)

	_, err = enc.Decrypt(base64Short())
	require.Error(t, err)
}

func TestEncryptionIsEncrypted(t *testing.T) {
	key := make([]byte, 32)
	enc, _ := NewEncryptionWithKey(key)

	encrypted, _ := enc.Encrypt("test")
	assert.True(t, enc.IsEncrypted(encrypted))

	assert.False(t, enc.IsEncrypted("plaintext"))
	assert.False(t, enc.IsEncrypted("not-base64"))
}

func base64Short() string {
	return "YWJj" // base64 of "abc", too short to be valid encrypted data
}
