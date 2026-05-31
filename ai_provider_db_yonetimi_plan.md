# AI Provider / Model DB Yönetimi — Uygulama Planı

> Amaç: AI provider ve model konfigürasyonlarını environment variable'lardan çıkarıp veritabanında yönetmek. Kullanıcı admin panel'den OpenAI, Anthropic veya Ollama (OpenAI-compatible) provider'ları ekleyip model seçebilmeli. API key'ler AES ile encrypt edilmeli.
>
> Tarih: 2026-05-31

---

## 1. Mevcut Durum

Tüm AI konfigürasyonu `BI_AI_*` environment variable'larından okunuyor (`internal/config/config.go`):

```text
BI_AI_PROVIDER=openai|openai-compatible|anthropic
BI_AI_API_KEY=sk-...
BI_AI_BASE_URL=http://localhost:11434/v1
BI_AI_MODEL=gpt-4o
BI_AI_TEMPERATURE=0.0
BI_AI_MAX_TOKENS=4096
```

Ayrıca NL→LogicalQuery path'i için ayrı override var:

```text
BI_AI_QUERY_PROVIDER=
BI_AI_QUERY_MODEL=
BI_AI_QUERY_BASE_URL=
BI_AI_QUERY_API_KEY=
```

**Sorunlar:**

- Yeni provider eklemek için deploy/restart gerekiyor
- Birden fazla model arasında switch yapılamıyor
- API key'ler env'de plain text
- Kullanıcı self-service olarak model değiştiremiyor
- Ollama + OpenAI aynı anda kullanılamıyor (sadece bir tane aktif)

---

## 2. Hedef Mimari

```text
DB (ai_providers + ai_models)
  ↓
ProviderRegistry (in-memory cache, hot-reload)
  ↓
Service.ProcessQuestion()
  ↓
Active Provider/Model → Provider interface → LLM call
```

Admin panel'den:

1. Provider ekle (OpenAI, Anthropic, Ollama)
2. Her provider'a model ekle (gpt-4o, claude-sonnet-4-20250514, llama3.1 vb.)
3. API key gir (AES encrypted)
4. Aktif model seç (query, describe, embedding, translation için ayrı ayrı)

---

## 3. Veritabanı Şeması

### 3.1 `ai_providers` Tablosu

```sql
CREATE TABLE ai_providers (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    provider_type TEXT NOT NULL CHECK (provider_type IN ('openai', 'openai-compatible', 'anthropic')),
    base_url TEXT NOT NULL,
    api_key_encrypted TEXT,
    is_active BOOLEAN NOT NULL DEFAULT true,
    http_timeout_seconds INT NOT NULL DEFAULT 120,
    rate_limit_per_minute INT DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (name)
);

CREATE INDEX idx_ai_providers_type ON ai_providers(provider_type);
CREATE INDEX idx_ai_providers_active ON ai_providers(is_active);
```

**Not:** `api_key_encrypted` AES ile encrypt edilmiş. Mevcut `security.Encryption` kullanılacak.
Ollama gibi keyless provider'larda `api_key_encrypted = NULL`.

### 3.2 `ai_models` Tablosu

```sql
CREATE TABLE ai_models (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_id UUID NOT NULL REFERENCES ai_providers(id) ON DELETE CASCADE,
    model_id TEXT NOT NULL,
    display_name TEXT NOT NULL,
    purpose TEXT NOT NULL CHECK (purpose IN (
        'query',
        'describe',
        'embedding',
        'translation',
        'judge'
    )),
    max_tokens INT NOT NULL DEFAULT 4096,
    temperature DOUBLE PRECISION NOT NULL DEFAULT 0.0,
    top_p DOUBLE PRECISION DEFAULT 0.0,
    num_ctx INT DEFAULT 0,
    max_prompt_input_runes INT DEFAULT 80000,
    is_default BOOLEAN NOT NULL DEFAULT false,
    is_active BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider_id, model_id, purpose)
);

CREATE INDEX idx_ai_models_purpose ON ai_models(purpose);
CREATE INDEX idx_ai_models_default ON ai_models(purpose, is_default) WHERE is_default = true;
```

**Purpose açıklamaları:**

