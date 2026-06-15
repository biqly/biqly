#!/bin/sh
set -eu

# Apply the auth + mail schema migrations to their databases in the shared
# Postgres instance, mirroring init-metadata-db.sh. This local setup applies
# migrations directly with psql (it does not use golang-migrate's
# schema_migrations tracking), so a fresh `make dev-up` brings up all three
# databases fully migrated. Runs on first init only (empty data dir).
apply_migrations() {
  db="$1"
  dir="$2"
  for migration in "$dir"/*.up.sql; do
    if [ ! -e "$migration" ]; then
      echo "No up migrations found in ${dir}."
      return 0
    fi
    echo "Applying ${db} migration: ${migration##*/}"
    psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER}" -d "${db}" -f "$migration"
  done
}

apply_migrations bi_auth /migrations/auth
apply_migrations bi_mail /migrations/mail
