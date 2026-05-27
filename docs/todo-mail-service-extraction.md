# Mail Servisi Ayrıştırma Planı

Mevcut durumda tüm email/SMTP kodu `internal/auth/` paketi içinde gömülü.
Amaç: email gönderim altyapısını bağımsız bir `internal/mail/` paketine ve `cmd/mail/` binary'sine taşımak.

---

> **Ön koşul (tamamlandı):** Döngüsel bağımlılığı önlemek için `NormalizeEmail`/`MaskEmail`
> bağımsız leaf paket `internal/emailaddr/`'a (`Normalize`/`Mask`) taşındı. `auth` ve `mail`
> ikisi de bunu import eder; `auth.NormalizeEmail`/`auth.MaskEmail` artık ince wrapper.

## 1. Yeni Paket: `internal/mail/`

- [x] `internal/mail/` dizinini oluştur
- [x] `internal/mail/sender.go` — `EmailSender` interface'ini taşı (`internal/auth/email.go:25-33`):
  ```go
  type EmailSender interface {
      SendEmailVerification(ctx, email, token) error
      SendPasswordReset(ctx, email, token) error
      SendEmailChangeConfirmation(ctx, email, token, newEmail bool) error
      SendAccountUnlock(ctx, email, token) error
      SendNewDeviceLogin(ctx, email, info) error
      SendAccountDeletionScheduled(ctx, email, purgeAt) error
      SendDuplicateRegistrationNotice(ctx, email) error
      SendMagicLink(ctx, email, token) error
  }
  ```
- [x] `internal/mail/smtp.go` — `SMTPEmailSender` struct'ını ve tüm metodlarını taşı (`internal/auth/email.go:42-...`):
  - `SMTPEmailSender` struct, `NewSMTPEmailSender`, `Close`, tüm `Send*` metodları
  - `sendTemplate`, queue worker, retry mantığı
  - `emailJob` struct
  - `frontendURL` helper
- [x] `internal/mail/mock.go` — `MockEmailSender`'ı taşı (`internal/auth/email.go:277-...`)
- [x] `internal/mail/errors.go` — `ErrEmailBlocked`, `ErrEmailRateLimited` senntinel error'ları taşı
- [x] `internal/mail/templates.go` — `emailTemplate`, `emailTemplateRegistry`, `builtinEmailTemplates` taşı (`internal/auth/email_templates.go`)
- [x] `internal/mail/templates_test.go` — `email_templates_test.go` testlerini taşı
- [x] `internal/mail/mime.go` — MIME boundary ve multipart oluşturma kodunu taşı (`internal/auth/email_mime.go`) + `assets/abi-logo.png` taşındı
- [x] `internal/mail/blocklist.go` — `EmailBlockListRepo` interface + SQL implementasyonunu taşı (`internal/auth/email_blocklist.go`)
- [x] `internal/mail/blocklist_test.go` — blocklist testlerini taşı (`internal/auth/email_blocklist_test.go`)
- [x] `internal/mail/config.go` — SMTP ve email ayarlarını tutan struct:
  ```go
  type Config struct {
      SMTPHost           string
      SMTPPort           int
      SMTPUser           string
      SMTPPass           string
      SMTPFrom           string
      FrontendBaseURL    string
      EmailDefaultLocale string
      EmailDailyLimit    int
      EmailQueueSize     int
      EmailRetries       int
  }
  ```

## 2. `internal/auth/` Paketini Güncelle

- [x] `internal/auth/email.go` — Dosya tamamen silindi (`NormalizeEmail` zaten `validator.go`'daydı, gönderim kodu mail'e taşındı)
- [x] `internal/auth/email_templates.go` — Dosyayı tamamen sil (taşındı)
- [x] `internal/auth/email_mime.go` — Dosyayı tamamen sil (taşındı)
- [x] `internal/auth/email_blocklist.go` — Dosyayı tamamen sil (taşındı)
- [x] `internal/auth/email_blocklist_test.go` — Dosyayı tamamen sil (taşındı)
- [x] `internal/auth/service.go` — Import'u `biqly/internal/mail` olarak değiştir:
  - `emailSender mail.EmailSender` alanı ✓
  - `NewAuthService` parametresi `mail.EmailSender` alıyor ✓
  - `DeviceLoginInfo` → `mail.DeviceLoginInfo` ✓
