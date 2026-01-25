#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../.."

GOCACHE_DIR="${GOCACHE:-}"
if [[ -z "${GOCACHE_DIR}" ]]; then
  GOCACHE_DIR="$(mktemp -d)"
  trap 'rm -rf "${GOCACHE_DIR}"' EXIT
  export GOCACHE="${GOCACHE_DIR}"
fi

go test ./internal/parity -count=1 -v
