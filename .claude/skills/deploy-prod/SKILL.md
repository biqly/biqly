---
name: deploy-prod
description: >
  Full dev→prod landing pipeline for the biqly repo: merge dev into main, push,
  watch GitHub Actions image builds, bump values-prod.yaml image tags, helm
  upgrade the prag cluster, verify pods, then re-sync dev with main. Trigger
  when the user says "prod'a çıkar", "proda çıkar", "prod'a al", "deploy to
  prod", "land and deploy prod", or invokes /deploy-prod. Do NOT trigger for
  dev-only work or when the user just asks to commit/push.
---

# Deploy to prod (biqly → prag cluster)

The user saying the trigger phrase IS the explicit prod go-ahead — do not ask
again for permission to touch prod. DO stop and report (without proceeding) if
any step below fails or finds a surprise; never improvise around a failed gate.

Announce each phase in one line as you go. At the end, report: merge commit,
image tag, helm revision, pod status, and confirmation that dev == main.

## Phase 0 — Preflight

1. `git -C . status` must be clean. If not, stop and show the user what's
   uncommitted — never stash/discard on their behalf.
2. `git fetch origin`. Current work is expected on `dev`. If local `dev` is
   behind `origin/dev`, stop and report (a concurrent session may be active).
3. `kubectl config current-context` must be `prag`. Anything else: stop.
4. Docker must be running (`docker info -f '{{.ServerVersion}}'`) — the tag
   bump script uses `docker manifest inspect`. `yq` lives at
   `/opt/homebrew/bin/yq`.

## Phase 1 — Merge dev → main and push

1. `git push origin dev` (so origin/dev is current before the merge).
2. `git checkout main && git pull origin main`.
3. `git merge dev` (merge commit is the house style — do not rebase/squash).
   On conflict: stop, show the conflicting files, let the user drive.
4. `git push origin main`.

## Phase 2 — Watch GitHub Actions image builds

Skip this phase AND Phase 3 when the diff being landed touches ONLY
`deploy/**` and docs (no new images get built — `ci.yml` skips deploy-only
changes): the existing pinned tags stay correct; go straight to Phase 4.

1. `SHA=$(git rev-parse HEAD)` (the merge commit on main).
2. Poll `gh run list --commit "$SHA" --json name,status,conclusion,workflowName`
   until every `build-*` workflow (and `ci`) for that commit reports
   `completed`. Poll every ~60–90s; builds typically take a few minutes.
3. Any build concludes `failure` → stop, fetch the failing job's log tail via
   `gh run view <id> --log-failed`, and report. Do not deploy on red.

## Phase 3 — Bump prod image tags

1. On `main` with the merge commit checked out, run `scripts/helm-bump-tags.sh`
   — it checks `ghcr.io/biqly/<svc>:sha-$SHA` exists per service and rewrites
   the pinned tag paths in `deploy/helm/biqly/values-prod.yaml`.
2. If it updated anything: commit ONLY that file on main —
   `chore(deploy): bump prod image tags to sha-<short8>` — and push main.
   (House style: bump commits land directly on main.)
3. If a service's image is unexpectedly missing (script says "not found" for a
   service whose code changed), stop and report — the build likely isn't done
   or failed.

## Phase 4 — Helm upgrade

1. `helm dependency build deploy/helm/biqly` — ALWAYS. Stale `charts/*.tgz`
   silently deploys old subchart templates even when `helm template` renders
   the edited directory.
2. `helm upgrade --install biqly deploy/helm/biqly -n biqly \
      -f deploy/helm/biqly/values-prod.yaml --force-conflicts`
   `--force-conflicts` is required: stale `argocd-controller` and
   `kubectl-patch` field managers own fields on the biqly-ai Deployment and
   HTTPRoute from old experiments; the flag transfers ownership to helm.
3. Note the new revision from `helm -n biqly history biqly --max 3`.

## Phase 5 — Verify

1. `kubectl -n biqly get pods` — every pod Running/Ready, and watch for ~60s
   that RESTARTS is not climbing (the cluster is a single node with maxSurge=0,
   so pods restart in place; give them time to settle).
2. Any CrashLoopBackOff/OOMKilled: `kubectl -n biqly logs <pod> --previous`
   plus the termination state, report immediately, and ASK before rolling
   back (`helm rollback`) — never roll back silently.
3. Quick gateway probe: `curl -s -o /dev/null -w '%{http_code}' https://abi.il1.nl/health`
   should be 200.

## Phase 6 — Re-sync dev with main

1. `git checkout dev && git merge main` (brings the bump commit back; should
   be fast-forward or a trivial merge).
2. `git push origin dev`.
3. Confirm `git rev-parse dev main` are equal — that's the resting state.
