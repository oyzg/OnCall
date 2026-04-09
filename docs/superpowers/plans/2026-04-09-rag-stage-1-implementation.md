# RAG Stage 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver stage-1 hybrid RAG so knowledge indexing and retrieval use real embedding, Elasticsearch lexical recall, Milvus vector recall, and unified Go-facing reports.

**Architecture:** Keep Go as the business-facing retrieval boundary and Python as the AI retrieval implementation. Validate stage-1 through failing tests first, then tighten Python indexing/retrieval behavior and Go gateway/report compatibility without leaking storage-engine details into business handlers.

**Tech Stack:** Go, Gin, FastAPI, Elasticsearch, Milvus, OpenAI-compatible embeddings, project-local Go tests, Python compile verification

---

### Task 1: Lock Stage-1 Retrieval Contract

**Files:**
- Modify: `backend/go-api/internal/ai/retrieval/service_test.go`
- Modify: `backend/go-api/internal/ai/gateway/client.go`
- Modify: `backend/go-api/internal/ai/retrieval/service.go`

- [ ] **Step 1: Write failing Go test for remote hybrid report consumption**

Add a test that injects a stub remote retriever returning populated backend fields and verifies `RetrieveWithOptions` preserves `strategy`, backend names, and reference scoring fields.

- [ ] **Step 2: Run the focused Go test and verify it fails for the intended reason**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall/backend/go-api
env GOPROXY=https://proxy.golang.org,direct GOCACHE=/Users/ouyangzhenguang/project/OnCall/tmp/go-build GOMODCACHE=/Users/ouyangzhenguang/project/OnCall/tmp/go-mod-cache go test ./internal/ai/retrieval -run TestRetrieveWithOptionsUsesRemoteHybridReport -v
```

Expected: failure showing missing or incorrect remote report preservation.

- [ ] **Step 3: Implement the minimal Go changes**

Ensure the remote gateway/report path preserves all stage-1 fields needed by the UI and diagnostics.

- [ ] **Step 4: Re-run the focused Go test until it passes**

Run the same command as Step 2.

- [ ] **Step 5: Keep the retrieval package green**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall/backend/go-api
env GOPROXY=https://proxy.golang.org,direct GOCACHE=/Users/ouyangzhenguang/project/OnCall/tmp/go-build GOMODCACHE=/Users/ouyangzhenguang/project/OnCall/tmp/go-mod-cache go test ./internal/ai/retrieval -v
```

---

### Task 2: Validate Python Hybrid Index/Retrieve Behavior

**Files:**
- Create: `backend/python-ai/tests/test_hybrid_rag.py`
- Modify: `backend/python-ai/app/services/hybrid_rag.py`
- Modify: `backend/python-ai/pyproject.toml`

- [ ] **Step 1: Write failing Python tests for stage-1 fallback and report fields**

Cover:
- local fallback retrieval returns non-empty references for seeded chunks
- `build_strategy_name` returns the expected stage-1 labels
- `build_answer` describes active backends correctly

- [ ] **Step 2: Run the focused Python tests and verify they fail correctly**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall/backend/python-ai
./.venv/bin/python -m unittest tests.test_hybrid_rag -v
```

Expected: failures tied to missing test dependencies or behavior mismatches, not syntax/import typos.

- [ ] **Step 3: Implement the minimal Python changes**

Tighten helper behavior and any stage-1 retrieval/report logic needed to satisfy the tests.

- [ ] **Step 4: Re-run the focused Python tests until they pass**

Run the same command as Step 2.

- [ ] **Step 5: Keep syntax verification green**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall/backend/python-ai
./.venv/bin/python -m compileall app
```

---

### Task 3: Verify End-to-End Stage-1 Compatibility

**Files:**
- Modify: `README.md` (only if behavior/config docs need correction)
- Modify: `backend/python-ai/.env.example` (only if new settings are required for stage-1)

- [ ] **Step 1: Re-read config expectations against implementation**

Confirm embedding provider, model, dimension, Elasticsearch URL, and Milvus collection naming are documented consistently.

- [ ] **Step 2: Run focused Go verification**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall/backend/go-api
env GOPROXY=https://proxy.golang.org,direct GOCACHE=/Users/ouyangzhenguang/project/OnCall/tmp/go-build GOMODCACHE=/Users/ouyangzhenguang/project/OnCall/tmp/go-mod-cache go test ./internal/ai/retrieval ./internal/platform/httpserver -run 'TestRetrieveWithOptionsUses|TestServer' -v
```

- [ ] **Step 3: Run focused Python verification**

Run:

```bash
cd /Users/ouyangzhenguang/project/OnCall/backend/python-ai
./.venv/bin/python -m unittest tests.test_hybrid_rag -v
./.venv/bin/python -m compileall app
```

- [ ] **Step 4: Update docs if verification exposed config mismatches**

Adjust only the minimum required docs/config examples.
