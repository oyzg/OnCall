# AI OnCall Architecture Diagrams

## 1. System Context

```mermaid
flowchart LR
    User["Web User"] --> Web["Vue 3 Web Console"]
    Web --> Go["Go API (Gin)"]
    Go --> Py["Python AI (FastAPI)"]
    Go --> MySQL["MySQL (target)"]
    Go --> Redis["Redis (target)"]
    Go --> ES["Elasticsearch"]
    Go --> Milvus["Milvus"]
    Go --> MinIO["MinIO / Object Storage"]
    Py --> LLM["OpenAI-compatible LLM (planned)"]
    Py --> ES
    Py --> Milvus
```

## 2. Current Layered Architecture

```mermaid
flowchart TB
    subgraph Frontend["Frontend"]
        Pages["Pages"]
        Stores["Pinia Stores"]
        API["HTTP Services"]
    end

    subgraph Backend["Go API"]
        Router["Gin Router"]
        Middleware["Middleware"]
        App["Application Services"]
        Domain["Domain Models"]
        Persistence["Current File Persistence / Future DB Repos"]
    end

    subgraph AI["Python AI"]
        FastAPI["FastAPI Routes"]
        Chains["LangChain Prompt/Chain Layer"]
        Graphs["LangGraph Placeholder"]
        Rules["Rule-based Analysis"]
    end

    subgraph Infra["Infra"]
        Files["Local JSON Stores"]
        Search["Elasticsearch"]
        Vector["Milvus"]
        Cache["Redis"]
        DB["MySQL"]
    end

    Pages --> Stores --> API
    API --> Router
    Router --> Middleware --> App --> Domain --> Persistence
    App --> FastAPI --> Chains --> Rules
    Persistence --> Files
    App --> Search
    App --> Vector
    App --> Cache
    App --> DB
```

## 3. Login Flow

```mermaid
sequenceDiagram
    participant U as User
    participant F as Frontend
    participant G as Go API
    participant A as Audit

    U->>F: submit username/password
    F->>G: POST /api/v1/auth/login
    G->>G: validate credentials
    G->>A: record login audit
    G-->>F: access_token + user
    F->>F: save token and auth state
```

## 4. Chat and Retrieval Flow

```mermaid
sequenceDiagram
    participant U as User
    participant F as Frontend
    participant G as Go API
    participant R as Retrieval
    participant K as Knowledge Store

    U->>F: send message
    F->>G: POST /sessions/:id/messages/stream
    G->>R: retrieve(query)
    R->>K: read ready chunks
    K-->>R: chunk list
    R-->>G: answer + references
    G-->>F: SSE chunk events
    G-->>F: SSE done event
    F->>F: render answer and citations
```

## 5. Knowledge Ingestion Flow

```mermaid
sequenceDiagram
    participant U as User
    participant F as Frontend
    participant G as Go API
    participant K as Knowledge Service
    participant FS as Local Storage

    U->>F: upload file or text
    F->>G: POST /knowledge/documents
    G->>K: create document record
    K->>FS: write raw file
    K->>K: async process document
    K->>FS: write chunk file
    K->>K: update status to ready
    G-->>F: document metadata
```

## 6. Alert Analysis Flow

```mermaid
sequenceDiagram
    participant U as User
    participant F as Frontend
    participant G as Go API
    participant P as Python AI
    participant AU as Audit

    U->>F: click analyze alert
    F->>G: POST /alerts/:id/analyze
    G->>P: send alert payload
    P->>P: prompt + rule analysis
    P-->>G: structured analysis result
    G->>G: persist analysis on alert
    G->>AU: record audit log
    G-->>F: alert detail with analysis
```

## 7. Tool Invocation Flow

```mermaid
sequenceDiagram
    participant U as User
    participant F as Frontend
    participant G as Go API
    participant T as Tool Service
    participant AU as Audit

    U->>F: call tool
    F->>G: POST /tools/:toolName/call
    G->>T: validate params and roles
    T->>T: execute built-in tool
    T->>T: persist tool call log
    G->>AU: record audit log
    G-->>F: tool result
```

## 8. Architecture Notes

- Current persistence is file-backed for speed of iteration.
- The target architecture is still `Vue + Go API + Python AI + MySQL + Redis + Elasticsearch + Milvus`.
- Go is the business orchestration center.
- Python AI is the AI capability service boundary.
- Search and vector infrastructure are designed as retrievable side systems, not the source of truth.
