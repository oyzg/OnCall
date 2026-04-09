from __future__ import annotations

import hashlib
import json
import math
from dataclasses import dataclass
from functools import lru_cache
from typing import Any
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

from app.core.config import AppSettings
from app.core.exceptions import AppError
from app.schemas.rag import (
    RAGChunkInput,
    RAGDeleteRequest,
    RAGDeleteResponse,
    RAGIndexRequest,
    RAGIndexResponse,
    RAGReference,
    RAGRetrieveRequest,
    RAGRetrieveResponse,
)


@dataclass
class QueryPlan:
    normalized: str
    terms: list[str]
    expanded_terms: list[str]
    char_terms: list[str]
    rewritten_query: str


@dataclass
class HybridHit:
    chunk_id: str
    document_id: str
    document_title: str
    category: str
    chunk_index: int
    chunk: str
    lexical_score: float = 0.0
    semantic_score: float = 0.0
    boost_score: float = 0.0
    score: float = 0.0
    match_reasons: list[str] | None = None


class HybridRAGService:
    def __init__(self, settings: AppSettings) -> None:
        self.settings = settings
        self._embedder: Any | None = None
        self._embedder_backend = ""
        self._embedder_error = ""
        self._milvus: Any | None = None
        self._milvus_backend = ""
        self._local_chunks: dict[str, dict[str, Any]] = {}

    def index_document(self, request: RAGIndexRequest) -> RAGIndexResponse:
        if not request.chunks:
            raise AppError("BAD_REQUEST", "chunks are required", 400)

        embeddings = self._embed_texts([chunk.content for chunk in request.chunks])
        self._validate_embeddings(embeddings, len(request.chunks))
        self._store_local_chunks(request, embeddings)

        lexical_backend = "local_fallback"
        vector_backend = "local_fallback"

        try:
            self._ensure_es_index()
            self._index_elasticsearch(request, embeddings)
            lexical_backend = "elasticsearch"
        except Exception:
            lexical_backend = "local_fallback"

        try:
            self._index_milvus(request, embeddings)
            vector_backend = self._milvus_backend or "milvus"
        except Exception:
            vector_backend = "local_fallback"

        return RAGIndexResponse(
            status="indexed",
            indexed_chunks=len(request.chunks),
            embedding_backend=self._embedder_backend_name(),
            vector_backend=vector_backend,
            lexical_backend=lexical_backend,
        )

    def delete_document(self, request: RAGDeleteRequest) -> RAGDeleteResponse:
        prefix = f"{request.document_id}:"
        stale_keys = [chunk_id for chunk_id in self._local_chunks if chunk_id.startswith(prefix)]
        for chunk_id in stale_keys:
            del self._local_chunks[chunk_id]

        try:
            self._delete_from_elasticsearch(request.document_id)
        except Exception:
            pass

        try:
            self._delete_from_milvus(request.document_id)
        except Exception:
            pass

        return RAGDeleteResponse(status="deleted", document_id=request.document_id)

    def retrieve(self, request: RAGRetrieveRequest) -> RAGRetrieveResponse:
        query = request.query.strip()
        limit = request.limit if request.limit > 0 else 4
        plan = build_query_plan(query)
        if not plan.normalized:
            raise AppError("BAD_REQUEST", "query is required", 400)

        lexical_hits, lexical_backend = self._retrieve_lexical(plan, request.category, limit * 4)
        semantic_hits, vector_backend = self._retrieve_semantic(plan, request.category, limit * 4)
        references = self._fuse_hits(lexical_hits, semantic_hits, plan, limit)

        scanned_chunks = len(
            [
                item
                for item in self._local_chunks.values()
                if not request.category or item["category"] == request.category
            ]
        )
        scanned_docs = len(
            {
                item["document_id"]
                for item in self._local_chunks.values()
                if not request.category or item["category"] == request.category
            }
        )

        return RAGRetrieveResponse(
            query=query,
            rewritten_query=plan.rewritten_query,
            query_terms=plan.terms,
            expanded_terms=plan.expanded_terms,
            answer=build_answer(
                query,
                references,
                embedding_backend=self._embedder_backend_name(),
                vector_backend=vector_backend,
                lexical_backend=lexical_backend,
            ),
            references=references,
            scanned_docs=scanned_docs,
            scanned_chunks=scanned_chunks,
            matched_chunks=len(references),
            lexical_candidates=len(lexical_hits),
            semantic_candidates=len(semantic_hits),
            reranked_chunks=len(references),
            strategy=build_strategy_name(self._embedder_backend_name(), lexical_backend, vector_backend),
            requested_limit=limit,
            embedding_backend=self._embedder_backend_name(),
            vector_backend=vector_backend,
            lexical_backend=lexical_backend,
        )

    def _validate_embeddings(self, embeddings: list[list[float]], expected_count: int) -> None:
        if len(embeddings) != expected_count:
            raise AppError(
                "EMBEDDING_COUNT_MISMATCH",
                f"expected {expected_count} embeddings, got {len(embeddings)}",
                500,
            )

        for index, embedding in enumerate(embeddings):
            if len(embedding) != self.settings.embedding_dimension:
                raise AppError(
                    "EMBEDDING_DIMENSION_MISMATCH",
                    (
                        "embedding dimension mismatch at chunk "
                        f"{index}: expected {self.settings.embedding_dimension}, got {len(embedding)}"
                    ),
                    500,
                )

    def _store_local_chunks(self, request: RAGIndexRequest, embeddings: list[list[float]]) -> None:
        prefix = f"{request.document_id}:"
        stale_keys = [chunk_id for chunk_id in self._local_chunks if chunk_id.startswith(prefix)]
        for chunk_id in stale_keys:
            del self._local_chunks[chunk_id]

        for chunk, embedding in zip(request.chunks, embeddings, strict=True):
            chunk_id = build_chunk_id(chunk)
            normalized = normalize_text(chunk.content)
            self._local_chunks[chunk_id] = {
                "chunk_id": chunk_id,
                "document_id": chunk.document_id,
                "document_title": chunk.document_title,
                "category": chunk.category,
                "chunk_index": chunk.index,
                "chunk": chunk.content,
                "normalized": normalized,
                "terms": tokenize(normalized),
                "char_terms": char_terms(normalized),
                "embedding": embedding,
            }

    def _retrieve_lexical(self, plan: QueryPlan, category: str, limit: int) -> tuple[list[HybridHit], str]:
        try:
            hits = self._search_elasticsearch(plan, category, limit)
            return hits, "elasticsearch"
        except Exception:
            return self._search_local_lexical(plan, category, limit), "local_fallback"

    def _retrieve_semantic(self, plan: QueryPlan, category: str, limit: int) -> tuple[list[HybridHit], str]:
        try:
            hits = self._search_milvus(plan, category, limit)
            return hits, self._milvus_backend or "milvus"
        except Exception:
            return self._search_local_semantic(plan, category, limit), "local_fallback"

    def _fuse_hits(
        self,
        lexical_hits: list[HybridHit],
        semantic_hits: list[HybridHit],
        plan: QueryPlan,
        limit: int,
    ) -> list[RAGReference]:
        fused: dict[str, HybridHit] = {}
        window = max(self.settings.rag_fusion_window, 1)

        for rank, hit in enumerate(lexical_hits, start=1):
            current = fused.get(hit.chunk_id) or hit
            current.score += 0.65 / (window + rank)
            current.lexical_score = max(current.lexical_score, hit.lexical_score or hit.score)
            current.match_reasons = unique_strings((current.match_reasons or []) + (hit.match_reasons or []))
            fused[hit.chunk_id] = current

        for rank, hit in enumerate(semantic_hits, start=1):
            current = fused.get(hit.chunk_id) or hit
            current.score += 0.8 / (window + rank)
            current.semantic_score = max(current.semantic_score, hit.semantic_score or hit.score)
            current.match_reasons = unique_strings((current.match_reasons or []) + (hit.match_reasons or []))
            fused[hit.chunk_id] = current

        items = list(fused.values())
        accepted: list[HybridHit] = []
        for item in items:
            item.boost_score = self._boost_score(plan, item)
            item.score = round_score(item.score + item.boost_score * 0.15)
            item.chunk = self._stitch_context(item)
            item.match_reasons = unique_strings((item.match_reasons or []) + self._build_reasons(item))
            if self._accept_hit(plan, item):
                accepted.append(item)

        accepted.sort(key=lambda hit: (-hit.score, hit.document_title, hit.chunk_index))
        if not accepted:
            rescued = self._rescue_semantic_hit(plan, semantic_hits)
            if rescued is not None:
                rescued.boost_score = max(rescued.boost_score, self._boost_score(plan, rescued))
                rescued.score = max(round_score(rescued.semantic_score + rescued.boost_score * 0.2), rescued.score)
                rescued.match_reasons = unique_strings((rescued.match_reasons or []) + ["context_boost"])
                accepted = [rescued]

        results: list[RAGReference] = []
        doc_hits: dict[str, int] = {}
        for item in accepted:
            reranked_score = item.score * math.pow(0.90, doc_hits.get(item.document_id, 0))
            doc_hits[item.document_id] = doc_hits.get(item.document_id, 0) + 1
            results.append(
                RAGReference(
                    document_id=item.document_id,
                    document_title=item.document_title,
                    category=item.category,
                    chunk_index=item.chunk_index,
                    chunk=item.chunk,
                    score=round_score(reranked_score),
                    lexical_score=round_score(item.lexical_score),
                    semantic_score=round_score(item.semantic_score),
                    boost_score=round_score(item.boost_score),
                    match_reasons=explain_match_reasons(item.match_reasons or []),
                )
            )
            if len(results) >= limit:
                break
        return results

    def _rescue_semantic_hit(self, plan: QueryPlan, semantic_hits: list[HybridHit]) -> HybridHit | None:
        title_like_query = any(term in {"sop", "runbook", "procedure"} for term in plan.expanded_terms)
        for hit in semantic_hits:
            normalized = normalize_text(hit.chunk)
            title_normalized = normalize_text(hit.document_title)
            if any(term and (term in normalized or term in title_normalized) for term in plan.expanded_terms):
                return hit
        if title_like_query and semantic_hits:
            return semantic_hits[0]
        return None

    def _accept_hit(self, plan: QueryPlan, hit: HybridHit) -> bool:
        normalized = normalize_text(hit.chunk)
        title_normalized = normalize_text(hit.document_title)
        chunk_terms = tokenize(normalized)
        title_terms = tokenize(title_normalized)
        chunk_char_terms = char_terms(normalized)

        term_overlap = lexical_overlap(plan.expanded_terms, chunk_terms)
        title_term_overlap = lexical_overlap(plan.expanded_terms, title_terms)
        char_overlap = lexical_overlap(plan.char_terms, chunk_char_terms)
        query_length = len(plan.normalized.replace(" ", ""))
        short_query = query_length <= 4

        if plan.normalized and plan.normalized in normalized:
            return True
        if plan.normalized and plan.normalized in title_normalized:
            return True
        if hit.lexical_score >= 0.12:
            return True
        if term_overlap >= 0.12 or title_term_overlap >= 0.12 or char_overlap >= 0.2:
            return True
        if hit.boost_score >= 0.18:
            return True
        if short_query:
            return False
        return hit.semantic_score >= 0.55

    def _build_reasons(self, hit: HybridHit) -> list[str]:
        reasons: list[str] = []
        if hit.lexical_score > 0:
            reasons.append("lexical_hit")
        if hit.semantic_score > 0:
            reasons.append("vector_hit")
        if hit.boost_score > 0:
            reasons.append("context_boost")
        return reasons

    def _boost_score(self, plan: QueryPlan, hit: HybridHit) -> float:
        normalized = normalize_text(hit.chunk)
        title_normalized = normalize_text(hit.document_title)
        title_terms = tokenize(title_normalized)
        score = 0.0
        if plan.normalized and plan.normalized in normalized:
            score += 0.4
        if plan.normalized and plan.normalized in title_normalized:
            score += 0.45
        if plan.terms and any(term == hit.category for term in plan.terms):
            score += 0.1
        if any(term in normalized for term in plan.terms[:2]):
            score += 0.08
        if lexical_overlap(plan.expanded_terms, title_terms) >= 0.12:
            score += 0.3
        return min(score, 1.0)

    def _stitch_context(self, hit: HybridHit) -> str:
        sibling_chunks = [
            item
            for item in self._local_chunks.values()
            if item["document_id"] == hit.document_id
            and abs(item["chunk_index"] - hit.chunk_index) <= 1
        ]
        sibling_chunks.sort(key=lambda item: item["chunk_index"])
        context = " ".join(item["chunk"] for item in sibling_chunks)
        return context.strip() or hit.chunk

    def _embed_texts(self, texts: list[str]) -> list[list[float]]:
        api_embeddings = self._embed_texts_by_api(texts)
        if api_embeddings is not None:
            return api_embeddings

        model = self._get_embedder()
        if model is None:
            self._embedder_backend = "hash_fallback"
            return [hash_embedding(text, self.settings.embedding_dimension) for text in texts]

        embeddings = model.encode(texts, normalize_embeddings=True)
        self._embedder_backend = self.settings.embedding_model_name
        return [list(map(float, row)) for row in embeddings]

    def _embed_texts_by_api(self, texts: list[str]) -> list[list[float]] | None:
        if not self._should_use_api_embeddings():
            return None
        if not self.settings.openai_api_key:
            self._embedder_error = "missing OPENAI_API_KEY"
            return None

        base_url = self.settings.openai_base_url.rstrip("/")
        endpoint = base_url if base_url.endswith("/embeddings") else f"{base_url}/embeddings"
        payload = {
            "model": self.settings.embedding_api_model,
            "input": texts,
        }

        headers = {
            "Content-Type": "application/json",
        }
        if self.settings.openai_api_key:
            headers["Authorization"] = f"Bearer {self.settings.openai_api_key}"

        request = Request(
            url=endpoint,
            method="POST",
            data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
            headers=headers,
        )

        try:
            with urlopen(request, timeout=self.settings.embedding_api_timeout_seconds) as response:
                raw = json.loads(response.read().decode("utf-8"))
        except HTTPError as exc:
            detail = exc.read().decode("utf-8")
            self._embedder_error = detail or str(exc)
            return None
        except Exception as exc:
            self._embedder_error = str(exc)
            return None

        items = raw.get("data", [])
        if len(items) != len(texts):
            self._embedder_error = "embedding api returned unexpected item count"
            return None

        embeddings: list[list[float]] = []
        for item in items:
            embedding = item.get("embedding")
            if not isinstance(embedding, list):
                self._embedder_error = "embedding api returned invalid embedding payload"
                return None
            embeddings.append([float(value) for value in embedding])

        self._embedder_backend = f"openai:{self.settings.embedding_api_model}"
        self._embedder_error = ""
        return embeddings

    def _should_use_api_embeddings(self) -> bool:
        provider = self.settings.embedding_provider.strip().lower()
        if provider == "local":
            return False
        if provider in {"openai", "openai_compatible", "api"}:
            return bool(self.settings.openai_base_url and self.settings.embedding_api_model)
        return bool(self.settings.openai_base_url and self.settings.embedding_api_model)

    def _get_embedder(self) -> Any | None:
        if self._embedder is not None:
            return self._embedder

        try:
            from sentence_transformers import SentenceTransformer

            model_source = self.settings.embedding_model_path or self.settings.embedding_model_name
            self._embedder = SentenceTransformer(
                model_source,
                local_files_only=self.settings.embedding_local_only,
            )
            self._embedder_error = ""
            return self._embedder
        except Exception as exc:
            self._embedder = None
            self._embedder_error = str(exc)
            return None

    def _embedder_backend_name(self) -> str:
        return self._embedder_backend or self.settings.embedding_model_name

    def embedding_health(self) -> tuple[str, str]:
        if self._should_use_api_embeddings():
            base_url = self.settings.openai_base_url.rstrip("/")
            if not base_url or not self.settings.embedding_api_model:
                return "fallback", "hash_fallback (missing embedding api configuration)"
            if not self.settings.openai_api_key:
                return "fallback", "hash_fallback (missing OPENAI_API_KEY)"
            return "up", f"openai:{self.settings.embedding_api_model} via {base_url}"

        embedder = self._get_embedder()
        if embedder is None:
            detail = self._embedder_error or "model unavailable"
            if self.settings.embedding_local_only:
                detail = f"hash_fallback (local_only: {detail})"
            else:
                detail = f"hash_fallback ({detail})"
            return "fallback", detail

        model_source = self.settings.embedding_model_path or self.settings.embedding_model_name
        mode = "local_only" if self.settings.embedding_local_only else "remote_allowed"
        return "up", f"{model_source} ({mode})"

    def _ensure_es_index(self) -> None:
        path = f"/{self.settings.rag_es_index}"
        try:
            self._es_request("GET", path)
        except AppError:
            mapping = {
                "mappings": {
                    "properties": {
                        "chunk_id": {"type": "keyword"},
                        "document_id": {"type": "keyword"},
                        "document_title": {"type": "text"},
                        "category": {"type": "keyword"},
                        "chunk_index": {"type": "integer"},
                        "content": {"type": "text"},
                    }
                }
            }
            self._es_request("PUT", path, mapping)

    def _index_elasticsearch(self, request: RAGIndexRequest, _embeddings: list[list[float]]) -> None:
        bulk_lines: list[str] = []
        for chunk in request.chunks:
            chunk_id = build_chunk_id(chunk)
            bulk_lines.append(json.dumps({"index": {"_index": self.settings.rag_es_index, "_id": chunk_id}}))
            bulk_lines.append(
                json.dumps(
                    {
                        "chunk_id": chunk_id,
                        "document_id": chunk.document_id,
                        "document_title": chunk.document_title,
                        "category": chunk.category,
                        "chunk_index": chunk.index,
                        "content": chunk.content,
                    },
                    ensure_ascii=False,
                )
            )
        payload = "\n".join(bulk_lines) + "\n"
        self._es_request("POST", "/_bulk", payload, {"Content-Type": "application/x-ndjson"})

    def _search_elasticsearch(self, plan: QueryPlan, category: str, limit: int) -> list[HybridHit]:
        filters: list[dict[str, Any]] = []
        if category:
            filters.append({"term": {"category": category}})

        payload = {
            "size": limit,
            "query": {
                "bool": {
                    "must": [
                        {
                            "multi_match": {
                                "query": plan.rewritten_query or plan.normalized,
                                "fields": ["content^3", "document_title^2", "category"],
                            }
                        }
                    ],
                    "filter": filters,
                }
            },
        }
        result = self._es_request("POST", f"/{self.settings.rag_es_index}/_search", payload)
        hits = result.get("hits", {}).get("hits", [])
        max_score = max((hit.get("_score", 0.0) for hit in hits), default=1.0) or 1.0

        items: list[HybridHit] = []
        for hit in hits:
            source = hit.get("_source", {})
            items.append(
                HybridHit(
                    chunk_id=source.get("chunk_id", hit.get("_id", "")),
                    document_id=source.get("document_id", ""),
                    document_title=source.get("document_title", ""),
                    category=source.get("category", ""),
                    chunk_index=int(source.get("chunk_index", 0)),
                    chunk=source.get("content", ""),
                    lexical_score=round_score(float(hit.get("_score", 0.0)) / max_score),
                    match_reasons=["elasticsearch"],
                )
            )
        return items

    def _index_milvus(self, request: RAGIndexRequest, embeddings: list[list[float]]) -> None:
        entities = [
            [build_chunk_id(chunk) for chunk in request.chunks],
            [chunk.document_id for chunk in request.chunks],
            [chunk.document_title[:512] for chunk in request.chunks],
            [chunk.category[:128] for chunk in request.chunks],
            [chunk.index for chunk in request.chunks],
            [chunk.content[:8000] for chunk in request.chunks],
            embeddings,
        ]
        document_id = request.document_id.replace('"', '\\"')
        collection = self._get_milvus_collection()
        try:
            self._write_milvus_entities(collection, document_id, entities)
        except Exception as exc:
            if not self._is_stale_collection_error(exc):
                raise
            self._reset_milvus_collection()
            collection = self._get_milvus_collection()
            self._write_milvus_entities(collection, document_id, entities)

    def _delete_from_milvus(self, document_id: str) -> None:
        collection = self._get_milvus_collection()
        escaped = document_id.replace('"', '\\"')
        collection.delete(f'document_id == "{escaped}"')
        collection.flush()

    def _search_milvus(self, plan: QueryPlan, category: str, limit: int) -> list[HybridHit]:
        collection = self._get_milvus_collection()
        query_vector = self._embed_texts([plan.normalized])[0]
        expr = f'category == "{category}"' if category else None
        search_params = {"metric_type": "COSINE", "params": {}}
        try:
            results = collection.search(
                data=[query_vector],
                anns_field="embedding",
                param=search_params,
                limit=limit,
                output_fields=["chunk_id", "document_id", "document_title", "category", "chunk_index", "content"],
                expr=expr,
            )
        except Exception as exc:
            if not self._is_stale_collection_error(exc):
                raise
            self._reset_milvus_collection()
            collection = self._get_milvus_collection()
            results = collection.search(
                data=[query_vector],
                anns_field="embedding",
                param=search_params,
                limit=limit,
                output_fields=["chunk_id", "document_id", "document_title", "category", "chunk_index", "content"],
                expr=expr,
            )

        hits: list[HybridHit] = []
        for item in results[0]:
            entity = item.entity
            hits.append(
                HybridHit(
                    chunk_id=entity.get("chunk_id"),
                    document_id=entity.get("document_id"),
                    document_title=entity.get("document_title"),
                    category=entity.get("category"),
                    chunk_index=int(entity.get("chunk_index")),
                    chunk=entity.get("content"),
                    semantic_score=round_score(float(item.score)),
                    match_reasons=["milvus"],
                )
            )
        return hits

    def _write_milvus_entities(self, collection: Any, document_id: str, entities: list[list[Any]]) -> None:
        try:
            collection.delete(f'document_id == "{document_id}"')
        except Exception:
            pass
        collection.insert(entities)
        collection.flush()

    def _reset_milvus_collection(self) -> None:
        self._milvus = None
        self._milvus_backend = ""

    def _is_stale_collection_error(self, exc: Exception) -> bool:
        return "collection not found" in str(exc).lower()

    def _get_milvus_collection(self) -> Any:
        if self._milvus is not None:
            return self._milvus

        try:
            from pymilvus import Collection, CollectionSchema, DataType, FieldSchema, connections, utility

            alias = "oncall"
            connections.connect(alias=alias, host=self.settings.milvus_address.split(":")[0], port=self.settings.milvus_address.split(":")[1])

            if not utility.has_collection(self.settings.rag_milvus_collection, using=alias):
                schema = CollectionSchema(
                    [
                        FieldSchema(name="chunk_id", dtype=DataType.VARCHAR, is_primary=True, max_length=128),
                        FieldSchema(name="document_id", dtype=DataType.VARCHAR, max_length=128),
                        FieldSchema(name="document_title", dtype=DataType.VARCHAR, max_length=512),
                        FieldSchema(name="category", dtype=DataType.VARCHAR, max_length=128),
                        FieldSchema(name="chunk_index", dtype=DataType.INT64),
                        FieldSchema(name="content", dtype=DataType.VARCHAR, max_length=8192),
                        FieldSchema(name="embedding", dtype=DataType.FLOAT_VECTOR, dim=self.settings.embedding_dimension),
                    ],
                    description="AI OnCall knowledge chunks",
                )
                collection = Collection(name=self.settings.rag_milvus_collection, schema=schema, using=alias)
                collection.create_index(
                    field_name="embedding",
                    index_params={"index_type": "AUTOINDEX", "metric_type": "COSINE", "params": {}},
                )
            else:
                collection = Collection(name=self.settings.rag_milvus_collection, using=alias)

            collection.load()
            self._milvus = collection
            self._milvus_backend = "milvus"
            return collection
        except Exception as exc:
            raise AppError("MILVUS_UNAVAILABLE", str(exc), 503) from exc

    def _search_local_lexical(self, plan: QueryPlan, category: str, limit: int) -> list[HybridHit]:
        hits: list[HybridHit] = []
        for item in self._iter_local_chunks(category):
            score = lexical_overlap(plan.expanded_terms, item["terms"])
            if score <= 0:
                continue
            hits.append(
                HybridHit(
                    chunk_id=item["chunk_id"],
                    document_id=item["document_id"],
                    document_title=item["document_title"],
                    category=item["category"],
                    chunk_index=item["chunk_index"],
                    chunk=item["chunk"],
                    lexical_score=round_score(score),
                    match_reasons=["local_lexical"],
                )
            )
        hits.sort(key=lambda hit: (-hit.lexical_score, hit.document_title, hit.chunk_index))
        return hits[:limit]

    def _search_local_semantic(self, plan: QueryPlan, category: str, limit: int) -> list[HybridHit]:
        query_vector = self._embed_texts([plan.normalized])[0]
        hits: list[HybridHit] = []
        for item in self._iter_local_chunks(category):
            score = cosine_similarity(query_vector, item["embedding"])
            if score <= 0:
                continue
            hits.append(
                HybridHit(
                    chunk_id=item["chunk_id"],
                    document_id=item["document_id"],
                    document_title=item["document_title"],
                    category=item["category"],
                    chunk_index=item["chunk_index"],
                    chunk=item["chunk"],
                    semantic_score=round_score(score),
                    match_reasons=["local_vector"],
                )
            )
        hits.sort(key=lambda hit: (-hit.semantic_score, hit.document_title, hit.chunk_index))
        return hits[:limit]

    def _iter_local_chunks(self, category: str) -> list[dict[str, Any]]:
        return [
            item
            for item in self._local_chunks.values()
            if not category or item["category"] == category
        ]

    def _delete_from_elasticsearch(self, document_id: str) -> None:
        payload = {"query": {"term": {"document_id": document_id}}}
        self._es_request("POST", f"/{self.settings.rag_es_index}/_delete_by_query", payload)

    def _es_request(
        self,
        method: str,
        path: str,
        payload: dict[str, Any] | str | None = None,
        headers: dict[str, str] | None = None,
    ) -> dict[str, Any]:
        body: bytes | None = None
        request_headers = {"Content-Type": "application/json"}
        if headers:
            request_headers.update(headers)

        if payload is not None:
            if isinstance(payload, str):
                body = payload.encode("utf-8")
            else:
                body = json.dumps(payload, ensure_ascii=False).encode("utf-8")

        request = Request(
            url=f"{self.settings.elasticsearch_url.rstrip('/')}{path}",
            method=method,
            data=body,
            headers=request_headers,
        )

        try:
            with urlopen(request, timeout=4) as response:
                raw = response.read().decode("utf-8").strip()
                return json.loads(raw) if raw else {}
        except HTTPError as exc:
            detail = exc.read().decode("utf-8")
            raise AppError("ELASTICSEARCH_ERROR", detail or str(exc), exc.code) from exc
        except URLError as exc:
            raise AppError("ELASTICSEARCH_UNAVAILABLE", str(exc.reason), 503) from exc


