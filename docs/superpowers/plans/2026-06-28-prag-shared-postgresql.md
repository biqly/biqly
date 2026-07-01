# Prag Shared PostgreSQL Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move biqly and zlitter onto one isolated PostgreSQL 18.4 instance in the prag cluster, backed by the external Hitachi disk and protected by scoped Cilium policies and daily logical backups.

**Architecture:** A new `/Users/baris.dogu/src/prag-postgresql` Helm repository owns the shared database, static retained PVs, backup CronJob, and database-side Cilium policy. The existing biqly and zlitter charts stop managing PostgreSQL and consume namespace-local DSN Secrets that point to `prag-postgresql.postgresql.svc.cluster.local:5432`. The cutover uses a maintenance window, verified custom-format dumps, source/target baselines, and retained old PVs.

**Tech Stack:** Kubernetes 1.36/k3s, Helm 3, Bitnami PostgreSQL chart 18.6.7, PostgreSQL 18.4, Cilium 1.19, Bash, `kubectl`, `pg_dump`, `pg_restore`, static local PVs.

---

## Repository and File Map

### New repository: `/Users/baris.dogu/src/prag-postgresql`

- `Chart.yaml` and `Chart.lock`: independent Helm chart and pinned PostgreSQL dependency.
- `values.yaml`: image digest, existing Secret contract, resources, persistence, metrics, and backup defaults.
- `templates/_helpers.tpl`: stable labels and names used by every first-party resource.
- `templates/initdb-configmap.yaml`: creates additional databases, roles, grants, and required extensions.
- `templates/persistence.yaml`: static `Retain` PVs and bound PVCs for data and backups.
- `templates/backup-serviceaccount.yaml`: no-API-access identity for the backup job.
- `templates/backup-cronjob.yaml`: daily compressed logical backups, checksums, metadata, and seven-day retention.
- `templates/cilium-networkpolicy.yaml`: PostgreSQL ingress from exact biqly/zlitter clients and backup-job egress.
- `templates/tests/test-connection.yaml`: Helm test using the backup role.
- `scripts/preflight.sh`: refuses to run outside `prag` and verifies node, disk, paths, free space, and CRDs.
- `scripts/create-secrets.sh`: generates URL-safe credentials and applies namespace-local Secrets without committing plaintext.
- `scripts/capture-baseline.sh`: captures extensions, migration tables, exact table counts, sequences, and database sizes.
- `scripts/migrate.sh`: creates and verifies source dumps, restores them, and preserves artifacts.
- `scripts/validate.sh`: compares baselines, checks role isolation, connectivity, probes, and backups.
- `tests/render.sh`: deterministic Helm render assertions.
- `.gitignore`: excludes generated credentials, dumps, baselines, and rendered manifests.
- `README.md`: install, cutover, rollback, backup, and delayed cleanup runbook.

### biqly repository: `/Users/baris.dogu/src/biqly/biqly`

- `deploy/helm/biqly/values-prod.yaml`: disable embedded PostgreSQL.
- `deploy/helm/biqly/values.yaml`: document the external PostgreSQL service and keep external Secret names stable.
- `deploy/helm/biqly/templates/cnp-postgresql.yaml`: stop rendering old database ingress when the subchart is disabled.
- `deploy/helm/biqly/templates/cnp-metadata.yaml`: replace broad cluster PostgreSQL egress with the shared endpoint selector.
- `deploy/helm/biqly/templates/cnp-shared-postgresql-egress.yaml`: cover auth, mail, and worker database clients.
- `deploy/helm/biqly/templates/otel-infra-collector.yaml`: disable its
  duplicate PostgreSQL receiver in external mode because the shared chart owns
  postgres-exporter.
- `deploy/helm/biqly/tests/render-shared-postgresql.sh`: render-time regression checks.

### zlitter Helm submodule: `/Users/baris.dogu/zlitter/helm`

