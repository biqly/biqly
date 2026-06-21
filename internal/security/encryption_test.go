package security

import (
	"encoding/base64"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

func makeTestKeyB64() string {
	return base64.StdEncoding.EncodeToString(makeTestKey())
}

func TestDecodeEncryptionKeyB64_Valid(t *testing.T) {
	keyB64 := makeTestKeyB64()
	key, err := decodeEncryptionKeyB64(keyB64)
	require.NoError(t, err)
	assert.Equal(t, 32, len(key))
	assert.Equal(t, makeTestKey(), key)
}

func TestDecodeEncryptionKeyB64_InvalidBase64(t *testing.T) {
	_, err := decodeEncryptionKeyB64("not-valid-base64!!!")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "base64 encoded")
}

func TestDecodeEncryptionKeyB64_WrongLength(t *testing.T) {
	// 16 bytes encoded is valid base64 but wrong key length
	short := base64.StdEncoding.EncodeToString([]byte("0123456789abcdef"))
	_, err := decodeEncryptionKeyB64(short)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "32 bytes")
}

func TestNewEncryptionFromBase64_Valid(t *testing.T) {
	enc, err := NewEncryptionFromBase64(makeTestKeyB64())
	require.NoError(t, err)
	require.NotNil(t, enc)

	// Verify it works
	ct, err := enc.Encrypt("test")
	require.NoError(t, err)
	pt, err := enc.Decrypt(ct)
	require.NoError(t, err)
	assert.Equal(t, "test", pt)
}

func TestNewEncryptionFromBase64_Empty(t *testing.T) {
	_, err := NewEncryptionFromBase64("")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestNewEncryptionFromBase64_InvalidKey(t *testing.T) {
	_, err := NewEncryptionFromBase64("aGVsbG8=") // valid base64 but only 5 bytes
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid encryption key")
}

func TestNewEncryption_EnvSet(t *testing.T) {
	t.Setenv("BI_ENCRYPTION_KEY", makeTestKeyB64())
	enc, err := NewEncryption()
	require.NoError(t, err)
	require.NotNil(t, enc)

	ct, err := enc.Encrypt("env-test")
	require.NoError(t, err)
	pt, err := enc.Decrypt(ct)
	require.NoError(t, err)
	assert.Equal(t, "env-test", pt)
}

func TestNewEncryption_EnvNotSet(t *testing.T) {
	_ = os.Unsetenv("BI_ENCRYPTION_KEY")
	_, err := NewEncryption()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BI_ENCRYPTION_KEY")
}

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
	enc, err := NewEncryptionWithKey(key)
	require.NoError(t, err)

	encrypted1, err := enc.Encrypt("same")
	require.NoError(t, err)
	encrypted2, err := enc.Encrypt("same")
	require.NoError(t, err)

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
	enc, err := NewEncryptionWithKey(key)
	require.NoError(t, err)

	_, err = enc.Decrypt("not-base64!!!")
	require.Error(t, err)

	_, err = enc.Decrypt(base64Short())
	require.Error(t, err)
}

func TestEncryptionIsEncrypted(t *testing.T) {
	key := make([]byte, 32)
	enc, err := NewEncryptionWithKey(key)
	require.NoError(t, err)

	encrypted, err := enc.Encrypt("test")
	require.NoError(t, err)
	assert.True(t, enc.IsEncrypted(encrypted))

	assert.False(t, enc.IsEncrypted("plaintext"))
	assert.False(t, enc.IsEncrypted("not-base64"))
	assert.False(t, enc.IsEncrypted("c29tZS1sb25nLWJhc2U2NC1ibG9iLXRoYXQtaXMtbm90LWNpcGhlcnRleHQ="))
}

func base64Short() string {
	return "YWJj" // base64 of "abc", too short to be valid encrypted data
}
