---
name: deploy-prod
description: >
  Full dev→prod landing pipeline for the biqly repo: merge dev into main and
  push — then CI builds images, Flux image automation promotes the new tags into
  the biqly-gitops repo, and FluxCD reconciles the prag cluster. Fully hands-off
  after the push: no local tag bump, no local helm. Your job is to land the
  merge and verify the deploy. Trigger when the user says "prod'a çıkar", "proda
  çıkar", "prod'a al", "deploy to prod", "land and deploy prod", or invokes
  /deploy-prod. Do NOT trigger for dev-only work or when the user just asks to
  commit/push.
---

# Deploy to prod (biqly → prag cluster, GitOps via FluxCD)

The trigger phrase is the user's intent to deploy — proceed through the
read-only preflight (Phase 0) without re-asking. But before the one
irreversible step — pushing the `dev`→`main` merge, which starts the CD —
confirm with the user, showing the exact commits/diff that will land on `main`.
DO stop and report (without proceeding) if any step below fails or finds a
surprise; never improvise around a failed gate.

**How prod deploys now (fully automated, Flux-native).** The cluster's desired
state lives in a SEPARATE private repo `biqly/biqly-gitops` — the Flux entrypoint
(`clusters/prag/`), the Helm chart (`deploy/helm/biqly/`), and `values-prod.yaml`.
FluxCD is bootstrapped on the prag cluster (`flux-system` namespace) and watches
ONLY `biqly-gitops` (branch `main`, path `clusters/prag`, ~1 min interval); a
`HelmRelease` named `biqly` renders the chart with `values.yaml` + `values-prod.yaml`
into the `biqly` namespace. The app repo `biqly/biqly` holds only application code.
The loop after a push to `biqly/biqly` `main`:

1. CI + the `build-*` workflows build and push each service image to
   `ghcr.io/biqly/*`, now tagged with a sortable `<YYYYMMDDHHmmss>-<sha>`
   timestamp (legacy `sha-<sha>`/`latest` still published during the transition
   but ignored by automation).
2. Flux image-automation (running against biqly-gitops) advances the deploy: an
   `ImageRepository` per service scans GHCR, an `ImagePolicy` selects the newest
   tag by timestamp, and `ImageUpdateAutomation` (`biqly`) writes the selected
   tags into biqly-gitops's `values-prod.yaml` (via `# {"$imagepolicy": ...}`
   setter markers) and commits+pushes to biqly-gitops `main` as `fluxcdbot`.
3. Flux source-controller + helm-controller pick up that commit within ~1 min
   and reconcile the HelmRelease → rollout.

So **all you do is land the merge on `biqly/biqly` `main`.** There is no bump
script, no `auto-bump-prod` workflow, no local `helm upgrade`/`helm dependency
build`. Promotion happens in the biqly-gitops repo, and the apply is Flux. Local
`helm`/`docker`/`yq` are not needed. Note: the promotion commit lands on
**biqly-gitops**, NOT on `biqly/biqly` — nothing comes back to the monorepo.

Announce each phase in one line as you go. At the end, report: merge commit,
image tag, Flux HelmRelease revision, pod status, and confirmation that
dev == main.

## Phase 0 — Preflight

1. `git -C . status` must be clean. If not, stop and show the user what's
   uncommitted — never stash/discard on their behalf.
2. `git fetch origin`. Current work is expected on `dev`. If local `dev` is
   behind `origin/dev`, stop and report (a concurrent session may be active).
3. `kubectl config current-context` must be `prag` (needed for read-only
   verification in Phase 5). Anything else: stop. (No local docker/helm/yq
   needed — promotion runs in Flux.)
4. Flux must be healthy: `flux check` and `flux get helmrelease biqly -n
   flux-system` (READY=True). If Flux is unhealthy or suspended, stop and
   report — nothing will deploy until it is fixed.

## Phase 1 — Merge dev → main and push

1. `git push origin dev` (so origin/dev is current before the merge).
2. `git checkout main && git pull origin main`.
3. `git merge dev` (merge commit / fast-forward is the house style — do not
   rebase/squash). On conflict: stop, show the conflicting files, let the user
   drive.
4. `git push origin main`.

## Phase 2 — Watch GitHub Actions image builds

If the diff touches ONLY `deploy/**`, `.claude/**` and docs, no service images
rebuild (CI skips deploy-only changes) and Flux image automation has nothing to
advance; Flux still reconciles any chart/config change on its own, so skip to
Phase 4.

