# AI OnCall Showcase Guide

## 1. What This Project Is

AI OnCall is a full-stack intelligent on-call platform for incident response scenarios.

It combines:

- Vue 3 web console
- Go business backend
- Python AI service
- RAG-style knowledge retrieval
- Alert analysis and troubleshooting workflow
- Tool execution and audit trail

Project positioning:

- It is not only a chat demo.
- It is a business-oriented operations platform with AI-enhanced workflows.

## 2. Project Highlights

### 2.1 Full-stack closed loop

The system covers:

- login
- session chat
- knowledge ingestion
- retrieval
- alert center
- alert analysis
- tool center
- audit and operations insight

This is stronger than a single isolated RAG demo.

### 2.2 Clear service boundaries

The system is deliberately split into:

- Vue frontend
- Go business backend
- Python AI service

This makes it easier to explain engineering boundaries in interviews.

### 2.3 Business-first AI integration

AI is integrated into concrete business flows:

- alert analysis
- knowledge retrieval
- troubleshooting chat
- tool recommendation

Instead of presenting AI as a detached toy capability.

### 2.4 Progressive delivery strategy

The project was built phase by phase:

- first make the chain runnable
- then improve persistence, lifecycle, filtering, diagnostics, and integration coverage

This shows pragmatic engineering decision-making.

## 3. Technical Challenges and Tradeoffs

### 3.1 Why file persistence first

Tradeoff:

- We chose local JSON persistence first for sessions, knowledge, alerts, tool logs, and audit logs.

Reason:

- It reduced early implementation cost.
- It let us stabilize domain models and UI flows before introducing database complexity.

Cost:

- It is not suitable for multi-instance deployment or strong concurrency guarantees.

### 3.2 Why current RAG is still incomplete

Current state:

- precomputed text chunks
- keyword and partial-match retrieval
- citation rendering
- retrieval diagnostics

Not yet included:

- embeddings
- Milvus vector recall
- Elasticsearch keyword index sync
- hybrid retrieval
- rerank
- real LLM answer synthesis

Reason:

- We prioritized proving the retrieval chain and product flow first.

### 3.3 Why AI analysis is not yet real LLM inference

Current state:

- Go calls Python AI service
- Python returns structured analysis using prompt-oriented structure plus rules

Reason:

- This locks the API and workflow boundary before introducing external model dependency and cost.

Meaning:

- The architecture is ready for real model integration later without changing the business API shape.

### 3.4 Why Go and Python coexist

Go owns:

- business APIs
- auth
- sessions
- knowledge metadata
- alerts
- tools
- audit

Python owns:

- AI workflow boundary
- analysis service
- future LangChain/LangGraph orchestration

This split is practical and easy to explain.

## 4. What Is Already Complete

- Full frontend shell and main pages
- JWT-style login flow
- Session chat and SSE streaming
- Knowledge upload, processing, preview, retry, delete
- Retrieval testing and citation rendering
- Alert ingest, detail, timeline, linked session
- Tool center and tool call logs
- Audit center and audit statistics
- Integration tests for main backend chains
- Demo deployment assets with Dockerfiles and demo scripts

## 5. What Should Be Improved Next

Priority order:

1. Upgrade RAG to embedding + Milvus + Elasticsearch hybrid retrieval
2. Replace rule-based alert analysis with real OpenAI-compatible model inference
3. Migrate file-backed business data into MySQL
4. Add real RBAC and user persistence
5. Upgrade tool center from built-in tools to external system adapters

## 6. Resume Version

### 6.1 Medium-length version

Built an AI-driven on-call platform with `Vue 3 + Go + Python`, covering authentication, session chat, knowledge management, retrieval-based troubleshooting, alert center, tool invocation, audit logging, integration testing, and containerized demo deployment. Designed clear business and AI service boundaries, implemented SSE chat streaming, structured alert analysis, retrieval diagnostics, and phased delivery from runnable MVP to multi-module platform.

### 6.2 Short version

Built a full-stack AI OnCall platform with `Vue 3 + Gin + FastAPI`, including chat, knowledge retrieval, alert analysis, tool center, audit trail, and Dockerized demo environment.

## 7. Interview Talking Points

### 7.1 One-minute introduction

This project is a full-stack AI OnCall platform for operations scenarios. I used Vue 3 for the console, Go for the core business backend, and Python for the AI service boundary. The system supports login, chat, knowledge ingestion, retrieval testing, alert management, alert analysis, tool invocation, and audit logging. The main design goal was not just to build a chatbot, but to build a complete business platform where AI is embedded into on-call workflows.

### 7.2 Three-minute structure

You can explain it in this order:

1. Business goal
   Help engineers handle alerts, troubleshoot with knowledge, use tools, and retain an audit trail.

2. Architecture
   Frontend with Vue, business orchestration in Go, AI capability in Python, and search/vector infrastructure as side systems.

3. Main chains
   Session chat, knowledge ingestion and retrieval, alert center and AI analysis, tool invocation, audit logging.

4. Tradeoffs
   Early phases used file-backed persistence and rule-based AI analysis to stabilize product flows first. The architecture is ready for MySQL, real RAG, and real LLM inference later.

## 8. How To Demo It

Recommended order:

1. Login with demo account
2. Open dashboard and explain the module layout
3. Upload one knowledge document and show it becoming ready
4. Run retrieval test and show citations
5. Open alert center, inspect one alert, and generate AI analysis
6. Enter linked troubleshooting session from the alert
7. Open tool center and call `service_status` or `knowledge_search`
8. Open audit page and show cross-module operation records

## 9. Most Important Message

The strongest way to present this project is:

- not "I made a chat UI"
- not "I wrapped a model API"

Instead:

- "I built a full-stack AI operations platform with explicit service boundaries, business workflows, retrieval, analysis, tooling, auditability, and deployable demo assets."