- `zlitter/values.yaml`: make external database use independent of `postgresql.enabled`.
- `zlitter/values-rollouts.yaml`: disable embedded PostgreSQL and configure `zlitter-db`.
- `zlitter/templates/api-deployment.yaml` and `zlitter/templates/api-rollout.yaml`: mount the external DB Secret whenever `api.database.enabled`.
- `zlitter/templates/worker-deployment.yaml`, `zlitter/templates/scheduler-deployment.yaml`, and split service templates: decouple DB Secret/init checks from the subchart.
- `zlitter/templates/cnp-postgresql.yaml`: stop rendering old ingress and add shared-database egress selectors.
- `zlitter/templates/db-secret.yaml`: remain disabled when `db.create=false`.
- `zlitter/tests/render-external-postgresql.sh`: render-time regression checks.

### zlitter meta repository: `/Users/baris.dogu/zlitter`

- `helm`: update the Helm submodule pointer after the chart commit.

## Safety Invariants

- Every operational script starts with `set -euo pipefail` and refuses to run unless the current context is exactly `prag`.
- No application workload, PVC/PV, source database, or old disk path is
  deleted. Completed diagnostic/test pods and guarded `migration_validation_*`
  databases may be removed after their results are recorded.
- Old database workloads stop only inside the approved maintenance window.
- Old PVCs, PVs, Secrets, and host paths remain for at least seven days.
- Plaintext passwords, DSNs, dumps, and baseline exports never enter Git.
- The applications do not restart against the target until source/target validation passes.

### Task 1: Scaffold and pin the independent PostgreSQL chart

**Files:**
- Create: `/Users/baris.dogu/src/prag-postgresql/.gitignore`
- Create: `/Users/baris.dogu/src/prag-postgresql/Chart.yaml`
- Create: `/Users/baris.dogu/src/prag-postgresql/values.yaml`
- Create: `/Users/baris.dogu/src/prag-postgresql/templates/_helpers.tpl`
- Create: `/Users/baris.dogu/src/prag-postgresql/tests/render.sh`

- [x] **Step 1: Create the repository and failing render test**

Create `.gitignore`:

```gitignore
.generated/
*.dump
*.dump.sha256
*-baseline.tsv
rendered.yaml
```

Create `tests/render.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
rendered="$(mktemp)"
trap 'rm -f "$rendered"' EXIT
helm lint "$root"
helm template prag-postgresql "$root" --namespace postgresql >"$rendered"
grep -q 'kind: StatefulSet' "$rendered"
grep -q 'sha256:ac855a024049fc14d6b73e38ae3b53cf2d93ae69bf7c6174eda550a5de6f5227' "$rendered"
grep -q 'storageClassName: hitachi-local' "$rendered"
! grep -q 'type: LoadBalancer' "$rendered"
! grep -Eq 'password: .*[^<{]$' "$rendered"
```

Run: `bash tests/render.sh`

Expected: FAIL because `Chart.yaml` does not exist.

- [x] **Step 2: Create the chart metadata and pinned values**

Create `Chart.yaml`:

```yaml
apiVersion: v2
name: prag-postgresql
description: Shared PostgreSQL for prag application namespaces
type: application
version: 0.1.0
appVersion: "18.4"
dependencies:
  - name: postgresql
    version: 18.6.7
    repository: https://charts.bitnami.com/bitnami
```

Create `values.yaml` with this contract:

```yaml
global:
  imagePullSecrets: []

postgresql:
  fullnameOverride: prag-postgresql
  image:
    registry: registry-1.docker.io
    repository: bitnami/postgresql
    tag: "18.4.0"
    digest: sha256:ac855a024049fc14d6b73e38ae3b53cf2d93ae69bf7c6174eda550a5de6f5227
    pullPolicy: IfNotPresent
  architecture: standalone
  auth:
    enablePostgresUser: true
    username: biqly
    database: bi_metadata
    existingSecret: prag-postgresql-auth
    secretKeys:
      adminPasswordKey: postgres-password
      userPasswordKey: password
  primary:
    persistence:
      enabled: true
      existingClaim: prag-postgresql-data
    extraEnvVarsSecret: prag-postgresql-auth
    initdb:
      scriptsConfigMap: prag-postgresql-initdb
    resources:
      requests:
        cpu: 100m
        memory: 512Mi
      limits:
        cpu: "1500m"
        memory: 2Gi
    nodeSelector:
      kubernetes.io/hostname: prag
  metrics:
    enabled: true
    image:
      registry: quay.io
      repository: prometheuscommunity/postgres-exporter
      tag: v0.18.1
      digest: sha256:fb96c4413985d4b23ab02b19022b3d70a86c8e0a62f41ab15ebb6f4673781a5d
    collectors:
      wal: false
    service:
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "9187"

persistence:
  storageClassName: hitachi-local
  nodeName: prag
  data:
    size: 50Gi
    path: /mnt/hitachi-1tb/postgresql/data
  backup:
    size: 20Gi
    path: /mnt/hitachi-1tb/postgresql/backups

backup:
  schedule: "0 2 * * *"
  retentionDays: 7
  secretName: prag-postgresql-auth
```

