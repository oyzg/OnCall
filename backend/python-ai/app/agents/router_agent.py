from __future__ import annotations

from dataclasses import dataclass
from typing import Any

from app.grpc.mappers import RouterDecision, TraceEntry, bootstrap_proto_modules

bootstrap_proto_modules()

from app.gen.proto.ai import runtime_pb2


@dataclass(slots=True)
class RouterAgent:
    def route(self, request: Any) -> RouterDecision:
        if isinstance(request, runtime_pb2.AnalyzeAlertRequest):
            reason = "direct alert analysis request"
            return RouterDecision(
                route="alert_analysis",
                reason=reason,
                needs_rag=True,
                needs_tooling=True,
                trace=[TraceEntry(stage="router", message=reason, tags=self._tags_for_alert_request(request))],
            )

        message = request.message.strip().lower()
        allowed_tools = {tool.strip().lower() for tool in request.allowed_tools}
        has_linked_alert = request.HasField("linked_alert")

        if has_linked_alert:
            reason = "linked alert context detected"
            return RouterDecision(
                route="alert_analysis",
                reason=reason,
                needs_rag=True,
                needs_tooling=True,
                trace=[TraceEntry(stage="router", message=reason, tags=self._tags_for_conversation_request(request))],
            )

        tool_keywords = ("tool", "lookup", "check", "execute", "run")
        if allowed_tools and any(keyword in message for keyword in tool_keywords):
            reason = "tool-eligible chat request will be handled inside the business graph"
            return RouterDecision(
                route="chat_qa",
                reason=reason,
                needs_rag=True,
                needs_tooling=True,
                trace=[TraceEntry(stage="router", message=reason, tags=self._tags_for_conversation_request(request))],
            )

        reason = "general conversation or direct question"
        return RouterDecision(
            route="chat_qa",
            reason=reason,
            needs_rag=bool(message),
            trace=[TraceEntry(stage="router", message=reason, tags=self._tags_for_conversation_request(request))],
        )

    def _tags_for_alert_request(self, request: runtime_pb2.AnalyzeAlertRequest) -> list[str]:
        tags = [request.metadata.request_id, request.alert_id, request.service]
        return [tag for tag in tags if tag]

    def _tags_for_conversation_request(self, request: runtime_pb2.RunConversationTurnRequest) -> list[str]:
        alert_id = request.linked_alert.alert_id if request.HasField("linked_alert") else ""
        tags = [request.metadata.request_id, request.session_id, alert_id]
        return [tag for tag in tags if tag]
