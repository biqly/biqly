package auth

import (
	"strings"
	"testing"
	"time"
)

func TestGenerateTOTPSecret_UniqueAndBase32(t *testing.T) {
	s1, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	s2, _ := GenerateTOTPSecret()
	if s1 == s2 {
		t.Fatalf("expected distinct secrets")
	}
	if strings.ContainsAny(s1, "=") {
		t.Fatalf("padding not stripped: %q", s1)
	}
}

func TestVerifyTOTP_CurrentAndSkewWindows(t *testing.T) {
	secret, _ := GenerateTOTPSecret()
	now := time.Unix(1_700_000_000, 0)

	code, err := generateTOTPCode(secret, uint64(now.Unix()/totpPeriod))
	if err != nil {
		t.Fatalf("code gen: %v", err)
	}
	if !VerifyTOTP(secret, code, now) {
		t.Fatalf("current code should verify")
	}

	prev, _ := generateTOTPCode(secret, uint64(now.Unix()/totpPeriod)-1)
	if !VerifyTOTP(secret, prev, now) {
		t.Fatalf("previous step should verify (skew=1)")
	}
	next, _ := generateTOTPCode(secret, uint64(now.Unix()/totpPeriod)+1)
	if !VerifyTOTP(secret, next, now) {
		t.Fatalf("next step should verify (skew=1)")
	}

	farFuture, _ := generateTOTPCode(secret, uint64(now.Unix()/totpPeriod)+5)
	if VerifyTOTP(secret, farFuture, now) {
		t.Fatalf("far-future code should not verify")
	}
}

func TestVerifyTOTP_RejectsMalformed(t *testing.T) {
	secret, _ := GenerateTOTPSecret()
	if VerifyTOTP(secret, "", time.Now()) {
		t.Fatal("empty code accepted")
	}
	if VerifyTOTP(secret, "12345", time.Now()) {
		t.Fatal("5-digit code accepted")
	}
	if VerifyTOTP(secret, "abcdef", time.Now()) {
		t.Fatal("non-numeric code accepted")
	}
}

func TestBuildOTPAuthURL(t *testing.T) {
	url := BuildOTPAuthURL("Biqly", "user@example.com", "JBSWY3DPEHPK3PXP")
	if !strings.HasPrefix(url, "otpauth://totp/") {
		t.Fatalf("missing scheme: %s", url)
	}
	for _, want := range []string{"secret=JBSWY3DPEHPK3PXP", "issuer=Biqly", "digits=6", "period=30"} {
		if !strings.Contains(url, want) {
			t.Fatalf("missing %q in %q", want, url)
		}
	}
}

func TestRecoveryCodeFormat(t *testing.T) {
	plain, hashes, err := generateRecoveryCodes(3)
	if err != nil {
		t.Fatalf("gen: %v", err)
	}
	if len(plain) != 3 || len(hashes) != 3 {
		t.Fatalf("expected 3 codes, got %d/%d", len(plain), len(hashes))
	}
	for _, code := range plain {
		if !strings.Contains(code, "-") {
			t.Fatalf("expected dashed format: %q", code)
		}
		if normalizeRecoveryCode(code) == code {
			t.Fatalf("normalize should strip dashes: %q", code)
		}
	}
	if plain[0] == plain[1] {
		t.Fatalf("recovery codes collided")
	}
}