- `query` — NL → LogicalQuery üretimi
- `describe` — Tablo/kolon AI açıklama üretimi
- `embedding` — Tablo/kolon embedding üretimi
- `translation` — Açıklama çeviri
- `judge` — Eval judge (LLM-assisted eval)

Her purpose için en fazla 1 `is_default = true` model olabilir.

---

## 4. Backend Go Değişiklikleri

### 4.1 Yeni Dosyalar

#### `internal/ai/provider_store.go`

```go
type ProviderStore struct {
    db        *pgxpool.Pool
    encryptor *security.Encryption
    cache     sync.RWMutex
    providers map[string]*ResolvedProvider  // purpose → resolved config
}

type ResolvedProvider struct {
    ProviderType string
    BaseURL      string
    APIKey       string
    ModelID      string
    MaxTokens    int
    Temperature  float64
    TopP         float64
    NumCtx       int
    MaxPromptRunes int
}

func (s *ProviderStore) ResolveForPurpose(ctx context.Context, purpose string) (*ResolvedProvider, error)
func (s *ProviderStore) RefreshCache(ctx context.Context) error
func (s *ProviderStore) CreateProvider(ctx context.Context, input CreateProviderInput) (string, error)
func (s *ProviderStore) UpdateProvider(ctx context.Context, id string, input UpdateProviderInput) error
func (s *ProviderStore) DeleteProvider(ctx context.Context, id string) error
func (s *ProviderStore) ListProviders(ctx context.Context) ([]ProviderRow, error)
func (s *ProviderStore) CreateModel(ctx context.Context, input CreateModelInput) (string, error)
func (s *ProviderStore) UpdateModel(ctx context.Context, id string, input UpdateModelInput) error
func (s *ProviderStore) DeleteModel(ctx context.Context, id string) error
func (s *ProviderStore) ListModels(ctx context.Context, providerID string) ([]ModelRow, error)
func (s *ProviderStore) SetDefaultModel(ctx context.Context, modelID, purpose string) error
```

**Cache stratejisi:**

- Startup'ta tüm default modelleri yükle
- `/api/ai/providers` endpoint'leri cache'i invalidate eder
- `RefreshCache()` her call'da değil, invalidate sonrası çağrılır
- Fallback: DB'de hiç kayıt yoksa env variable'lardan oku (backward compatibility)

### 4.2 Değişecek Dosyalar

#### `internal/config/config.go`

- `AIConfig`'e `DBManaged bool` alanı ekle
- `Load()` sırasında `BI_AI_DB_MANAGED=true` ise env fallback olarak kalsın, gerçek değer DB'den gelecek
- `AIConfig`'in mevcut tüm alanları backward-compatible kalacak

#### `internal/app/ai_dependencies.go`

```go
// Mevcut:
aiClient, err := ai.NewProvider(cfg.AI)

// Yeni:
var providerStore *ai.ProviderStore
if cfg.AI.DBManaged {
    providerStore = ai.NewProviderStore(db, encryptor)
    if err := providerStore.RefreshCache(ctx); err != nil {
        slog.Warn("provider cache refresh failed, using env fallback", "error", err)
    }
}

aiClient := resolveProvider(ctx, cfg.AI, providerStore, "query")
```

`resolveProvider()` fonksiyonu:

1. `providerStore`'dan purpose="query" için default model'i resolve et
2. Bulamazsa env variable'lardan fallback yap
3. `ai.NewProvider(resolvedConfig)` ile Provider oluştur

#### `internal/ai/service.go`

