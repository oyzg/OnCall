from __future__ import annotations

import unittest

from app.grpc.mappers import bootstrap_proto_modules

bootstrap_proto_modules()

from app.agents.alert_analysis_agent import AlertAnalysisAgent
from app.agents.router_agent import RouterAgent
from app.gen.proto.ai import runtime_pb2
from app.gen.proto.common import metadata_pb2
from app.graphs.router_graph import build_router_graph
from app.grpc.services.runtime_service import RuntimeService


class FakeLLMClient:
    def complete(self, prompt: str) -> str:
        return prompt

    def complete_structured(self, *, prompt: str, fallback: dict[str, object]) -> dict[str, object]:
        del prompt
        return dict(fallback)


class AlertAnalysisGraphTest(unittest.TestCase):
    def test_router_graph_routes_analyze_alert_requests_to_alert_analysis(self) -> None:
        graph = build_router_graph(RouterAgent())

        decision = graph.invoke(
            runtime_pb2.AnalyzeAlertRequest(
                metadata=metadata_pb2.RequestMetadata(
                    request_id="req-route-1",
                    user_id="user-1",
                    session_id="session-1",
                ),
                alert_id="alert-1",
                title="Payment API timeout",
                service="payment-api",
                environment="prod",
                severity="P1",
                source="prometheus",
                summary="Timeouts are increasing on checkout.",
                description="The alert is linked to elevated latency.",
                labels=["team=payments"],
                triggered_at="2026-04-09T10:00:00Z",
                linked_session_id="session-1",
                user_id="user-1",
                user_roles=["oncall"],
            )
        )

        self.assertEqual("alert_analysis", decision.route)
        self.assertTrue(decision.trace)
        self.assertEqual("router", decision.trace[0].stage)

    def test_runtime_analyze_alert_propagates_router_trace_and_structured_output(self) -> None:
        service = RuntimeService(
            router_agent=RouterAgent(),
            alert_analysis_agent=AlertAnalysisAgent(llm_client=FakeLLMClient()),
            llm_client=FakeLLMClient(),
        )

        response = service.AnalyzeAlert(
            runtime_pb2.AnalyzeAlertRequest(
                metadata=metadata_pb2.RequestMetadata(
                    request_id="req-route-2",
                    user_id="user-2",
                    session_id="session-2",
                ),
                alert_id="alert-2",
                title="Order service CPU burn",
                service="order-service",
                environment="prod",
                severity="P2",
                source="prometheus",
                summary="CPU usage is above 90 percent on multiple pods.",
                description="Sustained CPU burn began after a batch job started.",
                labels=["team=orders"],
                triggered_at="2026-04-09T11:00:00Z",
                linked_session_id="session-2",
                user_id="user-2",
                user_roles=["oncall"],
            ),
            context=None,
        )

        self.assertEqual("ready", response.status)
        self.assertTrue(response.workflow)
        self.assertTrue(response.source)
        self.assertTrue(response.trace)

        stages = [entry.stage for entry in response.trace]
        self.assertIn("router", stages)
        self.assertIn("alert_analysis", stages)


if __name__ == "__main__":
    unittest.main()
