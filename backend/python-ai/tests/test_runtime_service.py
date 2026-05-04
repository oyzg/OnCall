from __future__ import annotations

import unittest
from unittest.mock import MagicMock, patch

from fastapi.testclient import TestClient

from app.agents.alert_analysis_agent import AlertAnalysisAgent
from app.agents.chat_qa_agent import ChatQAAgent
from app.agents.router_agent import RouterAgent
from app.agents.tool_agent import ToolAgent
from app.core.config import AppSettings
from app.grpc.mappers import bootstrap_proto_modules

bootstrap_proto_modules()

from app.gen.proto.ai import runtime_pb2
from app.gen.proto.common import metadata_pb2
from app.grpc.services.runtime_service import RuntimeService
from app.grpc.server import create_server
from app.main import app as fastapi_app


class FakeLLMClient:
    def complete(self, prompt: str) -> str:
        return f"complete:{prompt}"

    def chat(self, messages) -> str:
        return "chat:" + messages[-1]["content"]

    def complete_structured(self, *, prompt: str, fallback: dict[str, object]) -> dict[str, object]:
        del prompt
        return dict(fallback)


class EmptyRAGService:
    def retrieve(self, request):
        del request
        return None

    def retrieve_alert_context(self, queries, *, limit=2):
        del queries, limit
        return []


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


