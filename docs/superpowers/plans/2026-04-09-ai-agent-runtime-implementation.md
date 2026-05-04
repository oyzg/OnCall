# AI Agent Runtime Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the current HTTP plus rule-based AI boundary with a gRPC-based Python AI runtime that uses LangGraph workflows, OpenAI-compatible model access, and multiple focused agents for alert analysis and chat QA.

**Architecture:** Keep Go as the business-facing API and persistence boundary, and move all AI orchestration into Python. Deliver this in four passes: lock the proto/runtime contract, stand up Python gRPC runtime and Go gateway, migrate alert analysis onto Router plus Alert Analysis Agent, then migrate chat QA onto Router plus Chat QA Agent while preserving the existing Go SSE shape.

**Tech Stack:** Go, Gin, FastAPI, gRPC, Protocol Buffers, Buf, Python grpcio/grpcio-tools, LangChain, LangGraph, OpenAI-compatible models, unittest, Go test

---

### Task 1: Replace the Legacy AI Proto Contract With a Runtime Contract

**Files:**
- Create: `proto/ai/runtime.proto`
- Modify: `proto/buf.gen.yaml`
- Modify: `scripts/proto/gen.sh`
- Delete or deprecate references in: `proto/ai/alert.proto`
- Delete or deprecate references in: `proto/ai/chat.proto`
- Delete or deprecate references in: `proto/ai/rag.proto`
- Create: `backend/python-ai/app/gen/proto/__init__.py`

- [ ] **Step 1: Write the new runtime proto before any implementation**

Define a single service boundary for:

- `AnalyzeAlert`
- `RunConversationTurn`
- `Health`

Include message types for:

- request metadata
- alert analysis result
- conversation history
- citations
- tool calls
- trace events

- [ ] **Step 2: Add Python stub generation to the proto toolchain**

Extend the generation setup so a single command regenerates:

- Go protobuf and gRPC stubs
- Python protobuf and gRPC stubs under `backend/python-ai/app/gen/proto`

Prefer keeping `buf generate` for Go and adding Python generation to `scripts/proto/gen.sh` if Buf Python plugin setup is awkward.

- [ ] **Step 3: Run proto generation and verify the files are created**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall
./scripts/proto/gen.sh
```

Expected:

- generated Go files appear under `backend/go-api/gen/proto/ai`
- generated Python files appear under `backend/python-ai/app/gen/proto`

- [ ] **Step 4: Verify generated code is importable**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall/backend/python-ai
./.venv/bin/python - <<'PY'
from app.gen.proto.ai import runtime_pb2, runtime_pb2_grpc
print("python proto imports ok")
PY
```

Expected: `python proto imports ok`

- [ ] **Step 5: Commit the protocol boundary**

```bash
git add proto/ai/runtime.proto proto/buf.gen.yaml scripts/proto/gen.sh backend/python-ai/app/gen/proto
git commit -m "feat: add ai runtime proto contract"
```

---

### Task 2: Stand Up the Python gRPC Runtime Skeleton

**Files:**
- Create: `backend/python-ai/app/grpc/server.py`
- Create: `backend/python-ai/app/grpc/services/runtime_service.py`
- Create: `backend/python-ai/app/grpc/mappers.py`
- Create: `backend/python-ai/app/llm/openai_compatible.py`
- Create: `backend/python-ai/app/agents/router_agent.py`
- Create: `backend/python-ai/app/agents/alert_analysis_agent.py`
- Create: `backend/python-ai/app/agents/chat_qa_agent.py`
- Create: `backend/python-ai/app/agents/tool_agent.py`
- Create: `backend/python-ai/app/graphs/state.py`
- Create: `backend/python-ai/app/graphs/router_graph.py`
- Create: `backend/python-ai/app/graphs/alert_analysis_graph.py`
- Create: `backend/python-ai/app/graphs/chat_qa_graph.py`
- Modify: `backend/python-ai/app/core/config.py`
- Modify: `backend/python-ai/pyproject.toml`
- Modify: `backend/python-ai/app/main.py`
- Create: `backend/python-ai/tests/test_runtime_service.py`
- Create: `backend/python-ai/tests/test_router_agent.py`

