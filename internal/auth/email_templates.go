package auth

import (
	"bytes"
	"fmt"
	"html/template"
	"strings"
)

// emailTemplate holds one locale's copy of the strings needed to render a
// transactional email: a subject line, a plain-text body, and an HTML body.
//
// Subject and text bodies are pre-rendered with a tiny placeholder substituter
// (no template engine) because the inputs are limited to a fixed set of
// internally-generated URLs and operator-controlled metadata. The HTML body
// uses html/template so its output is auto-escaped against XSS.
type emailTemplate struct {
	Subject  string
	Text     string
	HTML     string
	htmlTmpl *template.Template
}

func (t *emailTemplate) compile(name string) error {
	hx, err := template.New(name + ".html").Parse(t.HTML)
	if err != nil {
		return fmt.Errorf("parse html template %s: %w", name, err)
	}
	t.htmlTmpl = hx
	return nil
}

func (t *emailTemplate) render(data map[string]any) (subject, textBody, htmlBody string, err error) {
	subject = renderPlain(t.Subject, data)
	textBody = renderPlain(t.Text, data)
	var htm bytes.Buffer
	if err = t.htmlTmpl.Execute(&htm, data); err != nil {
		return "", "", "", fmt.Errorf("render html body: %w", err)
	}
	return subject, textBody, htm.String(), nil
}

// renderPlain performs literal placeholder substitution for plain-text email
// bodies and subjects. It supports two constructs and intentionally nothing
// else, so injected user input cannot pivot into template-engine features:
//
//	{{.Field}}                   — replaced with fmt.Sprint(data["Field"])
//	{{if .Field}}A{{else}}B{{end}} — single-level conditional, no nesting
//
// Missing fields render as empty string. Conditionals consider the value
// truthy when it is a non-zero/non-empty value of common types.
func renderPlain(tmpl string, data map[string]any) string {
	tmpl = renderConditionals(tmpl, data)
	var out strings.Builder
	out.Grow(len(tmpl))
	for {
		open := strings.Index(tmpl, "{{")
		if open < 0 {
			out.WriteString(tmpl)
			return out.String()
		}
		close := strings.Index(tmpl[open:], "}}")
		if close < 0 {
			out.WriteString(tmpl)
			return out.String()
		}
		close += open
		out.WriteString(tmpl[:open])
		expr := strings.TrimSpace(tmpl[open+2 : close])
		if strings.HasPrefix(expr, ".") {
			key := expr[1:]
			if v, ok := data[key]; ok {
				fmt.Fprint(&out, v)
			}
		}
		tmpl = tmpl[close+2:]
	}
}

func renderConditionals(tmpl string, data map[string]any) string {
	for {
		ifIdx := strings.Index(tmpl, "{{if .")
		if ifIdx < 0 {
			return tmpl
		}
		ifEnd := strings.Index(tmpl[ifIdx:], "}}")
		if ifEnd < 0 {
			return tmpl
		}
		ifEnd += ifIdx
		expr := strings.TrimSpace(tmpl[ifIdx+2 : ifEnd])
		key := strings.TrimPrefix(expr, "if .")
		endIdx := strings.Index(tmpl[ifEnd:], "{{end}}")
		if endIdx < 0 {
			return tmpl
		}
		endIdx += ifEnd
		body := tmpl[ifEnd+2 : endIdx]
		var truthy, falsy string
		if elseIdx := strings.Index(body, "{{else}}"); elseIdx >= 0 {
			truthy = body[:elseIdx]
			falsy = body[elseIdx+len("{{else}}"):]
		} else {
			truthy = body
		}
		var chosen string
		if isTruthy(data[key]) {
			chosen = truthy
		} else {
			chosen = falsy
		}
		tmpl = tmpl[:ifIdx] + chosen + tmpl[endIdx+len("{{end}}"):]
	}
}

func isTruthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case string:
		return x != ""
	case int:
		return x != 0
	case int64:
		return x != 0
	case float64:
		return x != 0
	default:
		return true
	}
}

// emailTemplateRegistry resolves a (template name, locale) pair to a compiled
// emailTemplate. It always falls back to the registry default locale and
// then to English when the requested locale is not registered.
type emailTemplateRegistry struct {
	defaultLocale string
	templates     map[string]map[string]*emailTemplate // template name → locale → template
}

