from __future__ import annotations

from dataclasses import dataclass, field
from typing import Callable, Generic, TypeVar

from app.grpc.mappers import CitationEntry, ToolCallEntry, TraceEntry


TInput = TypeVar("TInput")
TOutput = TypeVar("TOutput")


@dataclass(slots=True)
class GraphRunner(Generic[TInput, TOutput]):
    name: str
    _invoke: Callable[[TInput], TOutput]

    def invoke(self, state: TInput) -> TOutput:
        return self._invoke(state)


@dataclass(slots=True)
class RouterState:
    route: str = ""
    reason: str = ""
    trace: list[TraceEntry] = field(default_factory=list)


@dataclass(slots=True)
class AlertAnalysisState:
    trace: list[TraceEntry] = field(default_factory=list)


@dataclass(slots=True)
class ChatQAState:
    answer: str = ""
    citations: list[CitationEntry] = field(default_factory=list)
    trace: list[TraceEntry] = field(default_factory=list)


@dataclass(slots=True)
class ToolState:
    answer: str = ""
    tool_calls: list[ToolCallEntry] = field(default_factory=list)
    trace: list[TraceEntry] = field(default_factory=list)
