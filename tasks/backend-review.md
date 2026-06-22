# Backend Review — 2026-06-22

> Kapsam: Backend best-practice review (sınırlar, güvenlik, hata yönetimi, context
> yayılımı, config, observability, DB, performans, testler). Önceki P0/P1/P2 sweep'leri
> (`tasks/todo.md` → "Backend Go Code Review") zaten uygulanmıştı; bu tur kalan
> concrete ihlalleri aradı.

## Review Metodolojisi

- `AGENTS.md`, `CONTEXT.md`, `tasks/lessons.md` okundu.
- Sentinel-error karşılaştırmaları (`err == sql.ErrNoRows` vb.) → **0 bulgu** (önceki errorlint geçişi temiz).
- Raw SQL concatenation / parametresiz SQL → güvenli (metadata_rows.go identifier'ları validated metadata'dan, değerler `$N` placeholder).
- `context.Background()`/`context.TODO()` non-test kullanım → sadece audit worker (`db_writer.go`, `//nolint:contextcheck` root ctx — doğru pattern).
- `rows.Err()` kontroleri → lint (`rowserrcheck`) temiz.
- DSN/secret logging → **1 bulgu** (düzeltildi).
- Executor: read-only check + timeout + row-limit + parameterize → **sağlam**.
- DryRun: read-only guard (`security.NewReadOnlyChecker().Check`) → **sağlam** (P2 fix yerinde).
- `RunInTx`: panic-rollback → **sağlam** (P1 fix yerinde).
- `make lint-go` → **0 issues** (review öncesi ve sonrası).

## Yapılan Düzeltme

- [x] **DSN plaintext log (güvenlik):** `internal/app/dependencies.go:508` — Redis DSN'si
  (`redis://:password@host:port`) Info seviyesinde log'lanıyordu. Fix: `"dsn", dsn` →
  `"addr", opt.Addr` (host:port only, credential'siz). `opt` zaten `redis.ParseURL`
  çıktısı olarak scope'ta. Doğrulama: `gofmt` + `go test ./internal/app/...` + `make lint-go` (0 issues).

## Kalan Takip İşleri (sırayla)

### S1 — Observability: 500 hata log'larına request context ekle (orta) — TAMAMLANDI

**Durum:** Güvenlik riski YOK — `writeError` helper'ı (`response.go:44`) status ≥ 500'de
client'a `"internal server error"` sanitize ediyor, raw `err.Error()` sızdırılmıyor.
Ancak server-side log request-scoped context'ten yoksundu (`request_id`, `user_id`,
`workspace_id` yok), bu da log korelasyonunu zorlaştırıyordu.

**Düzeltildi:** Aşağıdaki siteler `writeInternalError(ctx, w, status, "deskriptif mesaj", err)`
kullanımına geçirildi. Bu helper `request_id`/`user_id`/`workspace_id` ile structured log verir.
- `internal/http/handlers/ai_user_models.go:129,162,187,193,222`
- `internal/http/handlers/ai_providers.go:299` (StatusBadGateway)

**Başarı kriteri:** ✓ Client response hala `"internal server error"` (davranış korundu);
✓ log artık deskriptif mesaj + request context içeriyor; ✓ lint (0 issues) + test temiz.

### S2 — Yapısal borç (düşük öncelik, `tasks/todo.md`'den carry-over)

- [x] **God objects (10):** `Metrics` (79 alan), `auth.Config` (35), `Dependencies` (34) vb.
     Çoğu DTO/config; refactor opsiyonel, bug gizlemiyorlar. → ATLANDI
- [x] **High-arity fonksiyonlar:** `recordMetricsAndState` (14 param) → `observeAIRequestParams` struct'ı
- [x] **High-arity fonksiyonlar:** `NewRBACHandler` (10 param) → `RBACHandlerDeps` struct'ı
- [x] **`.gograph/boundaries.json` oluşturuldu:** `gograph boundaries --create` ile auto-generate, `internal_core`'a `pgx/v5/stdlib` eklendi.

### S3 — Test boşlukları (düşük, `tasks/todo.md`'den carry-over)

- [x] `internal/core` `DryRun` read-only guard entegrasyon testi — `query_service_dryrun_test.go`
     (2 test: guard pas geçer, compile hatası guard'tan önce döner).
     `make dev-up` ile yerelde çalışır; CI'da `BI_METADATA_DB_DSN` yoksa skip.

## Notes

- `writeError` ile `writeInternalError` arasındaki fark **sadece observability** — ikisi
  de client'a aynı sanitize mesajı gönderir. `writeInternalError` ek olarak structured
  log'a request context katar. Bu yüzden S1 güvenlik değil, observability iyileştirmesidir.
- Tüm yüksek riskli backend yolları (executor, compiler, AI pipeline, permission checks)
  önceki review turlarında sağlamlaştırılmış; bu tur yeni critical ihlal bulmadı.