Create helpers that emit `app.kubernetes.io/name: prag-postgresql`,
`app.kubernetes.io/instance: {{ .Release.Name }}`, and
`app.kubernetes.io/part-of: prag-postgresql`.

- [x] **Step 3: Vendor and verify the dependency**

Run: `helm dependency update /Users/baris.dogu/src/prag-postgresql`

Expected: `Chart.lock` and `charts/postgresql-18.6.7.tgz` are created.

Run: `bash tests/render.sh`

Expected: FAIL only on resources not implemented by later tasks.

- [x] **Step 4: Initialize Git and commit**

```bash
git init
git add .gitignore Chart.yaml Chart.lock charts/postgresql-18.6.7.tgz values.yaml templates/_helpers.tpl tests/render.sh
git commit -m "feat: scaffold shared prag postgresql chart"
```

### Task 2: Add retained external-disk persistence

**Files:**
- Create: `/Users/baris.dogu/src/prag-postgresql/templates/persistence.yaml`
- Modify: `/Users/baris.dogu/src/prag-postgresql/tests/render.sh`

- [x] **Step 1: Add failing PV/PVC assertions**

Append assertions for:

```bash
grep -q 'name: prag-postgresql-data-pv' "$rendered"
grep -q 'path: /mnt/hitachi-1tb/postgresql/data' "$rendered"
grep -q 'name: prag-postgresql-backup-pv' "$rendered"
grep -q 'path: /mnt/hitachi-1tb/postgresql/backups' "$rendered"
test "$(grep -c 'persistentVolumeReclaimPolicy: Retain' "$rendered")" -eq 2
```

Run and expect FAIL.

- [x] **Step 2: Implement static PVs and bound PVCs**

Create two `PersistentVolume` resources with `hostPath.type: Directory`,
`ReadWriteOnce`, `hitachi-local`, `Retain`, and node affinity for hostname
`prag`. Create `prag-postgresql-data` and `prag-postgresql-backups` PVCs in
`.Release.Namespace`, each bound with `volumeName` to its matching PV.

The data PVC must carry:

```yaml
metadata:
  name: prag-postgresql-data
spec:
  accessModes: [ReadWriteOnce]
  storageClassName: hitachi-local
  volumeName: prag-postgresql-data-pv
  resources:
    requests:
      storage: 50Gi
```

- [x] **Step 3: Render, lint, and commit**

Run: `helm lint . && bash tests/render.sh`

Expected: persistence assertions PASS.

```bash
git add templates/persistence.yaml tests/render.sh
git commit -m "feat: add retained hitachi persistence"
```

### Task 3: Add isolated roles, databases, and extensions

**Files:**
- Create: `/Users/baris.dogu/src/prag-postgresql/templates/initdb-configmap.yaml`
- Modify: `/Users/baris.dogu/src/prag-postgresql/tests/render.sh`

- [x] **Step 1: Add failing initialization assertions**

Assert rendered output contains all database names, `pg_trgm`, `citext`,
`pgcrypto`, `pg_read_all_data`, and `ON_ERROR_STOP`.

- [x] **Step 2: Implement deterministic first-boot initialization**

