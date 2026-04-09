from __future__ import annotations

from dataclasses import dataclass
from datetime import UTC, datetime

from app.grpc.mappers import AlertAnalysisResult, TraceEntry, bootstrap_proto_modules

bootstrap_proto_modules()

from app.gen.proto.ai import runtime_pb2
from app.llm.openai_compatible import OpenAICompatibleClient, build_openai_compatible_client


@dataclass(slots=True)
class AlertAnalysisAgent:
    llm_client: OpenAICompatibleClient | None = None

    def __post_init__(self) -> None:
        if self.llm_client is None:
            self.llm_client = build_openai_compatible_client()

    def analyze(self, request: runtime_pb2.AnalyzeAlertRequest) -> AlertAnalysisResult:
        severity = request.severity.strip().upper() or "P3"
        prompt = (
            f"{request.service} in {request.environment} has a {severity} alert: {request.title}. "
            f"Summary: {request.summary}. Description: {request.description}."
        )
        summary = self.llm_client.complete(prompt)
        severity_assessment = self._severity_assessment(severity)
        possible_causes = self._possible_causes(request, severity)
        suggested_actions = self._suggested_actions(request)
        recommended_tools = self._recommended_tools(severity)
        knowledge_queries = self._knowledge_queries(request)

        return AlertAnalysisResult(
            summary=summary,
            severity_assessment=severity_assessment,
            possible_causes=possible_causes,
            suggested_actions=suggested_actions,
            recommended_tools=recommended_tools,
            knowledge_queries=knowledge_queries,
            workflow="alert_analysis",
            confidence=0.9 if severity in {"P0", "P1"} else 0.6,
            source="python-ai-runtime-alert-agent",
            generated_at=datetime.now(UTC).isoformat(),
            trace=[
                TraceEntry(
                    stage="alert_analysis",
                    message="generated deterministic alert triage",
                    tags=[request.alert_id, request.service],
                )
            ],
        )

    def _severity_assessment(self, severity: str) -> str:
        if severity == "P0":
            return "P0 requires immediate escalation and incident coordination."
        if severity == "P1":
            return "P1 suggests user-facing impact or core path risk."
        if severity == "P2":
            return "P2 should be triaged with the incident owner and recent changes."
        return "P3 is usually suitable for monitoring and light investigation."

    def _possible_causes(self, request: runtime_pb2.AnalyzeAlertRequest, severity: str) -> list[str]:
        text = " ".join([request.title, request.summary, request.description, request.source]).lower()
        causes: list[str] = []
        if any(keyword in text for keyword in ("timeout", "latency", "delay")):
            causes.append("Latency or timeout regression in the request path.")
        if any(keyword in text for keyword in ("error", "5xx", "exception")):
            causes.append("Error rate spike caused by recent change or dependency instability.")
        if any(keyword in text for keyword in ("cpu", "memory", "load")):
            causes.append("Resource pressure on the service or its runtime environment.")
        if not causes:
            causes.append(f"Recent change or downstream dependency drift may explain the {severity} alert.")
        return causes[:3]

    def _suggested_actions(self, request: runtime_pb2.AnalyzeAlertRequest) -> list[str]:
        actions = [
            f"Inspect {request.service} health, logs, and recent release activity.",
            "Check dependent services, databases, and infrastructure saturation.",
        ]
        if request.linked_session_id:
            actions.append("Continue the linked incident session and record findings.")
        return actions

    def _recommended_tools(self, severity: str) -> list[str]:
        tools = ["service_status", "recent_alerts", "knowledge_search"]
        if severity in {"P0", "P1"}:
            tools.append("platform_overview")
        return tools

    def _knowledge_queries(self, request: runtime_pb2.AnalyzeAlertRequest) -> list[str]:
        queries = [
            f"{request.service} {request.title}",
            f"{request.service} {request.summary}",
        ]
        for label in list(request.labels)[:2]:
            queries.append(f"{request.service} {label}")
        return queries[:4]
