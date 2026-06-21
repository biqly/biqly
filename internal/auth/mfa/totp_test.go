package mfa

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/testutil"
	"github.com/google/uuid"
)

func TestGenerateTOTPSecret_UniqueAndBase32(t *testing.T) {
	s1, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	s2, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if s1 == s2 {
		t.Fatalf("expected distinct secrets")
	}
	if strings.ContainsAny(s1, "=") {
		t.Fatalf("padding not stripped: %q", s1)
	}
}

func TestVerifyTOTP_CurrentAndSkewWindows(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	now := time.Unix(1_700_000_000, 0)
	counter, ok := totpCounter(now)
	if !ok {
		t.Fatal("counter")
	}

	code, err := generateTOTPCode(secret, counter)
	if err != nil {
		t.Fatalf("code gen: %v", err)
	}
	if !VerifyTOTP(secret, code, now) {
		t.Fatalf("current code should verify")
	}

	prev, err := generateTOTPCode(secret, counter-1)
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	if !VerifyTOTP(secret, prev, now) {
		t.Fatalf("previous step should verify (skew=1)")
	}
	next, err := generateTOTPCode(secret, counter+1)
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	if !VerifyTOTP(secret, next, now) {
		t.Fatalf("next step should verify (skew=1)")
	}

	farFuture, err := generateTOTPCode(secret, counter+5)
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}
	if VerifyTOTP(secret, farFuture, now) {
		t.Fatalf("far-future code should not verify")
	}
}

func TestVerifyTOTP_RejectsMalformed(t *testing.T) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
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

func TestVerifyCodeRejectsReplayedTOTPStep(t *testing.T) {
	db := testutil.OpenAuthDB(t)
	ctx := context.Background()

	var userID string
	err := db.QueryRowContext(ctx, `
		INSERT INTO users (email, display_name, password_hash, email_verified)
		VALUES ($1, 'TOTP Replay', 'hash', TRUE)
		RETURNING id
	`, "totp-replay-"+uuid.NewString()+"@example.com").Scan(&userID)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		testutil.PurgeAuthUserByID(ctx, t, db)
	})

	secret, err := GenerateTOTPSecret()
	if err != nil {
		t.Fatalf("generate secret: %v", err)
	}
	repo := NewMFARepository(db, nil)
	if err := repo.Upsert(ctx, userID, "totp", secret, nil); err != nil {
		t.Fatalf("upsert mfa: %v", err)
	}
	if err := repo.Enable(ctx, userID); err != nil {
		t.Fatalf("enable mfa: %v", err)
	}

	counter, ok := totpCounter(time.Now())
	if !ok {
		t.Fatal("current totp counter overflowed")
	}
	code, err := generateTOTPCode(secret, counter)
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}

	svc := NewMFAService(repo, nil, "Biqly")
	if err := svc.VerifyCode(ctx, userID, code); err != nil {
		t.Fatalf("first verify: %v", err)
	}
	if err := svc.VerifyCode(ctx, userID, code); !errors.Is(err, ErrTOTPCodeAlreadyUsed) {
		t.Fatalf("replayed verify error = %v, want %v", err, ErrTOTPCodeAlreadyUsed)
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