- [ ] **Step 1: Write failing Python tests for the runtime entrypoints**

Add tests that prove:

- `AnalyzeAlert` returns a structured placeholder response through the gRPC service mapper
- `RunConversationTurn` returns placeholder `answer`, `route`, and `status`
- Router Agent can classify `alert_analysis` and `chat_qa` requests

- [ ] **Step 2: Run the focused Python tests and verify they fail for missing runtime pieces**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall/backend/python-ai
./.venv/bin/python -m unittest tests.test_runtime_service tests.test_router_agent -v
```

Expected: failures complaining about missing runtime service / router agent behavior, not syntax errors.

- [ ] **Step 3: Add Python runtime dependencies and config**

Update `pyproject.toml` and config to support:

- `grpcio`
- `grpcio-tools` or equivalent generation/runtime dependency
- model name / timeout / base URL settings for OpenAI-compatible access
- optional feature flags for dev fallback behavior

- [ ] **Step 4: Implement the minimal gRPC service skeleton**

Implement:

- gRPC server bootstrap
- request/response mappers
- minimal Router Agent
- placeholder Alert / Chat / Tool agents
- placeholder graphs that return deterministic structured results

Keep FastAPI alive for `/healthz`, but do not route the business chain through HTTP anymore.

- [ ] **Step 5: Re-run the focused Python tests until they pass**

Run the same command as Step 2.

- [ ] **Step 6: Verify Python syntax and imports stay green**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall/backend/python-ai
./.venv/bin/python -m compileall app
```

- [ ] **Step 7: Commit the Python runtime skeleton**

```bash
git add backend/python-ai/app/grpc backend/python-ai/app/llm backend/python-ai/app/agents backend/python-ai/app/graphs backend/python-ai/app/core/config.py backend/python-ai/pyproject.toml backend/python-ai/app/main.py backend/python-ai/tests/test_runtime_service.py backend/python-ai/tests/test_router_agent.py
git commit -m "feat: add python ai runtime skeleton"
```

---

### Task 3: Switch the Go AI Gateway From HTTP to gRPC

**Files:**
- Modify: `backend/go-api/internal/ai/gateway/client.go`
- Modify: `backend/go-api/internal/ai/eino/orchestrator.go`
- Modify: `backend/go-api/internal/platform/grpcclient/client.go`
- Modify: `backend/go-api/go.mod`
- Create: `backend/go-api/internal/ai/gateway/client_test.go`
- Create or modify: `backend/go-api/internal/ai/analyzer/service_test.go`

- [ ] **Step 1: Write failing Go tests for gRPC-backed gateway behavior**

Cover:

- alert analysis request mapping from Go domain structs to proto request
- conversation turn response mapping from proto response to Go gateway response
- graceful handling of Python runtime unavailability

- [ ] **Step 2: Run the focused Go tests and verify they fail for missing gRPC wiring**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall/backend/go-api
env GOPROXY=https://proxy.golang.org,direct GOCACHE=/Users/ouyangzhenguang/project/OnCall/tmp/go-build GOMODCACHE=/Users/ouyangzhenguang/project/OnCall/tmp/go-mod-cache go test ./internal/ai/gateway ./internal/ai/analyzer -v
```

Expected: failures showing the client still uses HTTP or lacks proto-backed wiring.

- [ ] **Step 3: Implement the gRPC client and thin orchestrator**

Replace the legacy HTTP path with:

- gRPC connection bootstrap
- generated runtime client usage
- request/response mapping
- fallback handling preserved at the analyzer layer

Keep the public Go-facing `gateway.Client` API stable where possible so upper layers do not need to know about proto internals.

- [ ] **Step 4: Re-run the focused Go tests until they pass**

Run the same command as Step 2.

- [ ] **Step 5: Verify the health/probe layer still compiles**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall/backend/go-api
env GOPROXY=https://proxy.golang.org,direct GOCACHE=/Users/ouyangzhenguang/project/OnCall/tmp/go-build GOMODCACHE=/Users/ouyangzhenguang/project/OnCall/tmp/go-mod-cache go test ./internal/platform/grpcclient ./internal/platform/observability -v
```

