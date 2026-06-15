package mfa

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // G505: RFC 6238 TOTP uses HMAC-SHA1 for authenticator compatibility.
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	totpDigits    = 6
	totpPeriod    = 30
	totpSkewSteps = 1
)

// GenerateTOTPSecret returns a fresh base32-encoded 160-bit secret without padding.
func GenerateTOTPSecret() (string, error) {
	buf := make([]byte, 20)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return strings.TrimRight(base32.StdEncoding.EncodeToString(buf), "="), nil
}

// BuildOTPAuthURL returns the otpauth://totp/... URI for QR encoding.
func BuildOTPAuthURL(issuer, account, secret string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", strconv.Itoa(totpDigits))
	q.Set("period", strconv.Itoa(totpPeriod))
	return "otpauth://totp/" + label + "?" + q.Encode()
}

func generateTOTPCode(secret string, counter uint64) (string, error) {
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(secret))
	if err != nil {
		return "", fmt.Errorf("invalid totp secret: %w", err)
	}

	buf := make([]byte, 8)
	binary.BigEndian.PutUint64(buf, counter)

	mac := hmac.New(sha1.New, key)
	if _, err := mac.Write(buf); err != nil {
		return "", err
	}
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	binCode := (uint32(sum[offset])&0x7f)<<24 |
		(uint32(sum[offset+1])&0xff)<<16 |
		(uint32(sum[offset+2])&0xff)<<8 |
		(uint32(sum[offset+3]) & 0xff)

	mod := uint32(1)
	for range totpDigits {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", totpDigits, binCode%mod), nil
}

func totpCounter(now time.Time) (uint64, bool) {
	unix := now.Unix()
	if unix < 0 {
		return 0, false
	}
	return uint64(unix) / totpPeriod, true
}

func totpCounterWithSkew(counter uint64, skew int64) (uint64, bool) {
	if skew < 0 {
		neg := uint64(-skew)
		if counter < neg {
			return 0, false
		}
		return counter - neg, true
	}
	return counter + uint64(skew), true
}

// VerifyTOTP checks code against secret with a ±totpSkewSteps window around now.
func VerifyTOTP(secret, code string, now time.Time) bool {
	ok, _ := VerifyTOTPStep(secret, code, now)
	return ok
}

// VerifyTOTPStep is like VerifyTOTP but also returns the matched time-step
// counter on success. Callers can use the step to enforce single-use
// semantics (replay protection). On failure ok is false and step is 0.
func VerifyTOTPStep(secret, code string, now time.Time) (ok bool, step uint64) {
	code = strings.TrimSpace(code)
	if len(code) != totpDigits {
		return false, 0
	}
	counter, ok := totpCounter(now)
	if !ok {
		return false, 0
	}
	for skew := -int64(totpSkewSteps); skew <= int64(totpSkewSteps); skew++ {
		s, ok := totpCounterWithSkew(counter, skew)
		if !ok {
			continue
		}
		generated, err := generateTOTPCode(secret, s)
		if err != nil {
			return false, 0
		}
		if subtle.ConstantTimeCompare([]byte(generated), []byte(code)) == 1 {
			return true, s
		}
	}
	return false, 0
}
