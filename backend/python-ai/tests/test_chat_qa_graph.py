from __future__ import annotations

import unittest

from app.grpc.mappers import bootstrap_proto_modules

bootstrap_proto_modules()

from app.agents.chat_qa_agent import ChatQAAgent
from app.agents.tool_agent import ToolAgent
from app.agents.router_agent import RouterAgent
from app.gen.proto.ai import runtime_pb2
from app.gen.proto.common import metadata_pb2
from app.graphs.chat_qa_graph import build_chat_qa_graph
from app.grpc.services.runtime_service import RuntimeService


class FakeLLMClient:
    def chat(self, messages: list[dict[str, str]]) -> str:
        return "runtime chat answer"


class EmptyRAGService:
    def retrieve(self, request):
        del request
        return None


class FakeRAGService:
    def retrieve(self, request):
        del request
        return type(
            "RAGResponse",
            (),
            {
                "references": [
                    type(
                        "Reference",
                        (),
                        {
                            "document_id": "doc-1",
                            "document_title": "Deployment Runbook",
                            "category": "runbook",
                            "chunk": "Check the deployment timeline before rollback.",
                            "score": 0.91,
                        },
                    )()
                ]
            },
        )()


class FakeToolGateway:
    def execute_tool(self, *, user_id: str, user_roles: list[str], tool_name: str, parameters: dict[str, object]):
        del user_id, user_roles
        return {
            "status": "success",
            "result": {
                "tool_name": tool_name,
                **parameters,
                "risk": "degraded",
            },
        }


class ChatQAGraphTest(unittest.TestCase):
    def test_router_routes_general_conversation_to_chat_qa(self) -> None:
        decision = RouterAgent().route(
            runtime_pb2.RunConversationTurnRequest(
                metadata=metadata_pb2.RequestMetadata(
                    request_id="req-chat-route-1",
                    user_id="user-1",
                    session_id="session-1",
                ),
                session_id="session-1",
                user_id="user-1",
                user_roles=["oncall"],
                message="How do I verify the latest deploy?",
                retrieval_limit=3,
            )
        )

        self.assertEqual("chat_qa", decision.route)
        self.assertTrue(decision.trace)
        self.assertEqual("router", decision.trace[0].stage)

    def test_runtime_service_returns_chat_qa_response_shape(self) -> None:
        service = RuntimeService(
            router_agent=RouterAgent(),
            chat_qa_agent=ChatQAAgent(llm_client=FakeLLMClient(), rag_service=EmptyRAGService()),
            llm_client=FakeLLMClient(),
        )

        response = service.RunConversationTurn(
            runtime_pb2.RunConversationTurnRequest(
                metadata=metadata_pb2.RequestMetadata(
                    request_id="req-chat-route-2",
                    user_id="user-2",
                    session_id="session-2",
                ),
                session_id="session-2",
                user_id="user-2",
                user_roles=["oncall"],
                message="How should I check the deployment timeline?",
                history=[runtime_pb2.ConversationMessage(role="user", content="The deploy just finished.")],
                retrieval_limit=3,
            ),
            context=None,
        )

        self.assertEqual("ready", response.status)
        self.assertEqual("chat_qa", response.route)
        self.assertTrue(response.answer)
        self.assertTrue(response.trace)
        self.assertEqual("chat_qa", response.trace[-1].stage)

    def test_chat_graph_executes_tool_inside_workflow_when_question_requires_it(self) -> None:
        graph = build_chat_qa_graph(
            ChatQAAgent(
                llm_client=FakeLLMClient(),
                rag_service=FakeRAGService(),
                tool_agent=ToolAgent(llm_client=FakeLLMClient(), tool_gateway=FakeToolGateway()),
            )
        )

        response = graph.invoke(
            runtime_pb2.RunConversationTurnRequest(
                metadata=metadata_pb2.RequestMetadata(
                    request_id="req-chat-route-3",
                    user_id="user-3",
                    session_id="session-3",
                ),
                session_id="session-3",
                user_id="user-3",
                user_roles=["ops"],
                message="Please check payment-api status and deployment timeline",
                allowed_tools=["service_status"],
                retrieval_limit=3,
            )
        )

        self.assertEqual("ready", response.status)
        self.assertTrue(response.citations)
        self.assertTrue(response.tool_calls)
        self.assertEqual("service_status", response.tool_calls[0].name)
        stages = [entry.stage for entry in response.trace]
        self.assertIn("rag", stages)
        self.assertIn("tool", stages)
        self.assertEqual("chat_qa", response.trace[-1].stage)


if __name__ == "__main__":
    unittest.main()
