#!/usr/bin/env bash
set -euo pipefail

# Prepares testdata/adventureworks/ so the test-datasource container can run
# install.sql at first init. Steps:
#   1. Clone lorint/AdventureWorks-for-Postgres (install.sql + ruby converter)
#   2. Download the Microsoft AdventureWorks 2014 OLTP CSV archive
#   3. Unzip CSVs into the same directory
#   4. Run update_csvs.rb (in a one-off ruby:3-alpine container) to convert
#      the CSVs from UTF-16/Windows-1252 to UTF-8 + Postgres-friendly format
#
# Re-running the script is a no-op once the .csvs_ready marker exists. To
# force a fresh fetch, delete testdata/adventureworks/ and re-run.

REPO_URL="https://github.com/lorint/AdventureWorks-for-Postgres.git"
ZIP_URL="https://github.com/Microsoft/sql-server-samples/releases/download/adventureworks/AdventureWorks-oltp-install-script.zip"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
TARGET="${SCRIPT_DIR}/../testdata/adventureworks"
DONE_MARKER="${TARGET}/.csvs_ready"

# Earlier fetch runs may have created the marker without converting pipe CSVs
# (see update_csvs.rb patch below). Force a reconversion in that case.
if [ -f "${DONE_MARKER}" ] && [ -f "${TARGET}/BusinessEntity.csv" ]; then
  if grep -q '+|' "${TARGET}/BusinessEntity.csv" 2>/dev/null; then
    echo "AdventureWorks CSVs look unconverted (pipe-delimited); removing marker to re-run conversion."
    rm -f "${DONE_MARKER}"
  fi
fi

if [ -f "${DONE_MARKER}" ]; then
  echo "AdventureWorks already prepared at ${TARGET}"
  exit 0
fi

require() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "ERROR: '$1' is required on the host." >&2
    exit 1
  }
}

require git
require curl
require unzip
require docker

# 1) Clone the converter scripts.
if [ ! -d "${TARGET}/.git" ]; then
  mkdir -p "$(dirname "${TARGET}")"
  echo "Cloning ${REPO_URL}..."
  git clone --depth 1 "${REPO_URL}" "${TARGET}"
fi

# Overwrite upstream update_csvs.rb — current MS OLTP CSV mix UTF-8 and UTF-16;
# upstream always opens non-Address files as UTF-16LE, which fails on UTF-8 rows
# and never writes pipe-delimited conversions (see scripts/adventureworks-update_csvs.rb).
cp "${SCRIPT_DIR}/adventureworks-update_csvs.rb" "${TARGET}/update_csvs.rb"

# 2) Download the Microsoft CSV archive.
ZIP_PATH="${TARGET}/AdventureWorks-oltp-install-script.zip"
if [ ! -f "${ZIP_PATH}" ]; then
  echo "Downloading AdventureWorks OLTP CSV archive..."
  curl -fsSL -o "${ZIP_PATH}" "${ZIP_URL}"
fi

# 3) Extract CSVs.
echo "Extracting CSVs..."
unzip -o -q "${ZIP_PATH}" -d "${TARGET}"

# 4) Convert CSVs (UTF-16 -> UTF-8, line endings, escaping). Run in a
#    container so the host doesn't need Ruby installed. Use the host UID/GID
#    so resulting files are owned by the user, not root.
echo "Converting CSVs (running update_csvs.rb in ruby:3-alpine)..."
docker run --rm \
  --user "$(id -u):$(id -g)" \
  -v "${TARGET}:/data" \
  -w /data \
  ruby:3-alpine \
  ruby update_csvs.rb

# 5) Cleanup the archive and mark complete.
rm -f "${ZIP_PATH}"
touch "${DONE_MARKER}"
echo "AdventureWorks ready at ${TARGET}"
