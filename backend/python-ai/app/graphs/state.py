from __future__ import annotations

from dataclasses import dataclass, field
from typing import Any, Callable, Generic, TypeVar, TypedDict

from app.grpc.mappers import (
    AlertAnalysisResult,
    CitationEntry,
    ConversationTurnResult,
    RouterDecision,
    ToolCallEntry,
    TraceEntry,
)


TInput = TypeVar("TInput")
TOutput = TypeVar("TOutput")


@dataclass(slots=True)
class GraphRunner(Generic[TInput, TOutput]):
    name: str
    _invoke: Callable[[TInput], TOutput]

    def invoke(self, state: TInput) -> TOutput:
        return self._invoke(state)


class RouterGraphState(TypedDict, total=False):
    request: Any
    route: str
    reason: str
    needs_rag: bool
    needs_tooling: bool
    trace: list[TraceEntry]
    decision: RouterDecision


class AlertAnalysisGraphState(TypedDict, total=False):
    request: Any
    fallback: dict[str, object]
    structured: dict[str, object]
    recommended_tools: list[str]
    knowledge_queries: list[str]
    suggested_actions: list[str]
    tool_suggested_actions: list[str]
    tool_calls: list[ToolCallEntry]
    rag_references: list[Any]
    trace: list[TraceEntry]
    result: AlertAnalysisResult


class ChatQAGraphState(TypedDict, total=False):
    request: Any
    history: list[Any]
    answer: str
    citations: list[CitationEntry]
    tool_name: str
    tool_calls: list[ToolCallEntry]
    tool_summaries: list[str]
    trace: list[TraceEntry]
    result: ConversationTurnResult


@dataclass(slots=True)
class ToolState:
    answer: str = ""
    tool_calls: list[ToolCallEntry] = field(default_factory=list)
    trace: list[TraceEntry] = field(default_factory=list)