The ConfigMap must provide `00-shared-databases.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
psql=(psql -v ON_ERROR_STOP=1 --username postgres)

"${psql[@]}" \
  --set=biqly_password="$BIQLY_PASSWORD" \
  --set=zlitter_password="$ZLITTER_PASSWORD" \
  --set=backup_password="$BACKUP_PASSWORD" <<'SQL'
ALTER ROLE biqly PASSWORD :'biqly_password';
CREATE ROLE zlitter LOGIN PASSWORD :'zlitter_password';
CREATE ROLE prag_backup LOGIN PASSWORD :'backup_password';
GRANT pg_read_all_data TO prag_backup;
CREATE DATABASE bi_auth OWNER biqly;
CREATE DATABASE bi_mail OWNER biqly;
CREATE DATABASE zlitter OWNER zlitter;
SQL

"${psql[@]}" --dbname bi_metadata -c 'CREATE EXTENSION IF NOT EXISTS pg_trgm'
"${psql[@]}" --dbname zlitter -c 'CREATE EXTENSION IF NOT EXISTS citext'
"${psql[@]}" --dbname zlitter -c 'CREATE EXTENSION IF NOT EXISTS pg_trgm'
"${psql[@]}" --dbname zlitter -c 'CREATE EXTENSION IF NOT EXISTS pgcrypto'

for database in bi_metadata bi_auth bi_mail zlitter; do
  "${psql[@]}" -c "GRANT CONNECT ON DATABASE ${database} TO prag_backup"
done
```

Do not put any literal password in the ConfigMap.

- [x] **Step 3: Verify and commit**

Run: `helm lint . && bash tests/render.sh`

Expected: PASS through initialization assertions.

```bash
git add templates/initdb-configmap.yaml tests/render.sh
git commit -m "feat: initialize isolated application databases"
```

### Task 4: Add scoped Cilium policy and logical backups

**Files:**
- Create: `/Users/baris.dogu/src/prag-postgresql/templates/backup-serviceaccount.yaml`
- Create: `/Users/baris.dogu/src/prag-postgresql/templates/backup-cronjob.yaml`
- Create: `/Users/baris.dogu/src/prag-postgresql/templates/cilium-networkpolicy.yaml`
- Create: `/Users/baris.dogu/src/prag-postgresql/templates/tests/test-connection.yaml`
- Modify: `/Users/baris.dogu/src/prag-postgresql/tests/render.sh`

- [x] **Step 1: Add failing security and backup assertions**

Assert:

```bash
grep -q 'kind: CiliumNetworkPolicy' "$rendered"
grep -q 'k8s:io.kubernetes.pod.namespace: biqly' "$rendered"
grep -q 'k8s:io.kubernetes.pod.namespace: zlitter' "$rendered"
grep -q 'port: "5432"' "$rendered"
grep -q 'concurrencyPolicy: Forbid' "$rendered"
grep -q 'successfulJobsHistoryLimit: 3' "$rendered"
grep -q 'automountServiceAccountToken: false' "$rendered"
grep -q 'find .* -mtime +7 .* -delete' "$rendered"
! grep -q 'rm -rf' "$rendered"
```

Run and expect FAIL.

- [x] **Step 2: Implement database ingress**

Select the PostgreSQL primary by Bitnami labels. Permit 5432 from:

- biqly components `ai`, `auth`, `catalog`, `mail`, `query`, and `worker`;
- zlitter components `admin`, `analytics`, `auth`, `browser-worker`,
  `category`, `feed`, `media`, `profile`, `scheduler`, `search`, and `worker`;
- `app.kubernetes.io/component: backup` in the `postgresql` namespace.

Use `k8s:io.kubernetes.pod.namespace` in every cross-namespace endpoint
selector. Do not use `fromEntities: cluster`.

- [x] **Step 3: Implement backup-job egress**

Add a second `CiliumNetworkPolicy` that selects
`app.kubernetes.io/component: backup`. Permit DNS to kube-dns on TCP/UDP 53
and TCP/5432 only to the Bitnami primary endpoint in the `postgresql`
namespace. Set the backup pod's node selector to
`kubernetes.io/hostname: prag` so its RWO local PVC is always mountable.

- [x] **Step 4: Implement the backup CronJob**

The job mounts only `prag-postgresql-backups`, reads `BACKUP_PASSWORD` from
`prag-postgresql-auth`, sets `PGHOST=prag-postgresql`,
`PGUSER=prag_backup`, and runs:

