package mail

import (
	"crypto/rand"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"maps"
	"mime/quotedprintable"
	"strings"
	"time"
)

//go:embed assets/abi-logo.png
var logoBytes []byte

// buildMultipartMessage produces an RFC 2045/2046 MIME message.
// If the HTML body contains references to "cid:abi-logo", it constructs a
// multipart/related message enclosing a multipart/alternative body and the logo
// image inline attachment. Otherwise, it defaults to a standard multipart/alternative message.
func buildMultipartMessage(headers map[string]string, textBody, htmlBody string) ([]byte, error) {
	hasLogo := len(logoBytes) > 0 && strings.Contains(htmlBody, "cid:abi-logo")

	if !hasLogo {
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

	relatedBoundary, err := randomBoundary()
	if err != nil {
		return nil, err
	}
	altBoundary, err := randomBoundary()
	if err != nil {
		return nil, err
	}

	hdr := map[string]string{
		"MIME-Version": "1.0",
		"Content-Type": fmt.Sprintf("multipart/related; type=\"multipart/alternative\"; boundary=%q", relatedBoundary),
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

	// Outer related part: first element is the alternative part containing text & html
	fmt.Fprintf(&b, "--%s\r\n", relatedBoundary)
	fmt.Fprintf(&b, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", altBoundary)

	// Inner alternative part: text/plain
	fmt.Fprintf(&b, "--%s\r\n", altBoundary)
	b.WriteString("Content-Type: text/plain; charset=\"utf-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	if err := writeQP(&b, textBody); err != nil {
		return nil, err
	}
	b.WriteString("\r\n")

	// Inner alternative part: text/html
	fmt.Fprintf(&b, "--%s\r\n", altBoundary)
	b.WriteString("Content-Type: text/html; charset=\"utf-8\"\r\n")
	b.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	if err := writeQP(&b, htmlBody); err != nil {
		return nil, err
	}
	b.WriteString("\r\n")

	// End of multipart/alternative
	fmt.Fprintf(&b, "--%s--\r\n\r\n", altBoundary)

	// Outer related part: image attachment
	fmt.Fprintf(&b, "--%s\r\n", relatedBoundary)
	b.WriteString("Content-Type: image/png; name=\"abi-logo.png\"\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n")
	b.WriteString("Content-ID: <abi-logo>\r\n")
	b.WriteString("Content-Disposition: inline; filename=\"abi-logo.png\"\r\n\r\n")
	b.WriteString(base64Wrap(logoBytes))
	b.WriteString("\r\n")

	// End of multipart/related
	fmt.Fprintf(&b, "--%s--\r\n", relatedBoundary)
	return []byte(b.String()), nil
}

func base64Wrap(data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	var out strings.Builder
	for len(encoded) > 0 {
		chunk := 76
		if len(encoded) < chunk {
			chunk = len(encoded)
		}
		out.WriteString(encoded[:chunk])
		out.WriteString("\r\n")
		encoded = encoded[chunk:]
	}
	return out.String()
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