- [ ] **Step 6: Commit the Go gRPC boundary**

```bash
git add backend/go-api/internal/ai/gateway/client.go backend/go-api/internal/ai/eino/orchestrator.go backend/go-api/internal/platform/grpcclient/client.go backend/go-api/go.mod backend/go-api/go.sum backend/go-api/internal/ai/gateway/client_test.go backend/go-api/internal/ai/analyzer/service_test.go
git commit -m "feat: switch ai gateway to grpc"
```

---

### Task 4: Migrate Alert Analysis Onto Router Plus Alert Analysis Agent

**Files:**
- Modify: `backend/python-ai/app/agents/router_agent.py`
- Modify: `backend/python-ai/app/agents/alert_analysis_agent.py`
- Modify: `backend/python-ai/app/agents/tool_agent.py`
- Modify: `backend/python-ai/app/graphs/router_graph.py`
- Modify: `backend/python-ai/app/graphs/alert_analysis_graph.py`
- Modify: `backend/python-ai/app/grpc/services/runtime_service.py`
- Modify: `backend/python-ai/app/llm/openai_compatible.py`
- Modify: `backend/python-ai/app/services/hybrid_rag.py`
- Modify: `backend/go-api/internal/ai/analyzer/service.go`
- Create: `backend/python-ai/tests/test_alert_analysis_agent.py`
- Create: `backend/python-ai/tests/test_alert_analysis_graph.py`
- Create or modify: `backend/go-api/internal/platform/httpserver/server_integration_test.go`

- [ ] **Step 1: Write failing Python tests for structured alert analysis output**

Cover:

- Router sends alert analysis requests to the alert path
- Alert Analysis Agent returns all required structured fields
- Tool Agent can be invoked and normalized inside alert workflow
- graph returns a trace list and non-empty `workflow` / `source`

- [ ] **Step 2: Run the focused Python tests and verify they fail for missing agent behavior**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall/backend/python-ai
./.venv/bin/python -m unittest tests.test_alert_analysis_agent tests.test_alert_analysis_graph -v
```

Expected: failures tied to missing structured output or routing behavior.

- [ ] **Step 3: Implement the alert workflow on real model access**

Add:

- OpenAI-compatible chat completion wrapper
- prompt assembly for alert analysis
- optional RAG/tool enrichment
- strict structured output normalization back into proto response shape

Do not remove Go fallback behavior yet; only stop relying on it for the happy path.

- [ ] **Step 4: Re-run the focused Python tests until they pass**

Run the same command as Step 2.

- [ ] **Step 5: Add or update Go integration coverage for alert analysis**

Extend `server_integration_test.go` so the test shape now expects:

- Python runtime path is attempted through gateway client abstraction
- failure path still degrades cleanly
- success path preserves structured fields used by the frontend

- [ ] **Step 6: Run the focused Go integration test**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall/backend/go-api
env GOPROXY=https://proxy.golang.org,direct GOCACHE=/Users/ouyangzhenguang/project/OnCall/tmp/go-build GOMODCACHE=/Users/ouyangzhenguang/project/OnCall/tmp/go-mod-cache go test ./internal/platform/httpserver -run TestIntegrationCoreWorkflows -v
```

- [ ] **Step 7: Commit the alert analysis migration**