def build_query_plan(query: str) -> QueryPlan:
    normalized = normalize_text(query)
    terms = unique_strings(normalized.split())
    phrase_expanded = phrase_synonyms(normalized)
    expanded = unique_strings(
        terms
        + phrase_expanded
        + [item for term in terms for item in expand_query_term(term) + synonyms(term)]
    )
    return QueryPlan(
        normalized=normalized,
        terms=terms,
        expanded_terms=expanded,
        char_terms=char_terms(normalized),
        rewritten_query=" ".join(expanded),
    )


def build_answer(
    query: str,
    references: list[RAGReference],
    *,
    embedding_backend: str,
    vector_backend: str,
    lexical_backend: str,
) -> str:
    backend_label = describe_backends(embedding_backend, lexical_backend, vector_backend)
    if not references:
        return f"当前{backend_label}没有命中直接相关的片段。建议补充更具体的服务名、错误码、指标名或故障现象，再重新检索。"

    lines = [f"基于{backend_label}，当前最相关的信息如下：", ""]
    for reference in references:
        lines.append(f"- [{reference.document_title}] {reference.chunk}")
    lines.append("")
    lines.append(f"问题：{query.strip()}")
    lines.append("建议先按以上引用片段排查，再结合工具中心和告警时间线继续缩小范围。")
    return "\n".join(lines)


