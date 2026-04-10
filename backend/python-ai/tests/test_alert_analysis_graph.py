from __future__ import annotations

import unittest

from app.grpc.mappers import bootstrap_proto_modules

bootstrap_proto_modules()

from app.agents.alert_analysis_agent import AlertAnalysisAgent
from app.agents.tool_agent import ToolAgent
from app.agents.router_agent import RouterAgent
from app.gen.proto.ai import runtime_pb2
from app.gen.proto.common import metadata_pb2
from app.graphs.alert_analysis_graph import build_alert_analysis_graph
from app.graphs.router_graph import build_router_graph
from app.grpc.services.runtime_service import RuntimeService


class FakeLLMClient:
    def complete(self, prompt: str) -> str:
        return prompt

    def complete_structured(self, *, prompt: str, fallback: dict[str, object]) -> dict[str, object]:
        del prompt
        payload = dict(fallback)
        payload["recommended_tools"] = ["service_status", "recent_alerts", "knowledge_search"]
        return payload


class FakeAlertRAGService:
    def retrieve_alert_context(self, queries, *, limit=2):
        del queries, limit
        return [
            {
                "document_id": "doc-alert-1",
                "document_title": "Payment Incident SOP",
                "category": "runbook",
                "chunk_index": 0,
                "chunk": "Check recent deploys and confirm whether errors correlate with a dependency timeout.",
                "score": 0.88,
                "lexical_score": 0.0,
                "semantic_score": 0.0,
                "boost_score": 0.0,
                "match_reasons": [],
            }
        ]


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

    def test_alert_graph_executes_tools_inside_workflow_when_analysis_requires_it(self) -> None:
        graph = build_alert_analysis_graph(
            AlertAnalysisAgent(
                llm_client=FakeLLMClient(),
                rag_service=FakeAlertRAGService(),
                tool_agent=ToolAgent(llm_client=FakeLLMClient(), tool_gateway=FakeToolGateway()),
            )
        )

        response = graph.invoke(
            runtime_pb2.AnalyzeAlertRequest(
                metadata=metadata_pb2.RequestMetadata(
                    request_id="req-route-3",
                    user_id="user-3",
                    session_id="session-3",
                ),
                alert_id="alert-3",
                title="Payment API timeout",
                service="payment-api",
                environment="prod",
                severity="P1",
                source="prometheus",
                summary="Timeouts increased after deployment.",
                description="Users are seeing failures on checkout.",
                labels=["team=payments"],
                triggered_at="2026-04-09T12:00:00Z",
                linked_session_id="session-3",
                user_id="user-3",
                user_roles=["ops"],
            )
        )

        self.assertEqual("ready", response.status)
        self.assertTrue(response.recommended_tools)
        self.assertTrue(response.tool_calls)
        stages = [entry.stage for entry in response.trace]
        self.assertIn("rag", stages)
        self.assertIn("tool", stages)
        self.assertIn("alert_analysis", stages)
        tool_messages = [entry.message for entry in response.trace if entry.stage == "tool"]
        self.assertTrue(any("executed" in message for message in tool_messages))


if __name__ == "__main__":
    unittest.main()
