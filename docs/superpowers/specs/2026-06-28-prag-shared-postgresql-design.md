# Prag Shared PostgreSQL Design

> **Status:** Implemented in the `prag` cluster; retained as the design record for the completed migration.

## Objective

Replace the PostgreSQL instances embedded in the `biqly` and `zlitter` Helm
releases with one independently managed PostgreSQL instance in the `prag`
Kubernetes cluster. Preserve application-level isolation with separate roles
and databases, store data and backups on the external Hitachi disk, and limit
network access with Cilium policies.

The migration may use a short maintenance window. The expected interruption is
5–15 minutes because the current application data is approximately 1 GiB.

## Current State

| Application | PostgreSQL | Databases | Approximate application data | Persistent storage |
| --- | --- | --- | ---: | --- |
| biqly | 18.4 | `bi_metadata`, `bi_auth`, `bi_mail` | 134 MiB | `/mnt/hitachi-1tb/biqly/postgresql` |
| zlitter | 17.6 | `zlitter` | 776 MiB | `/mnt/hitachi-1tb/zlitter/postgresql` |

Both existing persistent volumes use the `hitachi-local` storage class and the
`Retain` reclaim policy. Existing Cilium policies are scoped around each
application's in-namespace PostgreSQL instance and do not describe a shared
cross-namespace database.

Required extensions are:

- `bi_metadata`: `pg_trgm`
- `zlitter`: `citext`, `pg_trgm`, and `pgcrypto`

## Chosen Approach

Create an independent `prag-postgresql` Helm chart and release. Do not make the
shared database lifecycle a responsibility of either application release.

Alternatives rejected:

- Hosting the shared instance in the biqly chart would couple zlitter's
  availability and database lifecycle to biqly deployments.
- Introducing CloudNativePG would add operator and CRD lifecycle complexity
  without providing high availability on the current single-node, single-disk
  cluster.

## Architecture

The release creates a dedicated `postgresql` namespace with:

- one PostgreSQL 18.4-compatible StatefulSet;
- a version- and digest-pinned image, never `latest`;
- a headless Service for StatefulSet identity;
- a client Service at `postgresql.postgresql.svc.cluster.local:5432`, retaining
  its ClusterIP for in-cluster clients while Cilium advertises
  `192.168.0.164:5432` for routed administration;
- readiness, liveness, and startup probes;
- explicit CPU and memory requests and limits;
- a non-root security context, disabled privilege escalation, dropped
  capabilities, and no service-account token mount;
- a dedicated ServiceAccount with no Kubernetes API permissions;
- ServiceMonitor or compatible Prometheus scrape annotations for the existing
  monitoring stack;
- a backup CronJob and a separately mountable backup PVC.

The database is not exposed through a Gateway or Ingress. Its Cilium
LoadBalancer permits only the Zima/OpenWrt Tailscale source
`100.94.156.3/32`; PostgreSQL authentication remains required.

## Storage

Create static local persistent volumes on the existing Hitachi mount:

| Purpose | Host path | Requested capacity | Reclaim policy |
| --- | --- | ---: | --- |
| PostgreSQL data | `/mnt/hitachi-1tb/postgresql/data` | 50 GiB | `Retain` |
| Logical backups | `/mnt/hitachi-1tb/postgresql/backups` | 20 GiB | `Retain` |

The manifests include node affinity for the `prag` node. Deployment preflight
must verify that `/mnt/hitachi-1tb` is the intended external filesystem, that
both directories exist with safe ownership and permissions, and that free disk
space is sufficient.

The old biqly and zlitter PVs and PVCs remain intact for at least seven days
after cutover. No old database directory is deleted as part of this migration.

## Database and Credential Isolation

Create separate, non-superuser application roles:

- a biqly role that owns and can connect only to `bi_metadata`, `bi_auth`, and
  `bi_mail`;
- a zlitter role that owns and can connect only to `zlitter`;
- a backup role with only the privileges required for logical backups;
- a distinct administrator credential that is not mounted into application
  pods.

Passwords are generated randomly. Plaintext credentials and rendered Secret
manifests must never be committed. The authoritative credentials are Kubernetes
Secrets in the `postgresql` namespace. Each application namespace receives only
its own DSN Secret, with the host set to the shared ClusterIP Service DNS name.

## Application Configuration

### biqly

- Disable the PostgreSQL subchart in production values.
- Keep the existing `BI_METADATA_DB_DSN`, `BI_AUTH_DB_DSN`, and
  `BI_MAIL_DB_DSN` interfaces.
- Point the corresponding Secret values to the shared Service.
- Remove or disable policies and VIP resources that select the old embedded
  PostgreSQL pods.
