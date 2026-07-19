# Flux'u ayrı GitOps repoya taşıma + native Image Automation

## Hedef
- Ayrı bir private repo (`biqly/biqly-gitops`) oluştur; Flux **sadece** onu izlesin.
- Custom `auto-bump-prod.yml` + `scripts/helm-bump-tags.sh`'ı kaldır; yerine Flux
  **image-reflector-controller + image-automation-controller** kullan (registry'yi
  tarayıp yeni tag'i seçip gitops repoya commit'lesin).

## Mevcut durum (tespit edildi)
- Flux `flux-system` GitRepository → `ssh://git@github.com/biqly/biqly` (main), path `./clusters/prag`.
- `biqly` HelmRelease chart'ı aynı repodaki `deploy/helm/biqly`'den render ediyor; values-prod.yaml image tag'lerini CI auto-bump yazıyor.
- Emsal: `zlitter` zaten ayrı repo (`zlitter/helm`) + kendi deploy key ile izleniyor.
- Image tag formatı: `sha-<40hex>` (build-*.yml `prefix=sha-`; ci.yml api/frontend `format=long` prefix'siz — **doğrulanacak/uyumlulaştırılacak**).
- GHCR paketleri **private** → ImageRepository pull-secret gerektirir.
- `deploy/helm/biqly` umbrella chart: `file://charts/*` subchart'ları + committed `postgresql-*.tgz`; `charts/*.tgz` gitignore'lu (postgresql hariç). Flux source-controller subchart'ları kaynaktan rebuild ediyor.
- Cluster bu oturumdan kubectl ile takılıyor → cluster adımlarını kullanıcı uygular / ayrı doğrularız.

## Karar değişiklikleri (image automation'a geçince)
- **PAT gereksiz**: promotion'ı CI değil, Flux image-automation-controller yapıyor; git write-back için gitops repo deploy key'i **read-write** olmalı (bootstrap default read-only).
- **Tag şeması değişmeli**: `sha-<hex>` sıralanamaz. Yeni tag = `<YYYYMMDDHHmmss>-<sha>` (numerical order). Geçişte eski `sha-<hex>` tag'i de yayınlanmaya devam (dual-tag).

## Yeni repo yapısı: `biqly/biqly-gitops` (private)
```
README.md
clusters/prag/
  flux-system/              # gotk-components (+ image-reflector, image-automation), gotk-sync -> gitops repo
  biqly.yaml                # HelmRelease (chart path deploy/helm/biqly, source: flux-system)
  zlitter.yaml              # tasindi
  zlitter-gitrepository.yaml
  image-automation/
    imagerepositories.yaml  # her pinlenen image icin (values-prod'daki image.repository'lerden turetilir)
    imagepolicies.yaml      # numerical order, timestamp extract
    imageupdateautomation.yaml  # gitops repoya commit
deploy/helm/biqly/          # tam chart + values.yaml + values-prod.yaml ($imagepolicy setter marker'lari ile)
```

## Asamalar

### Faz 0 - Icerik hazirligi (guvenli, cluster'a dokunmaz)  [TAMAM]
- [x] values-prod.yaml'daki tum image.repository -> servis eslemesini kesinlestir (9 image: auth/mail/frontend/catalog/query/worker/agent/mcp/ai; auth+mail migrate ayni image'i paylasir).
- [x] Yeni repo agacini scratchpad'de kur (chart + clusters kopyasi).
- [x] values-prod.yaml'a her tag satirina marker ekle (11 tag -> 9 policy).
- [x] ImageRepository/ImagePolicy/ImageUpdateAutomation manifest'lerini yaz.
- [x] gotk-sync.yaml url'ini gitops repoya cevir (ssh://git@github.com/biqly/biqly-gitops); helm template dogrulandi (22 image ref render).

### Faz 1 - Repo olustur (gh) & push  [TAMAM]
- [x] https://github.com/biqly/biqly-gitops olusturuldu (private, 139 dosya push edildi). Sadece postgresql tgz commit'li; 9 subchart source dir mevcut.

### Faz 2 - Monorepo CI degisikligi  [KOD HAZIR, commit bekliyor]
- [x] 9 build-*.yml + ci.yml (api+frontend): siralanabilir tag eklendi
      (type=raw,value={{date 'YYYYMMDDHHmmss'}}-{{sha}},enable={{is_default_branch}}); sha- ve latest korundu.
- [x] auto-bump-prod.yml + scripts/helm-bump-tags.sh silindi.
- [x] Makefile: helm-bump-tags target + .PHONY girisi kaldirildi; helm-upgrade-prod artik helm-deps'e bagli (manuel fallback bozulmadi).
- [x] ci.yml baslik yorumu "auto-bump" -> "Flux image automation" guncellendi.
- [ ] DEFER (Faz 3 sonrasi): monorepodan clusters/ + deploy/helm/biqly kaldir.
      DIKKAT: mevcut Flux hala monorepoyu izliyor; Faz 3 (re-point) ONCESINDE
      silinirse prune ile prod silinir. Kesinlikle Faz 3'ten sonra.
