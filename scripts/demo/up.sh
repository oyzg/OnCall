#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE_FILE="$ROOT_DIR/deploy/docker-compose/docker-compose.demo.yml"

cd "$ROOT_DIR"
docker compose -f "$COMPOSE_FILE" up -d --build

cat <<'EOF'

AI OnCall demo stack is starting.

Services:
  Web:        http://127.0.0.1:5173
  Go API:     http://127.0.0.1:8080/healthz
  Python AI:  http://127.0.0.1:8000/healthz

Suggested next step:
  ./scripts/demo/load-demo-data.sh
EOF
