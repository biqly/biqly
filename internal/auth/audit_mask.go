package auth

import (
	"fmt"
	"net"
	"strings"
)

func MaskEmail(email string) string {
	if email == "" {
		return ""
	}
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return "***"
	}
	local, domain := email[:at], email[at+1:]
	switch {
	case len(local) <= 1:
		return "*@" + domain
	case len(local) == 2:
		return local[:1] + "*@" + domain
	default:
		return local[:1] + strings.Repeat("*", len(local)-2) + local[len(local)-1:] + "@" + domain
	}
}

func MaskIP(addr string) string {
	if addr == "" {
		return ""
	}
	host := addr
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = h
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return "***"
	}
	if v4 := ip.To4(); v4 != nil {
		return fmt.Sprintf("%d.%d.***.***", v4[0], v4[1])
	}
	v6 := ip.To16()
	if v6 == nil {
		return "***"
	}
	return strings.Join([]string{
		toHex16(v6[0:2]),
		toHex16(v6[2:4]),
		"****",
		"****",
		"****",
		"****",
		"****",
		"****",
	}, ":")
}

func MaskToken(token string) string {
	if len(token) <= 8 {
		return "***"
	}
	return token[:4] + "***" + token[len(token)-4:]
}

func toHex16(b []byte) string {
	const hexDigits = "0123456789abcdef"
	if len(b) != 2 {
		return "****"
	}
	return string([]byte{
		hexDigits[b[0]>>4],
		hexDigits[b[0]&0x0f],
		hexDigits[b[1]>>4],
		hexDigits[b[1]&0x0f],
	})
}
