# GitHub Branch Protection

Apply this repository ruleset to protect `main` before enabling production GitOps.

Required settings:

- Block direct pushes to `main`.
- Require pull requests before merging.
- Require at least one approval.
- Dismiss stale approvals when new commits are pushed.
- Require conversation resolution before merge.
- Require status checks to pass before merge.
- Require branches to be up to date before merge.

Required status checks:

- `Go test`
- `golangci-lint`
- `Helm lint`
- `Backend (Go)`
- `Frontend (Vite + TS)`

Notes:

- GitHub branch protection is repository state, not a versioned file. Keep this document as the intended policy and apply it in GitHub repository settings or with the GitHub API.
- Keep `argocd-image-updater` write-back constrained to image tag changes under `deploy/helm/biqly/values-prod.yaml`.
