from __future__ import annotations

import unittest
from unittest.mock import MagicMock, patch

from fastapi.testclient import TestClient

from app.grpc.mappers import bootstrap_proto_modules

bootstrap_proto_modules()

from app.gen.proto.ai import runtime_pb2
from app.gen.proto.common import metadata_pb2
from app.grpc.services.runtime_service import RuntimeService
from app.grpc.server import create_server
from app.main import app as fastapi_app


class RuntimeServiceTest(unittest.TestCase):
    def setUp(self) -> None:
        self.service = RuntimeService()

    def test_analyze_alert_returns_deterministic_proto_response(self) -> None:
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
        self.assertEqual("alert_analysis", response.workflow)
        self.assertIn("payment-api", response.summary)
        self.assertTrue(response.trace)
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
        self.assertEqual([], list(response.tool_calls))

    def test_run_conversation_turn_routes_tool_requests(self) -> None:
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
                message="Please check the service with a tool",
                history=[],
                allowed_tools=["service_status"],
                retrieval_limit=3,
            ),
            context=None,
        )

        self.assertEqual("ready", response.status)
        self.assertEqual("tool", response.route)
        self.assertTrue(response.tool_calls)
        self.assertEqual("tool", response.trace[-1].stage)

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


if __name__ == "__main__":
    unittest.main()
