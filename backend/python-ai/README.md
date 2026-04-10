# Python AI Service

This service hosts the AI runtime for AI OnCall:

- FastAPI for HTTP health/debug endpoints
- gRPC `RuntimeService` for Go-to-Python runtime calls
- LangGraph-style routing for alert analysis and chat QA
- OpenAI-compatible model access plus hybrid RAG

## Environment

Copy the example file before local development:

```bash
cp .env.example .env
```

Key runtime variables:

```env
HTTP_PORT=8000
GRPC_HOST=0.0.0.0
GRPC_PORT=50051
OPENAI_BASE_URL=https://api.openai.com/v1
OPENAI_API_KEY=your_api_key
RUNTIME_API_MODEL=gpt-4.1-mini
RUNTIME_API_TIMEOUT_SECONDS=30
EMBEDDING_PROVIDER=openai_compatible
EMBEDDING_API_MODEL=text-embedding-3-small
EMBEDDING_DIMENSION=1536
```

If `OPENAI_API_KEY` is missing, the runtime falls back to deterministic local responses. That keeps development and tests runnable, but it is not real model inference.

## Local Startup

Start the service with the repo script:

```bash
../../scripts/dev/python-ai.sh
```

The FastAPI app starts the gRPC runtime in its lifespan hook, so local development uses one Python process for both HTTP and gRPC.

## Runtime Contract

Regenerate protobuf stubs after contract changes:

```bash
cd ../..
./scripts/proto/gen.sh
```

Current runtime behavior:

- Alert analysis is routed through the runtime and returns structured fields plus trace events
- Chat QA is routed through the runtime and Go still streams by chunking the fully buffered answer
- `/healthz` reports runtime model, gRPC endpoint assumptions, embedding readiness, Elasticsearch, and Milvus state
