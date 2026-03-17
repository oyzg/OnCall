# AI OnCall

AI OnCall is a monorepo for an intelligent on-call platform with:

- `frontend/web`: Vue 3 web console
- `backend/go-api`: Go core business backend
- `backend/python-ai`: Python AI service
- `proto`: gRPC contracts reserved for Go/Python communication
- `deploy/docker-compose`: local and demo deployment assets
- `docs`: product, architecture, integration, and demo documents

## Core Capabilities

- Login and role-based access
- Session chat with SSE streaming reply
- Knowledge document management and retrieval testing
- Alert center with AI analysis and linked troubleshooting chat
- Tool center with execution logs
- Audit and operations insights page

## Local Development

Start infrastructure only:

```bash
./scripts/dev/up.sh
```

Start frontend, Go API, Python AI, and infrastructure together:

```bash
./scripts/dev/start-all.sh
```

Start services separately when needed:

```bash
./scripts/dev/web.sh
./scripts/dev/go-api.sh
./scripts/dev/python-ai.sh
```

## Demo Deployment

Start the demo stack with Docker Compose:

```bash
./scripts/demo/up.sh
```

Load demo data:

```bash
./scripts/demo/load-demo-data.sh
```

Stop the demo stack:

```bash
./scripts/demo/down.sh
```

Demo entry points:

- Web: `http://127.0.0.1:5173`
- Go API: `http://127.0.0.1:8080/healthz`
- Python AI: `http://127.0.0.1:8000/healthz`

Demo accounts:

- `admin / OnCallAdmin2026!`
- `ops / OnCallOps2026!`

## Verification

Backend tests:

```bash
cd backend/go-api
env GOPROXY=https://proxy.golang.org,direct \
  GOCACHE="$(pwd)/../../tmp/go-build" \
  GOMODCACHE="$(pwd)/../../tmp/go-mod-cache" \
  go test ./...
```

Frontend build:

```bash
cd frontend/web
npm run build
```

Python syntax check:

```bash
python3 -m compileall backend/python-ai/app
```

## Documents

- [Development Plan](/Users/ouyangzhenguang/project/OnCall/docs/development-plan.md)
- [Phase 12 Integration](/Users/ouyangzhenguang/project/OnCall/docs/phase-12-integration.md)
- [Phase 13 Demo](/Users/ouyangzhenguang/project/OnCall/docs/phase-13-demo.md)

## Notes

- Python AI currently uses FastAPI plus a prompt/rule-based analysis service. Real model inference can be added later without changing the business API shape.
- Go startup scripts force `GOPROXY=https://proxy.golang.org,direct` by default and use project-local caches under `tmp/`.
- The demo environment targets “fast startup and clear walkthrough”, not production deployment.
