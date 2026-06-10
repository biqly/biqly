#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
blocklist="$root/scripts/locale_literal_blocklist.txt"
baseline="${LOCALE_LITERAL_BASELINE:-$root/scripts/locale_literal_baseline.txt}"
baseline_mode=false
update_baseline=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --baseline)
      baseline_mode=true
      ;;
    --update-baseline)
      baseline_mode=true
      update_baseline=true
      ;;
    --baseline-file)
      if [[ $# -lt 2 ]]; then
        echo "--baseline-file requires a path" >&2
        exit 2
      fi
      baseline="$2"
      shift
      ;;
    -h|--help)
      echo "usage: $0 [--baseline] [--update-baseline] [--baseline-file path]" >&2
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 2
      ;;
  esac
  shift
done

if [[ ! -f "$blocklist" ]]; then
  echo "missing locale literal blocklist: $blocklist" >&2
  exit 2
fi

tmp_findings=$(mktemp)
tmp_sorted=$(mktemp)
trap 'rm -f "$tmp_findings" "$tmp_sorted"' EXIT

cd "$root"

# Keep the scope narrow: production Go files only. Tests and testdata may keep
# natural-language examples; JSON lexicons are governed by their own tests.
grep -RInF \
  -f "$blocklist" \
  --include='*.go' \
  --exclude='*_test.go' \
  --exclude-dir='testdata' \
  "internal/ai/routing" \
  "internal/ai/ambiguity" \
  "internal/semanticgen" >"$tmp_findings" || true

sort "$tmp_findings" >"$tmp_sorted"

if [[ ! -s "$tmp_sorted" ]]; then
  echo "no locale literal warnings"
  exit 0
fi

if [[ "$update_baseline" == true ]]; then
  mkdir -p "$(dirname "$baseline")"
  cp "$tmp_sorted" "$baseline"
  echo "updated locale literal baseline: $baseline"
  exit 0
fi

if [[ "$baseline_mode" == true ]]; then
  if [[ ! -f "$baseline" ]]; then
    echo "missing locale literal baseline: $baseline" >&2
    echo "run with --update-baseline after reviewing current findings" >&2
    exit 1
  fi
  if ! diff -u "$baseline" "$tmp_sorted"; then
    echo "locale literal findings changed; update scripts/locale_literal_baseline.txt or remove the new literal" >&2
    exit 1
  fi
  echo "locale literal warnings match baseline ($(wc -l <"$tmp_sorted") findings)"
  exit 0
fi

echo "locale literal warnings:" >&2
cat "$tmp_sorted" >&2
echo "new locale literals are not allowed; move text into the locale layer" >&2
exit 1
