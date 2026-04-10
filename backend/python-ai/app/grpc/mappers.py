from __future__ import annotations

import sys
from dataclasses import dataclass, field
from datetime import UTC, datetime
from typing import Iterable


from app.gen.proto import ai as proto_ai
from app.gen.proto import common as proto_common


def bootstrap_proto_modules() -> None:
    sys.modules.setdefault("ai", proto_ai)
    sys.modules.setdefault("common", proto_common)


bootstrap_proto_modules()

from app.gen.proto.ai import runtime_pb2


@dataclass(slots=True)
class TraceEntry:
    stage: str
    message: str
    severity: str = "info"
    timestamp: str = ""
    tags: list[str] = field(default_factory=list)


@dataclass(slots=True)
class CitationEntry:
    source: str = ""
    title: str = ""
    url: str = ""
    snippet: str = ""
    document_id: str = ""
    score: float = 0.0


@dataclass(slots=True)
class ToolCallEntry:
    name: str = ""
    arguments_json: str = ""
    outcome: str = ""
    summary: str = ""


@dataclass(slots=True)
class RouterDecision:
    route: str
    reason: str
    needs_rag: bool = False
    needs_tooling: bool = False
    trace: list[TraceEntry] = field(default_factory=list)


@dataclass(slots=True)
class AlertAnalysisResult:
    status: str = "ready"
    summary: str = ""
    severity_assessment: str = ""
    possible_causes: list[str] = field(default_factory=list)
    suggested_actions: list[str] = field(default_factory=list)
    recommended_tools: list[str] = field(default_factory=list)
    knowledge_queries: list[str] = field(default_factory=list)
    tool_calls: list[ToolCallEntry] = field(default_factory=list)
    workflow: str = "alert_analysis"
    confidence: float = 0.5
    source: str = "python-ai-runtime"
    generated_at: str = ""
    error: str = ""
    trace: list[TraceEntry] = field(default_factory=list)


@dataclass(slots=True)
class ConversationTurnResult:
    status: str = "ready"
    answer: str = ""
    citations: list[CitationEntry] = field(default_factory=list)
    tool_calls: list[ToolCallEntry] = field(default_factory=list)
    route: str = "chat_qa"
    error: str = ""
    trace: list[TraceEntry] = field(default_factory=list)


@dataclass(slots=True)
class HealthResult:
    status: str = "ok"
    service: str = ""
    version: str = "0.1.0"
    error: str = ""
    trace: list[TraceEntry] = field(default_factory=list)


def _now() -> str:
    return datetime.now(UTC).isoformat()


def _trace_to_proto(entries: Iterable[TraceEntry]) -> list[runtime_pb2.TraceEvent]:
    return [
        runtime_pb2.TraceEvent(
            stage=entry.stage,
            message=entry.message,
            severity=entry.severity,
            timestamp=entry.timestamp or _now(),
            tags=list(entry.tags),
        )
        for entry in entries
    ]


def conversation_request_to_alert_request(
    request: runtime_pb2.RunConversationTurnRequest,
) -> runtime_pb2.AnalyzeAlertRequest:
    linked_alert = request.linked_alert if request.HasField("linked_alert") else None
    labels = list(linked_alert.labels) if linked_alert is not None else []
    return runtime_pb2.AnalyzeAlertRequest(
        metadata=request.metadata,
        alert_id=linked_alert.alert_id if linked_alert is not None else "",
        title=linked_alert.title if linked_alert is not None else request.message,
        service=linked_alert.service if linked_alert is not None else "",
        environment=linked_alert.environment if linked_alert is not None else "",
        severity=linked_alert.severity if linked_alert is not None else "",
        source=linked_alert.source if linked_alert is not None else "",
        summary=linked_alert.summary if linked_alert is not None else request.message,
        description=linked_alert.description if linked_alert is not None else "",
        labels=labels,
        triggered_at=linked_alert.triggered_at if linked_alert is not None else "",
        linked_session_id=linked_alert.linked_session_id if linked_alert is not None else request.session_id,
        user_id=request.user_id,
        user_roles=list(request.user_roles),
    )


def trace_to_proto(entries: Iterable[TraceEntry]) -> list[runtime_pb2.TraceEvent]:
    return _trace_to_proto(entries)


def alert_result_to_proto(result: AlertAnalysisResult) -> runtime_pb2.AnalyzeAlertResponse:
    return runtime_pb2.AnalyzeAlertResponse(
        status=result.status,
        summary=result.summary,
        severity_assessment=result.severity_assessment,
        possible_causes=list(result.possible_causes),
        suggested_actions=list(result.suggested_actions),
        recommended_tools=list(result.recommended_tools),
        knowledge_queries=list(result.knowledge_queries),
        tool_calls=[
            runtime_pb2.ToolCall(
                name=item.name,
                arguments_json=item.arguments_json,
                outcome=item.outcome,
                summary=item.summary,
            )
            for item in result.tool_calls
        ],
        workflow=result.workflow,
        confidence=result.confidence,
        source=result.source,
        generated_at=result.generated_at or _now(),
        error=result.error,
        trace=trace_to_proto(result.trace),
    )


def conversation_result_to_proto(result: ConversationTurnResult) -> runtime_pb2.RunConversationTurnResponse:
    return runtime_pb2.RunConversationTurnResponse(
        answer=result.answer,
        citations=[
            runtime_pb2.Citation(
                source=item.source,
                title=item.title,
                url=item.url,
                snippet=item.snippet,
                document_id=item.document_id,
                score=item.score,
            )
            for item in result.citations
        ],
        tool_calls=[
            runtime_pb2.ToolCall(
                name=item.name,
                arguments_json=item.arguments_json,
                outcome=item.outcome,
                summary=item.summary,
            )
            for item in result.tool_calls
        ],
        route=result.route,
        status=result.status,
        error=result.error,
        trace=trace_to_proto(result.trace),
    )


def health_result_to_proto(result: HealthResult) -> runtime_pb2.HealthResponse:
    return runtime_pb2.HealthResponse(
        status=result.status,
        version=result.version,
        service=result.service,
        error=result.error,
        trace=trace_to_proto(result.trace),
    )
