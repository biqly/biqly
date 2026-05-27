package mail

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmailTemplateRegistryEnglishFallback(t *testing.T) {
	reg, err := newEmailTemplateRegistry("en")
	require.NoError(t, err)

	subject, text, html, err := reg.Render("verification", "fr", map[string]any{"URL": "https://example.com/verify?token=xyz"})
	require.NoError(t, err)
	assert.Equal(t, "Verify your ABI account", subject, "missing locale must fall back to default")
	assert.Contains(t, text, "https://example.com/verify?token=xyz")
	assert.Contains(t, html, `href="https://example.com/verify?token=xyz"`)
}

func TestEmailTemplateRegistryTurkish(t *testing.T) {
	reg, err := newEmailTemplateRegistry("en")
	require.NoError(t, err)

	subject, text, html, err := reg.Render("password_reset", "tr", map[string]any{"URL": "https://x/y"})
	require.NoError(t, err)
	assert.Contains(t, subject, "parolanızı")
	assert.Contains(t, text, "https://x/y")
	assert.Contains(t, html, `href="https://x/y"`)
}

func TestEmailTemplateConditionalSubstitution(t *testing.T) {
	reg, err := newEmailTemplateRegistry("en")
	require.NoError(t, err)

	_, textNew, _, err := reg.Render("email_change", "en", map[string]any{"URL": "u", "NewEmail": true})
	require.NoError(t, err)
	assert.Contains(t, textNew, "Confirm this as your new")

	_, textOld, _, err := reg.Render("email_change", "en", map[string]any{"URL": "u", "NewEmail": false})
	require.NoError(t, err)
	assert.Contains(t, textOld, "Confirm this email change")
}

func TestEmailTemplateHTMLEscapingAppliesToData(t *testing.T) {
	// HTML body uses html/template; substituted data must be escaped so
	// hostile UAs cannot inject script tags via the new-device email.
	reg, err := newEmailTemplateRegistry("en")
	require.NoError(t, err)

	hostile := `<script>alert(1)</script>`
	_, _, html, err := reg.Render("new_device", "en", map[string]any{
		"OccurredAt":  "now",
		"IPAddress":   "127.0.0.1",
		"UserAgent":   hostile,
		"SecurityURL": "https://example.com/security",
	})
	require.NoError(t, err)
	assert.NotContains(t, html, hostile, "hostile UA must be HTML-escaped")
	assert.Contains(t, html, "&lt;script&gt;")
}

func TestBuildMultipartMessage(t *testing.T) {
	headers := map[string]string{
		"From":    "noreply@example.com",
		"To":      "user@example.com",
		"Subject": "Test",
	}
	msg, err := buildMultipartMessage(headers, "plain body", "<p>html body</p>")
	require.NoError(t, err)

	s := string(msg)
	assert.Contains(t, s, "MIME-Version: 1.0")
	assert.Contains(t, s, "multipart/alternative; boundary=")
	assert.Contains(t, s, "Content-Type: text/plain")
	assert.Contains(t, s, "Content-Type: text/html")
	// Plain and HTML bodies appear once each (quoted-printable wraps lines
	// but our short bodies are preserved verbatim).
	assert.Equal(t, 1, strings.Count(s, "plain body"))
	assert.Contains(t, s, "html body")
	// Multipart epilogue terminates the message.
	assert.True(t, strings.HasSuffix(strings.TrimRight(s, "\r\n"), "--"))
}

func TestBuildMultipartMessageWithLogo(t *testing.T) {
	headers := map[string]string{
		"From":    "noreply@example.com",
		"To":      "user@example.com",
		"Subject": "Test Logo",
	}
	msg, err := buildMultipartMessage(headers, "plain body", `<p>html body with <img src="cid:abi-logo" /></p>`)
	require.NoError(t, err)

	s := string(msg)
	assert.Contains(t, s, "MIME-Version: 1.0")
	assert.Contains(t, s, "multipart/related;")
	assert.Contains(t, s, "type=\"multipart/alternative\"")
	assert.Contains(t, s, "Content-Type: multipart/alternative;")
	assert.Contains(t, s, "Content-Type: text/plain")
	assert.Contains(t, s, "Content-Type: text/html")
	assert.Contains(t, s, "Content-Type: image/png; name=\"abi-logo.png\"")
	assert.Contains(t, s, "Content-ID: <abi-logo>")
	assert.Contains(t, s, "Content-Disposition: inline; filename=\"abi-logo.png\"")
	assert.Contains(t, s, "plain body")
	assert.Contains(t, s, "html body with")
}

