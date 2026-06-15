#!/bin/sh
set -eu

# Create the additional biqly system databases in the shared Postgres instance,
# mirroring the Kubernetes layout (one cluster, databases bi_metadata + bi_auth
# + bi_mail under a single role). bi_metadata is created via POSTGRES_DB; this
# script creates the rest. Runs only on first init (empty data dir), so plain
# CREATE is safe — no existence check needed.
for db in bi_auth bi_mail; do
  echo "Creating database: ${db}"
  createdb -U "${POSTGRES_USER}" -O "${POSTGRES_USER}" "${db}"
done
