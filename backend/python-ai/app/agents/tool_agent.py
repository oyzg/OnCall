from __future__ import annotations

from dataclasses import dataclass
from datetime import UTC, datetime
import json

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