```bash
set -euo pipefail
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
target="/backups/$stamp"
mkdir -p "$target"
for database in bi_metadata bi_auth bi_mail zlitter; do
  pg_dump --format=custom --compress=9 --file="$target/$database.dump" "$database"
  pg_restore --list "$target/$database.dump" >/dev/null
done
(cd "$target" && sha256sum ./*.dump >SHA256SUMS)
postgres --version >"$target/metadata.txt"
date -u +%FT%TZ >>"$target/metadata.txt"
touch "$target/SUCCESS"
find /backups -mindepth 2 -type f -path '/backups/20??????T??????Z/*' -mtime +7 -delete
find /backups -mindepth 1 -maxdepth 1 -type d -empty -name '20??????T??????Z' -delete
```

Set `concurrencyPolicy: Forbid`, `backoffLimit: 2`,
`activeDeadlineSeconds: 3600`, and explicit 25m/128Mi requests and
500m/512Mi limits.

- [x] **Step 5: Add a Helm connection test and verify**

The test pod uses the backup role, runs `pg_isready`, and executes
`SELECT current_setting('server_version_num')::int >= 180000`.

Run: `helm lint . && bash tests/render.sh`

Expected: all render assertions PASS.

- [x] **Step 6: Commit**

```bash
git add templates tests/render.sh
git commit -m "feat: secure and back up shared postgresql"
```

### Task 5: Add safe operational scripts and runbook

**Files:**
- Create: `/Users/baris.dogu/src/prag-postgresql/scripts/preflight.sh`
- Create: `/Users/baris.dogu/src/prag-postgresql/scripts/create-secrets.sh`
- Create: `/Users/baris.dogu/src/prag-postgresql/scripts/capture-baseline.sh`
- Create: `/Users/baris.dogu/src/prag-postgresql/scripts/migrate.sh`
- Create: `/Users/baris.dogu/src/prag-postgresql/scripts/validate.sh`
- Create: `/Users/baris.dogu/src/prag-postgresql/README.md`

- [x] **Step 1: Implement the common context guard**

Every script begins with:

```bash
#!/usr/bin/env bash
set -euo pipefail
expected_context="${KUBE_CONTEXT:-prag}"
actual_context="$(kubectl config current-context)"
if [[ "$actual_context" != "$expected_context" ]]; then
  echo "refusing: expected context $expected_context, got $actual_context" >&2
  exit 1
fi
```

- [x] **Step 2: Implement preflight**

Check the node is Ready, Cilium policy CRD exists, `hitachi-local` exists,
existing source pods are Ready, both source PVCs are Bound, the new paths do
not contain an existing PostgreSQL cluster, and the external filesystem has at
least 30 GiB free. Use a short-lived node debug pod only for mount inspection,
record its name, and remove it immediately after the check.

Expected success tail:

```text
PASS context=prag node=prag
PASS hitachi_mount=/mnt/hitachi-1tb free_gib>=30
PASS source_databases=2 source_pvcs=2
```

- [x] **Step 3: Implement Secret generation**

Use `umask 077`, a `mktemp -d` directory, and `openssl rand -hex 32`. Generate
admin, biqly, zlitter, and backup passwords once. Apply:

- `postgresql/prag-postgresql-auth` with `postgres-password`, `password`,
  `BIQLY_PASSWORD`, `ZLITTER_PASSWORD`, and `BACKUP_PASSWORD`;
- `biqly/biqly-db` with `BI_METADATA_DB_DSN`;
- `biqly/biqly-auth-db` with `BI_AUTH_DB_DSN`;
- `biqly/biqly-mail-db` with `BI_MAIL_DB_DSN`;
- `zlitter/zlitter-db` with `DATABASE_URL`.

All DSNs use `prag-postgresql.postgresql.svc.cluster.local:5432` and
`sslmode=disable`. Generate YAML with `kubectl create secret ... --dry-run=client
-o yaml`, then apply the generated file. Never print Secret data. Before
replacing an existing Secret, assert its key set contains no unrelated keys.

- [x] **Step 4: Implement baseline capture**

For every database, output deterministic TSV files containing:

```sql
SELECT extname, extversion FROM pg_extension ORDER BY 1;
SELECT schemaname, relname, n_live_tup
FROM pg_stat_user_tables ORDER BY 1, 2;
SELECT schemaname, sequencename, last_value
FROM pg_sequences ORDER BY 1, 2;
```

Also calculate exact `count(*)` for every ordinary user table after writers
are stopped. Store outputs under `.generated/baselines/<source|target>/`.

- [x] **Step 5: Implement migration**

