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

func TestRedactDSN_HidesURLPassword(t *testing.T) {
	in := "postgres://alice:s3cret@db.example:5432/app?sslmode=require" //nolint:gosec // test fixture DSN, not a real credential
	got := security.RedactDSN(in)
	if got == in {
		t.Fatalf("expected DSN to be redacted, got %q", got)
	}
	if containsStr(got, "s3cret") {
		t.Fatalf("password leaked in redacted DSN: %q", got)
	}
	if !containsStr(got, "alice") {
		t.Fatalf("expected username retained, got %q", got)
	}
}

func TestRedactDSN_HandlesNoCreds(t *testing.T) {
	in := "postgres://db.example:5432/app"
	if security.RedactDSN(in) != in {
		t.Fatalf("DSN without creds should be unchanged")
	}
}

func TestRedactDSN_KVStyle(t *testing.T) {
	in := "host=db.example user=alice password=hunter2 sslmode=disable"
	got := security.RedactDSN(in)
	if containsStr(got, "hunter2") {
		t.Fatalf("kv-style password leaked: %q", got)
	}
	if !containsStr(got, "password=") {
		t.Fatalf("kv-style password marker missing: %q", got)
	}
}

func TestRedactDSN_Empty(t *testing.T) {
	if got := security.RedactDSN(""); got != "" {
		t.Fatalf("empty DSN should remain empty, got %q", got)
	}
}

func containsStr(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
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

func TestRedactDSN_URLAndMySQLDetails(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{
			"postgres://alice:hunter%402@host:5432/db",
			"postgres://alice:***@host:5432/db",
		},
		{
			"alice:hunter2@tcp(127.0.0.1:3306)/db?pass=secret&foo=bar",
			"alice:***@tcp(127.0.0.1:3306)/db?pass=***&foo=bar",
		},
		{
			"host=localhost pass=secret password=secret2",
			"host=localhost pass=*** password=***",
		},
	}
	for _, tc := range cases {
		got := security.RedactDSN(tc.in)
		if got != tc.want {
			t.Errorf("RedactDSN(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
