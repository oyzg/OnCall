#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR/backend/go-api"

export GOCACHE="${ROOT_DIR}/tmp/go-build"
export GOMODCACHE="${ROOT_DIR}/tmp/go-mod"
export GOPROXY="${GOPROXY:-https://proxy.golang.org,direct}"

mkdir -p "$GOCACHE" "$GOMODCACHE"

go run ./cmd/server
