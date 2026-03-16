#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RUN_DIR="$ROOT_DIR/tmp/dev"
COMPOSE_FILE="$ROOT_DIR/deploy/docker-compose/docker-compose.dev.yml"
GO_CACHE_DIR="$ROOT_DIR/tmp/go-build"
GO_MOD_CACHE_DIR="$ROOT_DIR/tmp/go-mod"

GO_LOG="$RUN_DIR/go-api.log"
PYTHON_LOG="$RUN_DIR/python-ai.log"
WEB_LOG="$RUN_DIR/web.log"

GO_PID=""
PYTHON_PID=""
WEB_PID=""

require_cmd() {
  local cmd="$1"
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "Missing required command: $cmd" >&2
    exit 1
  fi
}

check_frontend_deps() {
  if [[ ! -d "$ROOT_DIR/frontend/web/node_modules" ]]; then
    cat >&2 <<'EOF'
Frontend dependencies are not installed.
Run:
  cd frontend/web && npm install
EOF
    exit 1
  fi
}

check_python_deps() {
  if ! python3 -c "import fastapi, uvicorn" >/dev/null 2>&1; then
    cat >&2 <<'EOF'
Python AI dependencies are not installed.
Run:
  cd backend/python-ai
  python3 -m venv .venv
  source .venv/bin/activate
  pip install -e .
EOF
    exit 1
  fi
}

start_infra() {
  echo "[1/4] Starting infrastructure with Docker Compose..."
  docker compose -f "$COMPOSE_FILE" up -d
}

start_go_api() {
  echo "[2/4] Starting Go API..."
  (
    cd "$ROOT_DIR/backend/go-api"
    export GOCACHE="$GO_CACHE_DIR"
    export GOMODCACHE="$GO_MOD_CACHE_DIR"
    export GOPROXY="${GOPROXY:-https://proxy.golang.org,direct}"
    mkdir -p "$GOCACHE" "$GOMODCACHE"
    go mod download
    go run ./cmd/server
  ) >"$GO_LOG" 2>&1 &
  GO_PID=$!
}

start_python_ai() {
  echo "[3/4] Starting Python AI service..."
  (
    cd "$ROOT_DIR/backend/python-ai"
    python3 -m uvicorn app.main:app --reload --host 0.0.0.0 --port 8000
  ) >"$PYTHON_LOG" 2>&1 &
  PYTHON_PID=$!
}

start_web() {
  echo "[4/4] Starting frontend..."
  (
    cd "$ROOT_DIR/frontend/web"
    npm run dev -- --host 0.0.0.0
  ) >"$WEB_LOG" 2>&1 &
  WEB_PID=$!
}

cleanup() {
  local code=$?

  for pid in "$GO_PID" "$PYTHON_PID" "$WEB_PID"; do
    if [[ -n "$pid" ]] && kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
    fi
  done

  exit "$code"
}

print_summary() {
  cat <<EOF

AI OnCall dev stack is starting.

Services:
  Frontend:   http://127.0.0.1:5173
  Go API:     http://127.0.0.1:8080/healthz
  Python AI:  http://127.0.0.1:8000/healthz

Logs:
  $WEB_LOG
  $GO_LOG
  $PYTHON_LOG

Press Ctrl-C to stop the app processes.
Docker infrastructure will keep running.
EOF
}

main() {
  mkdir -p "$RUN_DIR"
  mkdir -p "$GO_CACHE_DIR" "$GO_MOD_CACHE_DIR"

  require_cmd docker
  require_cmd go
  require_cmd npm
  require_cmd python3

  check_frontend_deps
  check_python_deps

  trap cleanup INT TERM EXIT

  start_infra
  start_go_api
  start_python_ai
  start_web

  print_summary
  wait
}

main "$@"