- `Service` struct'ına `providerStore *ProviderStore` ekle
- Her `ProcessQuestion()` call'unda aktif model'i resolve et (cache'den, hot-reload)
- `DescribeService`, `TranslationService`, `EmbedMetadataService` için aynı pattern

#### `internal/ai/client.go` ve `internal/ai/anthropic.go`

- Değişiklik yok. `NewClient(cfg)` ve `NewAnthropicProvider(cfg)` zaten `AIConfig` alıyor.
- `ProviderStore` resolve sonrası `AIConfig` üretip bu fonksiyonlara verecek.

### 4.3 Yeni HTTP Handler

#### `internal/http/handlers/ai_providers.go`

```text
POST   /api/ai/providers          — Provider ekle
GET    /api/ai/providers          — Provider'ları listele
GET    /api/ai/providers/{id}     — Provider detay (API key masked)
PUT    /api/ai/providers/{id}     — Provider güncelle
DELETE /api/ai/providers/{id}     — Provider sil (cascade: modelleri de silinir)

POST   /api/ai/providers/{id}/test — Bağlantı test (ping model)

POST   /api/ai/models             — Model ekle
GET    /api/ai/models             — Modelleri listele (?purpose=query)
GET    /api/ai/models/{id}        — Model detay
PUT    /api/ai/models/{id}        — Model güncelle
DELETE /api/ai/models/{id}        — Model sil
POST   /api/ai/models/{id}/default — Varsayılan yap (purpose bazlı)
```

**`/test` endpoint mantığı:**

1. Provider'dan base_url, api_key, model_id al
2. Küçük bir prompt gönder: `"Respond with OK"`
3. Yanıt geldiyse `{"status": "connected", "latency_ms": 230}`
4. Hata: `{"status": "error", "message": "401 Unauthorized"}`

### 4.4 API Key Güvenliği

- API key'ler DB'ye yazılırken mevcut `security.Encryption.Encrypt()` ile AES encrypt edilecek
- Okunurken `Decrypt()` ile plain text'e çevrilecek
- API response'da API key her zaman masked: `sk-...7k2d`
- Sadece admin rolü bu endpoint'lere erişebilir (`RequireAdmin` middleware)

---

## 5. Frontend Değişiklikleri

### 5.1 Admin Panel — AI Provider Yönetimi

#### `admin/AIProvidersPanel.tsx`

```text
┌─────────────────────────────────────────────────────────┐
│  AI Providers                                            │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌──────────────────┐  ┌──────────────────┐             │
│  │ OpenAI           │  │ Anthropic        │             │
│  │ api.openai.com   │  │ api.anthropic.com│             │
│  │ ● Active         │  │ ● Active         │             │
│  │ 3 models         │  │ 2 models         │             │
│  └──────────────────┘  └──────────────────┘             │
│  ┌──────────────────┐                                   │
│  │ Ollama (Local)   │  [+ Add Provider]                │
│  │ localhost:11434  │                                   │
│  │ ● Active         │                                   │
│  │ 1 model          │                                   │
│  └──────────────────┘                                   │
│                                                          │
├─────────────────────────────────────────────────────────┤
│  Active Models by Purpose                                │
├─────────────────────────────────────────────────────────┤
│  Query:       gpt-4o (OpenAI)         [Change]          │
│  Describe:    llama3.1 (Ollama)       [Change]          │
│  Embedding:   text-embedding-3-small (OpenAI) [Change]  │
│  Translation: gpt-4o-mini (OpenAI)    [Change]          │
│  Judge:       claude-sonnet-4-20250514 (Anthropic) [Change] │
└─────────────────────────────────────────────────────────┘
```

#### Provider Ekleme Modal

```text
┌─────────────────────────────────────┐
│  Add AI Provider                     │
├─────────────────────────────────────┤
│  Name:     [                    ]    │
│  Type:     [OpenAI ▾]                │
│            - OpenAI                  │
│            - OpenAI-Compatible        │
│            - Anthropic               │
│  Base URL: [https://api.openai.com/v1]│
│  API Key:  [••••••••••••••••]  👁    │
│                                      │
│  [Test Connection]  ✅ Connected (230ms)
│                                      │
│  [Cancel]           [Add Provider]   │
└─────────────────────────────────────┘
```

Type seçilince Base URL otomatik doldurulacak:

- `OpenAI` → `https://api.openai.com/v1`
- `Anthropic` → `https://api.anthropic.com/v1`
- `OpenAI-Compatible` → boş (kullanıcı girer)

API Key göster/gizle toggle. Test butonu ile bağlantı doğrulama.

#### Model Ekleme Modal

```text
┌─────────────────────────────────────┐
│  Add Model                           │
├─────────────────────────────────────┤
│  Provider:  [OpenAI ▾]               │
│  Model ID:  [gpt-4o            ]    │
│  Display:   [GPT-4o             ]    │
│  Purpose:   [Query ▾]                │
│             - Query (NL→SQL)         │
│             - Describe (metadata)     │
│             - Embedding (vectors)     │
│             - Translation             │
│             - Judge (eval)            │
│  Max Tokens: [4096]                  │
│  Temperature:[0.0 ]                  │
│  Max Prompt: [80000] runes           │
│  □ Set as default for this purpose   │
│                                      │
│  [Cancel]           [Add Model]      │
└─────────────────────────────────────┘
```

### 5.2 Mevcut Sayfaların Güncellenmesi

- `/ai-query` routing panel'deki "Runtime Settings" read-only gösterim → aktif model adı + provider adı göster
- `/settings` sayfasına "AI Configuration" bölümü ekle → provider yönetimine link
- Admin sidebar'a "AI Providers" link ekle

---

## 6. Geçiş Stratejisi (Migration)

### Phase 1 — DB + Env Fallback

1. Migration: `ai_providers` ve `ai_models` tablolarını oluştur
2. `ProviderStore` implementasyonu
3. `BI_AI_DB_MANAGED=true` aktifken DB'den oku, yoksa env fallback
4. HTTP handler'ları ekle
5. Frontend admin panel

### Phase 2 — Env → DB Seed

1. Startup sırasında DB boşsa, env variable'lardan otomatik seed:

   ```text
   BI_AI_PROVIDER + BI_AI_API_KEY + BI_AI_BASE_URL + BI_AI_MODEL
     → INSERT INTO ai_providers + ai_models
   ```

2. Seed sonrası artık DB yönetimde
3. `BI_AI_DB_MANAGED` default `true` olsun, `false` ile eski davranışa dönülebilir

### Phase 3 — Provider/Model Switch

1. Admin panel'den aktif model değiştir
2. `ProviderStore.RefreshCache()` çağrılır
3. Sonraki `ProcessQuestion()` call'u yeni modeli kullanır
4. Restart gerektirmez

---

## 7. Uygulama Checklist

### Backend

- [ ] Migration: `ai_providers` + `ai_models` tabloları
- [ ] `internal/ai/provider_store.go` — ProviderStore implementasyonu
- [ ] `internal/ai/provider_store_test.go` — Unit testler
- [ ] `internal/config/config.go` — `DBManaged` alanı + fallback logic
- [ ] `internal/app/ai_dependencies.go` — ProviderStore wiring + resolve logic
- [ ] `internal/app/dependencies.go` — Monolith wiring (aynı pattern)
- [ ] `internal/ai/service.go` — ProviderStore entegrasyonu (per-call resolve)
- [ ] `internal/ai/describe.go` — ProviderStore'dan describe model resolve
- [ ] `internal/ai/embed_metadata.go` — ProviderStore'dan embedding model resolve
- [ ] `internal/ai/translation.go` — ProviderStore'dan translation model resolve
- [ ] `internal/http/handlers/ai_providers.go` — CRUD + test endpoint
- [ ] `internal/http/handlers/ai_providers_test.go` — Handler testler
- [ ] `internal/http/router.go` — Route kayıtları
- [ ] Env → DB seed logic (auto-migration)

### Frontend

- [ ] `admin/AIProvidersPanel.tsx` — Provider listesi + kart grid
- [ ] `admin/AddProviderModal.tsx` — Provider ekleme formu
- [ ] `admin/AddModelModal.tsx` — Model ekleme formu
- [ ] `admin/ActiveModelsPanel.tsx` — Purpose bazlı aktif model görünümü
- [ ] `hooks/useAIProviders.ts` — Provider/model API hook'u
- [ ] Sidebar'a "AI Providers" link ekle
- [ ] `/ai-query` routing panel güncelleme (aktif model göster)
- [ ] `/settings` sayfasına AI config link

---

## 8. Güvenlik Kuralları

| Kural | Uygulama |
| --- | --- |
| API key encryption | `security.Encryption.Encrypt()` ile AES-256-GCM |
| API key masking | Response'da `sk-...7k2d` formatında |
| Admin-only access | `RequireAdmin` middleware |
| Provider test auth | Test endpoint'inde API key doğrulama |
| No env leak | Log'larda API key hiçbir zaman plain text görünmez |
| Fallback safety | DB yönetim kapalıysa env variable'lar kullanılır, restart-free |
| Connection validation | Provider ekleme/güncelleme sırasında test zorunlu olmasın ama opsiyonel sunulsun |
