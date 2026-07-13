# Backend Code Review — Güvenlik & Kod Tekrarı

**Tarih:** 2026-07-13
**Kapsam:** Tüm backend (`internal/**`, `cmd/**`, `services/**`) — 96 paket, ~941 dosya
**Yöntem:** 7 paralel review ajanı (domain bazında güvenlik + duplikasyon), her bulgu gograph + kaynak okuma ile teyit edildi. `writeError` hotspot ipucu araştırıldı ve gograph fan-in merge artifact'ı olduğu doğrulandı (gerçek tekrar sarmalayıcı katmanda — bkz. D5).

Severity: **CRITICAL** yok. **HIGH** 2, **MEDIUM** 7, **LOW** 10 (güvenlik) + duplikasyon 17.

## Durum (2026-07-13)

| Batch | Bulgular | Durum |
|-------|----------|-------|
| 1 | S1, S2 (HIGH) + regresyon testleri | ✅ tamamlandı (lint 0, testler yeşil, commit bekliyor) |
| 2 | S3, S4, S5, S6 (MEDIUM authz/IDOR/spend) | ✅ tamamlandı (lint 0, testler + helm render yeşil, commit bekliyor) |
| 3 | S7, S8 (query/SQL) | ✅ tamamlandı (lint 0, testler + go vet yeşil, commit bekliyor) |
| 4 | S9–S19 (LOW) | ✅ tamamlandı (lint 0, testler + go vet yeşil, commit bekliyor) |
| 5 | D1–D16 (kod tekrarı) | 🔄 D2 (bug fix) + D12 tamam; kalan pure-maintainability refactor'lar bekliyor |

