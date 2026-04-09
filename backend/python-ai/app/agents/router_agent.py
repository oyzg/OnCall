from __future__ import annotations

from dataclasses import dataclass

from app.grpc.mappers import RouterDecision, TraceEntry, bootstrap_proto_modules

bootstrap_proto_modules()

from app.gen.proto.ai import runtime_pb2


@dataclass(slots=True)
class RouterAgent:
    def route(self, request: runtime_pb2.RunConversationTurnRequest) -> RouterDecision:
        message = request.message.strip().lower()
        allowed_tools = {tool.strip().lower() for tool in request.allowed_tools}
        has_linked_alert = request.HasField("linked_alert")

        if has_linked_alert:
            reason = "linked alert context detected"
            return RouterDecision(
                route="alert_analysis",
                reason=reason,
                trace=[TraceEntry(stage="router", message=reason)],
            )

        tool_keywords = ("tool", "lookup", "check", "execute", "run")
        if allowed_tools and any(keyword in message for keyword in tool_keywords):
            reason = "tool keywords matched and allowed tools are available"
            return RouterDecision(
                route="tool",
                reason=reason,
                trace=[TraceEntry(stage="router", message=reason)],
            )

        reason = "general conversation or direct question"
        return RouterDecision(
            route="chat_qa",
            reason=reason,
            trace=[TraceEntry(stage="router", message=reason)],
        )
