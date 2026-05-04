#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR/backend/python-ai"

HTTP_PORT="${HTTP_PORT:-8000}"
GRPC_HOST="${GRPC_HOST:-0.0.0.0}"
GRPC_PORT="${GRPC_PORT:-50051}"

if [[ ! -x ".venv/bin/python" ]]; then
  echo "Missing backend/python-ai/.venv. Run: cd backend/python-ai && python3 -m venv .venv && .venv/bin/python -m pip install -e ." >&2
  exit 1
fi

echo "Starting python-ai HTTP on :${HTTP_PORT} and gRPC runtime on ${GRPC_HOST}:${GRPC_PORT}"
GRPC_HOST="$GRPC_HOST" GRPC_PORT="$GRPC_PORT" \
  .venv/bin/python -m uvicorn app.main:app --reload --host 0.0.0.0 --port "$HTTP_PORT"
