#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR/backend/python-ai"

if [[ ! -x ".venv/bin/python" ]]; then
  echo "Missing backend/python-ai/.venv. Run: cd backend/python-ai && python3 -m venv .venv && .venv/bin/python -m pip install -e ." >&2
  exit 1
fi

.venv/bin/python -m uvicorn app.main:app --reload --host 0.0.0.0 --port 8000