`migrate.sh dump` must:

- verify maintenance mode by rejecting any non-monitoring client sessions;
- create one custom-format dump per database;
- run `pg_restore --list`;
- write SHA-256 checksums;
- copy artifacts to the backup PVC through a temporary, labeled transfer pod.

The source-side dump command for each database is:

```bash
pg_dump --format=custom --compress=9 --no-owner --no-acl \
  --file="$workdir/$database.dump" "$source_dsn"
pg_restore --list "$workdir/$database.dump" >/dev/null
sha256sum "$workdir/$database.dump" >"$workdir/$database.dump.sha256"
```

`migrate.sh restore` must verify checksums, restore with `--clean --if-exists
--no-owner --exit-on-error`, use `--role=biqly` for biqly databases and
`--role=zlitter` for zlitter, and stop on the first error.

Use this restore shape:

```bash
sha256sum --check "$database.dump.sha256"
pg_restore --clean --if-exists --no-owner --no-acl --exit-on-error \
  --role="$target_role" --dbname="$target_dsn" "$database.dump"
```

- [x] **Step 6: Implement validation and README**

`validate.sh` compares source/target exact counts, sequences, extensions, and
migration versions; proves cross-role access is denied; checks pods, Services,
Cilium policy validity, and the latest backup checksum.

README must contain exact commands for preflight, install, maintenance entry,
dump, restore, target validation, application cutover, rollback before traffic,
rollback after traffic, and the seven-day delayed cleanup rule.

- [x] **Step 7: Shell-check, dry-run, and commit**

Run:

```bash
shellcheck scripts/*.sh tests/*.sh
bash tests/render.sh
```

Expected: no shellcheck findings and all render checks PASS.

```bash
git add scripts README.md
git commit -m "feat: add guarded migration operations"
```

### Task 6: Switch biqly chart contracts to shared PostgreSQL

**Files:**
- Modify: `/Users/baris.dogu/src/biqly/biqly/deploy/helm/biqly/values.yaml`
- Modify: `/Users/baris.dogu/src/biqly/biqly/deploy/helm/biqly/values-prod.yaml`
- Modify: `/Users/baris.dogu/src/biqly/biqly/deploy/helm/biqly/templates/cnp-metadata.yaml`
- Create: `/Users/baris.dogu/src/biqly/biqly/deploy/helm/biqly/templates/cnp-shared-postgresql-egress.yaml`
- Modify: `/Users/baris.dogu/src/biqly/biqly/deploy/helm/biqly/templates/otel-infra-collector.yaml`
- Create: `/Users/baris.dogu/src/biqly/biqly/deploy/helm/biqly/tests/render-shared-postgresql.sh`

- [ ] **Step 1: Write a failing render regression test**

Render production values and assert:

```bash
! grep -q 'name: biqly-postgresql$' "$rendered"
grep -q 'prag-postgresql.postgresql.svc.cluster.local' "$rendered"
grep -q 'k8s:io.kubernetes.pod.namespace: postgresql' "$rendered"
! grep -A20 'name: biqly-egress-metadata' "$rendered" | grep -q 'toEntities:'
```

Run and expect FAIL because embedded PostgreSQL still renders.

- [ ] **Step 2: Add explicit external database values**

Under `global`, add:

```yaml
postgresqlExternal:
  enabled: false
  host: prag-postgresql.postgresql.svc.cluster.local
  port: 5432
  namespace: postgresql
  instance: prag-postgresql
```

In `values-prod.yaml`, set:

```yaml
global:
  postgresqlExternal:
    enabled: true
postgresql:
  enabled: false
```

Keep `global.secrets.createSecrets: false` and existing Secret names unchanged.

- [ ] **Step 3: Narrow egress**

Remove PostgreSQL/5432 from the broad `toEntities: cluster` rule in
`cnp-metadata.yaml`. Add `cnp-shared-postgresql-egress.yaml` selecting the six
database client components and allowing only:

```yaml
toEndpoints:
  - matchLabels:
      k8s:io.kubernetes.pod.namespace: postgresql
      app.kubernetes.io/name: postgresql
      app.kubernetes.io/instance: prag-postgresql
      app.kubernetes.io/component: primary
toPorts:
  - ports:
      - port: "5432"
        protocol: TCP
```