```bash
git add backend/python-ai/app/agents/router_agent.py backend/python-ai/app/agents/alert_analysis_agent.py backend/python-ai/app/agents/tool_agent.py backend/python-ai/app/graphs/router_graph.py backend/python-ai/app/graphs/alert_analysis_graph.py backend/python-ai/app/grpc/services/runtime_service.py backend/python-ai/app/llm/openai_compatible.py backend/python-ai/app/services/hybrid_rag.py backend/python-ai/tests/test_alert_analysis_agent.py backend/python-ai/tests/test_alert_analysis_graph.py backend/go-api/internal/ai/analyzer/service.go backend/go-api/internal/platform/httpserver/server_integration_test.go
git commit -m "feat: migrate alert analysis to ai runtime"
```

---

### Task 5: Migrate Chat QA Onto Router Plus Chat QA Agent

**Files:**
- Modify: `backend/python-ai/app/agents/router_agent.py`
- Modify: `backend/python-ai/app/agents/chat_qa_agent.py`
- Modify: `backend/python-ai/app/agents/tool_agent.py`
- Modify: `backend/python-ai/app/graphs/chat_qa_graph.py`
- Modify: `backend/python-ai/app/grpc/services/runtime_service.py`
- Modify: `backend/python-ai/app/services/hybrid_rag.py`
- Modify: `backend/go-api/internal/session/api/handler.go`
- Modify: `backend/go-api/internal/ai/gateway/client.go`
- Modify: `backend/go-api/internal/session/application/service_test.go`
- Modify: `backend/go-api/internal/platform/httpserver/server_integration_test.go`
- Create: `backend/python-ai/tests/test_chat_qa_agent.py`
- Create: `backend/python-ai/tests/test_chat_qa_graph.py`

- [ ] **Step 1: Write failing Python tests for chat QA behavior**

Cover:

- Router selects chat path for conversation turns
- Chat QA Agent can consume recent history
- Chat QA Agent returns `answer`, `citations`, `tool_calls`, and `status`
- RAG integration can be turned on without breaking no-hit flows

- [ ] **Step 2: Run the focused Python tests and verify they fail for missing chat workflow**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall/backend/python-ai
./.venv/bin/python -m unittest tests.test_chat_qa_agent tests.test_chat_qa_graph -v
```

Expected: failures showing missing chat routing or response shaping.

- [ ] **Step 3: Implement Chat QA Agent and graph**

Use the new runtime pieces to:

- pass recent message history from Go into Python
- call RAG adapter when needed
- call Tool Agent when needed
- synthesize final answer plus citations

Keep the response fully buffered in Go and preserve the current SSE chunking behavior.

- [ ] **Step 4: Re-run the focused Python tests until they pass**

Run the same command as Step 2.

- [ ] **Step 5: Write or update failing Go tests for session handler behavior**

Cover:

- session handler no longer builds answers locally
- assistant message content, citations, and failure states come from gateway runtime responses
- SSE still emits `chunk` then `done`

- [ ] **Step 6: Run the focused Go tests and verify they fail for the old local-answer path**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall/backend/go-api
env GOPROXY=https://proxy.golang.org,direct GOCACHE=/Users/ouyangzhenguang/project/OnCall/tmp/go-build GOMODCACHE=/Users/ouyangzhenguang/project/OnCall/tmp/go-mod-cache go test ./internal/session ./internal/platform/httpserver -run 'Test.*Session|TestIntegrationCoreWorkflows' -v
```

Expected: failures because the handler still uses local retrieval-and-build behavior.

- [ ] **Step 7: Implement the Go session-handler switch**

Replace local answer generation in `session/api/handler.go` with runtime-backed conversation-turn handling while keeping:

- current API path
- current SSE event names
- current message persistence model

- [ ] **Step 8: Re-run the focused Go tests until they pass**

Run the same command as Step 6.

- [ ] **Step 9: Commit the chat QA migration**

```bash
git add backend/python-ai/app/agents/router_agent.py backend/python-ai/app/agents/chat_qa_agent.py backend/python-ai/app/agents/tool_agent.py backend/python-ai/app/graphs/chat_qa_graph.py backend/python-ai/app/grpc/services/runtime_service.py backend/python-ai/app/services/hybrid_rag.py backend/python-ai/tests/test_chat_qa_agent.py backend/python-ai/tests/test_chat_qa_graph.py backend/go-api/internal/session/api/handler.go backend/go-api/internal/ai/gateway/client.go backend/go-api/internal/session/application/service_test.go backend/go-api/internal/platform/httpserver/server_integration_test.go
git commit -m "feat: migrate chat qa to ai runtime"
```

