# Argo CD Image Updater (biqly)

Credentials **never** live in this repository. Create the git write secret in the cluster only.

## Prerequisites

- `kubectl` context pointing at the Argo CD cluster
- `helm` 3.x
- A **fine-grained GitHub PAT** with **Contents: write** on `biqly/biqly` (cannot be created via `gh api`; use GitHub UI once)
- GHCR pull secret `ghcr-registry` already in namespace `argocd` (same as Argo CD)

## One-time: fine-grained PAT + cluster secret

```bash
./deploy/argocd/setup-github-pat.sh
```

This opens [GitHub → New fine-grained PAT](https://github.com/settings/personal-access-tokens/new), prompts for the token, saves it to `.env.local` as `BIQLY_GITHUB_TOKEN`, validates push access, and runs the install script.

**GitHub UI checklist**

| Field | Value |
|--------|--------|
| Resource owner | `biqly` |
| Repository access | Only `biqly` |
| Contents | Read and write |
| Metadata | Read |

Username for HTTPS git is `x-access-token` (default); password is the PAT.

Non-interactive (after you have the PAT):

```bash
BIQLY_GITHUB_TOKEN='github_pat_...' ./deploy/argocd/setup-github-pat.sh
```

## Install / reinstall

```bash
# loads BIQLY_GITHUB_TOKEN from .env.local if exported in your shell
source .env.local 2>/dev/null || true
# optional when main has branch protection:
# export ARGOCD_IMAGE_UPDATER_USE_PR=true

./deploy/argocd/install-image-updater.sh
```

The script:

1. Creates or updates `argocd/argocd-image-updater-git` from env vars
2. Upgrades the Helm release (`deploy/argocd/image-updater-helm-values.yaml`)
3. Applies `deploy/argocd/image-updater.yaml`
4. If `ARGOCD_IMAGE_UPDATER_USE_PR=true`, patches the ImageUpdater CR for GitHub PR write-back

## Branch protection on `main`

When `main` is protected (direct push blocked), set `ARGOCD_IMAGE_UPDATER_USE_PR=true` before running the install script. Image Updater will open PRs instead of pushing to `main`.

## Verify

```bash
kubectl -n argocd get pods -l app.kubernetes.io/name=argocd-image-updater
kubectl -n argocd logs deploy/argocd-image-updater-controller --tail=80
```

Helm release name is still `argocd-image-updater`; the Deployment is `argocd-image-updater-controller` (controller-based chart ≥ v1).

Successful write-back updates `deploy/helm/biqly/.argocd-source-biqly.yaml` (not `values-prod.yaml` — helmvalues alias mapping failed for multi-image apps).

Tracked images: **ai**, **query**, **catalog**, **frontend**, **auth** (`auth.image.tag` + `auth.migrate.image.tag` on `ghcr.io/biqly/biqly-auth`), **migrate** (metadata DB job).

### GitHub Actions vs Image Updater

| Component | Role |
|-----------|------|
| `.github/workflows/build-*.yml` | Builds and pushes `ghcr.io/biqly/*:sha-<commit>` to GHCR |
| Argo CD Image Updater (cluster) | Polls GHCR, commits helm parameter overrides to `.argocd-source-biqly.yaml` |

The workflow runner does **not** edit `.argocd-source-biqly.yaml`. If auth tags are missing there, either the cluster `ImageUpdater` CR is stale (re-apply `deploy/argocd/image-updater.yaml`) or no new `biqly-auth` image was published yet (`build-auth.yml` only runs when auth paths change).

### Expected `.argocd-source-biqly.yaml` shape

Only `*.image.tag` (and `global.migrate.image.tag`) parameters — **no** `image.name`. A stray `image.name: ghcr.io/biqly/migrate` entry is invalid (Helm uses `global.migrate.image.repository` + `global.migrate.image.tag` in `templates/migrate-job.yaml`). Remove it if Image Updater reintroduces it; use `imageName: ghcr.io/biqly/migrate` without `:tag` in the CR so the updater does not split repository into `image.name`.

Until Image Updater runs, `values-prod.yaml` still pins `auth.image.tag` manually.

## CI noise / feedback loop

Image Updater commits only touch `deploy/**`. Without `paths-ignore`, each bump re-runs **CI** (which also pushes new `sha-*` frontend/api images), and `forceUpdate: true` makes Updater commit again → endless workflows.

Repo fix: `paths-ignore: deploy/**` on CI/Test push, and `forceUpdate: false` in `image-updater.yaml`. Re-apply the CR after changing the YAML:

```bash
kubectl apply -f deploy/argocd/image-updater.yaml
```

## Rotate token

```bash
export BIQLY_GITHUB_TOKEN='<new-pat>'
./deploy/argocd/install-image-updater.sh
```

Do **not** use a developer `gh auth token` (OAuth) in production; it expires and is tied to a user session.