- [x] **Step 4: Update PostgreSQL observability**

When `global.postgresqlExternal.enabled`, do not render the biqly
OpenTelemetry PostgreSQL receiver or reference `biqly-postgresql-auth`. The
shared chart's postgres-exporter is the only PostgreSQL metrics source.

- [ ] **Step 5: Render, lint, and commit**

Run:

```bash
helm lint deploy/helm/biqly -f deploy/helm/biqly/values-prod.yaml
bash deploy/helm/biqly/tests/render-shared-postgresql.sh
git diff --check
```

Expected: all checks PASS and no Secret values appear in rendered output.

```bash
git add deploy/helm/biqly
git commit -m "feat(deploy): use shared prag postgresql"
```

### Task 7: Make zlitter external PostgreSQL a first-class mode

**Files:**
- Modify: `/Users/baris.dogu/zlitter/helm/zlitter/values.yaml`
- Modify: `/Users/baris.dogu/zlitter/helm/zlitter/values-rollouts.yaml`
- Modify: every `/Users/baris.dogu/zlitter/helm/zlitter/templates/*deployment.yaml` and `*rollout.yaml` that gates database Secret/init behavior on `postgresql.enabled`
- Modify: `/Users/baris.dogu/zlitter/helm/zlitter/templates/cnp-postgresql.yaml`
- Create: `/Users/baris.dogu/zlitter/helm/zlitter/tests/render-external-postgresql.sh`

- [ ] **Step 1: Write the failing external-mode test**

Render with:

```yaml
postgresql:
  enabled: false
db:
  create: false
api:
  database:
    enabled: true
    existingSecret: zlitter-db
worker:
  database:
    enabled: true
    existingSecret: zlitter-db
```

Assert no PostgreSQL StatefulSet/Service/Secret renders, every DB client mounts
`zlitter-db`, every DB wait container remains present, and shared endpoint
egress renders. Run and expect FAIL because current templates require
`postgresql.enabled`.

- [ ] **Step 2: Decouple application database mode from subchart mode**

Replace conditions shaped like:

```gotemplate
{{- if and .Values.api.database.enabled .Values.postgresql.enabled }}
```

with:

```gotemplate
{{- if .Values.api.database.enabled }}
```

Apply the equivalent change for worker, scheduler, and split API services.
Resolve the Secret with each component's `database.existingSecret`. Init
containers continue to use the Secret-provided `DATABASE_URL`, so no service
name is hardcoded.

- [ ] **Step 3: Set production external values**

In `values-rollouts.yaml`, set:

```yaml
postgresql:
  enabled: false
db:
  create: false
api:
  database:
    enabled: true
    existingSecret: zlitter-db
worker:
  database:
    enabled: true
    existingSecret: zlitter-db
```

Set the scheduler database Secret to `zlitter-db` using its existing value
shape. Remove the obsolete `networkPolicy.postgresql.clientNamespaces` entry.

- [ ] **Step 4: Replace old database policies**

Render old PostgreSQL ingress only when `postgresql.enabled`. Add egress for
the shared PostgreSQL endpoint with the same namespace and Bitnami primary
labels used by the database chart. Remove any broad 5432 allowance that would
make the selector ineffective.

- [ ] **Step 5: Verify and commit the Helm submodule**

Run:

```bash
helm lint zlitter -f zlitter/values-rollouts.yaml
bash zlitter/tests/render-external-postgresql.sh
git diff --check
```

Expected: all checks PASS.

```bash
git add zlitter
git commit -m "feat: support external shared postgresql"
```

- [x] **Step 6: Commit the meta-repository submodule pointer**

```bash
cd /Users/baris.dogu/zlitter
git add helm
git commit -m "chore: bump helm for shared postgresql"
```

### Task 8: Preflight and install the empty shared database

**Files:**
- Runtime state only; no committed Secret values.

- [ ] **Step 1: Verify all repositories are clean and capture revisions**

Run `git status --short` and `git rev-parse HEAD` in prag-postgresql, biqly,
zlitter/helm, and zlitter. Expected: clean worktrees and four recorded SHAs.

- [ ] **Step 2: Run preflight**

Run: `./scripts/preflight.sh`

