#!/bin/sh
set -eu

for migration in /migrations/*.up.sql; do
  if [ ! -e "$migration" ]; then
    echo "No metadata up migrations found in /migrations."
    exit 0
  fi

  migration_name=${migration##*/}
  echo "Applying metadata migration: ${migration_name}"
  psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -f "$migration"
done
