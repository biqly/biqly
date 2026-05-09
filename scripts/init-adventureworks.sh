#!/bin/bash
set -euo pipefail

# Runs at first container init via /docker-entrypoint-initdb.d/.
# Creates the "Adventureworks" database and loads schema + data from
# install.sql. Skips silently if the seed data has not been fetched yet
# (run scripts/fetch-adventureworks.sh on the host first).

if [ ! -f /adventureworks/install.sql ] || [ ! -f /adventureworks/.csvs_ready ]; then
  echo "AdventureWorks data not prepared at /adventureworks — skipping seed."
  echo "Run scripts/fetch-adventureworks.sh on the host to populate testdata/adventureworks/,"
  echo "then 'docker compose down -v && docker compose up -d' to reseed."
  exit 0
fi

echo "Creating Adventureworks database..."
psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER}" -d postgres \
  -c 'CREATE DATABASE "Adventureworks";'

echo "Loading AdventureWorks schema and data..."
cd /adventureworks
psql -v ON_ERROR_STOP=1 -U "${POSTGRES_USER}" -d Adventureworks -f install.sql

echo "AdventureWorks seed complete."
