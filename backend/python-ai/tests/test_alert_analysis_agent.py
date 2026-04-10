from __future__ import annotations

import unittest

from app.grpc.mappers import ToolCallEntry, TraceEntry, bootstrap_proto_modules

bootstrap_proto_modules()

from app.agents.alert_analysis_agent import AlertAnalysisAgent
from app.gen.proto.ai import runtime_pb2
from app.gen.proto.common import metadata_pb2


class FakeLLMClient:
    def complete(self, prompt: str) -> str:
        return prompt

    def complete_structured(self, *, prompt: str, fallback: dict[str, object]) -> dict[str, object]:
        return {
            "summary": "User service error ratio increased after the latest release.",
            "severity_assessment": "P1 because checkout traffic is seeing sustained failures.",
            "possible_causes": [
                "A recent deploy introduced a dependency timeout.",
                "The backing datastore is rejecting requests under load.",
            ],
            "suggested_actions": [
                "Inspect the latest deployment diff.",
                "Open the user-service runbook.",
            ],
            "recommended_tools": [
                "service_status",
                "knowledge_search",
                "service_status",
            ],
            "knowledge_queries": [
                "user-service error ratio increased",
                "user-service checkout dependency timeout",
            ],
        }


class FakeToolAgent:
    def enrich_alert(self, request: runtime_pb2.AnalyzeAlertRequest, recommended_tools: list[str]) -> dict[str, object]:
        del request
        return {
            "recommended_tools": list(recommended_tools) + ["knowledge_search"],
            "suggested_actions": ["Run service_status before escalating."],
            "trace": [
                TraceEntry(
                    stage="tool",
                    message="normalized alert tool recommendations",
                    tags=["tool-enrichment"],
                )
            ],
        }

    def execute_alert_tools(self, request: runtime_pb2.AnalyzeAlertRequest, recommended_tools: list[str], *, limit: int = 2) -> dict[str, object]:
        del request, recommended_tools, limit
        return {
            "tool_calls": [
                ToolCallEntry(
                    name="service_status",
                    arguments_json='{"service":"user-service","environment":"prod"}',
                    outcome="success",
                    summary="Executed service_status for user-service in prod. Current risk is degraded.",
                )
            ],
            "suggested_actions": ["Use service_status: service is degraded in prod."],
            "trace": [
                TraceEntry(
                    stage="tool",
                    message="executed service_status successfully",
                    tags=["tool-execution"],
                )
            ],
        }


class FakeRAGService:
    def retrieve_alert_context(self, queries: list[str], *, limit: int = 2) -> dict[str, object]:
        del limit
        return {
            "references": [
                {
                    "document_id": "doc-1",
                    "document_title": "User Service Runbook",
                    "chunk": "Rollback the latest release if the dependency timeout keeps increasing.",
                }
            ],
            "suggested_actions": ["Open the user-service runbook."],
            "trace": [
                TraceEntry(
                    stage="rag",
                    message="retrieved alert knowledge context",
                    tags=queries[:1],
                )
            ],
        }


class AlertAnalysisAgentTest(unittest.TestCase):
    def setUp(self) -> None:
        self.agent = AlertAnalysisAgent(
            llm_client=FakeLLMClient(),
            tool_agent=FakeToolAgent(),
            rag_service=FakeRAGService(),
        )

    def test_returns_structured_alert_analysis_with_normalized_enrichment(self) -> None:
        result = self.agent.analyze(
            runtime_pb2.AnalyzeAlertRequest(
                metadata=metadata_pb2.RequestMetadata(
                    request_id="req-alert-1",
                    user_id="user-1",
                    session_id="session-1",
                ),
                alert_id="alert-1",
                title="User service error ratio increased",
                service="user-service",
                environment="prod",
                severity="P1",
                source="prometheus",
                summary="5xx ratio is elevated on checkout requests",
                description="The error budget is burning quickly after the latest release.",
                labels=["team=core", "service=user-service"],
                triggered_at="2026-04-09T10:00:00Z",
                linked_session_id="session-1",
                user_id="user-1",
                user_roles=["oncall"],
            )
        )

        self.assertEqual("ready", result.status)
        self.assertTrue(result.summary)
        self.assertTrue(result.severity_assessment)
        self.assertTrue(result.possible_causes)
        self.assertTrue(result.suggested_actions)
        self.assertEqual(["knowledge_search", "service_status"], result.recommended_tools)
        self.assertEqual(1, len(result.tool_calls))
        self.assertEqual("service_status", result.tool_calls[0].name)
        self.assertTrue(result.knowledge_queries)
        self.assertTrue(result.workflow)
        self.assertTrue(result.source)

        stages = [entry.stage for entry in result.trace]
        self.assertIn("tool", stages)
        self.assertIn("rag", stages)
        self.assertIn("alert_analysis", stages)
        self.assertTrue(any("req-alert-1" in entry.tags for entry in result.trace))


if __name__ == "__main__":
    unittest.main()
