from __future__ import annotations

import unittest

from app.grpc.mappers import bootstrap_proto_modules
from app.schemas.rag import RAGReference, RAGRetrieveResponse

bootstrap_proto_modules()

from app.agents.chat_qa_agent import ChatQAAgent
from app.gen.proto.ai import runtime_pb2
from app.gen.proto.common import metadata_pb2


class FakeLLMClient:
    def chat(self, messages: list[dict[str, str]]) -> str:
        prompt = messages[-1]["content"]
        return f"chat answer: {prompt}"


class FakeRAGService:
    def retrieve(self, request):
        return RAGRetrieveResponse(
            query=request.query,
            rewritten_query=request.query,
            answer="retrieved guidance",
            references=[
                RAGReference(
                    document_id="doc-1",
                    document_title="Cache Reset Runbook",
                    category="runbook",
                    chunk_index=0,
                    chunk="Reset the cache after checking the deployment timeline.",
                    score=0.91,
                )
            ],
        )


class EmptyRAGService:
    def retrieve(self, request):
        return RAGRetrieveResponse(
            query=request.query,
            rewritten_query=request.query,
            answer="",
            references=[],
        )


class ChatQAAgentTest(unittest.TestCase):
    def test_answer_consumes_recent_history_and_returns_citations(self) -> None:
        agent = ChatQAAgent(llm_client=FakeLLMClient(), rag_service=FakeRAGService())

        result = agent.answer(
            runtime_pb2.RunConversationTurnRequest(
                metadata=metadata_pb2.RequestMetadata(
                    request_id="req-chat-1",
                    user_id="user-1",
                    session_id="session-1",
                ),
                session_id="session-1",
                user_id="user-1",
                user_roles=["oncall"],
                message="How do I reset the cache safely?",
                history=[
                    runtime_pb2.ConversationMessage(role="user", content="The deploy just finished."),
                    runtime_pb2.ConversationMessage(role="assistant", content="Let's verify the cache state."),
                ],
                retrieval_limit=3,
            )
        )

        self.assertEqual("ready", result.status)
        self.assertIn("How do I reset the cache safely?", result.answer)
        self.assertEqual(1, len(result.citations))
        self.assertEqual("Cache Reset Runbook", result.citations[0].title)
        self.assertTrue(result.trace)
        self.assertEqual("chat_qa", result.trace[-1].stage)

    def test_answer_handles_no_rag_hits_without_breaking(self) -> None:
        agent = ChatQAAgent(llm_client=FakeLLMClient(), rag_service=EmptyRAGService())

        result = agent.answer(
            runtime_pb2.RunConversationTurnRequest(
                metadata=metadata_pb2.RequestMetadata(
                    request_id="req-chat-2",
                    user_id="user-2",
                    session_id="session-2",
                ),
                session_id="session-2",
                user_id="user-2",
                user_roles=["oncall"],
                message="What should I check next?",
                history=[runtime_pb2.ConversationMessage(role="user", content="CPU is still high.")],
                retrieval_limit=2,
            )
        )

        self.assertEqual("ready", result.status)
        self.assertIn("What should I check next?", result.answer)
        self.assertEqual([], result.citations)
        self.assertEqual("rag", result.trace[0].stage)


if __name__ == "__main__":
    unittest.main()
