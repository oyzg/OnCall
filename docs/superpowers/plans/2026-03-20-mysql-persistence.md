# MySQL Persistence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move sessions/messages, knowledge metadata/chunks, and alerts/handling records from local JSON persistence to MySQL-backed storage without changing HTTP APIs.

**Architecture:** Add a shared GORM-backed MySQL layer under `internal/platform/db`, introduce repository implementations per domain, and switch the HTTP server composition root to build services from those repositories. Keep raw files, RAG indexing, tool logs, and audit logs on their current storage paths.

**Tech Stack:** Go, Gin, GORM, MySQL, existing integration tests

---

### Task 1: Add failing repository-backed integration tests

**Files:**
- Modify: `backend/go-api/internal/platform/httpserver/server_integration_test.go`
- Create: `backend/go-api/internal/platform/db/mysql_test.go`

- [ ] Step 1: Add tests that expect session creation, message streaming, knowledge upload metadata, and seeded alerts to persist through repository-backed services.
- [ ] Step 2: Run `go test ./internal/platform/httpserver ./internal/platform/db` and confirm failure because MySQL-backed persistence is not implemented.

### Task 2: Add MySQL infrastructure and models

**Files:**
- Modify: `backend/go-api/go.mod`
- Create: `backend/go-api/internal/platform/db/mysql.go`
- Create: `backend/go-api/internal/platform/db/models/*.go`
- Modify: `backend/go-api/internal/platform/db/migrator.go`

- [ ] Step 1: Add GORM/MySQL dependencies.
- [ ] Step 2: Implement DB open/ping helpers and AutoMigrate registrations.
- [ ] Step 3: Add models for sessions/messages/message_references, knowledge_documents/knowledge_chunks, alerts/alert_handling_records.
- [ ] Step 4: Run focused DB tests.

### Task 3: Migrate session storage to MySQL repositories

**Files:**
- Modify: `backend/go-api/internal/session/application/service.go`
- Create: `backend/go-api/internal/session/application/repository.go`
- Create: `backend/go-api/internal/session/infrastructure/mysql_repository.go`

- [ ] Step 1: Add repository interface and wire service to it.
- [ ] Step 2: Implement MySQL repository for sessions, messages, and references.
- [ ] Step 3: Run session-related integration tests.

### Task 4: Migrate knowledge metadata/chunks to MySQL repositories

**Files:**
- Modify: `backend/go-api/internal/knowledge/application/service.go`
- Create: `backend/go-api/internal/knowledge/application/repository.go`
- Create: `backend/go-api/internal/knowledge/infrastructure/mysql_repository.go`

- [ ] Step 1: Keep raw file storage on disk, but move document metadata and chunk persistence to MySQL.
- [ ] Step 2: Preserve current indexing hooks and API behavior.
- [ ] Step 3: Run knowledge and retrieval tests.

### Task 5: Migrate alerts and handling records to MySQL repositories

**Files:**
- Modify: `backend/go-api/internal/alert/application/service.go`
- Create: `backend/go-api/internal/alert/application/repository.go`
- Create: `backend/go-api/internal/alert/infrastructure/mysql_repository.go`

- [ ] Step 1: Move alerts and handling records to MySQL.
- [ ] Step 2: Preserve seed, analyze, and link-session behavior.
- [ ] Step 3: Run alert integration tests.

### Task 6: Wire composition root and verify end-to-end

**Files:**
- Modify: `backend/go-api/internal/platform/httpserver/server.go`
- Modify: `backend/go-api/pkg/config/config.go`
- Modify: `backend/go-api/.env.example`
- Modify: `docs/database-design.md`

- [ ] Step 1: Build DB connection and repositories in the HTTP server composition root.
- [ ] Step 2: Keep startup healthy when MySQL is unavailable by failing loudly in tests and dev logs.
- [ ] Step 3: Run `go test ./...` and document the MySQL source-of-truth transition.
