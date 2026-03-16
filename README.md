# AI OnCall

AI OnCall is a monorepo for an intelligent on-call platform.

## Structure

- `frontend/web`: Vue 3 web console
- `backend/go-api`: Go core business backend
- `backend/python-ai`: Python AI service
- `proto`: gRPC contracts between Go and Python services
- `deploy/docker-compose`: local infrastructure
- `docs`: product and architecture documents

## Phase 1 Status

The repository is initialized with project skeletons for:

- Vue frontend
- Go API
- Python AI service
- gRPC proto contracts
- Docker Compose local dependencies

## Local Development

1. Start infrastructure:

```bash
./scripts/dev/up.sh
```

Or start everything with one command:

```bash
./scripts/dev/start-all.sh
```

2. Start frontend:

```bash
./scripts/dev/web.sh
```

3. Start Go API:

```bash
./scripts/dev/go-api.sh
```

4. Start Python AI service:

```bash
./scripts/dev/python-ai.sh
```

## Notes

- The Python service is prepared with `pyproject.toml` and uses `venv`/`pip` by default because `uv` is not installed in the current environment.
- Dependencies are declared but not installed automatically by this bootstrap step.
- The Go startup scripts force `GOPROXY=https://proxy.golang.org,direct` by default and use project-local Go caches under `tmp/` to reduce local environment issues.
