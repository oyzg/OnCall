import unittest

from app.core.config import AppSettings
from app.core.exceptions import AppError
from app.schemas.rag import RAGChunkInput, RAGIndexRequest
from app.services.hybrid_rag import HybridHit, HybridRAGService, QueryPlan, build_query_plan


class HybridRAGServiceTest(unittest.TestCase):
    def test_index_document_rejects_embedding_dimension_mismatch(self) -> None:
        service = FakeHybridRAGService(
            AppSettings(
                EMBEDDING_PROVIDER="local",
                EMBEDDING_DIMENSION=4,
            ),
            embeddings=[[0.1, 0.2, 0.3]],
        )

        request = RAGIndexRequest(
            user_id="user_1",
            document_id="doc_1",
            document_title="Payment Runbook",
            category="runbook",
            chunks=[
                RAGChunkInput(
                    document_id="doc_1",
                    document_title="Payment Runbook",
                    category="runbook",
                    index=0,
                    content="payment-api timeout troubleshooting guide",
                )
            ],
        )

        with self.assertRaises(AppError) as ctx:
            service.index_document(request)

        self.assertEqual("EMBEDDING_DIMENSION_MISMATCH", ctx.exception.code)

    def test_fuse_hits_prefers_chunks_supported_by_both_retrievers(self) -> None:
        service = HybridRAGService(
            AppSettings(
                EMBEDDING_PROVIDER="local",
                EMBEDDING_DIMENSION=4,
            )
        )
        service._local_chunks = {
            "doc_1:0": {
                "chunk_id": "doc_1:0",
                "document_id": "doc_1",
                "document_title": "Payment Runbook",
                "category": "runbook",
                "chunk_index": 0,
                "chunk": "payment-api timeout often maps to latency spikes and downstream issues",
                "normalized": "payment api timeout often maps to latency spikes and downstream issues",
                "terms": ["payment", "api", "timeout", "latency"],
                "char_terms": ["pa", "ay"],
                "embedding": [0.1, 0.2, 0.3, 0.4],
            },
            "doc_2:0": {
                "chunk_id": "doc_2:0",
                "document_id": "doc_2",
                "document_title": "Generic Incident Notes",
                "category": "runbook",
                "chunk_index": 0,
                "chunk": "generic troubleshooting checklist for broad incidents",
                "normalized": "generic troubleshooting checklist for broad incidents",
                "terms": ["generic", "troubleshooting"],
                "char_terms": ["ge", "en"],
                "embedding": [0.4, 0.3, 0.2, 0.1],
            },
        }

        plan = QueryPlan(
            normalized="payment api timeout",
            terms=["payment", "api", "timeout"],
            expanded_terms=["payment", "api", "timeout", "latency"],
            char_terms=["pa", "ay", "ti", "im"],
            rewritten_query="payment api timeout latency",
        )

        lexical_hits = [
            HybridHit(
                chunk_id="doc_1:0",
                document_id="doc_1",
                document_title="Payment Runbook",
                category="runbook",
                chunk_index=0,
                chunk="payment-api timeout often maps to latency spikes and downstream issues",
                lexical_score=0.9,
                match_reasons=["elasticsearch"],
            )
        ]
        semantic_hits = [
            HybridHit(
                chunk_id="doc_1:0",
                document_id="doc_1",
                document_title="Payment Runbook",
                category="runbook",
                chunk_index=0,
                chunk="payment-api timeout often maps to latency spikes and downstream issues",
                semantic_score=0.82,
                match_reasons=["milvus"],
            ),
            HybridHit(
                chunk_id="doc_2:0",
                document_id="doc_2",
                document_title="Generic Incident Notes",
                category="runbook",
                chunk_index=0,
                chunk="generic troubleshooting checklist for broad incidents",
                semantic_score=0.78,
                match_reasons=["milvus"],
            ),
        ]

        references = service._fuse_hits(lexical_hits, semantic_hits, plan, limit=2)

        self.assertEqual(2, len(references))
        self.assertEqual("Payment Runbook", references[0].document_title)
        self.assertIn("关键词命中", references[0].match_reasons)
        self.assertIn("语义命中", references[0].match_reasons)
        self.assertGreater(references[0].score, references[1].score)

    def test_build_query_plan_expands_chinese_slow_incident_phrases(self) -> None:
        plan = build_query_plan("支付服务卡了")

        self.assertIn("延迟", plan.expanded_terms)
        self.assertIn("timeout", plan.expanded_terms)
        self.assertIn("latency", plan.expanded_terms)

    def test_build_query_plan_expands_chinese_service_aliases(self) -> None:
        plan = build_query_plan("用户服务 SOP")

        self.assertIn("user-service", plan.expanded_terms)
        self.assertIn("user service", plan.expanded_terms)
        self.assertIn("runbook", plan.expanded_terms)

    def test_build_query_plan_expands_login_error_phrases(self) -> None:
        plan = build_query_plan("登录接口 5xx")

        self.assertIn("auth", plan.expanded_terms)
        self.assertIn("authentication", plan.expanded_terms)
        self.assertIn("error", plan.expanded_terms)

    def test_fuse_hits_accepts_title_style_query_with_semantic_support(self) -> None:
        service = HybridRAGService(
            AppSettings(
                EMBEDDING_PROVIDER="local",
                EMBEDDING_DIMENSION=4,
            )
        )
        service._local_chunks = {
            "doc_1:0": {
                "chunk_id": "doc_1:0",
                "document_id": "doc_1",
                "document_title": "User Service SOP",
                "category": "runbook",
                "chunk_index": 0,
                "chunk": "Review application logs for 5xx spikes and upstream auth provider health.",
                "normalized": "review application logs for 5xx spikes and upstream auth provider health",
                "terms": ["review", "application", "logs", "5xx", "auth", "provider", "health"],
                "char_terms": ["re", "ev"],
                "embedding": [0.1, 0.2, 0.3, 0.4],
            },
        }

        plan = build_query_plan("用户服务 SOP")
        semantic_hits = [
            HybridHit(
                chunk_id="doc_1:0",
                document_id="doc_1",
                document_title="User Service SOP",
                category="runbook",
                chunk_index=0,
                chunk="Review application logs for 5xx spikes and upstream auth provider health.",
                semantic_score=0.76,
                match_reasons=["milvus"],
            )
        ]

        references = service._fuse_hits([], semantic_hits, plan, limit=1)

        self.assertEqual(1, len(references))
        self.assertEqual("User Service SOP", references[0].document_title)

    def test_fuse_hits_accepts_alias_query_when_chunk_contains_expanded_terms(self) -> None:
        service = HybridRAGService(
            AppSettings(
                EMBEDDING_PROVIDER="local",
                EMBEDDING_DIMENSION=4,
            )
        )
        service._local_chunks = {
            "doc_1:0": {
                "chunk_id": "doc_1:0",
                "document_id": "doc_1",
                "document_title": "User Service SOP",
                "category": "runbook",
                "chunk_index": 0,
                "chunk": "Review application logs for 5xx spikes and upstream authentication provider health.",
                "normalized": "review application logs for 5xx spikes and upstream authentication provider health",
                "terms": ["review", "application", "logs", "5xx", "authentication", "provider", "health"],
                "char_terms": ["re", "ev"],
                "embedding": [0.1, 0.2, 0.3, 0.4],
            },
        }

        plan = build_query_plan("登录接口 5xx")
        semantic_hits = [
            HybridHit(
                chunk_id="doc_1:0",
                document_id="doc_1",
                document_title="User Service SOP",
                category="runbook",
                chunk_index=0,
                chunk="Review application logs for 5xx spikes and upstream authentication provider health.",
                semantic_score=0.24,
                match_reasons=["milvus"],
            )
        ]

        references = service._fuse_hits([], semantic_hits, plan, limit=1)

        self.assertEqual(1, len(references))
        self.assertEqual("User Service SOP", references[0].document_title)

    def test_fuse_hits_rescues_semantic_candidate_with_title_or_body_alias(self) -> None:
        service = HybridRAGService(
            AppSettings(
                EMBEDDING_PROVIDER="local",
                EMBEDDING_DIMENSION=4,
            )
        )
        plan = build_query_plan("用户服务 SOP")
        semantic_hits = [
            HybridHit(
                chunk_id="doc_1:0",
                document_id="doc_1",
                document_title="User Service SOP",
                category="runbook",
                chunk_index=0,
                chunk="Review authentication provider health after 5xx spikes.",
                semantic_score=0.08,
                match_reasons=["milvus"],
            )
        ]

        references = service._fuse_hits([], semantic_hits, plan, limit=1)

        self.assertEqual(1, len(references))
        self.assertEqual("User Service SOP", references[0].document_title)

    def test_fuse_hits_rescues_title_like_query_with_top_semantic_hit(self) -> None:
        service = HybridRAGService(
            AppSettings(
                EMBEDDING_PROVIDER="local",
                EMBEDDING_DIMENSION=4,
            )
        )
        plan = build_query_plan("用户服务 SOP")
        semantic_hits = [
            HybridHit(
                chunk_id="doc_1:0",
                document_id="doc_1",
                document_title="doc_1",
                category="runbook",
                chunk_index=0,
                chunk="Generic operating procedure document.",
                semantic_score=0.12,
                match_reasons=["milvus"],
            )
        ]

        references = service._fuse_hits([], semantic_hits, plan, limit=1)

        self.assertEqual(1, len(references))
        self.assertEqual("doc_1", references[0].document_title)

    def test_search_milvus_retries_after_stale_collection_error(self) -> None:
        service = RetryMilvusHybridRAGService(
            AppSettings(
                EMBEDDING_PROVIDER="local",
                EMBEDDING_DIMENSION=4,
            )
        )

        plan = build_query_plan("用户服务 SOP")
        hits = service._search_milvus(plan, "", 3)

        self.assertEqual(1, len(hits))
        self.assertEqual("User Service SOP", hits[0].document_title)
        self.assertEqual(2, service.collection_requests)

    def test_index_milvus_retries_after_stale_collection_error(self) -> None:
        service = RetryMilvusHybridRAGService(
            AppSettings(
                EMBEDDING_PROVIDER="local",
                EMBEDDING_DIMENSION=4,
            )
        )
        request = RAGIndexRequest(
            user_id="user_1",
            document_id="doc_1",
            document_title="User Service SOP",
            category="runbook",
            chunks=[
                RAGChunkInput(
                    document_id="doc_1",
                    document_title="User Service SOP",
                    category="runbook",
                    index=0,
                    content="user-service 5xx runbook",
                )
            ],
        )

        service._index_milvus(request, [[0.1, 0.2, 0.3, 0.4]])

        self.assertEqual(2, service.collection_requests)