1. `SHA=$(git rev-parse HEAD)` (the merge commit on main).
2. Poll `gh run list --commit "$SHA" --json name,status,conclusion,workflowName`
   until every `build-*` workflow (and `CI`) for that commit reports
   `completed`. Poll every ~60–90s; builds typically take a few minutes.
3. Any build concludes `failure` → stop, fetch the failing job's log tail via
   `gh run view <id> --log-failed`, and report. Flux won't promote a service
   whose build failed, so the deploy stalls until it's fixed.

## Phase 3 — Watch Flux image automation promote into biqly-gitops

There is nothing to run here — promotion is Flux-native, not a CI job. When the
builds finish and the timestamp tags land in GHCR, Flux selects them and pushes
the bump to `biqly-gitops` `main` as `fluxcdbot`. Confirm it fired:

1. `flux get images all -n flux-system` — the `ImageRepository`/`ImagePolicy`
   for each service should show the new timestamp tag as latest, and
   `ImageUpdateAutomation/biqly` should report a recent successful run.
2. Confirm the promotion commit landed in biqly-gitops (NOT in this repo):
   `gh api repos/biqly/biqly-gitops/commits --jq '.[0:3].[] | .commit.author.name + " " + .commit.message'`
   — expect a recent `fluxcdbot` commit updating the image tags. If a local
   clone exists at `/Users/baris.dogu/src/biqly/biqly-gitops`, `git -C
   /Users/baris.dogu/src/biqly/biqly-gitops fetch && git -C
   /Users/baris.dogu/src/biqly/biqly-gitops log origin/main --oneline -3` works too.
3. If no promotion appears after all builds are green, inspect the automation:
   `flux get images all -n flux-system` for policy errors, then
   `kubectl -n flux-system logs deploy/image-automation-controller` — likely a
   registry-auth (GHCR pull secret) or deploy-key/push issue. Do NOT hand-bump
   locally to work around it; fix the automation.

## Phase 4 — Let Flux reconcile (no local helm)

Do NOT run `helm upgrade` or `helm dependency build`. Flux builds the chart from
biqly-gitops (vendored bitnami postgresql keeps it hermetic; local `file://`
subcharts are rebuilt fresh by source-controller) and applies it with
`upgrade.force: true`, which takes over stale field managers server-side (e.g. an
old `argocd-controller` manager that may still linger on the biqly-ai HTTPRoute
until it is recreated) — so the old `--force-conflicts` dance is handled for you.

1. Optionally speed things up instead of waiting for the ~1 min git poll:
   `flux reconcile helmrelease biqly -n flux-system --with-source`. This is
   still Flux doing the deploy, not a local apply.
2. Watch `flux get helmrelease biqly -n flux-system` until READY=True with a new
   chart revision. The MESSAGE should read `Helm upgrade succeeded for release
   biqly/biqly.vN`.
3. If it reports a failure (e.g. a template/strict-YAML error — Flux's parser
   is stricter than the Helm CLI and rejects duplicate keys the CLI tolerated),
   stop and report the message; the running release is untouched because the
   render fails before apply.

## Phase 5 — Verify

1. `kubectl -n biqly get pods` — every app pod Running/Ready on the new image
   (`kubectl -n biqly get deploy <svc> -o jsonpath='{.spec.template.spec.containers[0].image}'`).
   The cluster is a single node with maxSurge=0, so a rolling deploy briefly
   terminates the old pod before the new one is Ready — a transient 503 during
   rollout is expected; wait for it to settle.
2. Any CrashLoopBackOff/OOMKilled: `kubectl -n biqly logs <pod> --previous`
   plus the termination state, report immediately, and ASK before rolling back
   — never roll back silently.
3. Quick gateway probe: `curl -s -o /dev/null -w '%{http_code}' https://abi.il1.nl/health`
   should be 200 (retry a few times while the rollout settles).

### Rollback (only if asked / on a bad deploy)

Because Flux owns the release, do NOT `helm rollback` — Flux would revert it on
the next reconcile. To roll back, revert the offending promotion commit on
`biqly-gitops` `main` and push; Flux reconciles back to the previous state. For
an emergency freeze, `flux suspend helmrelease biqly -n flux-system` first, then
act. ASK the user before rolling back.

## Phase 6 — Re-sync dev with main

Promotion commits now land on `biqly-gitops`, not this repo, so nothing comes
back to the monorepo — dev and main simply stay equal after the merge. Confirm
the resting state:

1. `git checkout dev && git merge main` (should be a no-op / fast-forward).
2. `git push origin dev` if anything moved.
3. Confirm `git rev-parse dev main` are equal — that's the resting state.