---

### Task 6: Stabilize Routing, Traceability, and Dev Runtime Startup

**Files:**
- Modify: `backend/python-ai/app/grpc/server.py`
- Modify: `backend/python-ai/app/grpc/services/runtime_service.py`
- Modify: `backend/python-ai/app/graphs/state.py`
- Modify: `backend/python-ai/app/agents/router_agent.py`
- Modify: `backend/python-ai/app/api/routes.py`
- Modify: `backend/python-ai/app/services/health.py`
- Modify: `backend/python-ai/README.md`
- Modify: `README.md`
- Modify: `backend/python-ai/.env.example`
- Modify: `scripts/dev/python-ai.sh`
- Create or modify: `backend/python-ai/tests/test_runtime_service.py`

- [ ] **Step 1: Write failing tests for route trace and health exposure**

Cover:

- route trace is included in both runtime responses
- health reflects runtime readiness, model config visibility, and gRPC availability assumptions

- [ ] **Step 2: Run the focused Python tests and verify they fail**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall/backend/python-ai
./.venv/bin/python -m unittest tests.test_runtime_service -v
```

Expected: failures about missing trace fields or health diagnostics.

- [ ] **Step 3: Implement trace normalization and startup polish**

Add:

- consistent trace-event formatting
- route decision reporting
- startup wiring so FastAPI and gRPC runtime can coexist in local development
- health responses that mention runtime components rather than only placeholder registries

- [ ] **Step 4: Re-run the focused Python tests until they pass**

Run the same command as Step 2.

- [ ] **Step 5: Update docs and env examples**

Document:

- required model env vars
- proto generation
- runtime startup expectations
- current limitation that Go still streams by chunking full answers

- [ ] **Step 6: Verify Python syntax and docs-aligned startup wiring**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall/backend/python-ai
./.venv/bin/python -m compileall app
```

- [ ] **Step 7: Commit the runtime stabilization**

```bash
git add backend/python-ai/app/grpc/server.py backend/python-ai/app/grpc/services/runtime_service.py backend/python-ai/app/graphs/state.py backend/python-ai/app/agents/router_agent.py backend/python-ai/app/api/routes.py backend/python-ai/app/services/health.py backend/python-ai/README.md backend/python-ai/.env.example README.md scripts/dev/python-ai.sh backend/python-ai/tests/test_runtime_service.py
git commit -m "chore: stabilize ai runtime startup and tracing"
```

---

### Task 7: Run Final Cross-Language Verification

**Files:**
- No new source files expected
- Modify only if verification exposes real contract mismatches

- [ ] **Step 1: Regenerate protobuf stubs from a clean state**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall
./scripts/proto/gen.sh
```

- [ ] **Step 2: Run Go package verification**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall/backend/go-api
env GOPROXY=https://proxy.golang.org,direct GOCACHE=/Users/ouyangzhenguang/project/OnCall/tmp/go-build GOMODCACHE=/Users/ouyangzhenguang/project/OnCall/tmp/go-mod-cache go test ./...
```

- [ ] **Step 3: Run Python package verification**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall/backend/python-ai
./.venv/bin/python -m unittest discover -s tests -v
./.venv/bin/python -m compileall app
```

- [ ] **Step 4: Run front-to-back compatibility verification**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall/frontend/web
npm run build
```

Expected: frontend still compiles without API shape regressions.

- [ ] **Step 5: Only if needed, apply minimal fixups and re-run affected checks**

Do not widen scope here. Fix only contract mismatches found by the checks above.

- [ ] **Step 6: Commit final verification fixups**

```bash
git add .
git commit -m "test: verify ai runtime integration"
```
