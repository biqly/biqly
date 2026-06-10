# 07 — Entegrasyon ve Test Planı

**Öncelik: Sürekli** — her iş paketinin (01–05) kabul kapısı; ayrıca paket-üstü doğrulamalar.

## Amaç

01–05'teki değişikliklerin regresyonsuz devreye alınması; kabul koşullarının makineyle
doğrulanabilir olması.

## Test Matrisi (iş paketi → kapı)

| Paket | Birim/Entegrasyon | Komutlar |
| --- | --- | --- |
| 01 Ambiguity | embedding cache, temporal post-check cap, çok-tur e2e, Tier-3 HTTP, recall dedup | `make test-go`, `make eval-regression` |
| 02 Runtime config | effective-config overlay (DB>env>default), 400 validasyonları, ≤30 sn yayılım, RBAC 403, audit satırı | `make test-go` + entegrasyon testi |
| 03 Observability | token split counter testi, Record* call-site varlığı, span zinciri | `make test-go` + manuel Jaeger/Grafana doğrulaması |
| 04 Frontend | useRuntimeConfig, PlatformSettingsPanel, ClarificationCard, ConfirmedQueriesPanel testleri | `make check-frontend` |
| 05 Helm | lint + template diff + dev rollout + env-config.js 404 | `helm lint`, `kubectl diff` |

## Yapılacaklar

### 1. Backend test standartları

1. Her yeni handler/effective-config fonksiyonu için tablo-testi (Go 1.26 idiomları:
   `errors.Is/AsType`, `for i := range n`).
2. Entegrasyon testi — runtime config yayılımı: PUT → cache TTL içinde
   `effectiveAmbiguityConfig` yeni değeri döner (saat mock'lanarak TTL sınanır; gerçek 30 sn
   bekleme yok).
3. Eval: ambiguity/temporal değişiklikleri golden case'lere yansıtılır;
   `make eval-regression` commit kapısı (stub provider). `make eval-live` **commit öncesi
   çalıştırılmaz** (nightly).
4. Race: `make test-go` zaten `-race`; cache/TTL kodu için eşzamanlı erişim testi ekle
   (embedding cache + runtime-config cache invalidation).

### 2. Frontend test standartları

1. Vitest + Testing Library; her yeni bileşen/hook PR'ında test dosyası zorunlu (04 §4 listesi).
2. Kapsanacak kritik akışlar: save→toast, hatalı input→alan hatası, izinsiz→salt-okunur,
   ClarificationCard tur göstergesi, GenerationTrace `defaultOpen`.
3. Playwright kararı: bu fazda **eklenmeyecek** (kurulum + CI maliyeti, tek geliştirici).
   Şart oluşursa (çoklu tarayıcı regresyonu, auth akışı kırılganlaşırsa) ayrı RFC.

### 3. DevOps doğrulama akışı

1. Sıra: `helm lint` → `helm template | kubectl diff` → dev rollout → migrate job logu →
   smoke (AI sorgusu at, clarification akışı çalışıyor mu) → prod sync (ArgoCD).
2. Runtime config tablosunun davranış doğrulaması: dev'de PUT ile değer yaz →
   pod restart etmeden AI sorgusunda etkisini gör → satırı sil → env fallback'e dönüş.
3. Rollback provası: `helm rollback` sonrası eski pod'ların yeni `ai_runtime_config`
   satırlarıyla çalışabildiğini doğrula (bilinmeyen key'ler yok sayılmalı — forward-compatible).

### 4. Gözlemlenebilirlik doğrulaması (03 + 05 sonrası)

1. Grafana: LLM errors, token split, ambiguity rounds, recall miss, NATS DLQ panelleri
   gerçek veri çiziyor (dev'de yapay yük: 20–30 AI sorgusu, birkaçı bilinçli belirsiz).
2. Alarm provası: eşik geçici düşürülerek bir kuralın fire ettiği görülür, sonra geri alınır.
3. Trace: tek sorgunun Jaeger'da kesintisiz span zinciri (03 kabul kriteriyle aynı).

### 5. Sürümleme / devreye alma sırası

1. 03 (metrik kablolama) → küçük, bağımsız, hemen merge.
2. 05 §1 (admin key kaldırma) → 02 §3 (RBAC) ile **aynı release'te** (frontend anahtar
   fallback'i kalkmadan RBAC hazır olmalı; sıralama: backend RBAC merge → frontend fallback
   kaldır → helm'den runtimeAdminKey kaldır → anahtar rotate).
3. 02 (config domain'leri) → 04 (panel) → 06 (tablo) → 01 (iyileştirmeler) herhangi bir sırada.

## Kabul Kriterleri

- [ ] `make precommit` (lint+test) + `make eval-regression` tüm paketlerde yeşil.
- [ ] Runtime config uçtan uca: UI'dan kaydet → ≤30 sn'de sorgu davranışı değişti → satır
      silinince env fallback (dev cluster'da manuel kanıt + entegrasyon testi).
- [ ] Rollback provası başarılı (eski imaj + yeni tablo içeriği uyumlu).
- [ ] Grafana panelleri ve en az bir alarm kuralı fiilen doğrulanmış.
- [ ] Admin-key sızıntısı kapanışı: env-config.js 404 + rotate edilmiş anahtar + JWT'li panel.

## Komut Referansı

```bash
# go
gofmt -w <files> && make lint-go && make test-go
deadcode -test $(go list ./... | grep -v '/frontend')
make eval-regression            # AI eval dokunulduysa

# frontend
make check-frontend             # lint + format:check + knip + test + build

# helm
helm lint deploy/helm/biqly -f deploy/helm/biqly/values-prod.yaml
helm template biqly deploy/helm/biqly -f deploy/helm/biqly/values-dev.yaml | kubectl diff -f -
helm upgrade --install biqly deploy/helm/biqly -n biqly -f deploy/helm/biqly/values-dev.yaml
```