def build_strategy_name(embedding_backend: str, lexical_backend: str, vector_backend: str) -> str:
    if lexical_backend == "elasticsearch" and vector_backend == "milvus" and embedding_backend != "hash_fallback":
        return "embedding_milvus_es_hybrid"
    if lexical_backend == "elasticsearch" and vector_backend == "local_fallback":
        return "embedding_es_hybrid_partial"
    if lexical_backend == "local_fallback" and vector_backend == "milvus":
        return "embedding_milvus_partial"
    return "local_hybrid_fallback"


def describe_backends(embedding_backend: str, lexical_backend: str, vector_backend: str) -> str:
    if lexical_backend == "elasticsearch" and vector_backend == "milvus" and embedding_backend != "hash_fallback":
        return "Embedding + Milvus + Elasticsearch 混合检索"
    if lexical_backend == "elasticsearch" and vector_backend == "local_fallback":
        return "Embedding + Elasticsearch 检索（向量回退本地）"
    if lexical_backend == "local_fallback" and vector_backend == "milvus":
        return "Embedding + Milvus 检索（词法回退本地）"
    if embedding_backend == "hash_fallback":
        return "本地 hash embedding + 本地 hybrid fallback"
    return "本地 hybrid fallback"


def build_chunk_id(chunk: RAGChunkInput) -> str:
    return f"{chunk.document_id}:{chunk.index}"


