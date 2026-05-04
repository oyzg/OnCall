from __future__ import annotations

import unittest

from app.grpc.mappers import bootstrap_proto_modules

bootstrap_proto_modules()

from app.gen.proto.ai import runtime_pb2
from app.gen.proto.common import metadata_pb2
from app.agents.router_agent import RouterAgent


class RouterAgentTest(unittest.TestCase):
    def setUp(self) -> None:
        self.agent = RouterAgent()

    def test_routes_alert_analysis_when_linked_alert_exists(self) -> None:
        decision = self.agent.route(
            runtime_pb2.RunConversationTurnRequest(
                metadata=metadata_pb2.RequestMetadata(
                    request_id="req-1",
                    user_id="user-1",
                    session_id="session-1",
                ),
                session_id="session-1",
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
                    linked_session_id="session-1",
                ),
                allowed_tools=[],
                retrieval_limit=3,
            )
        )

        self.assertEqual("alert_analysis", decision.route)
        self.assertIn("linked alert", decision.reason)

    def test_routes_general_questions_to_chat(self) -> None:
        decision = self.agent.route(
            runtime_pb2.RunConversationTurnRequest(
                metadata=metadata_pb2.RequestMetadata(
                    request_id="req-2",
                    user_id="user-1",
                    session_id="session-2",
                ),
                session_id="session-2",
                user_id="user-1",
                user_roles=["oncall"],
                message="What does this service do?",
                history=[],
                allowed_tools=[],
                retrieval_limit=3,
            )
        )

        self.assertEqual("chat_qa", decision.route)
        self.assertIn("general conversation", decision.reason)

    def test_keeps_tool_eligible_questions_in_chat_route_for_business_graph_execution(self) -> None:
        decision = self.agent.route(
            runtime_pb2.RunConversationTurnRequest(
                metadata=metadata_pb2.RequestMetadata(
                    request_id="req-3",
                    user_id="user-1",
                    session_id="session-3",
                ),
                session_id="session-3",
                user_id="user-1",
                user_roles=["oncall"],
                message="Please check the service with a tool",
                history=[],
                allowed_tools=["service_status"],
                retrieval_limit=3,
            )
        )

        self.assertEqual("chat_qa", decision.route)
        self.assertTrue(decision.needs_tooling)
        self.assertIn("business graph", decision.reason)


if __name__ == "__main__":
    unittest.main()
