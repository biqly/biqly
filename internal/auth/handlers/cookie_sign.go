package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"strings"
)

func signProtectedCookie(secret, payload string) string {
	if secret == "" {
		return payload
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	sig := base64.URLEncoding.EncodeToString(mac.Sum(nil))
	return payload + "." + sig
}

func verifyProtectedCookie(secret, value string) (payload string, ok bool) {
	if secret == "" {
		return value, true
	}
	idx := strings.LastIndex(value, ".")
	if idx <= 0 {
		return "", false
	}
	payload = value[:idx]
	sigB64 := value[idx+1:]
	sig, err := base64.URLEncoding.DecodeString(sigB64)
	if err != nil {
		return "", false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(payload))
	expected := mac.Sum(nil)
	if subtle.ConstantTimeCompare(sig, expected) != 1 {
		return "", false
	}
	return payload, true
}
