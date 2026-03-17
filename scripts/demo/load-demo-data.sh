#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
API_BASE_URL="${API_BASE_URL:-http://127.0.0.1:8080}"
KNOWLEDGE_FILE="$ROOT_DIR/examples/demo/knowledge/user-service-runbook.txt"
ALERT_PAYLOAD="$ROOT_DIR/examples/demo/alerts/payment-api-latency.json"

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Missing required command: $cmd" >&2
    exit 1
  fi
}

wait_for_api() {
  for _ in $(seq 1 30); do
    if curl -fsS "$API_BASE_URL/healthz" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  echo "Go API is not ready: $API_BASE_URL" >&2
  exit 1
}

get_token() {
  local response
  response="$(curl -fsS \
    -H "Content-Type: application/json" \
    -d '{"username":"admin","password":"OnCallAdmin2026!"}' \
    "$API_BASE_URL/api/v1/auth/login")"

  python3 - <<'PY' "$response"
import json
import sys
payload = json.loads(sys.argv[1])
print(payload["data"]["access_token"]["token"])
PY
}

main() {
  require_cmd curl
  require_cmd python3

  wait_for_api
  TOKEN="$(get_token)"

  curl -fsS \
    -X POST \
    -H "Authorization: Bearer ${TOKEN}" \
    -F "title=User Service SOP" \
    -F "category=runbook" \
    -F "file=@${KNOWLEDGE_FILE}" \
    "$API_BASE_URL/api/v1/knowledge/documents" >/dev/null

  curl -fsS \
    -X POST \
    -H "Content-Type: application/json" \
    --data @"$ALERT_PAYLOAD" \
    "$API_BASE_URL/api/v1/alerts/ingest" >/dev/null

  cat <<EOF
Demo data loaded.

Login:
  admin / OnCallAdmin2026!
  ops   / OnCallOps2026!

Recommended path:
  1. Open http://127.0.0.1:5173
  2. View alerts and generate AI analysis
  3. Open chat, knowledge, tools, and audit pages
EOF
}

main "$@"