- Preserve migration hooks, but run them only after restored-schema validation
  confirms the target is ready.

### zlitter

- Disable the PostgreSQL subchart in production values.
- Set `db.create=false`.
- Point API, worker, scheduler, and other database clients to an externally
  managed Secret containing `DATABASE_URL`.
- Remove or disable the embedded PostgreSQL VIP and policies.
- Preserve application migration behavior and prevent concurrent migration
  races during first startup.

## Network Security

The shared PostgreSQL endpoint has default-deny ingress. A
`CiliumNetworkPolicy` allows TCP/5432 only from explicitly selected database
client pods in the `biqly` and `zlitter` namespaces.

Application-side egress policies allow:

- DNS to the cluster DNS service;
- TCP/5432 to the shared PostgreSQL endpoint;
- other existing application-specific destinations that are unrelated to this
  migration.

Selectors use namespace labels plus stable application workload labels. Access
is not granted broadly with `fromEntities: cluster`.

The backup CronJob receives a separate rule permitting TCP/5432 to PostgreSQL.
Validation must prove both an allowed connection from each application and a
denied connection from an unrelated namespace.

## Backup Design

Run a daily CronJob that creates compressed custom-format dumps with `pg_dump`.
Each successful run writes:

- one dump per application database;
- a SHA-256 checksum file;
- a small metadata file recording PostgreSQL/client versions and completion
  time.

Delete successful backup sets older than seven days. A failed or incomplete
backup does not remove the most recent successful set. Jobs use
`concurrencyPolicy: Forbid`, bounded retries, resource limits, and an explicit
deadline.

The migration includes a manual backup-and-restore rehearsal. Backups on the
same physical disk protect against operator error and database corruption but
do not protect against complete disk loss. Off-host backup is outside this
change.

## Migration Procedure

1. Verify cluster context, node health, external-disk mount, available space,
   current database versions, extensions, sizes, and active client workloads.
2. Render and lint all three Helm charts and inspect the complete diff.
3. Install the shared PostgreSQL release with empty databases, roles,
   extensions, policies, and Secrets.
4. Verify probes, persistence, role isolation, monitoring, and allowed/denied
   network paths.
5. Record source baselines: schema migration versions, table row counts,
   sequence values, extensions, and database sizes.
6. Enter maintenance mode by stopping all biqly and zlitter database-writing
   workloads.
7. Confirm that no application sessions remain on either source.
8. Create custom-format dumps with a PostgreSQL 18 client, retain the dump
   files, calculate checksums, and verify each archive with `pg_restore --list`.
9. Restore the three biqly databases and the zlitter database with ownership
   mapped to their new application roles.
10. Compare source and target baselines. Resolve any mismatch before opening
    traffic.
11. Upgrade the biqly and zlitter releases to disable embedded PostgreSQL and
    consume the new DSN Secrets.
12. Start application workloads in a controlled order and verify migrations,
    readiness, logs, API smoke tests, background workers, and scheduler
    behavior.
13. Run the backup CronJob manually and perform a restore test into temporary
    validation databases.
14. End maintenance mode only after all acceptance checks pass.

## Validation and Acceptance Criteria

The migration is successful when:

- all four application databases restore without errors;
- source and target table row counts and sequence values match at the cutover
  boundary;
- required extensions exist at compatible versions;
- application roles cannot access the other application's databases;
- all database-using workloads become ready and remain stable;
- API smoke tests and representative read/write flows pass;
- workers and scheduled tasks process data without authentication or network
  errors;
- Prometheus can scrape PostgreSQL metrics;
- permitted Cilium connectivity succeeds and an unrelated pod is denied;
- a manual backup completes, its checksum verifies, and a test restore works;
- the old database paths and retained PVs remain untouched.

## Failure Handling and Rollback

Before traffic reopens, any failed check causes an immediate rollback:

1. keep application workloads stopped;
2. revert DSN configuration to the old services;
3. re-enable or restart the old PostgreSQL workloads;
4. verify old database health;
5. restart applications against the old databases.

After traffic has reopened, rollback requires another maintenance window.
Applications are stopped, writes made to the new instance are identified and
copied back or the databases are reverse-dumped, and only then are old
connections restored. The process must never silently discard post-cutover
writes.

Old PVCs, PVs, Secrets, and database directories are retained for at least
seven days. Their removal is a separate, explicitly approved operation after
the rollback window.

## Non-Goals

- PostgreSQL high availability or automated failover
- Cross-node storage
- Public or LAN database exposure
- Off-host or cloud backup
- Deleting the existing PostgreSQL data during this change
