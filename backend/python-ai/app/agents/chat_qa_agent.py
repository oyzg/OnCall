from __future__ import annotations

from dataclasses import dataclass
from datetime import UTC, datetime
from typing import Any

from app.agents.tool_agent import ToolAgent
from app.core.config import get_settings
from app.grpc.mappers import (
    CitationEntry,
    ConversationTurnResult,
    ToolCallEntry,
    TraceEntry,
    bootstrap_proto_modules,
)
from app.llm.openai_compatible import OpenAICompatibleClient, build_openai_compatible_client
from app.schemas.rag import RAGRetrieveRequest
from app.services.hybrid_rag import HybridRAGService, build_rag_service

bootstrap_proto_modules()

from app.gen.proto.ai import runtime_pb2


@dataclass(slots=True)
class ChatQAAgent:
    llm_client: OpenAICompatibleClient | None = None
    tool_agent: ToolAgent | Any | None = None
    rag_service: HybridRAGService | Any | None = None

    def __post_init__(self) -> None:
        if self.llm_client is None:
            self.llm_client = build_openai_compatible_client()
        if self.tool_agent is None:
            self.tool_agent = ToolAgent(llm_client=self.llm_client)
        if self.rag_service is None:
            self.rag_service = build_rag_service(get_settings())

    def answer(self, request: runtime_pb2.RunConversationTurnRequest) -> ConversationTurnResult:
        history = self.prepare_chat_context(request)
        citations, trace = self.retrieve_chat_context(request)
        tool_name = self.decide_chat_tool(request)
        tool_calls: list[ToolCallEntry] = []
        if tool_name:
            tool_result = self.execute_chat_tool(request, tool_name)
            tool_calls = list(tool_result.tool_calls)
            trace.extend(tool_result.trace)
        return self.finalize_chat_answer(request, history, citations, tool_calls, trace)

    def prepare_chat_context(
        self,
        request: runtime_pb2.RunConversationTurnRequest,
    ) -> list[runtime_pb2.ConversationMessage]:
        return self._recent_history(request.history)

    def retrieve_chat_context(
        self,
        request: runtime_pb2.RunConversationTurnRequest,
    ) -> tuple[list[CitationEntry], list[TraceEntry]]:
        rag_response = self._retrieve_knowledge(request)
        citations = self._citations_from_rag(rag_response)
        return citations, [
            TraceEntry(
                stage="rag",
                message="retrieved chat knowledge references" if citations else "no chat knowledge references found",
                timestamp=datetime.now(UTC).isoformat(),
                tags=[request.session_id, request.user_id],
            )
        ]

    def decide_chat_tool(self, request: runtime_pb2.RunConversationTurnRequest) -> str:
        if self.tool_agent is None or not hasattr(self.tool_agent, "select_tool"):
            return ""
        return str(self.tool_agent.select_tool(request) or "")

    def execute_chat_tool(
        self,
        request: runtime_pb2.RunConversationTurnRequest,
        tool_name: str,
    ) -> ConversationTurnResult:
        if not tool_name or self.tool_agent is None or not hasattr(self.tool_agent, "execute_selected_tool"):
            return ConversationTurnResult(route="tool", status="ready")
        return self.tool_agent.execute_selected_tool(request, tool_name)

    def finalize_chat_answer(
        self,
        request: runtime_pb2.RunConversationTurnRequest,
        history: list[runtime_pb2.ConversationMessage],
        citations: list[CitationEntry],
        tool_calls: list[ToolCallEntry],
        trace: list[TraceEntry],
    ) -> ConversationTurnResult:
        answer = self.llm_client.chat(
            [
                {
                    "role": "system",
                    "content": "Answer the user's operational question using recent conversation context, retrieved references, and any tool findings.",
                },
                {
                    "role": "user",
                    "content": self._build_prompt(request, history, citations, tool_calls),
                },
            ]
        )
        if not answer.strip():
            answer = self._fallback_answer(request, history, citations, tool_calls)

        final_trace = list(trace)
        final_trace.append(
            TraceEntry(
                stage="chat_qa",
                message="answered conversation turn with chat qa agent",
                timestamp=datetime.now(UTC).isoformat(),
                tags=[request.session_id, request.user_id],
            )
        )
        return ConversationTurnResult(
            answer=answer,
            citations=citations,
            tool_calls=tool_calls,
            route="chat_qa",
            status="ready",
            trace=final_trace,
        )

    def _recent_history(self, history: list[runtime_pb2.ConversationMessage]) -> list[runtime_pb2.ConversationMessage]:
        if not history:
            return []
        return list(history[-4:])

    def _retrieve_knowledge(self, request: runtime_pb2.RunConversationTurnRequest):
        if self.rag_service is None or not request.message.strip():
            return None
        try:
            return self.rag_service.retrieve(
                RAGRetrieveRequest(
                    query=request.message.strip(),
                    limit=request.retrieval_limit or 3,
                )
            )
        except Exception:
            return None

    def _citations_from_rag(self, rag_response) -> list[CitationEntry]:
        if rag_response is None:
            return []
        references = getattr(rag_response, "references", [])
        citations: list[CitationEntry] = []
        for reference in references[:3]:
            citations.append(
                CitationEntry(
                    source=reference.category,
                    title=reference.document_title,
                    snippet=reference.chunk,
                    document_id=reference.document_id,
                    score=reference.score,
                )
            )
        return citations

    def _build_prompt(
        self,
        request: runtime_pb2.RunConversationTurnRequest,
        history: list[runtime_pb2.ConversationMessage],
        citations: list[CitationEntry],
        tool_calls: list[ToolCallEntry],
    ) -> str:
        history_lines = [
            f"{message.role}: {message.content.strip()}"
            for message in history
            if message.content.strip()
        ]
        citation_lines = [
            f"{citation.title or citation.document_id}: {citation.snippet}"
            for citation in citations
            if citation.snippet.strip()
        ]
        tool_lines = [
            f"{tool_call.name}: {tool_call.summary}"
            for tool_call in tool_calls
            if tool_call.summary.strip()
        ]
        return (
            f"Current user message: {request.message.strip()}\n"
            f"Recent history: {' | '.join(history_lines) if history_lines else 'none'}\n"
            f"Retrieved references: {' | '.join(citation_lines) if citation_lines else 'none'}\n"
            f"Tool findings: {' | '.join(tool_lines) if tool_lines else 'none'}"
        )

    def _fallback_answer(
        self,
        request: runtime_pb2.RunConversationTurnRequest,
        history: list[runtime_pb2.ConversationMessage],
        citations: list[CitationEntry],
        tool_calls: list[ToolCallEntry],
    ) -> str:
        history_hint = history[-1].content.strip() if history and history[-1].content.strip() else "no recent history"
        if tool_calls:
            return (
                f"Tool findings for {request.message.strip()}: "
                f"{tool_calls[0].summary or tool_calls[0].name}. Continue from {history_hint}."
            )
        if citations:
            return (
                f"Based on the retrieved guidance, start with {citations[0].title or citations[0].document_id} "
                f"and continue investigating: {request.message.strip()}."
            )
        return f"Continue the conversation from '{history_hint}' and investigate: {request.message.strip()}."
