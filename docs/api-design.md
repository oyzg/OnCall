# AI OnCall API Design

## 1. Overview

This document summarizes the current HTTP API exposed by the Go backend.

Base URL:

- Local: `http://127.0.0.1:8080`

Protected API prefix:

- `/api/v1`

Authentication:

- `Authorization: Bearer <access_token>`

## 2. Unified Response Format

Normal JSON responses use one envelope:

```json
{
  "code": "OK",
  "message": "success",
  "request_id": "req_xxx",
  "data": {}
}
```

Failure responses follow the same shape:

```json
{
  "code": "BAD_REQUEST",
  "message": "username and password are required",
  "request_id": "req_xxx"
}
```

Exception:

- `POST /api/v1/sessions/:sessionID/messages/stream` returns `text/event-stream`

## 3. Public and Health Endpoints

### `GET /`

Purpose:

- Basic service info

### `GET /healthz`

Purpose:

- Go API health status and dependency probes

### `GET /proxy/python-ai/healthz`

Purpose:

- Temporary proxy for frontend dashboard health check

## 4. Auth APIs

### `POST /api/v1/auth/login`

Purpose:

- Login and get access token

Request:

```json
{
  "username": "admin",
  "password": "OnCallAdmin2026!"
}
```

Response data:

```json
{
  "access_token": {
    "token": "xxx",
    "expires_at": "2026-03-17T10:00:00Z"
  },
  "user": {
    "id": "user_admin",
    "username": "admin",
    "display_name": "Platform Admin",
    "roles": ["admin"]
  }
}
```

### `GET /api/v1/auth/me`

Purpose:

- Return current authenticated user

Response data:

```json
{
  "user": {
    "id": "user_admin",
    "username": "admin",
    "display_name": "Platform Admin",
    "roles": ["admin"]
  }
}
```

## 5. Session APIs

### `GET /api/v1/sessions`

Purpose:

- List current user's sessions

Query parameters:

- `query`: fuzzy match by title
- `limit`: optional max item count

Response data:

```json
{
  "sessions": []
}
```

### `POST /api/v1/sessions`

Purpose:

- Create a new session

Request:

```json
{
  "title": "payment-api latency issue"
}
```

### `GET /api/v1/sessions/:sessionID/messages`

Purpose:

- Query message history

Query parameters:

- `limit`: page size
- `before_id`: cursor for older messages

Response data:

```json
{
  "messages": [],
  "total": 20,
  "has_more": false,
  "next_cursor": ""
}
```

### `POST /api/v1/sessions/:sessionID/messages/stream`

Purpose:

- Send one user message and stream assistant reply

Request:

```json
{
  "content": "payment-api p95 increased, what should I check first?"
}
```

SSE events:

- `chunk`
- `done`

Example:

```text
event: chunk
data: {"message_id":"msg_xxx","delta":"基于知识库检索，"}

event: done
data: {"message_id":"msg_xxx","content":"...","references":[...]}
```

### `DELETE /api/v1/sessions/:sessionID`

Purpose:

- Delete one session

## 6. Knowledge APIs

### `GET /api/v1/knowledge/documents`

Purpose:

- List uploaded documents

Query parameters:

- `status`
- `category`
- `query`
- `limit`

### `POST /api/v1/knowledge/documents`

Purpose:

- Upload file or text content

Content type:

- `multipart/form-data`

Form fields:

- `title`
- `category`
- `content`
- `file`

Rules:

- `content` or `file` must provide at least one

### `GET /api/v1/knowledge/documents/:documentID`

Purpose:

- Get one document detail

### `DELETE /api/v1/knowledge/documents/:documentID`

Purpose:

- Delete one document

### `POST /api/v1/knowledge/documents/:documentID/reprocess`

Purpose:

- Retry failed or uploaded document processing

## 7. Retrieval API

### `POST /api/v1/rag/retrieve`

Purpose:

- Run current retrieval flow without going through chat

Request:

```json
{
  "query": "user-service 5xx ratio rise",
  "category": "general",
  "limit": 3
}
```

Response fields:

- `query`
- `answer`
- `references`
- `scanned_docs`
- `scanned_chunks`
- `matched_chunks`
- `strategy`
- `requested_limit`

Current strategy:

- `precomputed_text_chunks`

## 8. Alert APIs

### `POST /api/v1/alerts/ingest`

Purpose:

- Ingest one external alert

Request:

```json
{
  "title": "payment-api p95 latency spike",
  "service": "payment-api",
  "environment": "prod",
  "severity": "P1",
  "source": "prometheus",
  "summary": "5 minute p95 over 2s",
  "description": "check dependencies and db pool first",
  "labels": {
    "metric": "http_server_duration_p95"
  }
}
```

### `GET /api/v1/alerts`

Purpose:

- List alerts and aggregate stats

Query parameters:

- `status`
- `severity`
- `service`
- `query`

Response data:

- `alerts`
- `stats`

### `GET /api/v1/alerts/stats`

Purpose:

- Get alert summary only

### `GET /api/v1/alerts/:alertID`

Purpose:

- Get alert detail, handling timeline, and stats snapshot

### `POST /api/v1/alerts/:alertID/status`

Purpose:

- Update alert status

Request:

```json
{
  "status": "investigating",
  "comment": "started checking upstream dependencies"
}
```

### `POST /api/v1/alerts/:alertID/session`

Purpose:

- Create or reuse linked troubleshooting session

### `POST /api/v1/alerts/:alertID/analyze`

Purpose:

- Generate or refresh AI analysis

Notes:

- Current implementation calls Python AI service over HTTP
- Analysis currently uses prompt plus rule-based synthesis, not a real external LLM yet

## 9. Tool APIs

### `GET /api/v1/tools`

Purpose:

- List tools visible to current user

### `POST /api/v1/tools/:toolName/call`

Purpose:

- Invoke one tool

Request:

```json
{
  "parameters": {
    "service": "payment-api"
  }
}
```

Current built-in tools:

- `service_status`
- `recent_alerts`
- `knowledge_search`
- `platform_overview`

### `GET /api/v1/tools/logs`

Purpose:

- Query tool execution logs

Query parameters:

- `tool_name`
- `status`
- `limit`

## 10. Audit APIs

### `GET /api/v1/audit/logs`

Purpose:

- Query audit trail

Query parameters:

- `category`
- `action`
- `status`
- `actor`
- `limit`

### `GET /api/v1/audit/stats`

Purpose:

- Get audit statistics summary

## 11. Common Error Codes

Common codes currently used:

- `BAD_REQUEST`
- `UNAUTHORIZED`
- `FORBIDDEN`
- `NOT_FOUND`
- `INVALID_CREDENTIALS`
- `TOOL_EXECUTION_FAILED`
- `INTERNAL`

## 12. Known Gaps

- No OpenAPI or Swagger generation yet.
- SSE API is documented manually, not schema-generated.
- Some endpoints return nested domain structs directly, so response contracts are not yet fully decoupled from internal models.
- Pagination is only partially standardized.
- Python AI is not yet exposed through gRPC even though gRPC is the long-term decision.
