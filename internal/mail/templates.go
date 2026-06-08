package mail

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
	Locale   string
	htmlTmpl *template.Template
}

func (t *emailTemplate) compile(name string) error {
	wrapped := wrapHTML(t.HTML, t.Subject, t.Locale)
	hx, err := template.New(name + ".html").Parse(wrapped)
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
	parts := make([]string, 0, 8)
	for {
		open := strings.Index(tmpl, "{{")
		if open < 0 {
			parts = append(parts, tmpl)
			return strings.Join(parts, "")
		}
		closeIdx := strings.Index(tmpl[open:], "}}")
		if closeIdx < 0 {
			parts = append(parts, tmpl)
			return strings.Join(parts, "")
		}
		closeIdx += open
		parts = append(parts, tmpl[:open])
		expr := strings.TrimSpace(tmpl[open+2 : closeIdx])
		if strings.HasPrefix(expr, ".") {
			key := expr[1:]
			if v, ok := data[key]; ok {
				parts = append(parts, fmt.Sprint(v))
			}
		}
		tmpl = tmpl[closeIdx+2:]
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
			t.Locale = locale
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
			Subject: "Verify your ABI account",
			Text:    "Please verify your account by clicking the following link:\n{{.URL}}\n\nThis link will expire in 24 hours.\n\nIf you did not create an ABI account, you can ignore this email.\n",
			HTML: `<p>Please verify your account by clicking the button below:</p>
<div class="button-container">
  <a href="{{.URL}}" class="button" style="color: #ffffff;">Verify Account</a>
</div>
<p style="font-size: 13px; color: #64748b; margin-top: 24px;">This link will expire in 24 hours. If the button doesn't work, copy and paste this URL into your browser:</p>
<p style="font-size: 13px; color: #64748b; word-break: break-all;"><a href="{{.URL}}" style="color: #4f46e5;">{{.URL}}</a></p>
<p style="font-size: 13px; color: #64748b; margin-top: 16px;">If you did not create an ABI account, you can safely ignore this email.</p>`,
		},
		"tr": {
			Subject: "ABI hesabınızı doğrulayın",
			Text:    "Hesabınızı doğrulamak için aşağıdaki bağlantıya tıklayın:\n{{.URL}}\n\nBu bağlantı 24 saat içinde sona erecektir.\n\nBu hesabı siz oluşturmadıysanız bu e-postayı görmezden gelebilirsiniz.\n",
			HTML: `<p>Hesabınızı doğrulamak için aşağıdaki butona tıklayın:</p>
<div class="button-container">
  <a href="{{.URL}}" class="button" style="color: #ffffff;">Hesabımı Doğrula</a>
</div>
<p style="font-size: 13px; color: #64748b; margin-top: 24px;">Bu bağlantı 24 saat içinde sona erecektir. Eğer buton çalışmazsa aşağıdaki bağlantıyı kopyalayıp tarayıcınıza yapıştırın:</p>
<p style="font-size: 13px; color: #64748b; word-break: break-all;"><a href="{{.URL}}" style="color: #4f46e5;">{{.URL}}</a></p>
<p style="font-size: 13px; color: #64748b; margin-top: 16px;">Bu hesabı siz oluşturmadıysanız bu e-postayı görmezden gelebilirsiniz.</p>`,
		},
	},
	"password_reset": {
		"en": {
			Subject: "Reset your ABI password",
			Text:    "You requested to reset your password. Click the link below to set a new password:\n{{.URL}}\n\nThis link will expire in 1 hour.\n",
			HTML: `<p>You requested to reset your password. Click the button below to set a new password:</p>
<div class="button-container">
  <a href="{{.URL}}" class="button" style="color: #ffffff;">Reset Password</a>
</div>
<p style="font-size: 13px; color: #64748b; margin-top: 24px;">This link will expire in 1 hour. If the button doesn't work, copy and paste this URL into your browser:</p>
<p style="font-size: 13px; color: #64748b; word-break: break-all;"><a href="{{.URL}}" style="color: #4f46e5;">{{.URL}}</a></p>
<p style="font-size: 13px; color: #64748b; margin-top: 16px;">If you did not request this change, you can safely ignore this email.</p>`,
		},
		"tr": {
			Subject: "ABI parolanızı sıfırlayın",
			Text:    "Parolanızı sıfırlamak istediniz. Yeni bir parola belirlemek için aşağıdaki bağlantıya tıklayın:\n{{.URL}}\n\nBu bağlantı 1 saat içinde sona erecektir.\n",
			HTML: `<p>Parolanızı sıfırlamak istediniz. Yeni bir parola belirlemek için aşağıdaki butona tıklayın:</p>
<div class="button-container">
  <a href="{{.URL}}" class="button" style="color: #ffffff;">Parolayı Sıfırla</a>
</div>
<p style="font-size: 13px; color: #64748b; margin-top: 24px;">Bu bağlantı 1 saat içinde sona erecektir. Eğer buton çalışmazsa aşağıdaki bağlantıyı kopyalayıp tarayıcınıza yapıştırın:</p>
<p style="font-size: 13px; color: #64748b; word-break: break-all;"><a href="{{.URL}}" style="color: #4f46e5;">{{.URL}}</a></p>
<p style="font-size: 13px; color: #64748b; margin-top: 16px;">Eğer bu isteği siz yapmadıysanız bu e-postayı görmezden gelebilirsiniz.</p>`,
		},
	},
	"email_change": {
		"en": {
			Subject: "Confirm your ABI email change",
			Text:    "{{if .NewEmail}}Confirm this as your new ABI email address:{{else}}Confirm this email change by clicking the following link:{{end}}\n{{.URL}}\n\nThis link will expire in 48 hours.\n",
			HTML: `<p>{{if .NewEmail}}Confirm this as your new ABI email address by clicking the button below:{{else}}Confirm this email change by clicking the button below:{{end}}</p>
<div class="button-container">
  <a href="{{.URL}}" class="button" style="color: #ffffff;">Confirm Email Change</a>
</div>
<p style="font-size: 13px; color: #64748b; margin-top: 24px;">This link will expire in 48 hours. If the button doesn't work, copy and paste this URL into your browser:</p>
<p style="font-size: 13px; color: #64748b; word-break: break-all;"><a href="{{.URL}}" style="color: #4f46e5;">{{.URL}}</a></p>`,
		},
		"tr": {
			Subject: "ABI e-posta değişikliğinizi onaylayın",
			Text:    "{{if .NewEmail}}Bu adresin yeni ABI e-posta adresiniz olduğunu onaylayın:{{else}}E-posta değişikliğini onaylamak için aşağıdaki bağlantıya tıklayın:{{end}}\n{{.URL}}\n\nBu bağlantı 48 saat içinde sona erecektir.\n",
			HTML: `<p>{{if .NewEmail}}Aşağıdaki butona tıklayarak bunun yeni ABI e-posta adresiniz olduğunu onaylayın:{{else}}E-posta değişikliğini onaylamak için aşağıdaki butona tıklayın:{{end}}</p>
<div class="button-container">
  <a href="{{.URL}}" class="button" style="color: #ffffff;">E-postayı Onayla</a>
</div>
<p style="font-size: 13px; color: #64748b; margin-top: 24px;">Bu bağlantı 48 saat içinde sona erecektir. Eğer buton çalışmazsa aşağıdaki bağlantıyı kopyalayıp tarayıcınıza yapıştırın:</p>
<p style="font-size: 13px; color: #64748b; word-break: break-all;"><a href="{{.URL}}" style="color: #4f46e5;">{{.URL}}</a></p>`,
		},
	},
	"account_unlock": {
		"en": {
			Subject: "Your ABI account is locked",
			Text:    "Your account was locked due to multiple failed sign-in attempts.\n\nIf this was you, click below to unlock and reset your password:\n{{.URL}}\n\nThis link expires in 1 hour. If you didn't try to sign in, please change your password immediately.\n",
			HTML: `<p>Your account was locked due to multiple failed sign-in attempts.</p>
<p>If this was you, click the button below to unlock your account and reset your password:</p>
<div class="button-container">
  <a href="{{.URL}}" class="button" style="color: #ffffff;">Unlock Account</a>
</div>
<p style="font-size: 13px; color: #64748b; margin-top: 24px;">This link will expire in 1 hour. If the button doesn't work, copy and paste this URL into your browser:</p>
<p style="font-size: 13px; color: #64748b; word-break: break-all;"><a href="{{.URL}}" style="color: #4f46e5;">{{.URL}}</a></p>
<p style="color: #ef4444; font-weight: 500; font-size: 14px; margin-top: 20px;">If you did not try to sign in, please change your password immediately to protect your account.</p>`,
		},
		"tr": {
			Subject: "ABI hesabınız kilitlendi",
			Text:    "Birden fazla başarısız giriş denemesi nedeniyle hesabınız kilitlendi.\n\nBu sizdiyseniz aşağıya tıklayarak hesabı açın ve parolanızı sıfırlayın:\n{{.URL}}\n\nBu bağlantı 1 saat içinde sona erecektir. Bu sizden değilse parolanızı hemen değiştirin.\n",
			HTML: `<p>Birden fazla başarısız giriş denemesi nedeniyle hesabınız kilitlendi.</p>
<p>Bu sizdiyseniz, aşağıdaki butona tıklayarak hesabı açın ve parolanızı sıfırlayın:</p>
<div class="button-container">
  <a href="{{.URL}}" class="button" style="color: #ffffff;">Hesap Kilidini Aç</a>
</div>
<p style="font-size: 13px; color: #64748b; margin-top: 24px;">Bu bağlantı 1 saat içinde sona erecektir. Eğer buton çalışmazsa aşağıdaki bağlantıyı kopyalayıp tarayıcınıza yapıştırın:</p>
<p style="font-size: 13px; color: #64748b; word-break: break-all;"><a href="{{.URL}}" style="color: #4f46e5;">{{.URL}}</a></p>
<p style="color: #ef4444; font-weight: 500; font-size: 14px; margin-top: 20px;">Bu deneme size ait değilse, hesabınızı korumak için lütfen parolanızı hemen değiştirin.</p>`,
		},
	},
	"new_device": {
		"en": {
			Subject: "New sign-in to your ABI account",
			Text:    "We detected a sign-in to your ABI account from a new device.\n\nTime:       {{.OccurredAt}}\nIP address: {{.IPAddress}}\nDevice:     {{.UserAgent}}\n\nIf this wasn't you, change your password and revoke active sessions from the security page:\n{{.SecurityURL}}\n",
			HTML: `<p>We detected a sign-in to your ABI account from a new device.</p>
<div class="details-box">
  <ul class="details-list">
    <li><strong>Time:</strong> {{.OccurredAt}}</li>
    <li><strong>IP Address:</strong> {{.IPAddress}}</li>
    <li><strong>Device:</strong> {{.UserAgent}}</li>
  </ul>
</div>
<p>If this was you, you can safely ignore this email.</p>
<p style="color: #ef4444; font-weight: 500; margin-top: 20px;">If this wasn't you, please change your password and revoke active sessions immediately:</p>
<div class="button-container">
  <a href="{{.SecurityURL}}" class="button" style="background-color: #ef4444; color: #ffffff;">Go to Security Page</a>
</div>`,
		},
		"tr": {
			Subject: "ABI hesabınıza yeni cihazdan giriş yapıldı",
			Text:    "ABI hesabınıza yeni bir cihazdan giriş yapıldığını tespit ettik.\n\nZaman:      {{.OccurredAt}}\nIP adresi:  {{.IPAddress}}\nCihaz:      {{.UserAgent}}\n\nBu giriş size ait değilse parolanızı değiştirin ve güvenlik sayfasından aktif oturumları sonlandırın:\n{{.SecurityURL}}\n",
			HTML: `<p>ABI hesabınıza yeni bir cihazdan giriş yapıldığını tespit ettik.</p>
<div class="details-box">
  <ul class="details-list">
    <li><strong>Zaman:</strong> {{.OccurredAt}}</li>
    <li><strong>IP Adresi:</strong> {{.IPAddress}}</li>
    <li><strong>Cihaz:</strong> {{.UserAgent}}</li>
  </ul>
</div>
<p>Bu giriş size aitse, bu e-postayı görmezden gelebilirsiniz.</p>
<p style="color: #ef4444; font-weight: 500; margin-top: 20px;">Bu giriş size ait değilse, lütfen parolanızı değiştirin ve aktif oturumları hemen sonlandırın:</p>
<div class="button-container">
  <a href="{{.SecurityURL}}" class="button" style="background-color: #ef4444; color: #ffffff;">Güvenlik Sayfasına Git</a>
</div>`,
		},
	},
	"deletion_scheduled": {
		"en": {
			Subject: "Your ABI account is scheduled for deletion",
			Text:    "Your ABI account has been scheduled for deletion. All personal data will be permanently removed on {{.PurgeAt}} (UTC).\n\nTo cancel deletion before that date, sign in and restore your account from {{.AccountURL}}.\n",
			HTML: `<p>Your ABI account has been scheduled for deletion. All personal data and settings will be permanently removed on <strong>{{.PurgeAt}}</strong> (UTC).</p>
<p>To cancel the deletion before that date, please sign in and restore your account:</p>
<div class="button-container">
  <a href="{{.AccountURL}}" class="button" style="color: #ffffff;">Restore Account</a>
</div>`,
		},
		"tr": {
			Subject: "ABI hesabınız silinmek üzere planlandı",
			Text:    "ABI hesabınız silinmek üzere planlandı. Tüm kişisel verileriniz {{.PurgeAt}} (UTC) tarihinde kalıcı olarak silinecektir.\n\nBu tarihten önce silmeyi iptal etmek için giriş yapın ve hesabınızı şu sayfadan geri yükleyin: {{.AccountURL}}\n",
			HTML: `<p>ABI hesabınız silinmek üzere planlandı. Tüm kişisel verileriniz ve ayarlarınız <strong>{{.PurgeAt}}</strong> (UTC) tarihinde kalıcı olarak silinecektir.</p>
<p>Bu tarihten önce silme işlemini iptal etmek için lütfen giriş yapın ve hesabınızı geri yükleyin:</p>
<div class="button-container">
  <a href="{{.AccountURL}}" class="button" style="color: #ffffff;">Hesabı Geri Yükle</a>
</div>`,
		},
	},
	"duplicate_registration": {
		"en": {
			Subject: "Sign-up attempt on existing ABI account",
			Text:    "Someone tried to register a new ABI account using this email address, but an account already exists.\n\nIf this was you, you can sign in at {{.SignInURL}} or reset your password at {{.ForgotURL}}.\n\nIf you did not attempt this, you can safely ignore this email.\n",
			HTML: `<p>Someone tried to register a new ABI account using this email address, but an account with this address already exists.</p>
<p>If this was you, you can sign in to your existing account or reset your password if you forgot it:</p>
<div class="button-container" style="margin: 20px 0;">
  <a href="{{.SignInURL}}" class="button" style="color: #ffffff; margin-right: 12px;">Sign In</a>
  <a href="{{.ForgotURL}}" class="button" style="background-color: #f1f5f9; color: #475569 !important; border: 1px solid #cbd5e1; box-shadow: none;">Reset Password</a>
</div>
<p style="font-size: 13px; color: #64748b; margin-top: 16px;">If you did not attempt this, you can safely ignore this email.</p>`,
		},
		"tr": {
			Subject: "Mevcut ABI hesabınıza kayıt denemesi",
			Text:    "Birisi bu e-posta adresiyle yeni bir ABI hesabı oluşturmaya çalıştı, ancak zaten bir hesap mevcut.\n\nBu sizdiyseniz {{.SignInURL}} adresinden giriş yapabilir veya {{.ForgotURL}} adresinden parolanızı sıfırlayabilirsiniz.\n\nBu deneme size ait değilse bu e-postayı görmezden gelebilirsiniz.\n",
			HTML: `<p>Birisi bu e-posta adresiyle yeni bir ABI hesabı oluşturmaya çalıştı, ancak bu adrese kayıtlı mevcut bir hesap zaten var.</p>
<p>Bu sizdiyseniz, mevcut hesabınıza giriş yapabilir veya şifrenizi unuttuysanız sıfırlayabilirsiniz:</p>
<div class="button-container" style="margin: 20px 0;">
  <a href="{{.SignInURL}}" class="button" style="color: #ffffff; margin-right: 12px;">Giriş Yap</a>
  <a href="{{.ForgotURL}}" class="button" style="background-color: #f1f5f9; color: #475569 !important; border: 1px solid #cbd5e1; box-shadow: none;">Parolayı Sıfırla</a>
</div>
<p style="font-size: 13px; color: #64748b; margin-top: 16px;">Bu deneme size ait değilse, bu e-postayı güvenle yok sayabilirsiniz.</p>`,
		},
	},
	"magic_link": {
		"en": {
			Subject: "Your ABI sign-in link",
			Text:    "Click the link below to sign in to ABI:\n{{.URL}}\n\nThis link can be used once and expires in 10 minutes.\n\nIf you did not request this link, you can safely ignore this email.\n",
			HTML: `<p>Click the button below to sign in to your ABI account:</p>
<div class="button-container">
  <a href="{{.URL}}" class="button" style="color: #ffffff;">Sign In to ABI</a>
</div>
<p style="font-size: 13px; color: #64748b; margin-top: 24px;">This link can be used only once and will expire in 10 minutes. If the button doesn't work, copy and paste this URL into your browser:</p>
<p style="font-size: 13px; color: #64748b; word-break: break-all;"><a href="{{.URL}}" style="color: #4f46e5;">{{.URL}}</a></p>
<p style="font-size: 13px; color: #64748b; margin-top: 16px;">If you did not request this link, you can safely ignore this email.</p>`,
		},
		"tr": {
			Subject: "ABI giriş bağlantınız",
			Text:    "ABI'a giriş yapmak için aşağıdaki bağlantıya tıklayın:\n{{.URL}}\n\nBu bağlantı bir kez kullanılabilir ve 10 dakika içinde sona erer.\n\nBu bağlantıyı siz istemediyseniz bu e-postayı görmezden gelebilirsiniz.\n",
			HTML: `<p>ABI'a giriş yapmak için aşağıdaki butona tıklayın:</p>
<div class="button-container">
  <a href="{{.URL}}" class="button" style="color: #ffffff;">ABI'a Giriş Yap</a>
</div>
<p style="font-size: 13px; color: #64748b; margin-top: 24px;">Bu bağlantı yalnızca bir kez kullanılabilir ve 10 dakika içinde sona erer. Eğer buton çalışmazsa aşağıdaki bağlantıyı kopyalayıp tarayıcınıza yapıştırın:</p>
<p style="font-size: 13px; color: #64748b; word-break: break-all;"><a href="{{.URL}}" style="color: #4f46e5;">{{.URL}}</a></p>
<p style="font-size: 13px; color: #64748b; margin-top: 16px;">Bu bağlantıyı siz istemediyseniz bu e-postayı güvenle yok sayabilirsiniz.</p>`,
		},
	},
	"invitation": {
		"en": {
			Subject: "You are invited to join ABI",
			Text:    "You have been invited to join ABI with the role of {{.RoleName}}.\nTo accept this invitation and set up your account, click the link below:\n{{.URL}}\n\nThis invitation will expire on {{.ExpiresAt}}.\n",
			HTML: `<p>You have been invited to join ABI with the role of <strong>{{.RoleName}}</strong>.</p>
<p>To accept this invitation and set up your account, click the button below:</p>
<div class="button-container">
  <a href="{{.URL}}" class="button" style="color: #ffffff;">Set Up Account</a>
</div>
<p style="font-size: 13px; color: #64748b; margin-top: 24px;">This link will expire on {{.ExpiresAt}}. If the button doesn't work, copy and paste this URL into your browser:</p>
<p style="font-size: 13px; color: #64748b; word-break: break-all;"><a href="{{.URL}}" style="color: #4f46e5;">{{.URL}}</a></p>`,
		},
		"tr": {
			Subject: "ABI'a davet edildiniz",
			Text:    "ABI'a {{.RoleName}} rolüyle katılmaya davet edildiniz.\nBu daveti kabul etmek ve hesabınızı kurmak için aşağıdaki bağlantıya tıklayın:\n{{.URL}}\n\nBu davet {{.ExpiresAt}} tarihine kadar geçerlidir.\n",
			HTML: `<p>ABI'a <strong>{{.RoleName}}</strong> rolüyle katılmaya davet edildiniz.</p>
<p>Bu daveti kabul etmek ve hesabınızı kurmak için aşağıdaki butona tıklayın:</p>
<div class="button-container">
  <a href="{{.URL}}" class="button" style="color: #ffffff;">Hesabımı Kur</a>
</div>
<p style="font-size: 13px; color: #64748b; margin-top: 24px;">Bu davet {{.ExpiresAt}} tarihine kadar geçerlidir. Eğer buton çalışmazsa aşağıdaki bağlantıyı kopyalayıp tarayıcınıza yapıştırın:</p>
<p style="font-size: 13px; color: #64748b; word-break: break-all;"><a href="{{.URL}}" style="color: #4f46e5;">{{.URL}}</a></p>`,
		},
	},
	"drift_alert": {
		"en": {
			Subject: "[Biqly] Schema drift detected in model: {{.ModelName}}",
			Text:    "Schema drift has been detected in model \"{{.ModelName}}\".\n\nDetails:\n{{.DriftsText}}\n\nYou can view and resolve these issues in the model editor:\n{{.ModelURL}}\n",
			HTML: `<p>Schema drift has been detected in model <strong>{{.ModelName}}</strong>.</p>
<p>Here are the details of the detected drift items:</p>
<table style="width: 100%; border-collapse: collapse; margin: 16px 0;">
  <thead>
    <tr style="background-color: #f8fafc; border-bottom: 2px solid #e2e8f0;">
      <th style="padding: 10px; text-align: left; font-size: 13px; font-weight: 600; color: #475569;">Severity</th>
      <th style="padding: 10px; text-align: left; font-size: 13px; font-weight: 600; color: #475569;">Type</th>
      <th style="padding: 10px; text-align: left; font-size: 13px; font-weight: 600; color: #475569;">Field / Path</th>
      <th style="padding: 10px; text-align: left; font-size: 13px; font-weight: 600; color: #475569;">Description</th>
    </tr>
  </thead>
  <tbody>
    {{range .Drifts}}
    <tr style="border-bottom: 1px solid #e2e8f0;">
      <td style="padding: 10px; font-size: 13px;">
        {{if eq .Severity "critical"}}
          <span style="background-color: #fef2f2; color: #991b1b; padding: 2px 6px; border-radius: 4px; font-weight: 600; font-size: 11px; text-transform: uppercase;">Critical</span>
        {{else if eq .Severity "warning"}}
          <span style="background-color: #fffbeb; color: #92400e; padding: 2px 6px; border-radius: 4px; font-weight: 600; font-size: 11px; text-transform: uppercase;">Warning</span>
        {{else}}
          <span style="background-color: #f0fdf4; color: #166534; padding: 2px 6px; border-radius: 4px; font-weight: 600; font-size: 11px; text-transform: uppercase;">Info</span>
        {{end}}
      </td>
      <td style="padding: 10px; font-size: 13px; color: #0f172a;">{{.Type}}</td>
      <td style="padding: 10px; font-size: 13px; font-family: monospace; color: #475569;">
        {{if .Field}}<strong>{{.Field}}</strong> ({{.ColumnRef}}){{else}}{{.ColumnRef}}{{end}}
      </td>
      <td style="padding: 10px; font-size: 13px; color: #334155;">{{.Description}}</td>
    </tr>
    {{end}}
  </tbody>
</table>
<div class="button-container" style="margin-top: 24px;">
  <a href="{{.ModelURL}}" class="button" style="color: #ffffff;">View Model in Editor</a>
</div>`,
		},
		"tr": {
			Subject: "[Biqly] {{.ModelName}} modelinde şema sapması tespit edildi",
			Text:    "\"{{.ModelName}}\" modelinde şema sapması tespit edildi.\n\nDetaylar:\n{{.DriftsText}}\n\nBu sorunları model düzenleyicisinde inceleyebilir ve çözebilirsiniz:\n{{.ModelURL}}\n",
			HTML: `<p><strong>{{.ModelName}}</strong> modelinde şema sapması tespit edildi.</p>
<p>Tespit edilen sapma detayları aşağıdadır:</p>
<table style="width: 100%; border-collapse: collapse; margin: 16px 0;">
  <thead>
    <tr style="background-color: #f8fafc; border-bottom: 2px solid #e2e8f0;">
      <th style="padding: 10px; text-align: left; font-size: 13px; font-weight: 600; color: #475569;">Önem Derecesi</th>
      <th style="padding: 10px; text-align: left; font-size: 13px; font-weight: 600; color: #475569;">Tür</th>
      <th style="padding: 10px; text-align: left; font-size: 13px; font-weight: 600; color: #475569;">Alan / Yol</th>
      <th style="padding: 10px; text-align: left; font-size: 13px; font-weight: 600; color: #475569;">Açıklama</th>
    </tr>
  </thead>
  <tbody>
    {{range .Drifts}}
    <tr style="border-bottom: 1px solid #e2e8f0;">
      <td style="padding: 10px; font-size: 13px;">
        {{if eq .Severity "critical"}}
          <span style="background-color: #fef2f2; color: #991b1b; padding: 2px 6px; border-radius: 4px; font-weight: 600; font-size: 11px; text-transform: uppercase;">Kritik</span>
        {{else if eq .Severity "warning"}}
          <span style="background-color: #fffbeb; color: #92400e; padding: 2px 6px; border-radius: 4px; font-weight: 600; font-size: 11px; text-transform: uppercase;">Uyarı</span>
        {{else}}
          <span style="background-color: #f0fdf4; color: #166534; padding: 2px 6px; border-radius: 4px; font-weight: 600; font-size: 11px; text-transform: uppercase;">Bilgi</span>
        {{end}}
      </td>
      <td style="padding: 10px; font-size: 13px; color: #0f172a;">{{.Type}}</td>
      <td style="padding: 10px; font-size: 13px; font-family: monospace; color: #475569;">
        {{if .Field}}<strong>{{.Field}}</strong> ({{.ColumnRef}}){{else}}{{.ColumnRef}}{{end}}
      </td>
      <td style="padding: 10px; font-size: 13px; color: #334155;">{{.Description}}</td>
    </tr>
    {{end}}
  </tbody>
</table>
<div class="button-container" style="margin-top: 24px;">
  <a href="{{.ModelURL}}" class="button" style="color: #ffffff;">Modeli Düzenleyicide Görüntüle</a>
</div>`,
		},
	},
}

