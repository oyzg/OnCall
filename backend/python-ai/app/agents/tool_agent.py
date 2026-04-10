from __future__ import annotations

from dataclasses import dataclass
from datetime import UTC, datetime
import json
from typing import Any

from app.grpc.mappers import ConversationTurnResult, ToolCallEntry, TraceEntry, bootstrap_proto_modules
from app.llm.openai_compatible import OpenAICompatibleClient, build_openai_compatible_client

bootstrap_proto_modules()

from app.gen.proto.ai import runtime_pb2


@dataclass(slots=True)
class ToolAgent:
    llm_client: OpenAICompatibleClient | None = None

    def __post_init__(self) -> None:
        if self.llm_client is None:
            self.llm_client = build_openai_compatible_client()

    def enrich_alert(self, request: runtime_pb2.AnalyzeAlertRequest, recommended_tools: list[str]) -> dict[str, Any]:
        normalized_tools = self._normalize_alert_tools(request, recommended_tools)
        suggested_actions = [
            f"Run {tool} to confirm the current blast radius."
            for tool in normalized_tools[:2]
        ]
        return {
            "recommended_tools": normalized_tools,
            "suggested_actions": suggested_actions,
            "trace": [
                TraceEntry(
                    stage="tool",
                    message="normalized alert tool recommendations",
                    timestamp=datetime.now(UTC).isoformat(),
                    tags=[tag for tag in [request.metadata.request_id, request.alert_id, request.service] if tag],
                )
            ],
        }

    def execute(self, request: runtime_pb2.RunConversationTurnRequest) -> ConversationTurnResult:
        tool_calls = [
            ToolCallEntry(
                name=tool,
                arguments_json=json.dumps(
                    {
                        "session_id": request.session_id,
                        "user_id": request.user_id,
                        "message": request.message,
                    },
                    ensure_ascii=True,
                    sort_keys=True,
                ),
                outcome="ready",
                summary="deterministic tool skeleton",
            )
            for tool in request.allowed_tools
        ]
        answer = self.llm_client.complete("Tool execution requested" if tool_calls else "No tools were available")
        return ConversationTurnResult(
            answer=answer,
            tool_calls=tool_calls,
            route="tool",
            trace=[
                TraceEntry(
                    stage="tool",
                    message="constructed deterministic tool call skeleton",
                    timestamp=datetime.now(UTC).isoformat(),
                )
            ],
        )

    def _normalize_alert_tools(
        self,
        request: runtime_pb2.AnalyzeAlertRequest,
        recommended_tools: list[str],
    ) -> list[str]:
        normalized = [tool.strip() for tool in recommended_tools if tool.strip()]
        normalized.extend(["service_status", "recent_alerts", "knowledge_search"])

        text = " ".join([request.title, request.summary, request.description]).lower()
        if any(keyword in text for keyword in ("timeout", "latency", "cpu", "memory", "5xx", "error")):
            normalized.append("platform_overview")

        seen: set[str] = set()
        result: list[str] = []
        for tool in sorted(normalized):
            if tool in seen:
                continue
            seen.add(tool)
            result.append(tool)
        return result