- [ ] DEFER: deploy-prod skill dokumanlari (.claude/.codex/.cursor/SKILL.md) eski akisi anlatiyor -> guncellenmeli.

NOT: Bu degisiklikler dev branch'inde; main'e merge edilene kadar (deploy-prod)
etkin degil. Ideal siralama: Faz 2 merge ile Faz 3 (cluster) birlikte yapilmali,
yoksa main'de auto-bump yok + Flux henuz yeni repoyu izlemiyor -> yeni push'lar
deploy olmaz (mevcut pinned tag'ler calismaya devam eder, kesinti yok).

### Faz 3 - Cluster cutover  [TAMAM - 2026-07-19]
- [x] flux bootstrap github (biqly-gitops, path clusters/prag, --read-write-key, --components-extra=image-reflector,image-automation). image controller'lar kuruldu.
- [x] Deploy key: bootstrap mevcut key'i reuse etti; key biqly/biqly'de tekildi -> oradan silinip biqly-gitops'a write-access ile eklendi.
- [x] ghcr-auth (dockerconfigjson) flux-system'de olusturuldu; 10 ImageRepository GHCR'i basariyla tariyor.
- [x] apiVersion fix: flux image API'leri v1 serve ediyor (v1beta2 degil) -> manifest'ler v1'e cevrildi (commit ac0746f).
- [x] Cutover DOGRULANDI: GitRepository->biqly-gitops READY, Kustomization READY (ac0746f), HelmRelease biqly v173 upgrade succeeded, tum pod'lar Running (restart YOK = no-op), gateway HTTP 200.
- NOT: ImagePolicy'ler su an NotReady (LATEST=<none>): henuz <timestamp>-<sha> tag'li image yok. dev->main merge + CI sonrasi cozulur; ilk otomatik promotion o zaman.

### Faz 4 - Kalan (KARAR/ADIM bekliyor)
- [ ] biqly/biqly dev->main merge -> CI timestamp'li image yayinlar -> ImagePolicy cozulur -> ilk Flux otomatik promotion.
- [ ] Monorepo temizligi (clusters/ + deploy/helm/biqly): ARTIK GUVENLI (Flux biqly/biqly izlemiyor). AMA local dev (make helm-upgrade-prod/values-dev) chart'a bagimli -> silmeden once local-dev stratejisine karar ver.
- [ ] deploy-prod SKILL.md "trigger phrase = go-ahead" ifadesi (guvenlik taramasi isaretledi) -> onay kapisi eklensin mi?

### Faz 2b - ArgoCD kalintilarinin temizligi  [TAMAM]
- [x] deploy/helm/biqly/.argocd-source-biqly.yaml silindi (monorepo + gitops).
- [x] Workflow path-filter'lari (.argocd-source-*.yaml) kaldirildi: build-migrate, semgrep(x2), codeql(x2).
- [x] cloudflared config.yaml argocd.il1.nl route + README mention kaldirildi.
- [x] Bayat yorumlar Flux'a guncellendi: configmaps.yaml, values-prod.yaml (mail), migrate-job.yaml, .env.example.
- [ ] Kalan (bilincli): clusters/prag/biqly.yaml force:true yorumu (argocd-controller stale field-manager) — operasyonel olarak hala gecerli, birakildi.
- [ ] Kalan (dusuk oncelik): docs/AGENTS.md/CLAUDE.md/skills icindeki argocd anlatimlari.

### Faz 2c - migrate image'i Flux-native promotion  [TAMAM]
- Sebep: yeni <timestamp>-<sha> tag semasi, migrate-job'un catalog-lockstep'ini (tag kopyalama) imkansiz kilar (timestamp'ler image basina farkli).
- [x] biqly-migrate ImageRepository + ImagePolicy eklendi (gitops).
- [x] values-prod global.migrate.image.tag + $imagepolicy marker (12. marker) eklendi.
- [x] migrate-job.yaml lockstep dormant kalir (sha- legacy fallback); yorum argocd'den arindirildi.
- Dogrulama: gitops chart render OK, 10 IR + 10 IP, 12 marker.

### Faz 2d - ci.yml build duplikasyonu  [TAMAM]
- [x] docker-api job'u silindi (olu ghcr.io/biqly/api image; api != query farkli binary, hicbir yerde deploy edilmiyor).
- [x] docker-frontend korundu -> frontend image'i normal build alinmaya devam ediyor.
- Kalan job'lar: backend, lint, frontend, docker-frontend. (backend/lint test.yml ile ortusme bilincli birakildi.)

## Basari kriteri
- Flux yalniz biqly-gitops'u izliyor; monorepoya push image automation disinda deploy tetiklemiyor.
- Yeni image push'u -> ImagePolicy secer -> gitops repoya otomatik commit -> HelmRelease yeni tag'e rollout.
- auto-bump-prod.yml/helm-bump-tags.sh yok.

## Review
(implementasyon sonrasi doldurulacak)