const htmlEmailShell = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>%s</title>
<style>
  body {
    margin: 0;
    padding: 0;
    width: 100%% !important;
    background-color: #f8fafc;
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    -webkit-font-smoothing: antialiased;
  }
  .wrapper {
    width: 100%%;
    background-color: #f8fafc;
    padding: 40px 16px;
  }
  .container {
    max-width: 520px;
    margin: 0 auto;
    background-color: #ffffff;
    border: 1px solid #e2e8f0;
    border-radius: 12px;
    box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.05), 0 2px 4px -1px rgba(0, 0, 0, 0.03);
    overflow: hidden;
  }
  .header {
    padding: 32px 32px 20px 32px;
    border-bottom: 1px solid #f1f5f9;
  }
  .logo {
    font-size: 24px;
    font-weight: 800;
    color: #0f172a;
    letter-spacing: -0.5px;
    text-decoration: none;
  }
  .logo-dot {
    color: #4f46e5;
  }
  .content {
    padding: 32px;
  }
  h2 {
    font-size: 20px;
    font-weight: 700;
    color: #0f172a;
    margin-top: 0;
    margin-bottom: 16px;
    line-height: 1.3;
  }
  p {
    margin-top: 0;
    margin-bottom: 16px;
    font-size: 15px;
    line-height: 1.6;
    color: #334155;
  }
  .button-container {
    margin: 24px 0;
  }
  .button {
    display: inline-block;
    background-color: #4f46e5;
    color: #ffffff !important;
    padding: 12px 24px;
    font-size: 14px;
    font-weight: 600;
    text-decoration: none;
    border-radius: 8px;
    box-shadow: 0 1px 2px 0 rgba(0, 0, 0, 0.05);
    text-align: center;
  }
  .button:hover {
    background-color: #4338ca;
  }
  .details-box {
    background-color: #f1f5f9;
    border-radius: 8px;
    padding: 16px;
    margin: 24px 0;
    border: 1px solid #e2e8f0;
  }
  .details-list {
    margin: 0;
    padding: 0;
    list-style: none;
  }
  .details-list li {
    font-size: 14px;
    line-height: 1.5;
    color: #475569;
    margin-bottom: 8px;
  }
  .details-list li:last-child {
    margin-bottom: 0;
  }
  .details-list strong {
    color: #0f172a;
    font-weight: 600;
  }
  .footer {
    padding: 24px 32px;
    background-color: #f8fafc;
    border-top: 1px solid #f1f5f9;
    text-align: center;
  }
  .footer-text {
    font-size: 12px;
    line-height: 1.5;
    color: #94a3b8;
    margin: 0;
  }
  @media only screen and (max-width: 600px) {
    .wrapper {
      padding: 20px 8px;
    }
    .content {
      padding: 24px;
    }
    .header {
      padding: 24px 24px 16px 24px;
    }
  }
</style>
</head>
<body>
<div class="wrapper">
  <div class="container">
    <div class="header">
      <img src="cid:abi-logo" alt="ABI" style="display: block; height: 32px; max-height: 32px; border: 0;" />
    </div>
    <div class="content">
      <h2>%s</h2>
      %s
    </div>
    <div class="footer">
      <p class="footer-text">%s</p>
    </div>
  </div>
</div>
</body>
</html>`

// wrapHTML embeds a template's HTML body into a responsive, premium card layout.
// It also provides BCP-47 localized footers for Turkish and English environments.
func wrapHTML(bodyHTML, subject, locale string) string {
	footerText := "This is an automated email, please do not reply. To secure your ABI account, never share these links with anyone."
	if strings.HasPrefix(strings.ToLower(locale), "tr") {
		footerText = "Bu otomatik bir e-postadır, lütfen yanıtlamayın. ABI hesabınızın güvenliğini sağlamak için bu bağlantıları kimseyle paylaşmayın."
	}

	return fmt.Sprintf(htmlEmailShell, subject, subject, bodyHTML, footerText)
}