def explain_match_reasons(reasons: list[str]) -> list[str]:
    labels = {
        "elasticsearch": "关键词检索命中",
        "local_lexical": "关键词检索命中",
        "lexical_hit": "关键词命中",
        "milvus": "语义检索命中",
        "local_vector": "语义检索命中",
        "vector_hit": "语义命中",
        "context_boost": "上下文增强",
    }
    return unique_strings([labels.get(reason, reason) for reason in reasons])


def normalize_text(content: str) -> str:
    builder: list[str] = []
    for char in content.strip().lower():
        if char.isalnum() or "\u4e00" <= char <= "\u9fff":
            builder.append(char)
        else:
            builder.append(" ")
    return " ".join("".join(builder).split())


def tokenize(content: str) -> list[str]:
    terms = content.split()
    expanded: list[str] = []
    for term in terms:
        expanded.append(term)
        expanded.extend(expand_query_term(term))
    return unique_strings(expanded)


def char_terms(content: str) -> list[str]:
    raw = content.replace(" ", "")
    items: list[str] = []
    for size in range(2, min(4, len(raw)) + 1):
        for start in range(0, len(raw) - size + 1):
            items.append(raw[start : start + size])
    return unique_strings(items)


def expand_query_term(term: str) -> list[str]:
    items: list[str] = []
    if len(term) <= 4:
        items.append(term)
    for char in term:
        if char.isdigit() or ("\u4e00" <= char <= "\u9fff"):
            items.append(char)
    for size in range(2, min(4, len(term)) + 1):
        for start in range(0, len(term) - size + 1):
            items.append(term[start : start + size])
    return unique_strings(items)


