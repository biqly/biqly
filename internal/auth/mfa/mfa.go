package mfa

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/auth"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrMFANotEnabled  = errors.New("mfa not enabled")
	ErrMFACodeInvalid = errors.New("invalid mfa code")
)

const recoveryCodeCount = 10

type EnrollResult struct {
	Secret        string
	OTPAuthURL    string
	RecoveryCodes []string
}

type Service struct {
	repo     *Repository
	userRepo *auth.UserRepository
	issuer   string
}

func NewMFAService(repo *Repository, userRepo *auth.UserRepository, issuer string) *Service {
	if issuer == "" {
		issuer = "Biqly"
	}
	return &Service{repo: repo, userRepo: userRepo, issuer: issuer}
}

// Enroll creates a pending TOTP enrollment. Caller must call Verify with a
// code from the authenticator to activate it. Re-enrolling overwrites any
// pending or active enrollment for that user.
func (s *Service) Enroll(ctx context.Context, userID, accountLabel string) (*EnrollResult, error) {
	secret, err := GenerateTOTPSecret()
	if err != nil {
		return nil, err
	}

	plainCodes, hashes, err := generateRecoveryCodes(recoveryCodeCount)
	if err != nil {
		return nil, err
	}

	if err := s.repo.Upsert(ctx, userID, "totp", secret, hashes); err != nil {
		return nil, err
	}

	return &EnrollResult{
		Secret:        secret,
		OTPAuthURL:    BuildOTPAuthURL(s.issuer, accountLabel, secret),
		RecoveryCodes: plainCodes,
	}, nil
}

// Verify activates a pending enrollment by checking a TOTP code.
func (s *Service) Verify(ctx context.Context, userID, code string) error {
	enrol, err := s.repo.Get(ctx, userID)
	if err != nil {
		return err
	}
	if !VerifyTOTP(enrol.Secret, code, time.Now()) {
		return ErrMFACodeInvalid
	}
	if !enrol.Enabled {
		if err := s.repo.Enable(ctx, userID); err != nil {
			return err
		}
	}
	return s.repo.MarkUsed(ctx, userID)
}

// VerifyCode validates either a TOTP code, a recovery code, or a bypass code on an enabled
// enrollment. Recovery codes and bypass codes are single-use.
func (s *Service) VerifyCode(ctx context.Context, userID, code string) error {
	enrol, err := s.repo.Get(ctx, userID)
	if err != nil {
		return err
	}
	if !enrol.Enabled {
		return ErrMFANotEnabled
	}

	if VerifyTOTP(enrol.Secret, code, time.Now()) {
		return s.repo.MarkUsed(ctx, userID)
	}

	for _, h := range enrol.RecoveryCodes {
		if bcrypt.CompareHashAndPassword([]byte(h), []byte(normalizeRecoveryCode(code))) == nil {
			ok, err := s.repo.ConsumeRecoveryCode(ctx, userID, h)
			if err != nil {
				return err
			}
			if ok {
				return s.repo.MarkUsed(ctx, userID)
			}
		}
	}

	// Check bypass codes
	normalizedCode := strings.ToUpper(strings.TrimSpace(code))
	for _, h := range enrol.BypassCodes {
		if bcrypt.CompareHashAndPassword([]byte(h), []byte(normalizedCode)) == nil {
			ok, err := s.repo.ConsumeBypassCode(ctx, userID, h)
			if err != nil {
				return err
			}
			if ok {
				return s.repo.MarkUsed(ctx, userID)
			}
		}
	}

	return ErrMFACodeInvalid
}

func (s *Service) GenerateBypassCode(ctx context.Context, userID string) (string, error) {
	enrol, err := s.repo.Get(ctx, userID)
	if err != nil {
		return "", err
	}
	if !enrol.Enabled {
		return "", ErrMFANotEnabled
	}

	raw := make([]byte, 5)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	code := strings.TrimRight(base32.StdEncoding.EncodeToString(raw), "=")
	bypassCode := "BYPASS-" + code

	h, err := bcrypt.GenerateFromPassword([]byte(bypassCode), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash bypass code: %w", err)
	}

	if err := s.repo.AddBypassCode(ctx, userID, string(h)); err != nil {
		return "", err
	}
	return bypassCode, nil
}

func (s *Service) Disable(ctx context.Context, userID string) error {
	return s.repo.Disable(ctx, userID)
}

// Status returns nil enrollment for users who have never enrolled.
func (s *Service) Status(ctx context.Context, userID string) (*Enrollment, error) {
	enrol, err := s.repo.Get(ctx, userID)
	if errors.Is(err, ErrMFANotEnrolled) {
		return nil, nil //nolint:nilnil // not enrolled is a normal state, not an error
	}
	return enrol, err
}

// RegenerateRecoveryCodes issues a fresh set of recovery codes, invalidating prior ones.
func (s *Service) RegenerateRecoveryCodes(ctx context.Context, userID string) ([]string, error) {
	enrol, err := s.repo.Get(ctx, userID)
	if err != nil {
		return nil, err
	}
	if !enrol.Enabled {
		return nil, ErrMFANotEnabled
	}
	plain, hashes, err := generateRecoveryCodes(recoveryCodeCount)
	if err != nil {
		return nil, err
	}
	if err := s.repo.ReplaceRecoveryCodes(ctx, userID, hashes); err != nil {
		return nil, err
	}
	return plain, nil
}

// IsEnabled reports whether MFA is active for a user. Returns false on not-enrolled.
func (s *Service) IsEnabled(ctx context.Context, userID string) (bool, error) {
	enrol, err := s.repo.Get(ctx, userID)
	if errors.Is(err, ErrMFANotEnrolled) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return enrol.Enabled, nil
}

func generateRecoveryCodes(n int) (plain, hashes []string, err error) {
	plain = make([]string, n)
	hashes = make([]string, n)
	for i := range n {
		raw := make([]byte, 10)
		if _, err = rand.Read(raw); err != nil {
			return nil, nil, err
		}
		code := strings.TrimRight(base32.StdEncoding.EncodeToString(raw), "=")
		formatted := formatRecoveryCode(code)
		h, herr := bcrypt.GenerateFromPassword([]byte(normalizeRecoveryCode(formatted)), bcrypt.DefaultCost)
		if herr != nil {
			return nil, nil, fmt.Errorf("hash recovery code: %w", herr)
		}
		plain[i] = formatted
		hashes[i] = string(h)
	}
	return plain, hashes, nil
}

// formatRecoveryCode renders the raw base32 as XXXX-XXXX-XXXX-XXXX for display.
func formatRecoveryCode(code string) string {
	code = strings.ToUpper(code)
	if len(code) == 16 {
		return code[0:4] + "-" + code[4:8] + "-" + code[8:12] + "-" + code[12:16]
	}
	runes := []rune(code)
	var parts []string
	for i := 0; i < len(runes); i += 4 {
		end := min(i+4, len(runes))
		parts = append(parts, string(runes[i:end]))
	}
	return strings.Join(parts, "-")
}

func normalizeRecoveryCode(code string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(code), "-", ""))
}
