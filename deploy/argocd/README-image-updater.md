# Argo CD Image Updater (biqly)

Credentials **never** live in this repository. Create the git write secret in the cluster only.

## Prerequisites

- `kubectl` context pointing at the Argo CD cluster
- `helm` 3.x
- A **fine-grained GitHub PAT** (or machine user PAT) with **Contents: write** on `biqly/biqly`
- GHCR pull secret `ghcr-registry` already in namespace `argocd` (same as Argo CD)

## Install / reinstall

```bash
export BIQLY_GITHUB_TOKEN='<pat-with-repo-write>'
# optional: export BIQLY_GITHUB_USER='biqly-bot'
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
kubectl -n argocd logs deploy/argocd-image-updater --tail=50
```

Successful write-back updates `deploy/helm/biqly/.argocd-source-biqly.yaml` (not `values-prod.yaml` — helmvalues alias mapping failed for multi-image apps).

## Rotate token

```bash
export BIQLY_GITHUB_TOKEN='<new-pat>'
./deploy/argocd/install-image-updater.sh
```

Do **not** use a developer `gh auth token` (OAuth) in production; it expires and is tied to a user session.
