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

var mimeHeaderOrder = []string{"Date", "From", "To", "Subject", "MIME-Version", "Content-Type", "List-Unsubscribe", "Auto-Submitted"}

// buildMultipartMessage produces an RFC 2045/2046 MIME message.
// If the HTML body contains references to "cid:abi-logo", it constructs a
// multipart/related message enclosing a multipart/alternative body and the logo
// image inline attachment. Otherwise, it defaults to a standard multipart/alternative message.
func buildMultipartMessage(headers map[string]string, textBody, htmlBody string) ([]byte, error) {
	hasLogo := len(logoBytes) > 0 && strings.Contains(htmlBody, "cid:abi-logo")
	if !hasLogo {
		return buildAlternativeMessage(headers, textBody, htmlBody)
	}
	return buildRelatedMessage(headers, textBody, htmlBody)
}

func buildAlternativeMessage(headers map[string]string, textBody, htmlBody string) ([]byte, error) {
	boundary, err := randomBoundary()
	if err != nil {
		return nil, err
	}
	hdr := baseMIMEHeaders(fmt.Sprintf("multipart/alternative; boundary=%q", boundary), headers)

	var b strings.Builder
	if err := writeMIMEHeaders(&b, hdr); err != nil {
		return nil, err
	}
	if err := writeAlternativePart(&b, boundary, "text/plain", textBody); err != nil {
		return nil, err
	}
	if err := writeAlternativePart(&b, boundary, "text/html", htmlBody); err != nil {
		return nil, err
	}
	if err := writeBuilderf(&b, "--%s--\r\n", boundary); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func buildRelatedMessage(headers map[string]string, textBody, htmlBody string) ([]byte, error) {
	relatedBoundary, err := randomBoundary()
	if err != nil {
		return nil, err
	}
	altBoundary, err := randomBoundary()
	if err != nil {
		return nil, err
	}
	hdr := baseMIMEHeaders(fmt.Sprintf("multipart/related; type=\"multipart/alternative\"; boundary=%q", relatedBoundary), headers)

	var b strings.Builder
	if err := writeMIMEHeaders(&b, hdr); err != nil {
		return nil, err
	}

	// Outer related part: first element is the alternative part containing text & html
	if err := writeBuilderf(&b, "--%s\r\n", relatedBoundary); err != nil {
		return nil, err
	}
	if err := writeBuilderf(&b, "Content-Type: multipart/alternative; boundary=%q\r\n\r\n", altBoundary); err != nil {
		return nil, err
	}
	if err := writeAlternativePart(&b, altBoundary, "text/plain", textBody); err != nil {
		return nil, err
	}
	if err := writeAlternativePart(&b, altBoundary, "text/html", htmlBody); err != nil {
		return nil, err
	}

	// End of multipart/alternative
	if err := writeBuilderf(&b, "--%s--\r\n\r\n", altBoundary); err != nil {
		return nil, err
	}

	// Outer related part: image attachment
	if err := writeBuilderf(&b, "--%s\r\n", relatedBoundary); err != nil {
		return nil, err
	}
	if err := writeBuilderString(&b, "Content-Type: image/png; name=\"abi-logo.png\"\r\n"); err != nil {
		return nil, err
	}
	if err := writeBuilderString(&b, "Content-Transfer-Encoding: base64\r\n"); err != nil {
		return nil, err
	}
	if err := writeBuilderString(&b, "Content-ID: <abi-logo>\r\n"); err != nil {
		return nil, err
	}
	if err := writeBuilderString(&b, "Content-Disposition: inline; filename=\"abi-logo.png\"\r\n\r\n"); err != nil {
		return nil, err
	}
	if err := writeBuilderString(&b, base64Wrap(logoBytes)); err != nil {
		return nil, err
	}
	if err := writeBuilderString(&b, "\r\n"); err != nil {
		return nil, err
	}

	// End of multipart/related
	if err := writeBuilderf(&b, "--%s--\r\n", relatedBoundary); err != nil {
		return nil, err
	}
	return []byte(b.String()), nil
}

func baseMIMEHeaders(contentType string, headers map[string]string) map[string]string {
	hdr := map[string]string{
		"MIME-Version": "1.0",
		"Content-Type": contentType,
		"Date":         time.Now().UTC().Format(time.RFC1123Z),
	}
	maps.Copy(hdr, headers)
	return hdr
}

func writeMIMEHeaders(b *strings.Builder, hdr map[string]string) error {
	for _, k := range mimeHeaderOrder {
		if v, ok := hdr[k]; ok && v != "" {
			if err := writeBuilderf(b, "%s: %s\r\n", k, sanitizeHeaderValue(v)); err != nil {
				return err
			}
		}
	}
	return writeBuilderString(b, "\r\n")
}

// sanitizeHeaderValue strips CR, LF, and other control characters from a header
// value so server-side data interpolated into a header (e.g. a semantic model
// name in the Subject) cannot inject additional MIME headers (header injection).
func sanitizeHeaderValue(v string) string {
	return strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == 0 {
			return ' '
		}
		if r < 0x20 && r != '\t' {
			return ' '
		}
		return r
	}, v)
}

func writeAlternativePart(b *strings.Builder, boundary, contentType, body string) error {
	if err := writeBuilderf(b, "--%s\r\n", boundary); err != nil {
		return err
	}
	if err := writeBuilderf(b, "Content-Type: %s; charset=\"utf-8\"\r\n", contentType); err != nil {
		return err
	}
	if err := writeBuilderString(b, "Content-Transfer-Encoding: quoted-printable\r\n\r\n"); err != nil {
		return err
	}
	if err := writeQP(b, body); err != nil {
		return err
	}
	return writeBuilderString(b, "\r\n")
}

func writeBuilderf(b *strings.Builder, format string, args ...any) error {
	_, err := fmt.Fprintf(b, format, args...)
	return err
}

func writeBuilderString(b *strings.Builder, s string) error {
	_, err := b.WriteString(s)
	return err
}

func base64Wrap(data []byte) string {
	encoded := base64.StdEncoding.EncodeToString(data)
	chunks := make([]string, 0, (len(encoded)+75)/76)
	for len(encoded) > 0 {
		chunk := min(76, len(encoded))
		chunks = append(chunks, encoded[:chunk]+"\r\n")
		encoded = encoded[chunk:]
	}
	return strings.Join(chunks, "")
}

func writeQP(b *strings.Builder, body string) (err error) {
	w := quotedprintable.NewWriter(b)
	defer func() {
		if closeErr := w.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	if _, err := w.Write([]byte(body)); err != nil {
		return err
	}
	return nil
}

func randomBoundary() (string, error) {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return "biqly-" + hex.EncodeToString(buf[:]), nil
}