def synonyms(term: str) -> list[str]:
    table = {
        "sop": ["runbook", "procedure"],
        "p95": ["latency", "timeout", "延迟", "耗时"],
        "latency": ["p95", "timeout", "延迟"],
        "timeout": ["latency", "slow", "超时"],
        "error": ["5xx", "failure", "错误", "异常"],
        "5xx": ["error", "failure", "错误"],
        "mysql": ["database", "db", "数据库"],
        "db": ["database", "mysql", "数据库"],
        "告警": ["异常", "故障", "alert"],
        "故障": ["异常", "排障", "告警"],
        "错误率": ["异常率", "5xx", "error ratio"],
        "延迟": ["耗时", "timeout", "latency"],
    }
    return table.get(term, [])


def phrase_synonyms(normalized_query: str) -> list[str]:
    items: list[str] = []
    rules = {
        "卡了": ["延迟", "latency", "timeout", "慢", "变慢"],
        "变慢": ["延迟", "latency", "timeout", "卡顿"],
        "很慢": ["延迟", "latency", "timeout", "卡顿"],
        "超时": ["timeout", "延迟", "latency"],
        "卡顿": ["延迟", "latency", "timeout", "变慢"],
        "用户服务": ["user", "service", "user service", "user-service", "auth", "authentication"],
        "登录接口": ["login", "auth", "authentication", "user-service"],
        "登录": ["login", "auth", "authentication"],
    }
    for phrase, expansions in rules.items():
        if phrase in normalized_query:
            items.extend(expansions)
    return unique_strings(items)