- **S1** ✅ `internal/auth/handlers/handler.go:1239` → `requireSuperAdmin` guard; test: `handler_resend_verification_test.go`.
- **S2** ✅ `internal/http/handlers/composite.go:76` → `resolveDatasourceScope` + `filterCompositesByScope`; test: `composite_test.go`.
- **S3** ✅ `internal/http/ai_router.go` → `registerAISemanticModelRoutes` helper'ı, translate + describe-joins'e koşulsuz `RequireResolvedDatasourceAccess("write", modelDS)` (history/replay pattern'i).
- **S5** ✅ `internal/http/handlers/dashboard.go` → `dashboardScope` helper'ı; boş workspace yalnızca super-admin için unscoped, regular kullanıcıya boş sonuç/not-found. Create/List/Get/Update/Delete güncellendi.
- **S4** ✅ **Tamamlandı (tam cross-service resolver).** Katalog/api servisine `GET /internal/resource-datasource` endpoint'i eklendi (`model`→SemanticRepo.GetModel, `query`→MetaRepo.GetAIQueryHistoryByID; 400 unsupported, 404 not-found) — `internal/http/handlers/internal.go` + `internal_ports.go` + `catalog_router.go` + `pkg/internalapi/resource.go`. Auth servisine `HTTPResourceResolver` client (`internal/auth/workspace/resource_resolver.go`, X-Internal-Token). `SharingService.Share` artık kaynağı datasource'a çözüp çağıranın `DsAccess.CheckAccess`'ini doğruluyor (view/execute→read, edit→write), unsupported/not-found'da fail-closed (`sharing.go`). Config: `BI_AUTH_CATALOG_SERVICE_URL` + `BI_INTERNAL_API_TOKEN`; Helm: auth deployment env (security secret) + configmap. Testler: `sharing_test.go`'ya 4 ownership-guard case (denied→reddedilir, unresolvable→fail-closed, edit→write, view→read). Resolver/access nil ise (local dev) guard atlanır + startup warning. **Deploy notu:** prod auth pod'una `BI_INTERNAL_API_TOKEN` (biqly-security secret) sağlanmalı; aksi halde guard dormant kalır.
- **S7** ✅ **Tamamlandı.** `internal/dialect`'e `QuoteStringLiteral` capability'si eklendi: `BaseDialect` default'u yalnızca single-quote ikiliyor, backslash'i C-style escape eden dialect'ler (mysql/clickhouse/snowflake/databricks) backslash'i de escape ediyor. `internal/query/expr_compiler.go` `literalSQL` artık `d.QuoteStringLiteral` kullanıyor → MySQL/ClickHouse backslash breakout kapandı. Test: `internal/dialect/quote_string_literal_test.go` (per-dialect + breakout payload).
- **S8** ✅ **Tamamlandı.** `internal/query/compiler.go` `resolveBracketExpressions` (raw metrik SQL'inin ortak chokepoint'i — hem `metricExpressionRef` hem `qualifyMetricExpression`) artık çözümlenen SQL'e `CompileExpr` ile aynı `exprReadOnlyChecker`'ı uyguluyor → custom/raw metrik ifadeleri DML kaçıramıyor. Test: `compiler_custom_metric_guard_test.go`.
- **S6** ✅ **Tamamlandı.** Keşif, `BI_REDIS_DSN`'in zaten shared `biqly-config` ConfigMap'i üzerinden agent pod'una ulaştığını gösterdi — Helm/infra değişikliği gerekmedi. Yeni `internal/ai/SpendLimitedProvider` (ctx-workspace tabanlı, `ai.WithWorkspace`); `NewAgentDependencies` planner provider'ını sarıyor (`newAIRedisClient` + `NewSpendLimiter`, Close'da Redis kapatılıyor); `cmd/agent` processJob runCtx'e `job.WorkspaceID` ekliyor. Test: `internal/ai/spend_provider_test.go` (4 case). SpendLimiter fail-safe (nil client/redis error → pass-through). **Not:** cap yalnızca `BI_AI_WORKSPACE_DAILY_TOKEN_BUDGET` set edilince aktif (şu an her yerde 0/dormant — AI servisi dahil); aktivasyon ops kararı.

---

### Batch 4 (LOW) — tamamlandı

- **S9** ✅ `config.go` — `BI_ENCRYPTION_KEY` default'u boş (validation zaten required).
- **S10** ✅ `internal/agent/policy.go` — `firstInvalidJoin` boş allowlist'i "kısıtlama yok" olarak ele alıyor (deny-all over-blocking bug'ı düzeltildi); RunContext-driven kontrollerin advisory olduğu belgelendi.
- **S11** ✅ `internal/agent/policy.go` — prompt-injection heuristic'i non-authoritative telemetri olarak açıkça belgelendi.
- **S12** ✅ `internal/ai/provider/egress.go` — deployment-mode-aware `CheckProviderBaseURL`: cloud modda private/metadata host'ları reddediyor (SSRF), private/airgapped'te self-hosted'a izin. Create/UpdateProvider'da uygulanıyor. Test: `egress_provider_url_test.go`.
- **S13** ✅ `internal/query/validator.go` — sıfır/atlanmış limit maxRows'a floor'lanıyor. Test: `TestValidator_ZeroLimitFlooredToMaxRows`.
- **S14** ✅ `internal/http/handlers/ai_conversations.go` — `decodeJSON` (1 MiB cap).
- **S15** ✅ catalog/query/ai router'ları `/metrics`'i `APIKeyAuth(MetricsAPIKey)` ile gate ediyor (monolith ile aynı; boş key'de fail-open).
- **S16** ✅ `config.go` — prod'da `sslmode=disable` uyarısı.
- **S17** ✅ `internal/audit/db_writer.go` — non-UUID actor artık details JSON'ında ("actor") korunuyor (NULL'a atılmıyor). (Backpressure drop'u kasıtlı non-blocking tasarım — metric+alert var — değiştirilmedi.)
- **S18** ✅ `internal/queue/local.go` — handler hatasında log-and-continue (consumer ölmüyor). Test güncellendi.
- **S19** ✅ `internal/dbmigrate/migrate.go` — her migration dosyası transaction içinde; `isAlreadyAppliedError`'dan `23505` çıkarıldı.

## Güvenlik Bulguları

### S1 — [HIGH] Admin resend-verification endpoint'inde yetki kontrolü yok
- **kategori:** missing-authz / IDOR (email-bombing + enumeration)
- **dosya:** `internal/auth/handlers/handler.go:1239` (route: `handler.go:151`); servis: `internal/auth/service_email.go:81`
- **güven:** yüksek (iki bağımsız ajan doğruladı)
- **açıklama:** `POST /admin/users/{id}/resend-verification` yalnızca `requireUserID` (giriş yapılmış mı) kontrolü yapıyor; `AdminResendUserVerification` çağrı zincirinde `IsSuperAdmin` kontrolü yok. Aynı dosyadaki diğer TÜM `/admin/*` endpoint'leri (`requireSuperAdmin` / `ErrNotSuperAdmin`) yetki uyguluyor — bu tek istisna. Herhangi bir authenticated (admin olmayan) kullanıcı: (a) 404/400/200 dönüşlerinden user id + doğrulama durumu enumerate edebilir, (b) keyfi kullanıcılara doğrulama e-postası tetikleyebilir (email-bombing).
- **düzeltme:** `AdminResendUserVerification`'a `actorUserID` geçir + başına `IsSuperAdmin` guard ekle; `ErrNotSuperAdmin` → 403. Handler zaten actor id'yi alıp atıyor (`if _, ok := h.requireUserID(...)`) — yakala.

### S2 — [HIGH] Composite semantic modeller tüm tenant'larda okunabiliyor (datasource scoping yok)
- **kategori:** IDOR / missing-authz
- **dosya:** `internal/http/handlers/composite.go:76` (route: `internal/http/catalog_router.go:164`)
- **güven:** yüksek
- **açıklama:** `GET /semantic/composites` middleware guard'sız kayıtlı (`authMW` grubunun altında ama datasource kontrolü yok). Boş `datasource_id` ile repo her datasource/tenant'ın composite'lerini döndürüyor; saldırgan-tanımlı `datasource_id` ile o tenant'ın composite'lerini `CheckDatasourceAccess` olmadan döndürüyor. Kardeş `ListModels` handler'ı doğru şekilde `resolveDatasourceScope` çağırıyor — bu handler çağırmıyor. Hem in-process hem proxy modunda erişilebilir.
- **düzeltme:** `ListComposites`'e `ListModels` ile aynı `resolveDatasourceScope` filtresini uygula (ve/veya `datasource_id` verildiğinde route'u datasource-access kontrolüyle gate et).

### S3 — [MEDIUM] Cross-tenant semantic-model translate: ownership kontrolü yok + LLM spend
- **kategori:** IDOR
- **dosya:** `internal/http/ai_router.go:140` (handler: `internal/http/handlers/ai_semantic_translate.go:29`); ayrıca describe-joins `ai_router.go:143`
- **güven:** yüksek
- **açıklama:** `POST /ai/semantic/models/{id}/translate` sadece `aiUserMW` ile bağlı — `RequireResolvedDatasourceAccess` yok. `TranslateSemanticModel`, herhangi bir model id için `GetFullModel` yapıp LLM translator çalıştırıyor ve `entity_translations`'a `UpsertTranslation` ile yazıyor. `ai:query` yetkisi olan herhangi bir kullanıcı erişimi olmayan datasource'lara ait modellerde LLM harcaması tetikleyip translation içeriğini değiştirebilir. Route yorumu bunu "GetModel'in korumasız okuma duruşuyla eşleşiyor" diye gerekçelendiriyor ama `GET /semantic/models/{id}` aslında `modelRead` (`RequireResolvedDatasourceAccess`) ile korunuyor — gerekçe yanlış. Proxy modu da telafi etmiyor.
- **düzeltme:** Her iki route'u `RequireResolvedDatasourceAccess(authClient, "write", modelDS)` ile sar (catalog model route'larındaki `modelDS` resolver'ını yeniden kullan).

### S4 — [MEDIUM] Resource share oluştururken kaynak ownership doğrulanmıyor (IDOR / privilege escalation)
- **kategori:** IDOR
- **dosya:** `internal/auth/workspace/sharing.go:44`
- **güven:** orta
- **açıklama:** `SharingService.Share`, verilen herhangi bir `resource_id` için `owner_id = callerID` ile `resource_shares` satırı ekliyor. Tek guard (`WorkspaceID` set ise workspace membership) kaynağın gerçekten çağırana ait olduğunu doğrulamıyor. Authenticated bir kullanıcı başkasına ait bir kaynak için share oluşturabilir (ör. `shared_with=attacker`, `permission=edit`). `resource_shares`'e güvenen downstream authz saldırganı yetkili collaborator sayar.
- **düzeltme:** INSERT öncesi çağıranın `resource_type`/`resource_id` üzerinde gerçek owner (veya admin) olduğunu doğrula; değilse reddet. Downstream tüketicilerin bağımsız ownership uyguladığını da teyit et.

### S5 — [MEDIUM] Dashboard erişimi boş workspace context'i rol kontrolü olmadan super-admin gibi ele alıyor
- **kategori:** tenant-isolation
- **dosya:** `internal/dashboard/repository.go:42` (ayrıca :71, :126, :156); handler: `internal/http/handlers/dashboard.go:94`
- **güven:** orta
- **açıklama:** Dashboard repo boş `workspaceID`'yi "scope yok / hepsini gör" olarak ele alıyor ("ör. super_admin"). Tek çağıran `DashboardHandler`, `bimw.WorkspaceID(ctx)`'yi rol kontrolü olmadan geçiriyor. `resolveDatasourceScope` scope'suz erişimi `HasRole(RoleSuperAdmin)` ile gate ederken dashboard yolu etmiyor. Aktif workspace'i olmayan bir principal (PAT/session — `resolveDatasourceScope`'un açıkça öngördüğü durum) tüm workspace'lerin dashboard'larına read/update/delete erişimi kazanıyor. Route'lar (catalog_router.go:213-221) sadece JWT taşıyor.
- **düzeltme:** Dashboard handler'larında "scope yok" kararını `resolveDatasourceScope` gibi ver — boş workspace'i yalnızca `HasRole(ctx, RoleSuperAdmin)` iken geçir; aksi halde boş sonuç kümesi döndür.

### S6 — [MEDIUM] NATS agent runner planner LLM çağrılarında spend-cap uygulamıyor
- **kategori:** spend-cap / cost DoS
- **dosya:** `cmd/agent/main.go:52` / `internal/app/agent_dependencies.go:66` / `internal/agent/runtime.go:198`
- **güven:** yüksek
- **açıklama:** Web-agent yolu planner provider'ını `spendLimitedProvider` ile sarıp her completion'ı workspace günlük token bütçesine karşı kontrol ediyor (`ai_agent_chat.go:425`). NATS agent runner etmiyor: `NewAgentDependencies` ham provider ile `NewProviderPlanner` kuruyor, `SpendLimiter.Check` hiç çağrılmıyor, `AgentDependencies`'te `SpendLimiter` alanı bile yok. Her agent run planner tokenları için sıfır bütçe muhasebesiyle `MaxSteps`(≤6) completion + tool çağrısı yapıyor. Bütçesi tükenmiş workspace agent run'larla devam edebilir.
- **düzeltme:** Agent planner provider'ını web yolundaki gibi spend-limited provider'a sar (workspace `job.WorkspaceID`'den) veya `processJob`/`runToolStep`'te her planner/tool LLM çağrısı öncesi `SpendLimiter.Check` gate ekle.

### S7 — [MEDIUM] Inline string-literal escaping dialect-agnostic (MySQL/ClickHouse backslash breakout)
- **kategori:** sql-injection (dialect-specific)
- **dosya:** `internal/query/expr_compiler.go:144-164` (string case :149)
- **güven:** orta
- **açıklama:** `literalSQL` string literal'leri yalnızca single-quote ikileme (`'`→`''`) ile escape ediyor, dialect argümanı almıyor. `CompileExpr` `args==nil` ile çağrıldığında (her calculated-expression/metric/window yolu: `dimensionSQL`, `metricExpressionRef`, `buildWindowExpr` hepsi `nil` args geçiyor) `LiteralExpr` değerleri parametrelenmek yerine inline ediliyor. Request-supplied `WindowSpec.Expr`'in AST içeriği `validateWindowSelect` tarafından kontrol edilmiyor. MySQL/ClickHouse'da backslash string escape karakteri olduğundan `\` ile biten literal (`\' OR 1=1 -- `) `''`-only escaping'i atlatıp string'den çıkabilir. Postgres güvenli (standard_conforming_strings).
- **düzeltme:** Literal'leri parametrele (`args`'ı bu `CompileExpr` çağrılarından geçir → `d.Placeholder`), veya escaping'i dialect-aware yap (MySQL/ClickHouse için `\` escape), veya inline string literal'lerde backslash'i reddet.

### S8 — [MEDIUM] Custom-metric ham ifadesi AST read-only/DML guard'ı olmadan SQL'e ulaşıyor
- **kategori:** readonly-bypass
- **dosya:** `internal/query/compiler.go:537-559` (`resolveBareCustomExpression`), :618-634 (`metricAggregate`)
- **güven:** düşük
- **açıklama:** AST ifade yolu `exprReadOnlyChecker.Check` ile korunuyor; ham-string custom-metric yolu korunmuyor. `Aggregation=="custom"` ve `Expr` AST'siz metrik için `metricExpressionRef`→`resolveBareCustomExpression` metni aynen döndürüyor, `dialect.Aggregate("custom", expr)` unquoted döndürüyor. Publish doğrulaması asimetrik: `containsDMLKeyword` calculated *dimension*'lar için çalışıyor ama *metric*'ler için hiç çağrılmıyor. Model yazarı bağımsız read-only kontrol olmadan aggregate pozisyonuna ham SQL koyabilir.
- **düzeltme:** Custom-metric ifadelerini `CompileExpr`'in kullandığı doğrulanmış AST + `exprReadOnlyChecker` yolundan geçir, veya publish-time'da metric ifadelerine `containsDMLKeyword`/read-only doğrulaması uygula (dimension yolunu aynala).

### S9 — [MEDIUM] Config default'unda hardcoded placeholder encryption key
- **kategori:** hardcoded-secret / insecure-default
- **dosya:** `internal/config/config.go:510`
- **güven:** yüksek
- **açıklama:** `BI_ENCRYPTION_KEY` default'u source'a gömülü `"change-this-to-a-secure-32-byte-key!!"` literal'i. `validateLoadedConfig` (config.go:659) bu tam değeri startup'ta reddettiği ve `security.NewEncryption` base64 32-byte istediği için mitigate ediliyor. Yine de binary'de shipped placeholder secret var; `Load()`/validation'dan geçmeyen herhangi bir kod yolu bilinen key ile sessizce çalışır.
- **düzeltme:** Default'u boş string yap, yalnızca mevcut "required" validation'a güven — binary'de kullanılabilir key literal'i kalmasın.

### S10 — [LOW] PII / hidden-column / row-filter policy kuralları NATS agent yolunda etkisiz
- **kategori:** tool-contract (defense-in-depth kaybı)
- **dosya:** `cmd/agent/main.go:152` / `internal/agent/policy.go:280`
- **güven:** yüksek
- **açıklama:** `PolicyEngine.evaluateQuery` hidden-column/PII/join/row-filter denial'larını `RunContext` alanlarına karşı uyguluyor ama `processJob`'ın kurduğu `RunContext` bunları boş bırakıyor (yorumda kabul edilmiş). Böylece `firstHiddenColumn`/`firstUnmaskedPIIColumn`/row-filter kontrolleri no-op. Downstream Query servisinin compile-time doğrulamasına dayanan bir defense-in-depth kaybı. Yan etki: `AllowedJoins` boş olunca `firstInvalidJoin` planner'ın önerdiği HERHANGİ join'i reddediyor.
- **düzeltme:** `RunContext`'i çözümlenmiş semantic modelden doldur, veya bu guard'ları açıkça non-enforced olarak belgele ve yarı-aktif halini kaldır.

### S11 — [LOW] Prompt-injection heuristic'i kolayca atlatılabilir ve web tool'lar için hiç çalışmıyor
- **kategori:** prompt-injection
- **dosya:** `internal/agent/policy.go:225` / :326
- **güven:** yüksek
- **açıklama:** `containsPromptInjection` 5 sabit İngilizce substring eşliyor (yeniden ifade/çeviri/whitespace ile atlatılabilir) ve yalnızca non-web tool'lar için çalışıyor (`if !isWebTool`). Asıl agent yüzeyi olan MCP-parity web tool'lar (planner-authored argümanları `/api/*`'e ileten) kontrolü tamamen atlıyor.
- **düzeltme:** Bunu kontrol değil telemetri olarak ele al; katı tool contract + governed `/api/*` enforcement'a güven (zaten tasarım bu). Tutulacaksa tekdüze uygula ve non-authoritative olarak belgele.

### S12 — [LOW] Admin-konfigüre provider base URL airgapped dışında doğrulanmıyor (SSRF)
- **kategori:** ssrf (authenticated-admin)
- **dosya:** `internal/ai/provider_store.go:873` (`ListRemoteModels`), :900 (`TestConnection`); `internal/ai/remote_models.go:95`
- **güven:** orta
- **açıklama:** `TestConnection`/`ListRemoteModels` provider'ın `base_url`'una outbound HTTP yapıyor. `CheckEgress` yalnızca airgapped mode açıkken private host'ları blokluyor (default deployment airgapped değil). Provider CRUD'a sahip operatör `base_url`'u internal endpoint'lere (ör. `http://169.254.169.254/…`) yönlendirip server-side request tetikleyebilir; `ListRemoteModels` yanıt gövdesini parse edip döndürüyor. Provider CRUD admin-only olduğundan authenticated-admin SSRF.
- **düzeltme:** Create/update'te provider base URL'lerine allowlist/private-IP blok uygula (veya admin-tetiklemeli probe'larda egress kontrolünü airgapped'ten bağımsız her zaman çalıştır).

### S13 — [LOW] Limit alt sınırı yok — atlanmış/sıfır limit unbounded query'ye derleniyor
- **kategori:** resource-exhaustion
- **dosya:** `internal/query/validator.go:379-404`; `internal/query/compiler_nested.go:143`
- **güven:** orta
- **açıklama:** `validateLimitOffset` negatif ve tavan-üstü limit'i reddediyor ama `Limit==0`'ı kabul ediyor. `dialect.LimitOffset(0, offset)` LIMIT clause üretmiyor, böylece atlanmış/sıfır limitli `LogicalQuery` full-table scan'e derleniyor. Yalnızca `StructuredMetricQuery` yolu 1000 default enjekte ediyor; generic compile yolunda floor yok.
- **düzeltme:** `Limit <= 0` iken validator/compiler'da default/maksimum limit floor uygula.

### S14 — [LOW] CreateConversation JSON'u body-size cap olmadan decode ediyor
- **kategori:** dos
- **dosya:** `internal/http/handlers/ai_conversations.go:45`
- **güven:** orta
- **açıklama:** `CreateConversation` paylaşılan `decodeJSON`/`MaxBytesReader` yerine doğrudan `sonic.ConfigStd.NewDecoder(r.Body).Decode` kullanıyor — intrinsic body-size limiti yok. Normal deployment'ta bağlı (dsAccess middleware 1 MiB LimitReader, proxy 1 MiB cap) ama auth kapalıyken (`authClient==nil`, dsAccess pass-through) handler sınırsız gövdeyi belleğe okur.
- **düzeltme:** Tutarlı 1 MiB cap ve tekdüze 400/413 için `decodeJSON[aiConversationRequest](w, r)` kullan.

### S15 — [LOW] Split-service router'larda kimliksiz /metrics
- **kategori:** info-leak
- **dosya:** `internal/http/catalog_router.go:35`, `query_router.go:37`, `ai_router.go:39`
- **güven:** orta
- **açıklama:** Monolith `/metrics`'i `APIKeyAuth(MetricsAPIKey)` ile gate ediyor (router.go:87) ama per-service router'lar `r.Get("/metrics", MetricsHandler)`'ı kimliksiz açıyor. Bu pod'lara ağ erişimi olan herkes internal operasyonel metrikleri (route cardinality, latency, error count) scrape edebilir. Genelde ağ-kısıtlı ama monolith ile tutarsız.
- **düzeltme:** Service `/metrics` route'larını aynı `APIKeyAuth(MetricsAPIKey)` ile sar, veya network policy bağımlılığını belgele.

### S16 — [LOW] Insecure default DB DSN TLS'i kapatıyor (sslmode=disable)
- **kategori:** insecure-default / tls
- **dosya:** `internal/config/config.go:497`
- **güven:** yüksek
- **açıklama:** `BI_METADATA_DB_DSN` default'u `postgres://localhost:5432/bi_metadata?sslmode=disable`. Default'a güvenen (veya template olarak kopyalayan) deployment metadata DB bağlantısını (encrypted datasource secret + audit verisi taşıyan) TLS'siz çalıştırır. Localhost dev default ama prod guard'ı yok.
- **düzeltme:** Dev default'u koru ama `IsProduction()` ve `sslmode=disable` iken doğrula/uyar.

### S17 — [LOW] Audit event'leri backpressure'da sessizce düşüyor + system-actor attribution kayboluyor
- **kategori:** audit-omission
- **dosya:** `internal/audit/db_writer.go:84`, :35
- **güven:** yüksek
- **açıklama:** (1) `DBWriter.Write` 1000-derinlikli kanal doluyken event'leri düşürüyor (counter++ + warn). Sürekli yük altında audit kayıtları DB'ye ulaşmıyor. (2) `toNullUUID` UUID olmayan `UserID`'yi (ör. `security.SystemPolicy()`'den `"system"`, `"internal"`) SQL NULL'a çeviriyor; system/internal aksiyonlar için DB audit satırları actor attribution kaybediyor.
- **düzeltme:** Güvenlik-kritik event tipleri için blocking/bounded-wait yol düşün; non-UUID actor'ları NULL'a atmak yerine ayrı text kolona yaz.

### S18 — [LOW] Local queue subscriber ilk handler hatasında kalıcı ölüyor
- **kategori:** dlq-drop
- **dosya:** `internal/queue/local.go:52`
- **güven:** yüksek
- **açıklama:** `LocalAIJobQueue.Subscribe` handler hata döndürür döndürmez consume loop'unu sonlandırıyor — bir başarısız job tüm tüketimi retry/DLQ olmadan öldürüyor. Dev/in-process backend (NATS backend retry/DLQ'yu doğru yapıyor) olduğundan prod etkisi sınırlı ama transient hata tüm job işlemeyi sessizce durdurur.
- **düzeltme:** Loop'tan return yerine handler hatasında log-and-continue (at-least-once niyetiyle eşleş).

### S19 — [LOW] Migration'lar geniş "already applied" SQLSTATE'leri yutup applied işaretliyor; per-file transaction yok
- **kategori:** unsafe-migration
- **dosya:** `internal/dbmigrate/migrate.go:163`, :256-267 (`isAlreadyAppliedError`), :247-254 (`execSQL`)
- **güven:** yüksek
- **açıklama:** `Up` her dosyayı transaction'sız tek `ExecContext` ile çalıştırıp, hatanın kodu `isAlreadyAppliedError`'daysa (ki `23505` unique_violation dahil) dosyayı applied kaydediyor. Unique-violation ile yarıda başarısız olan migration sessizce applied işaretlenir, retry edilmez, kısmen-migrate şema tamamlanmış görünür. Attacker-controlled değil (dosyalar trusted dizinden).
- **düzeltme:** Her dosyayı transaction içine al (BEGIN/COMMIT); yutulan kodları idempotent-DDL ile sınırla (`23505`'i çıkar).

---

## Kod Tekrarı Bulguları

### D1 — [HIGH] Env-var parse helper'ları 4 config paketinde kopya, semantikleri sapmış
- **dosya:** `internal/config/config.go:809-975` · `internal/auth/config.go:109-205` · `internal/mail/config.go:~55-90` · `internal/ai/eval/live_config.go:55-77`
- **açıklama:** Dört paket kendi "env oku + default" helper ailesini tutuyor. `positiveIntEnv`/`nonNegativeIntEnv` auth ile mail'de byte-identical. `intEnv`(auth,mail) vs `getEnvAsInt`(config) vs `envIntDefault`(eval) aynı kontrat ama invalid input'ta farklı davranıyor (config `slog.Warn`+fallback, diğerleri sessiz fallback). Bool ve CSV parse da sapmış. Bir kopyaya yapılan fix diğerlerine yayılmaz — bu tam olarak yanlış-konfigüre deployment'ın bir serviste fark edilmemesinin yolu.
- **düzeltme:** Tek `internal/platform/env` (veya `internal/env` mevcut) paketi çıkar: `String/Int/PositiveInt/NonNegativeInt/Float/Bool/Duration/CSV`, tek belgelenmiş invalid-value politikası (warn+default). Dört loader'ı buna yönlendir; yerel kopyaları sil.

### D2 — [HIGH] Rune-truncation helper'ı 6 kez tekrarlanmış (bir kopyası byte-slicing bug'ı)
- **dosya:** `internal/ai/prompt/glossary.go:324/333` · `internal/ai/run_trace.go:65` · `internal/ai/describe.go:311` · `internal/ai/prompt/prompt.go:420` · `internal/http/handlers/ai_agent_step_summary.go:241` · `internal/agent/provider_planner.go:192`
- **açıklama:** "String'i N rune'a kes + ellipsis" helper'ı 6 yerde. İki kopya byte-identical. Diğerlerinde kazara sapma: biri `<=0` guard'ı ekliyor, biri `maxRunes-3`'te kesip ASCII `"..."` ekliyor, biri limiti hardcode ediyor, `provider_planner.truncate` **byte** ile kesiyor (UTF-8'i rune ortasından bölebilir — rune-based versiyonların önlemek için var olduğu bug). Exported consolidation point zaten var: `prompt.TruncateRunes`.
- **düzeltme:** Tek implementasyon tut (opsiyonel suffix parametreli), 5 kopyayı değiştir. Planner'ın byte-budget varyantı kasıtlıysa `truncateBytes` olarak adlandır ve nedenini belgele.

### D3 — [MEDIUM] Servis entrypoint bootstrap iskeleti 9 main.go dosyasında kopya
- **dosya:** `services/{catalog,query,mcp,ai}/cmd/main.go` · `cmd/{api,agent,worker,auth,mail}/main.go`
- **açıklama:** Her servis main'i aynı ~90 satır lifecycle'ı tekrar ediyor: config load → `SetupTracing`/`SetupLogExport` (aynı deferred-shutdown blokları) → deps → router → `http.Server{timeouts}` → goroutine ListenAndServe → signal-based graceful shutdown. Yalnızca isim/deps/router/WriteTimeout değişiyor. Zaten sapmış (query WriteTimeout'u config'den türetiyor, catalog 60s hardcode; auth/mail observability'yi kendi `setupXObservability`'sinde tekrar implemente ediyor).
- **düzeltme:** `internal/app.RunHTTPService(ctx, app.ServiceSpec{...})` helper'ı çıkar (observability setup/teardown + http.Server + graceful shutdown sahibi); her main ~15 satıra düşer. Aykırılar (auth/mail/worker) kademeli benimseyebilir.

### D4 — [MEDIUM] AI settings wire struct'ı handler ile SDK arasında kopya — drift zaten olmuş
- **dosya:** `internal/http/handlers/ai_settings.go:13` (`aiRuntimeSettingsResponse`) vs `pkg/aiclient/schema.go:171` (`SettingsResponse`)
- **açıklama:** `GET /api/ai/settings` yanıt struct'ı iki kez tutuluyor (~30 alan, identical JSON tag). Zaten sapmışlar: server `deployment_mode`, `db_managed`, `active_models` ve zengin `adminAmbiguityConfig` eklemiş, SDK anonymous inline struct bunlardan yoksun — SDK tüketicileri API'nin döndürdüğü alanları göremiyor.
- **düzeltme:** `pkg/aiclient.SettingsResponse`'ı tek doğruluk kaynağı yap (handler bu tipi döndürür/embed eder). Handler çıktısının SDK tipine losslessly unmarshal olduğunu doğrulayan golden test ekle.

### D5 — [MEDIUM] Paket-başı HTTP response wrapper katmanları — verbatim kopya + uyumsuz konvansiyonlar
- **dosya:** `internal/http/handlers/helpers.go:107` (`isMaxBytesError`) vs `internal/http/response/response.go:99` (verbatim) · `internal/agent/parity/backend.go:73` (`writeJSON` re-implement) · `internal/auth/handlers/`: `handler_rbac.go:1029-1055`, `decode.go:9-25`, `handler.go:413-424` (3 paralel error-writing konvansiyonu)
- **açıklama:** `writeError` fan-in'i (586) gograph merge artifact'ıydı — iki `writeError` de shared `response` paketine delege ediyor (kopya değil). Gerçek tekrar etrafında: (1) `isMaxBytesError` helpers.go ↔ response.go byte-identical; (2) `parity.writeJSON`, `response.WriteJSON`'ı çağırmak yerine yeniden implemente ediyor (nil-slice normalizasyonu eksik); (3) auth paketi tek başına 3 paralel error-writing konvansiyonu (`respondError` / `writeError` / doğrudan `response.WriteError`) taşıyor. İmza sapması, aynı kodun handler paketleri arasında taşınmasını imkânsız kılıyor.
- **düzeltme:** `response.IsMaxBytesError` export et + helpers.go kopyasını sil; `response.ReadBody` ekle; parity fake backend'i `response.WriteJSON` çağırsın; auth handler'larında tek konvansiyona indirgen (`(w, r, status, err)` formu — 5xx'te identity/request context loglayan tek form).

### D6 — [MEDIUM] `aggregateExpr` dialect Aggregate yazımlarını tekrar ediyor
- **dosya:** `internal/query/compiler.go:636-682`
- **açıklama:** `aggregateExpr` COUNT/COUNT DISTINCT/SUM/AVG/MIN/MAX'ı inline `if c.dialect.Name()=="clickhouse"` dallarıyla yeniden implemente ediyor — `internal/dialect/base.go`'da (`AggregateStandardSQL`/`AggregateClickHouseSQL`) zaten merkezi. İkisi elle senkron tutulmalı; yeni dialect yazımı iki yerde düzenlenmeli. String-based dialect kontrolü bypass ettiği dialect abstraction'ını da tekrar ediyor.
- **düzeltme:** `aggregateExpr`'i bir dialect metoduna delege et (derlenmiş expression alan bir `Aggregate` varyantı).

### D7 — [MEDIUM] `buildContainsFilter`/`buildStartsWithFilter`/`buildEndsWithFilter` neredeyse identical
- **dosya:** `internal/query/compiler_filter.go:123-190`
- **açıklama:** Üç LIKE-builder, bağlı değerin wildcard yerleşimi (`%v%`, `v%`, `%v`) dışında byte-identical. Her biri slice/empty/scalar dallanmasını ve `likeExpression` çağrısını tekrar ediyor. Sapma riski (ör. case-sensitivity fix'i yalnızca birine).
- **düzeltme:** Bir `func(string) string` pattern-wrapper ile parametrelenen tek helper çıkar.

### D8 — [LOW] `observability.Metrics` 87 alanlı god-object (repo'nun #1'i)
- **dosya:** `internal/platform/observability/metrics.go:40` (struct ~:141'e, `NewMetrics` ~:490'a)
- **açıklama:** Tek `Metrics` struct'ı query/cache/AI/LLM/agent/feedback/HTTP domainlerini kapsayan 87 collector alanı tutuyor, ~350 satır tekrarlı `f.NewCounter(...)` bloğuyla register ediliyor. gograph_godobj #1 (CRITICAL/79). Her metrik eklemesi bu tek dosyaya dokunuyor. İlgili: `app.Dependencies` (37 alan) aynı konsantrasyon.
- **düzeltme:** Domain başına bundle'lara böl (`QueryMetrics`/`AIMetrics`/`AgentMetrics`...) `Metrics`'e compose et; servisler yalnızca emit ettikleri bundle'a bağlansın. Mekanik refactor.

### D9 — [LOW] `/api/auth` ve `/auth` için identical route-registration bloğu
- **dosya:** `cmd/auth/main.go:325-339`
- **açıklama:** İki `r.Route(...)` bloğu byte-identical (publicLimit, CSRF, `RegisterAuthRoutes`, rbac + `RegisterAccountAdminRoutes`); yalnızca prefix farklı. Auth yüzeyine gelecek değişiklik iki yerde yapılmalı; biri unutulursa iki mount noktası sessizce sapıyor (güvenlik-ilgili yüzey).
- **düzeltme:** Tek `registerAuthSurface(r chi.Router)` closure çıkar, iki prefix altına mount et.

### D10 — [LOW] Üç identical SHA-256→hex token-hash helper'ı
- **dosya:** `internal/auth/session.go:31` (`HashToken`) · `magiclink.go:103` (`hashMagicLink`) · `account_state.go:270` (`hashUnlockToken`)
- **açıklama:** Üçü de SHA-256 digest + hex encode — aynı one-way hashing primitive'inin 3 kopyası. Biri keyed hash'e çevrilirse diğerleri sapar.
- **düzeltme:** Tek exported helper'a indirge (`HashToken`'ı yeniden kullan).

### D11 — [LOW] Duplike `oauth_accounts` upsert query'si
- **dosya:** `internal/auth/repository.go:452-478` (`LinkOAuthAccount`) ve :480-561 (`CreateUserWithOAuth`)
- **açıklama:** `INSERT INTO oauth_accounts ... ON CONFLICT ... DO UPDATE` + çevresindeki token-encryption/`expiresAt` kurulumu iki fonksiyonda verbatim.
- **düzeltme:** Paylaşılan `upsertOAuthAccount(ctx, execer, ...)` helper'ı çıkar, iki yoldan çağır.

### D12 — [LOW] Copy-paste `DatasourceForX` resolver metodları (IDOR sınırını besliyor)
- **dosya:** `internal/metadata/ai_saved_queries.go:373` · `knowledge_files.go:171` · `agent_runs.go:257`
- **açıklama:** Üç metod (`DatasourceForSavedQuery`/`DatasourceForKnowledgeFile`/`DatasourceForAgentRun`) tablo adı + sentinel error dışında byte-identical (`SELECT datasource_id::text FROM <t> WHERE id=$1::uuid`). Bunlar `RequireResolvedDatasourceAccess` middleware'ini besliyor — kopyalar arası sapma (ör. `::uuid` cast'i unutmak) IDOR korumasını doğrudan zayıflatır.
- **düzeltme:** Tek private helper `datasourceForEntity(ctx, table, id, notFound)` çıkar, üçü delege etsin.

### D13 — [LOW] Per-driver DSN composition boilerplate'i
- **dosya:** `internal/datasource/dsn.go:233-345` (`composePostgresDSN`/`composeSQLServerDSN`/`composeClickHouseDSN`/`composeOracleDSN`)
- **açıklama:** Dört compose fonksiyonu aynı iskeleti tekrar ediyor (`url.URL` + `url.UserPassword` + `Host` + tek SSL query param `switch` + `mergeExtraQuery`). Credential-taşıyan connection string kurdukları için SSL flag/extra-param sapması güvenlik-tutarlılık riski.
- **düzeltme:** `composeURLStyleDSN(scheme, p, sslParam func(...))` çıkar, driver başına küçük closure geç.

### D14 — [LOW] security paketinde SQL string-literal atlama mantığı duplike
- **dosya:** `internal/security/function_blocklist.go:148` (`skipFunctionStringLiteral`) vs `readonly.go:170` (`skipStringLiteral`)
- **açıklama:** İki güvenlik parser'ı SQL literal'lerini keyword/function eşlemesinden önce aynı tokenize etmeli; iki kopya arasında sapma bir payload'ın biri tarafından literal, diğeri tarafından kod sayılmasına yol açabilir.
- **düzeltme:** Her iki checker'ın paylaştığı tek literal-skipping primitive'inde birleştir.

### D15 — [LOW] Dialect zero-value normalizasyonu `NewCompiler` ve `normalizeExprDialect`'te duplike
- **dosya:** `internal/query/compiler.go:41-61`; `expr_compiler.go:62-98`
- **açıklama:** İkisi de zero-value dialect struct'ını (boş `QuoteLeft`) paket-global instance ile değiştiren aynı type-switch'i içeriyor. `normalizeExprDialect` daha fazla dialect (SQLite/Snowflake/Databricks/Oracle) kapsıyor — zaten sapmışlar; bu dialect'ler için doğrudan kurulan `Compiler` normalizasyonu atlıyor.
- **düzeltme:** Normalizasyonu `internal/dialect`'te tek exported helper'a taşı, iki yerden çağır.

### D16 — [LOW] Case-sensitive comparison / LIKE dialect mantığı dosyalar arası duplike
- **dosya:** `internal/query/compiler.go:1590-1602` (`likeExpression`); `compiler_filter.go:43-52` (`caseSensitiveComparison`)
- **açıklama:** İkisi de aynı mysql (`BINARY`) / sqlserver (`COLLATE Latin1_General_CS_AS`) case-sensitivity için `dialect.Name()` üzerinde switch yapıyor. Dialect-specific SQL, abstraction yerine string-name karşılaştırmasıyla query paketinde ve iki kez.
- **düzeltme:** Dialect metodu ekle (`CaseSensitiveCompare`/`CaseSensitiveLike`), iki caller kullansın.

---

## Öncelik / Düzeltme Sırası Önerisi

**Batch 1 — Güvenlik (önce bunlar):**
- S1 (HIGH, hızlı: super-admin guard), S2 (HIGH, `resolveDatasourceScope` uygula), S3 (MEDIUM, middleware sar) — hepsi missing-authz/IDOR, küçük ve yüksek etkili.
- S4, S5 (IDOR/tenant-isolation), S6 (spend-cap).
- S7/S8 (SQL/readonly dialect gap'leri — dikkatli, test gerektirir).
- S9, S16 (config insecure-default — hızlı).
- Kalan LOW'lar (S10–S15, S17–S19) tek batch.

**Batch 2 — Duplikasyon (davranış-koruyan refactor):**
- D1 (env helper birleştirme) + D5 (response wrapper) — güvenlik-ilgili tutarlılık, en yüksek değer.
- D2 (truncation, byte-slicing bug'ını da düzeltir), D12 (IDOR resolver), D14 (security parser).
- D6/D7/D15/D16 (query/dialect) — tek PR olarak.
- D3/D4/D8 (bootstrap/settings/metrics) — daha büyük mekanik refactor, ayrı.
- D9/D10/D11/D13 (auth/datasource küçük) — hızlı temizlik.

**Doğrulama:** Her düzeltmede gograph_plan → düzenle → gograph_review --uncommitted; `make lint-go` + `make test-go` + ilgili yolun testleri. Auth/query değişiklikleri için `make verify-main` düşün.
