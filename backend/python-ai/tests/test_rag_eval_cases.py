from __future__ import annotations

import json
from pathlib import Path
import unittest

from app.core.config import AppSettings
from app.core.exceptions import AppError
from app.schemas.rag import RAGChunkInput, RAGIndexRequest, RAGRetrieveRequest
from app.services.hybrid_rag import HybridRAGService


FIXTURE_PATH = Path(__file__).parent / "fixtures" / "rag_eval_cases.json"


class RagEvaluationCasesTest(unittest.TestCase):
    def test_fixed_queries_hit_expected_documents(self) -> None:
        fixture = json.loads(FIXTURE_PATH.read_text(encoding="utf-8"))
        service = EvaluationHybridRAGService(
            AppSettings(
                EMBEDDING_PROVIDER="local",
                EMBEDDING_DIMENSION=4,
            )
        )

        for document in fixture["documents"]:
            service.index_document(
                RAGIndexRequest(
                    user_id="user_eval",
                    document_id=document["document_id"],
                    document_title=document["document_title"],
                    category=document["category"],
                    chunks=[
                        RAGChunkInput(
                            document_id=document["document_id"],
                            document_title=document["document_title"],
                            category=document["category"],
                            index=0,
                            content=document["content"],
                        )
                    ],
                )
            )

        for query_case in fixture["queries"]:
            with self.subTest(query=query_case["query"]):
                report = service.retrieve(
                    RAGRetrieveRequest(
                        query=query_case["query"],
                        category=query_case["category"],
                        limit=3,
                    )
                )

                self.assertTrue(report.references, "expected at least one reference")
                self.assertEqual(query_case["expected_title"], report.references[0].document_title)
                self.assertTrue(report.references[0].match_reasons, "expected match reasons")


class EvaluationHybridRAGService(HybridRAGService):
    def __init__(self, settings: AppSettings) -> None:
        super().__init__(settings)
        self._embedding_map = {
            "payment-api p95 latency spike often indicates downstream timeout or dependency slowdown. Check service status, dependency latency, and recent releases first.": [1.0, 0.0, 0.0, 0.0],
            "mysql connection pool saturation and slow queries can cause database timeout, connection exhaustion, and cascading latency.": [0.0, 1.0, 0.0, 0.0],
            "user-service 5xx ratio usually increases after bad release, dependency exception, or upstream auth failure. Check logs and rollback history.": [0.0, 0.0, 1.0, 0.0],
            "payment api p95 timeout": [1.0, 0.0, 0.0, 0.0],
            "支付服务卡了": [1.0, 0.0, 0.0, 0.0],
            "mysql 连接池耗尽": [0.0, 1.0, 0.0, 0.0],
            "登录接口 5xx": [0.0, 0.0, 1.0, 0.0],
        }

    def _embed_texts(self, texts: list[str]) -> list[list[float]]:
        vectors: list[list[float]] = []
        for text in texts:
            normalized = text.strip()
            if normalized not in self._embedding_map:
                raise AssertionError(f"missing fixture embedding for {normalized!r}")
            vectors.append(self._embedding_map[normalized])
        self._embedder_backend = "fixture_embedding"
        return vectors

    def _ensure_es_index(self) -> None:
        return

    def _index_elasticsearch(self, request: RAGIndexRequest, embeddings: list[list[float]]) -> None:
        return

    def _index_milvus(self, request: RAGIndexRequest, embeddings: list[list[float]]) -> None:
        return

    def _search_elasticsearch(self, plan, category: str, limit: int):
        raise AppError("ELASTICSEARCH_UNAVAILABLE", "fixture local lexical evaluation", 503)

    def _search_milvus(self, plan, category: str, limit: int):
        raise AppError("MILVUS_UNAVAILABLE", "fixture local semantic evaluation", 503)


if __name__ == "__main__":
    unittest.main()
