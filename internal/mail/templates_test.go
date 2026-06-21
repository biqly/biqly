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

// --- isTruthy tests ---

func TestIsTruthy(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  bool
	}{
		{"nil", nil, false},
		{"true", true, true},
		{"false", false, false},
		{"empty string", "", false},
		{"non-empty string", "hello", true},
		{"int zero", 0, false},
		{"int non-zero", 42, true},
		{"int64 zero", int64(0), false},
		{"int64 non-zero", int64(-1), true},
		{"float64 zero", float64(0), false},
		{"float64 non-zero", 3.14, true},
		{"int8 (default case)", int8(1), true},
		{"uint (default case)", uint(0), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, isTruthy(tt.value))
		})
	}
}

// --- Render edge cases ---

func TestRender_UnknownTemplate(t *testing.T) {
	reg, err := newEmailTemplateRegistry("en")
	require.NoError(t, err)

	_, _, _, err = reg.Render("nonexistent", "en", nil)
	assert.ErrorContains(t, err, "unknown email template")
}

func TestRender_FallbackToEnglishWhenDefaultMissing(t *testing.T) {
	reg, err := newEmailTemplateRegistry("fr") // French default, but only en+tr exist
	require.NoError(t, err)

	subject, _, _, err := reg.Render("verification", "de", map[string]any{"URL": "x"})
	require.NoError(t, err)
	assert.Equal(t, "Verify your ABI account", subject) // falls back to en
}

func TestRender_DefaultLocaleFallback(t *testing.T) {
	reg, err := newEmailTemplateRegistry("tr") // Turkish default
	require.NoError(t, err)

	subject, _, _, err := reg.Render("verification", "de", map[string]any{"URL": "x"})
	require.NoError(t, err)
	assert.Contains(t, subject, "doğrulayın") // falls back to "tr" (default) first
}

// --- renderPlain tests ---

func TestRenderPlain(t *testing.T) {
	result := renderPlain("Hello {{.Name}}, your score is {{.Score}}", map[string]any{
		"Name":  "Alice",
		"Score": 95,
	})
	assert.Equal(t, "Hello Alice, your score is 95", result)
}

func TestRenderPlain_MissingField(t *testing.T) {
	result := renderPlain("Hello {{.Name}}", map[string]any{})
	assert.Equal(t, "Hello ", result)
}

func TestRenderPlain_NoPlaceholders(t *testing.T) {
	result := renderPlain("Just a plain string", map[string]any{"anything": "x"})
	assert.Equal(t, "Just a plain string", result)
}

func TestRenderPlain_MalformedOpenOnly(t *testing.T) {
	result := renderPlain("Hello {{.Name", map[string]any{"Name": "Alice"})
	assert.Equal(t, "Hello {{.Name", result)
}

func TestRenderPlain_MalformedNoClose(t *testing.T) {
	result := renderPlain("Hello {{", map[string]any{"Name": "Alice"})
	assert.Equal(t, "Hello {{", result)
}

func TestRenderPlain_NonStringValue(t *testing.T) {
	result := renderPlain("Count: {{.Count}}", map[string]any{"Count": 42})
	assert.Equal(t, "Count: 42", result)
}

// --- renderConditionals tests ---

func TestRenderConditionals(t *testing.T) {
	tmpl := "{{if .Flag}}yes{{else}}no{{end}}"
	result := renderConditionals(tmpl, map[string]any{"Flag": true})
	assert.Equal(t, "yes", result)

	result = renderConditionals(tmpl, map[string]any{"Flag": false})
	assert.Equal(t, "no", result)
}

func TestRenderConditionals_NoElse(t *testing.T) {
	tmpl := "{{if .Flag}}yes{{end}}"
	result := renderConditionals(tmpl, map[string]any{"Flag": true})
	assert.Equal(t, "yes", result)

	result = renderConditionals(tmpl, map[string]any{"Flag": false})
	assert.Equal(t, "", result)
}

func TestRenderConditionals_MissingEnd(t *testing.T) {
	tmpl := "{{if .Flag}}yes"
	result := renderConditionals(tmpl, map[string]any{"Flag": true})
	assert.Equal(t, tmpl, result, "should return unchanged when no {{end}} is found")
}

func TestRenderConditionals_MalformedIfExpr(t *testing.T) {
	tmpl := "{{if}}yes{{end}}"
	result := renderConditionals(tmpl, map[string]any{"Flag": true})
	// Just check it doesn't panic
	assert.Equal(t, tmpl, result)
}

func TestRenderConditionals_NoIfClose(t *testing.T) {
	tmpl := "{{if .Flag"
	result := renderConditionals(tmpl, map[string]any{"Flag": true})
	assert.Equal(t, tmpl, result)
}

// --- writeAlternativePart edge case through buildMultipartMessage ---

func TestBuildMultipartMessage_EmptyBodies(t *testing.T) {
	headers := map[string]string{
		"From":    "no-reply@x.com",
		"To":      "user@x.com",
		"Subject": "Empty test",
	}
	msg, err := buildMultipartMessage(headers, "", "")
	require.NoError(t, err)
	s := string(msg)
	assert.Contains(t, s, "Content-Type: text/plain")
	assert.Contains(t, s, "Content-Type: text/html")
}

// --- buildAlternativeMessage directly ---

func TestBuildAlternativeMessage(t *testing.T) {
	headers := map[string]string{
		"From":    "from@x.com",
		"To":      "to@x.com",
		"Subject": "Test",
	}
	msg, err := buildAlternativeMessage(headers, "text", "<p>html</p>")
	require.NoError(t, err)
	s := string(msg)
	assert.Contains(t, s, "multipart/alternative")
	assert.Contains(t, s, "text/plain")
	assert.Contains(t, s, "text/html")
	assert.Contains(t, s, "text")
	assert.Contains(t, s, "html")
}

