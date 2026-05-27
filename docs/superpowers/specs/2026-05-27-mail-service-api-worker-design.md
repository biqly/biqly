# Mail Servisi — Ayrı DB + HTTP API Worker (Tasarım)

Tarih: 2026-05-27
Durum: Onaylandı (kullanıcı), implementasyon planı bekliyor

## Amaç

Transactional email altyapısını auth process'inden tamamen ayırmak:

1. Mail'e ait tabloyu (`email_block_list`) aynı Postgres instance'ı içinde
   ayrı bir veritabanına (`bi_mail`) taşımak — best practice servis-başına-DB.
2. `cmd/mail` altında bağımsız bir worker process oluşturmak.
3. auth ↔ mail iletişimini HTTP API üzerinden kurmak (in-process çağrı değil).
4. Helm tarafında yeni subchart + DB + NetworkPolicy ayarlarını yapmak.

Bu çalışma, daha önce başlatılan mail ayrıştırmasının (`internal/mail` paketi,
`internal/emailaddr` leaf paketi) devamıdır ve önceki plandaki Section 3
(auth config'ten SMTP çıkarma), Section 6 (worker), Section 7 (infra) maddelerini
kapsar.

## Onaylanan Kararlar

| Karar | Seçim |
|---|---|
| İletişim/hata modeli | **Async**: mail `202 Accepted` döner, iç kuyruk + retry ile SMTP'ye gönderir. auth fire-and-forget. |
| API yüzeyi | **Generic**: tek endpoint `POST /internal/mail/send` `{template, to, data}`. Şablon/lokalizasyon/URL kurma mail'de. |
| DB taşıma | **Temiz reloc**: auth'tan DROP + `bi_mail`'de CREATE. (dev/staging — blocklist veri taşıması yok) |
| Servisler arası kimlik | Mevcut `InternalToken`, `X-Internal-Token` başlığı. Mail API route'suz, yalnız cluster-içi. |

## Mimari Akış

```
auth (login / register / email-change / ...)        mail-worker (cmd/mail)
  │  mail.APIClient  (mail.EmailSender impl)             │  chi HTTP server  :8890
  │  POST /internal/mail/send                            │  X-Internal-Token doğrula
  │  body: {template, to, data}                          │  → 202 Accepted (anında)
  │  header: X-Internal-Token                            │
  │  ◀──────────────── 202 ───────────────────────────  │  iç kuyruk + retry
  │     (hata loglanır, akış bloklanmaz)                 │     → blocklist (bi_mail)
  │                                                      │     → rate-limit (redis)
  │                                                      │     → render + SMTP gönder
```

## Bileşenler

### A. `internal/mail` paketi

#### `client.go` — `APIClient`
- `mail.EmailSender` interface'ini HTTP üzerinden implemente eder.
- Constructor: `NewAPIClient(baseURL, internalToken string, opts...) *APIClient`.
  Varsayılan `*http.Client` kısa timeout ile (örn. 5s).
- Her tipli metod generic isteğe maplenir:
  - `SendEmailVerification(ctx, email, token)` → `send(ctx, "verification", email, {"token": token})`
  - `SendPasswordReset` → `"password_reset"`, `{"token": token}`
  - `SendEmailChangeConfirmation` → `"email_change"`, `{"token": token, "new_email": newEmail}`
  - `SendAccountUnlock` → `"account_unlock"`, `{"token": token}`
  - `SendNewDeviceLogin` → `"new_device"`, `{"user_agent","ip_address","occurred_at"}`
  - `SendAccountDeletionScheduled` → `"deletion_scheduled"`, `{"purge_at": RFC3339}`
  - `SendDuplicateRegistrationNotice` → `"duplicate_registration"`, `{}`
  - `SendMagicLink` → `"magic_link"`, `{"token": token}`
- `send` gövdesi: `{"template": ..., "to": ..., "data": {...}}`. `X-Internal-Token` başlığı.
- 2xx → `nil`. 4xx/5xx → hata döner (auth tarafında loglanır, yutulur — mevcut davranış).

#### `server.go` — HTTP handler (worker tarafı)
- `Handler(sender *SMTPEmailSender, internalToken string) http.Handler` veya chi router.
- `POST /internal/mail/send`: token doğrula → JSON decode → `sender.SendTemplate(ctx, template, to, data)` → `202 Accepted`.
  - Bilinmeyen template → `400`. Token yanlış → `401`.
  - `SendTemplate` çağrısı kuyruğa atıp anında döndüğü için handler bloklanmaz.

#### `smtp.go` — `SendTemplate` generic girişi
- Yeni metod: `SendTemplate(ctx, template, to string, data map[string]any) error`.
  - Şablona göre URL augment eder (`token` → `frontendURL(...)`, `SecurityURL`/`AccountURL`/`SignInURL`/`ForgotURL` ekler).
  - Mevcut `Send*` metodları artık `SendTemplate`'in ince sarmalayıcısı (geriye dönük; worker tarafında doğrudan `SendTemplate` kullanılır).
  - Augment haritası tek yerde (`templateData` helper): wire `data` (token/flag/device) + mail'in ürettiği URL'ler.

#### `config.go` — `NewConfigFromEnv()`
- `BI_MAIL_*` env'lerinden `mail.Config` üretir:
  `BI_MAIL_SMTP_HOST/PORT/USER/PASS/FROM`, `BI_MAIL_FRONTEND_BASE_URL`,
  `BI_MAIL_EMAIL_DEFAULT_LOCALE`, `BI_MAIL_EMAIL_DAILY_LIMIT`,
  `BI_MAIL_EMAIL_QUEUE_SIZE`, `BI_MAIL_EMAIL_RETRIES`.
- Ek worker env'leri: `BI_MAIL_PORT` (default 8890), `BI_MAIL_DB_DSN`,
  `BI_MAIL_REDIS_DSN`, `BI_MAIL_INTERNAL_TOKEN`.

#### `blocklist.go`
- Değişiklik yok; artık `bi_mail` DSN'ine bağlanır.

### B. `cmd/mail/main.go` (worker)
- `bi_mail` DB aç + (opsiyonel) redis aç.
- `mail.NewEmailBlockListRepo(db)` + `mail.NewSMTPEmailSender(cfg, blockList, redis)` (queue+retry açık).
- chi server: `POST /internal/mail/send` (token korumalı), `GET /healthz` (liveness),
  `GET /readyz` (DB ping). Graceful shutdown: HTTP'yi kapat → `sender.Close()` ile kuyruğu drain et.

### C. `cmd/mail-migrate/main.go`
- `cmd/auth-migrate/main.go`'nun birebir kopyası; default `-dir migrations/mail`,
  DSN env `BI_MAIL_DB_DSN`.

### D. `internal/auth` değişiklikleri (önceki Section 3'ü kapatır)
- `Config`'ten SMTP/email alanları **tamamen çıkar**: `SMTPHost/Port/User/Pass/From`,
  `EmailDefaultLocale`, `EmailDailyLimit`, `EmailQueueSize`, `EmailRetries`.
  `FrontendBaseURL` auth'ta **korunur** (OAuth redirect vb. için kullanılıyor); email URL'leri
  artık mail tarafında kurulur, dolayısıyla `BI_MAIL_FRONTEND_BASE_URL` mail config'ine eklenir.
- Yeni alan: `MailServiceURL` (`BI_AUTH_MAIL_SERVICE_URL`, default `http://biqly-mail:8890`).
- `cmd/auth/main.go`: blocklist + SMTP sender wiring kaldırılır →
  `emailSender = mail.NewAPIClient(cfg.MailServiceURL, cfg.InternalToken)`.

### E. Migrations / DB
- `migrations/auth/030a_drop_email_block_list.up.sql`: `DROP TABLE IF EXISTS email_block_list;`
- `migrations/auth/030b_drop_email_block_list.down.sql`: tabloyu yeniden oluştur (028a içeriği).
- `migrations/mail/001a_email_block_list.up.sql`: tabloyu oluştur (028a içeriğiyle birebir).
- `migrations/mail/001b_email_block_list.down.sql`: `DROP TABLE IF EXISTS email_block_list;`
- `bi_mail` veritabanı + kullanıcı, `bi_auth` ile aynı mekanizmayla provision edilir
  (Postgres init / chart values — implementasyonda `bi_auth`'un yaratıldığı yer baz alınır).

### F. Helm — yeni `charts/mail` subchart (charts/auth baz alınarak)
- `Chart.yaml`, `values.yaml`, `templates/`:
  - `deployment.yaml` — worker; **route yok**, yalnız iç servis.
  - `service.yaml` — `:8890`.
  - `configmap.yaml` — `BI_MAIL_SMTP_HOST/PORT/FROM`, `BI_MAIL_FRONTEND_BASE_URL`,
    `BI_MAIL_EMAIL_DEFAULT_LOCALE`, limit/queue/retry, `BI_MAIL_PORT`, `BI_MAIL_REDIS_DSN`.
  - `secret.yaml` — `BI_MAIL_DB_DSN`, `BI_MAIL_SMTP_USER`, `BI_MAIL_SMTP_PASS`, `BI_MAIL_INTERNAL_TOKEN`.
  - `migrate-job.yaml` — `mail-migrate -dir migrations/mail`, `bi_mail` DSN.
  - `hpa.yaml`, `pdb.yaml`, `networkpolicy.yaml`.
- **NetworkPolicy**:
  - mail ingress ← auth (port 8890).
  - mail egress → postgres(5432) + redis(6379) + smtp(587).
  - auth egress'ten **smtp(587) kaldırılır**; auth egress → mail(8890) eklenir.
- auth subchart: SMTP `config`/`secrets` anahtarları kaldırılır; `BI_AUTH_MAIL_SERVICE_URL` eklenir.
- Umbrella `Chart.yaml` dependencies + `values.yaml`/`values-*.yaml`'a `mail:` bloğu.
- Postgres init'e `bi_mail` DB + kullanıcı.
- `Dockerfile.mail` (worker + mail-migrate binary), Makefile `build-mail` / `build-mail-migrate`.

## Test Stratejisi
- `internal/mail`:
  - `APIClient` ↔ `server.go` round-trip testi (`httptest.Server`): her tipli metod
    doğru `{template, data}` üretiyor mu; token doğrulaması; bilinmeyen template 400.
  - `SendTemplate` augment testi: token → doğru URL; `new_device` HTML escape (mevcut test korunur).
  - Mevcut `templates_test.go` / `blocklist_test.go` geçmeye devam eder.
- `go build ./...`, `go vet ./...`, `make lint` temiz.
- `helm template` / `helm lint` ile chart render doğrulaması; ArgoCD CI lint.

## Sınırlar / YAGNI
- Persistent/durable queue yok — in-memory kuyruk + retry yeterli (mevcut davranış). Worker
  yeniden başlarsa kuyruktaki bekleyen mailler kaybolabilir; auth zaten fire-and-forget.
- Mail için admin blocklist API'si bu kapsamda yok (mevcut kodda da yoktu).
- Çoklu worker'da rate-limit redis ile paylaşıldığı için tutarlı; blocklist DB'de.

## Riskler
- auth deploy'u mail'den önce gelirse email gönderimi başarısız olur (loglanır, akış sürer).
  NetworkPolicy/servis hazır olana dek mailler düşer — kabul edilebilir (fire-and-forget).
- `bi_mail` DB provision adımı mevcut Postgres init mekanizmasına bağlı; yanlış yapılırsa
  migrate-job başarısız olur (PreSync hook → deploy durur, güvenli).
