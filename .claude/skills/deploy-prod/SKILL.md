---
name: deploy-prod
description: >
  Full dev→prod landing pipeline for the biqly repo: merge dev into main and
  push — then CI builds images, the auto-bump-prod workflow rewrites
  values-prod.yaml tags, and FluxCD reconciles the prag cluster. Fully hands-off
  after the push: no local tag bump, no local helm. Your job is to land the
  merge and verify the deploy, then re-sync dev with main. Trigger when the user
  says "prod'a çıkar", "proda çıkar", "prod'a al", "deploy to prod", "land and
  deploy prod", or invokes /deploy-prod. Do NOT trigger for dev-only work or
  when the user just asks to commit/push.
---

# Deploy to prod (biqly → prag cluster, GitOps via FluxCD)

The user saying the trigger phrase IS the explicit prod go-ahead — do not ask
again for permission to touch prod. DO stop and report (without proceeding) if
any step below fails or finds a surprise; never improvise around a failed gate.

**How prod deploys now (fully automated).** FluxCD is bootstrapped on the prag
cluster (`flux-system` namespace, manifests under `clusters/prag/`). A
`GitRepository` watches `main` (~1 min interval) and a `HelmRelease` named
`biqly` renders `deploy/helm/biqly` with `values.yaml` + `values-prod.yaml` into
the `biqly` namespace. The loop after a push to `main`:

1. CI + the `build-*` workflows build and push each service image tagged
   `sha-<commit>` to `ghcr.io/biqly/*`.
2. The **`auto-bump-prod`** workflow (`.github/workflows/auto-bump-prod.yml`)
   fires on those builds' completion, pins each service in `values-prod.yaml` to
   its newest published image (`scripts/helm-bump-tags.sh`,
   `LATEST_PER_SERVICE=1`), and pushes the bump commit to `main`.
3. Flux picks up the bump within ~1 min and reconciles the HelmRelease → rollout.

So **all you do is land the merge on `main`.** Do NOT run the bump script, `git`
bump, `helm upgrade`, or `helm dependency build` from local — the bump is a CI
job and the apply is Flux. Local `helm`/`docker`/`yq` are not needed.

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
   needed — the bump runs in CI.)
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

If the diff touches ONLY `deploy/**`, `clusters/**`, `.claude/**` and docs, no
service images rebuild (CI skips deploy-only changes) and auto-bump has nothing
to advance; Flux still reconciles any chart/config change on its own, so skip to
Phase 4.

1. `SHA=$(git rev-parse HEAD)` (the merge commit on main).
2. Poll `gh run list --commit "$SHA" --json name,status,conclusion,workflowName`
   until every `build-*` workflow (and `CI`) for that commit reports
   `completed`. Poll every ~60–90s; builds typically take a few minutes.
3. Any build concludes `failure` → stop, fetch the failing job's log tail via
   `gh run view <id> --log-failed`, and report. Auto-bump won't advance a
   service whose build failed, so the deploy stalls until it's fixed.

## Phase 3 — Auto-bump (automatic — do NOT bump locally)

There is nothing to run here. When the builds finish, the `auto-bump-prod`
workflow pins each service in `values-prod.yaml` to its newest published image
and pushes a `chore(deploy): auto-bump prod image tags to sha-<short8>` commit
to `main` as `biqly-ci[bot]`. Just confirm it fired:

1. `gh run list --workflow=auto-bump-prod.yml -L 3` — the run for this release
   should be `completed`/`success` (it may run several times as builds finish;
   the last one carries the bump).
2. `git fetch origin && git log origin/main --oneline -3` — expect the bot's
   auto-bump commit at the tip.
3. If no bump appears after all builds are green, check the workflow logs
   (`gh run view <id> --log`) — likely a registry-auth or push issue — and
   report. Do NOT hand-bump locally to work around it; fix the workflow.

## Phase 4 — Let Flux reconcile (no local helm)

Do NOT run `helm upgrade` or `helm dependency build`. Flux builds the chart
(vendored bitnami postgresql keeps it hermetic; local `file://` subcharts are
rebuilt fresh by source-controller) and applies it with `upgrade.force: true`,
which takes over the stale `argocd-controller`/`kubectl-patch` field managers
on the biqly-ai Deployment/HTTPRoute — so the old `--force-conflicts` dance is
handled server-side.

1. Optionally speed things up instead of waiting for the ~1 min git poll:
   `flux reconcile helmrelease biqly -n flux-system --with-source`. This is
   still Flux doing the deploy, not a local apply.
2. Watch `flux get helmrelease biqly -n flux-system` until READY=True with the
   new chart revision `0.1.0+<sha8>.<n>` (sha8 = the tag-bump commit). The
   MESSAGE should read `Helm upgrade succeeded for release biqly/biqly.vN`.
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
the next reconcile. To roll back, revert the offending commit on `main` (e.g.
`git revert` the tag bump) and push; Flux reconciles back to the previous state.
For an emergency freeze, `flux suspend helmrelease biqly -n flux-system` first,
then act. ASK the user before rolling back.

## Phase 6 — Re-sync dev with main

1. `git checkout dev && git merge main` (brings the bump commit back; should
   be fast-forward or a trivial merge).
2. `git push origin dev`.
3. Confirm `git rev-parse dev main` are equal — that's the resting state.
