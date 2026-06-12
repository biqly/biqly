#!/usr/bin/env bash
set -euo pipefail

exec kubectl port-forward -n biqly svc/biqly-postgresql 15433:5432