def unique_strings(items: list[str]) -> list[str]:
    seen: set[str] = set()
    result: list[str] = []
    for item in items:
        value = item.strip()
        if not value or value in seen:
            continue
        seen.add(value)
        result.append(value)
    return result


def lexical_overlap(query_terms: list[str], chunk_terms: list[str]) -> float:
    if not query_terms or not chunk_terms:
        return 0.0
    chunk_set = set(chunk_terms)
    matched = sum(1 for term in query_terms if term in chunk_set)
    if matched == 0:
        return 0.0
    return matched / max(len(query_terms), 1)


def hash_embedding(content: str, dimension: int) -> list[float]:
    digest = hashlib.sha256(content.encode("utf-8")).digest()
    values: list[float] = []
    while len(values) < dimension:
        for byte in digest:
            values.append((byte / 255.0) * 2 - 1)
            if len(values) >= dimension:
                break
        digest = hashlib.sha256(digest).digest()
    return normalize_vector(values)


def cosine_similarity(left: list[float], right: list[float]) -> float:
    if not left or not right or len(left) != len(right):
        return 0.0
    return sum(a * b for a, b in zip(left, right, strict=True))


def normalize_vector(values: list[float]) -> list[float]:
    norm = math.sqrt(sum(value * value for value in values))
    if norm <= 0:
        return values
    return [value / norm for value in values]


def round_score(value: float) -> float:
    return round(value, 4)


def build_rag_service(settings: AppSettings) -> HybridRAGService:
    return HybridRAGService(settings)
