# 05 — DevOps: Helm / Config Yönetimi

**Öncelik: §1 = P0 (güvenlik), §2–§4 = P2**

## Amaç

Helm chart'ı runtime-config mimarisiyle hizalamak; admin anahtarı sızıntısını kapatmak;
prod observability ayarlarını düzeltmek.

## Mevcut Durum (doğrulandı, 2026-06-10)

| Bileşen | Durum | Konum |
| --- | --- | --- |
| AI config akışı: `global.config` + `ai.config` + `ambiguity.*` → ConfigMap → `envFrom` | ✅ Düzenli | `charts/ai/templates/configmap.yaml`, `deployment.yaml` |
| `frontend.runtimeAdminKey` | 🔴 **Güvenlik sorunu**: init container `BI_ADMIN_API_KEY`'i `env-config.js` → `window.__BIQLY_ENV__.adminApiKey` olarak tarayıcıya veriyor. values-dev'de `enabled: true` | `charts/frontend/templates/deployment.yaml:45-83` |
| Migration: PreSync Job, sync-wave -10, catalog SHA lockstep | ✅ Sağlam — seed/init-container'a gerek yok | `templates/migrate-job.yaml` |
| Prod tracing | ⚠️ `always_on` + `samplerArg: "1"` = **%100 sampling** (dev %25 ratio) | `values-prod.yaml` |
| Prod Prometheus | ⚠️ `serviceMonitor.enabled: false`, `prometheusRules.enabled: false` prod'da — Tier-1/2 metrikleri prod'da scrape edilmiyor | `values-prod.yaml` |
| Secrets | ✅ Prod `createSecrets: false` (önceden oluşturulmuş); AI provider API key'leri DB'de AES şifreli | `templates/secrets.yaml` |

## Yapılacaklar

### 1. 🔴 P0 — Admin anahtarının tarayıcıya sızdırılmasını kaldır

Backend zaten JWT super_admin'i kabul ediyor (`admin_middleware.go`) ve 02 §3 ile `ai:settings`
RBAC izni ekleniyor. Frontend'in paylaşılan anahtara ihtiyacı kalmıyor.

1. Frontend `apiClient.ts`'teki `BI_ADMIN_API_KEY` / `window.__BIQLY_ENV__.adminApiKey`
   fallback'ini kaldır; admin çağrıları yalnızca JWT bearer ile gitsin (02 §3 merge edildikten sonra).
2. `charts/frontend/templates/deployment.yaml`'dan `runtimeAdminKey` init-container bloğunu ve
   `values*.yaml`'dan `frontend.runtimeAdminKey.*` anahtarlarını kaldır; nginx config'ten
   `/env-config.js` istisnasını temizle.
3. `BI_ADMIN_API_KEY` secret'ı **kalır** — yalnızca makine-makine/operasyon kullanımı için
   (curl, CI). README/runbook'a not düş.
4. Yayınlanmış anahtar tarayıcılara servis edildiği için **rotate et**: `biqly-security`
   secret'ında `BI_ADMIN_API_KEY` değerini yenile, AI pod'larını rollout et.
5. Doğrulama: prod'da `curl https://abi.il1.nl/env-config.js` → 404; admin paneli JWT ile çalışıyor.

### 2. Runtime-config / Helm hizalaması

1. DB'ye taşınan anahtarlar (02'deki `ambiguity` ek alanları, `pii.detection_threshold`,
   `pii.default_masking_strategy`, `memory.*`) için values.yaml **default'ları kalır**
   (fallback). values.yaml'a yorum: `# DB runtime-config bunun üstüne yazar — docs/configuration.md`.
2. Seed mekanizması **eklenmeyecek** (karar): runtime config "override yoksa env fallback"
   modeliyle çalışıyor; boş tablo = env davranışı. Init-container seed gereksiz karmaşıklık.
3. UI'dan yönetilmeyecekler env-only kalır: `BI_AI_RATE_LIMIT_PER_MINUTE`,
   `BI_AI_MAX_PROMPT_RUNES`, `BI_AI_JOBS_ENABLED`, timeout'lar, `BI_AI_TRANSLATION_*`,
   `BI_PII_ENABLED` (master kill-switch), tüm secret'lar.

### 3. Prod observability düzeltmeleri

1. `values-prod.yaml` tracing: `sampler: parentbased_traceidratio`, `samplerArg: "0.1"`
   (%10; trafik artarsa düşürülebilir). `always_on` yalnızca dev/debug.
2. Prod'da metrik scrape stratejisine karar ver ve uygula — iki seçenekten biri:
   a) Prod cluster'a Prometheus varsa `serviceMonitor.enabled: true` + `prometheusRules.enabled: true`;
   b) Yoksa mevcut uzak OTEL collector'a metrics pipeline ekle (OTLP metrics).
   Mevcut durumda Tier-1/2 metrikleri prod'da hiçbir yere gitmiyor — 03'teki işin prod değeri
   bu madde kapatılmadan sıfır.
3. Alarm kuralları (`prometheusRules`): LLM error rate, ambiguity round-cap oranı,
   recall-miss oranı, NATS DLQ moves için eşikler tanımla (03'teki metrik adlarıyla).

### 4. Doğrulama prosedürü (her helm değişikliği için)

1. `helm lint deploy/helm/biqly -f deploy/helm/biqly/values-prod.yaml`
2. `helm template ... | kubectl diff -f -` ile staging'e karşı fark incele.
3. Dev'e uygula: `helm upgrade --install biqly deploy/helm/biqly -n biqly -f values-dev.yaml`;
   `kubectl -n biqly get pods` + migrate job loglarında yeni migration'ların uygulandığını gör.
4. ArgoCD sync sonrası `argocd app diff` temiz; image-updater commit'leriyle çakışma yok
   (yalnızca `deploy/**` değişiklikleri ci.yml'i tetiklemez — beklenen).

## Kabul Kriterleri

- [ ] `env-config.js` hiçbir ortamda servis edilmiyor; admin paneli yalnız JWT ile çalışıyor.
- [ ] `BI_ADMIN_API_KEY` rotate edildi.
- [ ] Prod trace sampling oranlı (≤ %10); Jaeger/collector yükü düştü.
- [ ] Prod'da `biqly_*` metrikleri bir backend'e akıyor ve Grafana çiziyor.
- [ ] `helm lint` + `kubectl diff` temiz; dev rollout sorunsuz.

## İlgili Dosyalar

- `deploy/helm/biqly/values.yaml`, `values-dev.yaml`, `values-prod.yaml`
- `deploy/helm/biqly/charts/frontend/templates/deployment.yaml`, `config/default.conf`
- `deploy/helm/biqly/charts/ai/templates/configmap.yaml`
- `deploy/helm/biqly/templates/{configmaps,secrets,servicemonitors,grafana-dashboards,migrate-job}.yaml`
- `frontend/src/api/apiClient.ts`