- [x] `internal/auth/handler.go` — `mail.ErrEmailRateLimited` referansı güncellendi
- [x] `internal/auth/magiclink.go` — Değişiklik gerekmedi (gönderimi `s.emailSender` üzerinden yapıyor)
- [ ] `internal/auth/config.go` — SMTP alanlarını çıkar veya `mail.Config` referansı ver (**Section 3'te**):
  - `SMTPHost`, `SMTPPort`, `SMTPUser`, `SMTPPass`, `SMTPFrom`
  - `EmailDefaultLocale`, `EmailDailyLimit`, `EmailQueueSize`, `EmailRetries`
- [x] `internal/auth/auth_test.go` — Import `biqly/internal/mail`, `mail.NewMockEmailSender()` kullanılıyor

## 3. `internal/auth/config.go` Refactor

- [ ] SMTP/email alanlarını `internal/auth/config.go`'dan çıkar
- [ ] `mail.Config` için ayrı bir constructor ekle (ör: `NewMailConfigFromEnv()`) veya auth Config içinde `MailConfig mail.Config` embed/göm
- [ ] `cmd/auth/main.go`'da yeni config yapısına göre mail sender'ı başlat:
  ```go
  mailCfg := mail.NewConfigFromEnv()
  emailSender := mail.NewSMTPSender(mailCfg, blockList, redis)
  ```

## 4. `internal/mail/types.go` — Paylaşılan Tipler

- [x] `DeviceLoginInfo` struct'ı `internal/mail/types.go`'ya taşındı; auth `mail.DeviceLoginInfo` kullanıyor
- [x] Döngüsel dependency çözüldü: `emailaddr` leaf paketi sayesinde `mail` paketi `auth`'u import etmiyor

## 5. Döngüsel Dependency Kontrolü

- [x] `internal/mail` paketi `internal/auth`'ı import etmiyor (doğrulandı)
  - `DeviceLoginInfo` → `mail/types.go`'da tanımlı, auth `mail.DeviceLoginInfo` kullanıyor
  - `NormalizeEmail`/`MaskEmail` → `internal/emailaddr`'a taşındı; hem auth hem mail import eder
- [x] Import döngüsü yok: `go build ./...` temiz

## 6. `cmd/mail/` Binary (Opsiyonel — Worker Modu)

- [ ] `cmd/mail/main.go` oluştur — standalone mail worker:
  - Redis queue dinleyip email gönderen bağımsız process
  - SMTP connection pooling
  - Health check endpoint (`/healthz`)
  - Graceful shutdown
- [ ] `Dockerfile.mail` oluştur — mail worker image
- [ ] `docker-compose.yml`'e `mail` service ekle

## 7. Build & Infrastructure

- [x] `go build ./internal/mail/` ile derleme kontrolü
- [x] `go build ./cmd/auth/` ile auth binary derleme kontrolü
- [x] `Dockerfile.auth` — değişiklik gerekmedi (auth binary yolu/yapısı aynı)
- [ ] `Makefile`'a `build-mail` target ekle (eğer `cmd/mail/` oluşturulduysa — Section 6)

## 8. Testler

- [x] `go test ./internal/mail/...` — taşınan testler geçti
- [x] `go test ./internal/auth/...` — auth testleri geçti (15.7s)
- [x] `go test ./...` — tüm proje testleri geçti, failure yok

## 9. Lint & Temizlik

- [x] `make lint` — golangci-lint 0 issue (kullanılmayan `nolint:gosec` direktifi de temizlendi)
- [x] `internal/auth/email.go` tamamen silindi, dead code yok
- [x] Taşınan semboller auth'ta kalmadı (grep ile doğrulandı)
- [x] `go vet ./...` temiz

## 10. Migration & DB (Değişiklik Gerekmiyor)

- [x] `email_block_list` tablosu aynı kaldı, sadece repository kodu taşındı — migration gerekmedi
- [x] `email_verification_tokens`, `password_reset_tokens`, `email_change_requests`, `magic_link_tokens` tabloları auth repoda kaldı — değişiklik yok

---

## Taşınacak Dosyalar Özeti

| Kaynak (internal/auth/) | Hedef (internal/mail/) |
|---|---|
| `email.go` → `EmailSender` interface, `SMTPEmailSender`, `MockEmailSender`, senntinel error'lar | `sender.go`, `smtp.go`, `mock.go`, `errors.go` |
| `email_templates.go` → template registry + builtin templates | `templates.go` |
| `email_templates_test.go` | `templates_test.go` |
| `email_mime.go` → MIME multipart oluşturma | `mime.go` |
| `email_blocklist.go` → `EmailBlockListRepo` | `blocklist.go` |
| `email_blocklist_test.go` | `blocklist_test.go` |
| (yeni) SMTP/email config struct | `config.go` |
| (yeni) Paylaşılan tipler (`DeviceLoginInfo`) | `types.go` |

## `internal/auth/`'da Kalacak

| Dosya | Neden |
|---|---|
| `validator.go` → `NormalizeEmail` wrapper (`emailaddr.Normalize`) | Email validation auth API yüzeyi |
| `audit_mask.go` → `MaskEmail` wrapper (`emailaddr.Mask`) | Log maskeleme auth API yüzeyi |
| `service.go` → `mail.EmailSender` import eder | Auth servisi mail interface'ini kullanır |
| `config.go` → SMTP alanları çıkarılacak (Section 3) | Auth kendi konfigürasyonunu tutar |
| `repository.go`, `handler.go`, vb. | Auth domain logic |

> Not: `email.go` tamamen silindi (gönderim kodu mail'e, `NormalizeEmail` zaten `validator.go`'daydı).
> Tek normalize/mask kaynağı: `internal/emailaddr` (auth wrapper'ları buna delege eder).
