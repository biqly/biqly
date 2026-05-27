package auth

import (
	"fmt"
	"net"
	"strings"

	"github.com/biqly/biqly/internal/emailaddr"
)

func MaskEmail(email string) string {
	return emailaddr.Mask(email)
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
