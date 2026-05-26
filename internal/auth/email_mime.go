package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"maps"
	"mime/quotedprintable"
	"strings"
	"time"
)

// buildMultipartMessage produces an RFC 2045/2046 multipart/alternative MIME
// message with both a plain-text and an HTML body. Headers must include at
// least From, To, and Subject; the function injects MIME-related headers and
// will not overwrite any caller-supplied entries.
func buildMultipartMessage(headers map[string]string, textBody, htmlBody string) ([]byte, error) {
	boundary, err := randomBoundary()
	if err != nil {
		return nil, err
	}
	hdr := map[string]string{
		"MIME-Version": "1.0",
		"Content-Type": fmt.Sprintf("multipart/alternative; boundary=%q", boundary),
		"Date":         time.Now().UTC().Format(time.RFC1123Z),
	}
	maps.Copy(hdr, headers)

	var b strings.Builder
	for _, k := range []string{"Date", "From", "To", "Subject", "MIME-Version", "Content-Type", "List-Unsubscribe", "Auto-Submitted"} {
		if v, ok := hdr[k]; ok && v != "" {
			fmt.Fprintf(&b, "%s: %s\r\n", k, v)
		}
	}
	b.WriteString("\r\n")

	// text/plain part
	fmt.Fprintf(&b, "--%s\r\n", boundary)
	b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	if err := writeQP(&b, textBody); err != nil {
		return nil, err
	}
	b.WriteString("\r\n")

	// text/html part
	fmt.Fprintf(&b, "--%s\r\n", boundary)
	b.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	if err := writeQP(&b, htmlBody); err != nil {
		return nil, err
	}
	b.WriteString("\r\n")

	fmt.Fprintf(&b, "--%s--\r\n", boundary)
	return []byte(b.String()), nil
}

func writeQP(b *strings.Builder, body string) error {
	w := quotedprintable.NewWriter(b)
	if _, err := w.Write([]byte(body)); err != nil {
		return err
	}
	return w.Close()
}

func randomBoundary() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return "biqly-" + hex.EncodeToString(buf[:]), nil
}