class FakeHybridRAGService(HybridRAGService):
    def __init__(self, settings: AppSettings, embeddings: list[list[float]]) -> None:
        super().__init__(settings)
        self._fake_embeddings = embeddings

    def _embed_texts(self, texts: list[str]) -> list[list[float]]:
        if len(texts) != 1:
            raise AssertionError(f"expected 1 text, got {len(texts)}")
        return self._fake_embeddings

    def _ensure_es_index(self) -> None:
        return

    def _index_elasticsearch(self, request: RAGIndexRequest, embeddings: list[list[float]]) -> None:
        return

    def _index_milvus(self, request: RAGIndexRequest, embeddings: list[list[float]]) -> None:
        return


class RetryMilvusHybridRAGService(HybridRAGService):
    def __init__(self, settings: AppSettings) -> None:
        super().__init__(settings)
        self.collection_requests = 0

    def _embed_texts(self, texts: list[str]) -> list[list[float]]:
        return [[0.1, 0.2, 0.3, 0.4] for _ in texts]

    def _get_milvus_collection(self):
        self.collection_requests += 1
        if self.collection_requests == 1:
            return BrokenCollection()
        return WorkingCollection()


class BrokenCollection:
    def search(self, **_kwargs):
        raise RuntimeError("collection not found")

    def delete(self, *_args, **_kwargs):
        raise RuntimeError("collection not found")

    def insert(self, *_args, **_kwargs):
        raise RuntimeError("collection not found")


class WorkingCollection:
    def __init__(self) -> None:
        self.entity = {
            "chunk_id": "doc_1:0",
            "document_id": "doc_1",
            "document_title": "User Service SOP",
            "category": "runbook",
            "chunk_index": 0,
            "content": "user-service 5xx runbook",
        }

    def search(self, **_kwargs):
        return [[SearchResultItem(self.entity, 0.86)]]

    def delete(self, *_args, **_kwargs):
        return None

    def insert(self, *_args, **_kwargs):
        return None

    def flush(self):
        return None


class SearchResultItem:
    def __init__(self, entity: dict, score: float) -> None:
        self.entity = entity
        self.score = score


if __name__ == "__main__":
    unittest.main()