func newEmailTemplateRegistry(defaultLocale string) (*emailTemplateRegistry, error) {
	if defaultLocale == "" {
		defaultLocale = "en"
	}
	r := &emailTemplateRegistry{
		defaultLocale: defaultLocale,
		templates:     map[string]map[string]*emailTemplate{},
	}
	for name, byLocale := range builtinEmailTemplates {
		r.templates[name] = map[string]*emailTemplate{}
		for locale, tmpl := range byLocale {
			t := tmpl
			if err := t.compile(name + "." + locale); err != nil {
				return nil, err
			}
			r.templates[name][locale] = t
		}
	}
	return r, nil
}

func (r *emailTemplateRegistry) Render(name, locale string, data map[string]any) (string, string, string, error) {
	byLocale, ok := r.templates[name]
	if !ok {
		return "", "", "", fmt.Errorf("unknown email template %q", name)
	}
	t, ok := byLocale[locale]
	if !ok {
		t = byLocale[r.defaultLocale]
	}
	if t == nil {
		t = byLocale["en"]
	}
	if t == nil {
		return "", "", "", fmt.Errorf("no template variant for %q", name)
	}
	return t.render(data)
}

// builtinEmailTemplates is the in-process registry of localized transactional
// emails. Keys are template names; nested keys are BCP-47 locale codes.
// Plain-text bodies stay close to the original copy used in pre-template
// SMTPEmailSender so existing tests continue to match key phrases.
var builtinEmailTemplates = map[string]map[string]*emailTemplate{
	"verification": {
		"en": {
			Subject: "Verify your Biqly account",
			Text:    "Please verify your account by clicking the following link:\n{{.URL}}\n\nThis link will expire in 24 hours.\n\nIf you did not create a Biqly account, you can ignore this email.\n",
			HTML:    `<p>Please verify your account by clicking the following link:</p><p><a href="{{.URL}}">{{.URL}}</a></p><p>This link will expire in 24 hours.</p><p>If you did not create a Biqly account, you can ignore this email.</p>`,
		},
		"tr": {
			Subject: "Biqly hesabınızı doğrulayın",
			Text:    "Hesabınızı doğrulamak için aşağıdaki bağlantıya tıklayın:\n{{.URL}}\n\nBu bağlantı 24 saat içinde sona erecektir.\n\nBu hesabı siz oluşturmadıysanız bu e-postayı görmezden gelebilirsiniz.\n",
			HTML:    `<p>Hesabınızı doğrulamak için aşağıdaki bağlantıya tıklayın:</p><p><a href="{{.URL}}">{{.URL}}</a></p><p>Bu bağlantı 24 saat içinde sona erecektir.</p><p>Bu hesabı siz oluşturmadıysanız bu e-postayı görmezden gelebilirsiniz.</p>`,
		},
	},
	"password_reset": {
		"en": {
			Subject: "Reset your Biqly password",
			Text:    "You requested to reset your password. Click the link below to set a new password:\n{{.URL}}\n\nThis link will expire in 1 hour.\n",
			HTML:    `<p>You requested to reset your password. Click the link below to set a new password:</p><p><a href="{{.URL}}">{{.URL}}</a></p><p>This link will expire in 1 hour.</p>`,
		},
		"tr": {
			Subject: "Biqly parolanızı sıfırlayın",
			Text:    "Parolanızı sıfırlamak istediniz. Yeni bir parola belirlemek için aşağıdaki bağlantıya tıklayın:\n{{.URL}}\n\nBu bağlantı 1 saat içinde sona erecektir.\n",
			HTML:    `<p>Parolanızı sıfırlamak istediniz. Yeni bir parola belirlemek için aşağıdaki bağlantıya tıklayın:</p><p><a href="{{.URL}}">{{.URL}}</a></p><p>Bu bağlantı 1 saat içinde sona erecektir.</p>`,
		},
	},
	"email_change": {
		"en": {
			Subject: "Confirm your Biqly email change",
			Text:    "{{if .NewEmail}}Confirm this as your new Biqly email address:{{else}}Confirm this email change by clicking the following link:{{end}}\n{{.URL}}\n\nThis link will expire in 48 hours.\n",
			HTML:    `<p>{{if .NewEmail}}Confirm this as your new Biqly email address:{{else}}Confirm this email change by clicking the following link:{{end}}</p><p><a href="{{.URL}}">{{.URL}}</a></p><p>This link will expire in 48 hours.</p>`,
		},
		"tr": {
			Subject: "Biqly e-posta değişikliğinizi onaylayın",
			Text:    "{{if .NewEmail}}Bu adresin yeni Biqly e-posta adresiniz olduğunu onaylayın:{{else}}E-posta değişikliğini onaylamak için aşağıdaki bağlantıya tıklayın:{{end}}\n{{.URL}}\n\nBu bağlantı 48 saat içinde sona erecektir.\n",
			HTML:    `<p>{{if .NewEmail}}Bu adresin yeni Biqly e-posta adresiniz olduğunu onaylayın:{{else}}E-posta değişikliğini onaylamak için aşağıdaki bağlantıya tıklayın:{{end}}</p><p><a href="{{.URL}}">{{.URL}}</a></p><p>Bu bağlantı 48 saat içinde sona erecektir.</p>`,
		},
	},
	"account_unlock": {
		"en": {
			Subject: "Your Biqly account is locked",
			Text:    "Your account was locked due to multiple failed sign-in attempts.\n\nIf this was you, click below to unlock and reset your password:\n{{.URL}}\n\nThis link expires in 1 hour. If you didn't try to sign in, please change your password immediately.\n",
			HTML:    `<p>Your account was locked due to multiple failed sign-in attempts.</p><p>If this was you, click below to unlock and reset your password:</p><p><a href="{{.URL}}">{{.URL}}</a></p><p>This link expires in 1 hour. If you didn't try to sign in, please change your password immediately.</p>`,
		},
		"tr": {
			Subject: "Biqly hesabınız kilitlendi",
			Text:    "Birden fazla başarısız giriş denemesi nedeniyle hesabınız kilitlendi.\n\nBu sizdiyseniz aşağıya tıklayarak hesabı açın ve parolanızı sıfırlayın:\n{{.URL}}\n\nBu bağlantı 1 saat içinde sona erecektir. Bu sizden değilse parolanızı hemen değiştirin.\n",
			HTML:    `<p>Birden fazla başarısız giriş denemesi nedeniyle hesabınız kilitlendi.</p><p>Bu sizdiyseniz aşağıya tıklayarak hesabı açın ve parolanızı sıfırlayın:</p><p><a href="{{.URL}}">{{.URL}}</a></p><p>Bu bağlantı 1 saat içinde sona erecektir. Bu sizden değilse parolanızı hemen değiştirin.</p>`,
		},
	},
	"new_device": {
		"en": {
			Subject: "New sign-in to your Biqly account",
			Text:    "We detected a sign-in to your Biqly account from a new device.\n\nTime:       {{.OccurredAt}}\nIP address: {{.IPAddress}}\nDevice:     {{.UserAgent}}\n\nIf this wasn't you, change your password and revoke active sessions from the security page:\n{{.SecurityURL}}\n",
			HTML:    `<p>We detected a sign-in to your Biqly account from a new device.</p><ul><li><strong>Time:</strong> {{.OccurredAt}}</li><li><strong>IP address:</strong> {{.IPAddress}}</li><li><strong>Device:</strong> {{.UserAgent}}</li></ul><p>If this wasn't you, change your password and revoke active sessions from the <a href="{{.SecurityURL}}">security page</a>.</p>`,
		},
		"tr": {
			Subject: "Biqly hesabınıza yeni cihazdan giriş yapıldı",
			Text:    "Biqly hesabınıza yeni bir cihazdan giriş yapıldığını tespit ettik.\n\nZaman:      {{.OccurredAt}}\nIP adresi:  {{.IPAddress}}\nCihaz:      {{.UserAgent}}\n\nBu giriş size ait değilse parolanızı değiştirin ve güvenlik sayfasından aktif oturumları sonlandırın:\n{{.SecurityURL}}\n",
			HTML:    `<p>Biqly hesabınıza yeni bir cihazdan giriş yapıldığını tespit ettik.</p><ul><li><strong>Zaman:</strong> {{.OccurredAt}}</li><li><strong>IP adresi:</strong> {{.IPAddress}}</li><li><strong>Cihaz:</strong> {{.UserAgent}}</li></ul><p>Bu giriş size ait değilse parolanızı değiştirin ve <a href="{{.SecurityURL}}">güvenlik sayfasından</a> aktif oturumları sonlandırın.</p>`,
		},
	},
	"deletion_scheduled": {
		"en": {
			Subject: "Your Biqly account is scheduled for deletion",
			Text:    "Your Biqly account has been scheduled for deletion. All personal data will be permanently removed on {{.PurgeAt}} (UTC).\n\nTo cancel deletion before that date, sign in and restore your account from {{.AccountURL}}.\n",
			HTML:    `<p>Your Biqly account has been scheduled for deletion. All personal data will be permanently removed on <strong>{{.PurgeAt}}</strong> (UTC).</p><p>To cancel deletion before that date, sign in and restore your account from <a href="{{.AccountURL}}">your account page</a>.</p>`,
		},
		"tr": {
			Subject: "Biqly hesabınız silinmek üzere planlandı",
			Text:    "Biqly hesabınız silinmek üzere planlandı. Tüm kişisel verileriniz {{.PurgeAt}} (UTC) tarihinde kalıcı olarak silinecektir.\n\nBu tarihten önce silmeyi iptal etmek için giriş yapın ve hesabınızı şu sayfadan geri yükleyin: {{.AccountURL}}\n",
			HTML:    `<p>Biqly hesabınız silinmek üzere planlandı. Tüm kişisel verileriniz <strong>{{.PurgeAt}}</strong> (UTC) tarihinde kalıcı olarak silinecektir.</p><p>Bu tarihten önce silmeyi iptal etmek için giriş yapın ve <a href="{{.AccountURL}}">hesap sayfanızdan</a> geri yükleyin.</p>`,
		},
	},
	"duplicate_registration": {
		"en": {
			Subject: "Sign-up attempt on existing Biqly account",
			Text:    "Someone tried to register a new Biqly account using this email address, but an account already exists.\n\nIf this was you, you can sign in at {{.SignInURL}} or reset your password at {{.ForgotURL}}.\n\nIf you did not attempt this, you can safely ignore this email.\n",
			HTML:    `<p>Someone tried to register a new Biqly account using this email address, but an account already exists.</p><p>If this was you, you can <a href="{{.SignInURL}}">sign in</a> or <a href="{{.ForgotURL}}">reset your password</a>.</p><p>If you did not attempt this, you can safely ignore this email.</p>`,
		},
		"tr": {
			Subject: "Mevcut Biqly hesabınıza kayıt denemesi",
			Text:    "Birisi bu e-posta adresiyle yeni bir Biqly hesabı oluşturmaya çalıştı, ancak zaten bir hesap mevcut.\n\nBu sizdiyseniz {{.SignInURL}} adresinden giriş yapabilir veya {{.ForgotURL}} adresinden parolanızı sıfırlayabilirsiniz.\n\nBu deneme size ait değilse bu e-postayı görmezden gelebilirsiniz.\n",
			HTML:    `<p>Birisi bu e-posta adresiyle yeni bir Biqly hesabı oluşturmaya çalıştı, ancak zaten bir hesap mevcut.</p><p>Bu sizdiyseniz <a href="{{.SignInURL}}">giriş yapabilir</a> veya <a href="{{.ForgotURL}}">parolanızı sıfırlayabilirsiniz</a>.</p><p>Bu deneme size ait değilse bu e-postayı görmezden gelebilirsiniz.</p>`,
		},
	},
	"magic_link": {
		"en": {
			Subject: "Your Biqly sign-in link",
			Text:    "Click the link below to sign in to Biqly:\n{{.URL}}\n\nThis link can be used once and expires in 10 minutes.\n\nIf you did not request this link, you can safely ignore this email.\n",
			HTML:    `<p>Click the link below to sign in to Biqly:</p><p><a href="{{.URL}}">{{.URL}}</a></p><p>This link can be used once and expires in 10 minutes.</p><p>If you did not request this link, you can safely ignore this email.</p>`,
		},
		"tr": {
			Subject: "Biqly giriş bağlantınız",
			Text:    "Biqly'a giriş yapmak için aşağıdaki bağlantıya tıklayın:\n{{.URL}}\n\nBu bağlantı bir kez kullanılabilir ve 10 dakika içinde sona erer.\n\nBu bağlantıyı siz istemediyseniz bu e-postayı görmezden gelebilirsiniz.\n",
			HTML:    `<p>Biqly'a giriş yapmak için aşağıdaki bağlantıya tıklayın:</p><p><a href="{{.URL}}">{{.URL}}</a></p><p>Bu bağlantı bir kez kullanılabilir ve 10 dakika içinde sona erer.</p><p>Bu bağlantıyı siz istemediyseniz bu e-postayı görmezden gelebilirsiniz.</p>`,
		},
	},
}