Expected: all PASS lines, context `prag`, at least 30 GiB free, and no existing
target database directory.

- [x] **Step 3: Create directories and credentials**

Create `/mnt/hitachi-1tb/postgresql/data` and `backups` on the prag node with
UID/GID 1001 and mode 0700. Run `./scripts/create-secrets.sh`.

Expected: four Secret names reported, no Secret data printed.

- [x] **Step 4: Install the shared release**

Run:

```bash
helm upgrade --install prag-postgresql . \
  --namespace postgresql \
  --create-namespace \
  --wait \
  --timeout 10m
```

Expected: release deployed and one Ready PostgreSQL pod.

- [ ] **Step 5: Validate the empty target and network boundaries**

Run Helm test, `validate.sh --empty-target`, a permitted connection from each
application namespace, and a denied connection from `default`.

Stop immediately if any denied path succeeds.

### Task 9: Enter maintenance mode and migrate data

**Files:**
- Runtime state and retained backup artifacts only.

- [ ] **Step 1: Capture online source metadata**

Run `capture-baseline.sh source --estimated` and save the output under the
timestamped migration directory.

- [x] **Step 2: Stop all database writers**

Scale or pause biqly database clients, zlitter Rollouts/Deployments, worker,
scheduler, and browser worker. Leave old PostgreSQL pods running.

Expected: no non-monitoring sessions in `pg_stat_activity` for five consecutive
checks.

- [x] **Step 3: Capture exact source baseline**

Run: `capture-baseline.sh source --exact`

Expected: four complete baseline sets and no query errors.

- [x] **Step 4: Dump and verify**

Run: `migrate.sh dump`

Expected: four verified `.dump` files, matching SHA-256 files, and a `SUCCESS`
marker on the external backup PV.

- [x] **Step 5: Restore and validate**

Run:

```bash
./scripts/migrate.sh restore
./scripts/capture-baseline.sh target --exact
./scripts/validate.sh --data
```

Expected: exact counts, sequences, extensions, and migration versions match.
If they do not, keep applications stopped and execute the pre-traffic rollback
from README.

### Task 10: Cut over applications and prove service health

**Files:**
- Runtime Helm release state only; source commits already prepared.

- [ ] **Step 1: Upgrade biqly**

Run the repo-documented production Helm command using `values-prod.yaml`.
Expected: embedded PostgreSQL resources no longer render; retained old PVC/PV
remain; migration hooks succeed; all biqly database clients become Ready.

- [ ] **Step 2: Upgrade zlitter**

Run the repo-documented production Helm command using
`values-rollouts.yaml`. Expected: embedded PostgreSQL resources no longer
render; retained old PVC/PV remain; all split API services, worker, scheduler,
and browser worker become Ready.

- [ ] **Step 3: Run application and policy validation**

Verify:

- biqly auth, metadata read/write, mail queue, query, and worker flows;
- zlitter auth, feed/search reads, worker ingestion, scheduler advisory lock,
  and browser job persistence;
- no authentication, DNS, connection-refused, or Cilium-denied DB errors;
- biqly role cannot connect to `zlitter`;
- zlitter role cannot connect to any `bi_*` database;
- unrelated namespace connection remains denied.

- [ ] **Step 4: Run backup and restore rehearsal**

Create a one-off Job from the CronJob, wait for success, verify checksums, and
restore into temporary validation databases. Compare exact table counts, then
remove only the temporary validation databases after recording success.

- [ ] **Step 5: Record the rollback window**

Record cutover timestamp, source and target SHAs, dump checksums, Helm
revisions, and old PV/PVC names in the migration directory. Keep the old data
for at least seven days. Cleanup is not part of this plan and requires separate
approval.

## Final Verification

Run:

```bash
helm list -A
kubectl get pods,pvc -n postgresql
kubectl get pods -n biqly
kubectl get pods -n zlitter
kubectl get ciliumnetworkpolicy -A
```

Expected:

- one Ready PostgreSQL pod in `postgresql`;
- all application workloads Ready;
- new data and backup PVCs Bound to `hitachi-local`;
- old application PostgreSQL PVs/PVCs retained;
- no external PostgreSQL LoadBalancer or NodePort;
- all Cilium policies valid;
- a successful verified backup exists;
- all four Git worktrees are clean.