// --- buildRelatedMessage edge cases ---

func TestBuildRelatedMessage_DirectCall(t *testing.T) {
	headers := map[string]string{
		"From":    "from@x.com",
		"To":      "to@x.com",
		"Subject": "Related test",
	}
	msg, err := buildRelatedMessage(headers, "plain body", "<p>html with <img src=\"cid:abi-logo\" /></p>")
	require.NoError(t, err)
	s := string(msg)
	assert.Contains(t, s, "multipart/related")
	assert.Contains(t, s, "abi-logo.png")
	assert.Contains(t, s, "Content-ID: <abi-logo>")
	assert.Contains(t, s, "plain body")
}

// --- wrapHTML tests ---

func TestWrapHTML_English(t *testing.T) {
	result := wrapHTML("<p>body</p>", "Subject Line", "en")
	assert.Contains(t, result, "Subject Line")
	assert.Contains(t, result, "<p>body</p>")
	assert.Contains(t, result, "This is an automated email")
	assert.NotContains(t, result, "otomatik")
}

func TestWrapHTML_Turkish(t *testing.T) {
	result := wrapHTML("<p>body</p>", "Konu", "tr")
	assert.Contains(t, result, "Bu otomatik bir e-postadır")
	assert.NotContains(t, result, "This is an automated email")
}

func TestWrapHTML_TurkishPrefix(t *testing.T) {
	result := wrapHTML("<p>body</p>", "Konu", "tr-TR")
	assert.Contains(t, result, "Bu otomatik bir e-postadır")
}

// --- newEmailTemplateRegistry errors ---

func TestNewEmailTemplateRegistry_DefaultLocaleEmpty(t *testing.T) {
	reg, err := newEmailTemplateRegistry("")
	require.NoError(t, err)
	// Should default to "en"
	subject, _, _, err := reg.Render("verification", "en", map[string]any{"URL": "x"})
	require.NoError(t, err)
	assert.Equal(t, "Verify your ABI account", subject)
}

// --- Email template compile ---

func TestEmailTemplate_Compile(t *testing.T) {
	tmpl := &emailTemplate{
		Subject: "Test {{.Name}}",
		Text:    "Hello {{.Name}}",
		HTML:    `<p>Hello {{.Name}}</p>`,
		Locale:  "en",
	}
	err := tmpl.compile("test.en")
	require.NoError(t, err)

	subject, text, html, err := tmpl.render(map[string]any{"Name": "World"})
	require.NoError(t, err)
	assert.Equal(t, "Test World", subject)
	assert.Equal(t, "Hello World", text)
	assert.Contains(t, html, "Hello World")
}

func TestEmailTemplate_Compile_InvalidHTML(t *testing.T) {
	tmpl := &emailTemplate{
		Subject: "Test",
		Text:    "Test",
		HTML:    `<p>{{.InvalidSyntax</p>`, // intentional: unclosed template action
		Locale:  "en",
	}
	err := tmpl.compile("bad.en")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parse html template")
}

func TestEmailTemplate_Render_MissingData(t *testing.T) {
	tmpl := &emailTemplate{
		Subject: "Hi {{.Name}}",
		Text:    "Hi {{.Name}}",
		HTML:    `<p>Hi {{.Name}}</p>`,
		Locale:  "en",
	}
	require.NoError(t, tmpl.compile("test.en"))

	subject, text, html, err := tmpl.render(nil)
	require.NoError(t, err)
	assert.Equal(t, "Hi ", subject)
	assert.Equal(t, "Hi ", text)
	assert.Contains(t, html, "Hi ")
}

// --- All template types Render ---

func TestRender_AllTemplateTypes(t *testing.T) {
	reg, err := newEmailTemplateRegistry("en")
	require.NoError(t, err)

	templates := []string{
		"verification",
		"password_reset",
		"email_change",
		"account_unlock",
		"new_device",
		"deletion_scheduled",
		"duplicate_registration",
		"magic_link",
		"invitation",
	}

	dataMap := map[string]any{
		"URL": "http://example.com/tok",
	}
	// Some templates need extra fields
	extraData := map[string]any{
		"NewEmail":    true,
		"OccurredAt":  "2026-01-01T00:00:00Z",
		"IPAddress":   "1.2.3.4",
		"UserAgent":   "Chrome",
		"SecurityURL": "http://example.com/security",
		"PurgeAt":     "2026-01-01T00:00:00Z",
		"AccountURL":  "http://example.com/account",
		"SignInURL":   "http://example.com/signin",
		"ForgotURL":   "http://example.com/forgot",
		"RoleName":    "admin",
		"ExpiresAt":   "2026-01-01T00:00:00Z",
		"ModelName":   "sales",
		"DriftsText":  "drift info",
		"Drifts":      []map[string]any{},
		"ModelURL":    "http://example.com/model",
	}

	for _, name := range templates {
		t.Run(name, func(t *testing.T) {
			d := make(map[string]any)
			for k, v := range dataMap {
				d[k] = v
			}
			for k, v := range extraData {
				d[k] = v
			}
			subject, text, html, err := reg.Render(name, "en", d)
			require.NoError(t, err, "template %q should render", name)
			assert.NotEmpty(t, subject, "template %q should have subject", name)
			assert.NotEmpty(t, text, "template %q should have text body", name)
			assert.NotEmpty(t, html, "template %q should have html body", name)
		})
	}
}
