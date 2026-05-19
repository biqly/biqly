package security_test

import (
	"testing"

	"github.com/biqly/biqly/internal/security"
)

func TestConnectionDSN_nilEncryptor(t *testing.T) {
	got, err := security.ConnectionDSN(nil, "postgres://localhost/db")
	if err != nil {
		t.Fatal(err)
	}
	if got != "postgres://localhost/db" {
		t.Fatalf("got %q", got)
	}
}

func TestConnectionDSN_plaintextWithEncryptor(t *testing.T) {
	enc, err := security.NewEncryptionWithKey(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	plain := "postgres://user:pass@host:5432/db" //nolint:gosec // test fixture DSN, not a real credential
	got, err := security.ConnectionDSN(enc, plain)
	if err != nil {
		t.Fatal(err)
	}
	if got != plain {
		t.Fatalf("plaintext should pass through, got %q", got)
	}
}

func TestConnectionDSN_roundTrip(t *testing.T) {
	enc, err := security.NewEncryptionWithKey(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	cipher, err := enc.Encrypt("secret-dsn")
	if err != nil {
		t.Fatal(err)
	}
	got, err := security.ConnectionDSN(enc, cipher)
	if err != nil {
		t.Fatal(err)
	}
	if got != "secret-dsn" {
		t.Fatalf("got %q", got)
	}
}