class RuntimeServiceTest(unittest.TestCase):
    def setUp(self) -> None:
        self.settings = AppSettings(
            APP_NAME="python-ai",
            APP_ENV="test",
            LOG_LEVEL="INFO",
            HTTP_PORT=8000,
            GRPC_HOST="127.0.0.1",
            GRPC_PORT=50051,
            OPENAI_BASE_URL="https://api.openai.com/v1",
            OPENAI_API_KEY="test-key",
            RUNTIME_API_MODEL="gpt-4.1-mini",
            RUNTIME_API_TIMEOUT_SECONDS=30,
            EMBEDDING_PROVIDER="openai_compatible",
            EMBEDDING_API_MODEL="text-embedding-3-small",
            EMBEDDING_API_TIMEOUT_SECONDS=15,
            ELASTICSEARCH_URL="http://127.0.0.1:9200",
            MILVUS_ADDRESS="127.0.0.1:19530",
            RAG_ES_INDEX="oncall_knowledge_chunks",
            RAG_MILVUS_COLLECTION="oncall_knowledge_chunks",
            EMBEDDING_MODEL_PATH="",
            EMBEDDING_MODEL_NAME="sentence-transformers/paraphrase-multilingual-MiniLM-L12-v2",
            EMBEDDING_LOCAL_ONLY=True,
            EMBEDDING_DIMENSION=1536,
            RAG_FUSION_WINDOW=60,
        )
        fake_llm = FakeLLMClient()
        empty_rag = EmptyRAGService()
        fake_tool_gateway = FakeToolGateway()
        self.service = RuntimeService(
            router_agent=RouterAgent(),
            alert_analysis_agent=AlertAnalysisAgent(
                llm_client=fake_llm,
                tool_agent=ToolAgent(llm_client=fake_llm, tool_gateway=fake_tool_gateway),
                rag_service=empty_rag,
            ),
            chat_qa_agent=ChatQAAgent(
                llm_client=fake_llm,
                tool_agent=ToolAgent(llm_client=fake_llm, tool_gateway=fake_tool_gateway),
                rag_service=empty_rag,
            ),
            tool_agent=ToolAgent(llm_client=fake_llm, tool_gateway=fake_tool_gateway),
            llm_client=fake_llm,
            settings=self.settings,
        )

    def test_analyze_alert_returns_router_backed_proto_response(self) -> None:
        response = self.service.AnalyzeAlert(
            runtime_pb2.AnalyzeAlertRequest(
                metadata=metadata_pb2.RequestMetadata(
                    request_id="req-1",
                    user_id="user-1",
                    session_id="session-1",
                ),
                alert_id="alert-1",
                title="Payment API timeout",
                service="payment-api",
                environment="prod",
                severity="P1",
                source="prometheus",
                summary="p95 latency increased",
                description="timeouts are increasing on checkout",
                labels=["team=payments"],
                triggered_at="2026-04-09T10:00:00Z",
                linked_session_id="session-1",
                user_id="user-1",
                user_roles=["oncall"],
            ),
            context=None,
        )

        self.assertEqual("ready", response.status)
        self.assertEqual("autonomous_plan_react_alert_analysis", response.workflow)
        self.assertIn("payment-api", response.summary)
        self.assertTrue(response.tool_calls)
        self.assertTrue(response.agent_plan)
        self.assertEqual([], list(response.pending_actions))
        self.assertTrue(response.trace)
        self.assertEqual("router", response.trace[0].stage)
        self.assertEqual("alert_analysis", response.trace[-1].stage)

    def test_run_conversation_turn_routes_alert_analysis_requests(self) -> None:
        response = self.service.RunConversationTurn(
            runtime_pb2.RunConversationTurnRequest(
                metadata=metadata_pb2.RequestMetadata(
                    request_id="req-3",
                    user_id="user-1",
                    session_id="session-3",
                ),
                session_id="session-3",
                user_id="user-1",
                user_roles=["oncall"],
                message="Please inspect the linked alert",
                linked_alert=runtime_pb2.LinkedAlert(
                    alert_id="alert-1",
                    title="Payment API timeout",
                    service="payment-api",
                    environment="prod",
                    severity="P1",
                    source="prometheus",
                    summary="p95 latency increased",
                    description="timeouts are increasing on checkout",
                    labels=["team=payments"],
                    triggered_at="2026-04-09T10:00:00Z",
                    linked_session_id="session-3",
                ),
                allowed_tools=[],
                retrieval_limit=3,
            ),
            context=None,
        )

        self.assertEqual("ready", response.status)
        self.assertEqual("alert_analysis", response.route)
        self.assertIn("payment-api", response.answer)
        self.assertTrue(response.trace)
        self.assertEqual("alert_analysis", response.trace[-1].stage)

    def test_run_conversation_turn_routes_chat_requests(self) -> None:
        response = self.service.RunConversationTurn(
            runtime_pb2.RunConversationTurnRequest(
                metadata=metadata_pb2.RequestMetadata(
                    request_id="req-2",
                    user_id="user-1",
                    session_id="session-2",
                ),
                session_id="session-2",
                user_id="user-1",
                user_roles=["oncall"],
                message="How do I reset the cache?",
                history=[],
                allowed_tools=[],
                retrieval_limit=3,
            ),
            context=None,
        )

        self.assertEqual("ready", response.status)
        self.assertEqual("chat_qa", response.route)
        self.assertIn("reset the cache", response.answer)
        self.assertTrue(response.tool_calls)
        self.assertEqual("knowledge_search", response.tool_calls[0].name)
        self.assertTrue(response.agent_plan)
        self.assertEqual([], list(response.pending_actions))

    def test_run_conversation_turn_executes_tools_inside_chat_workflow(self) -> None:
        response = self.service.RunConversationTurn(
            runtime_pb2.RunConversationTurnRequest(
                metadata=metadata_pb2.RequestMetadata(
                    request_id="req-4",
                    user_id="user-1",
                    session_id="session-4",
                ),
                session_id="session-4",
                user_id="user-1",
                user_roles=["oncall"],
                message="Please check payment-api with a tool",
                history=[],
                allowed_tools=["service_status"],
                retrieval_limit=3,
            ),
            context=None,
        )

        self.assertEqual("ready", response.status)
        self.assertEqual("chat_qa", response.route)
        self.assertTrue(response.tool_calls)
        self.assertEqual("service_status", response.tool_calls[0].name)
        self.assertEqual("success", response.tool_calls[0].outcome)
        self.assertIn("service", response.tool_calls[0].arguments_json)
        stages = [entry.stage for entry in response.trace]
        self.assertIn("router", stages)
        self.assertIn("tool", stages)
        self.assertEqual("chat_qa", response.trace[-1].stage)

    @patch("app.grpc.server.grpc.server")
    def test_create_server_fails_fast_when_bind_fails(self, grpc_server_factory: MagicMock) -> None:
        fake_server = MagicMock()
        fake_server.add_insecure_port.return_value = 0
        grpc_server_factory.return_value = fake_server

        with self.assertRaises(RuntimeError):
            create_server(bind=True)

    def test_create_server_can_skip_binding_explicitly(self) -> None:
        server = create_server(bind=False)

        self.assertIsNotNone(server)

    @patch("app.main.create_server")
    def test_fastapi_startup_owns_grpc_server(self, create_server_mock: MagicMock) -> None:
        fake_server = MagicMock()
        create_server_mock.return_value = fake_server

        with TestClient(fastapi_app):
            self.assertIs(fastapi_app.state.grpc_server, fake_server)
            fake_server.start.assert_called_once()

        fake_server.stop.assert_called_once_with(grace=0)

    @patch("app.grpc.services.runtime_service.build_health_report")
    def test_health_reports_runtime_components_as_trace(self, build_health_report_mock: MagicMock) -> None:
        build_health_report_mock.return_value = type(
            "HealthReportStub",
            (),
            {
                "service": "python-ai",
                "status": "degraded",
                "components": [
                    type("ComponentStub", (), {"name": "grpc_runtime", "status": "up", "detail": "127.0.0.1:50051"})(),
                    type("ComponentStub", (), {"name": "runtime_model", "status": "fallback", "detail": "stub fallback"})(),
                ],
            },
        )()

        response = self.service.Health(runtime_pb2.HealthRequest(metadata=metadata_pb2.RequestMetadata()), context=None)

        self.assertEqual("degraded", response.status)
        self.assertEqual("python-ai", response.service)
        self.assertTrue(response.trace)
        self.assertEqual("grpc_runtime", response.trace[0].stage)
        self.assertEqual("runtime_model", response.trace[1].stage)


if __name__ == "__main__":
    unittest.main()
