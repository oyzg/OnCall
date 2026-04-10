from __future__ import annotations

from dataclasses import dataclass
from datetime import UTC, datetime
from typing import Any

from app.agents.tool_agent import ToolAgent
from app.grpc.mappers import AlertAnalysisResult, TraceEntry, bootstrap_proto_modules
from app.llm.openai_compatible import OpenAICompatibleClient, build_openai_compatible_client
from app.schemas.rag import RAGReference
from app.services.hybrid_rag import HybridRAGService

bootstrap_proto_modules()

from app.gen.proto.ai import runtime_pb2


@dataclass(slots=True)
class AlertAnalysisAgent:
    llm_client: OpenAICompatibleClient | None = None
    tool_agent: ToolAgent | Any | None = None
    rag_service: HybridRAGService | Any | None = None

    def __post_init__(self) -> None:
        if self.llm_client is None:
            self.llm_client = build_openai_compatible_client()
        if self.tool_agent is None:
            self.tool_agent = ToolAgent(llm_client=self.llm_client)

    def analyze(self, request: runtime_pb2.AnalyzeAlertRequest) -> AlertAnalysisResult:
        fallback = self._fallback_payload(request)
        prompt = self._build_prompt(request, fallback, {}, [])
        structured = self.llm_client.complete_structured(prompt=prompt, fallback=fallback)
        structured_recommended_tools = self._normalize_list(
            structured.get("recommended_tools"),
            fallback["recommended_tools"],
        )
        tool_context = self._tool_context(request, structured_recommended_tools)
        knowledge_queries = self._merge_unique(
            self._normalize_list(structured.get("knowledge_queries"), fallback["knowledge_queries"]),
            tool_context.get("knowledge_queries", []),
        )
        rag_references = self._retrieve_rag_references(knowledge_queries)

        suggested_actions = self._merge_unique(
            self._normalize_list(structured.get("suggested_actions"), fallback["suggested_actions"]),
            tool_context.get("suggested_actions", []),
            self._rag_suggested_actions(rag_references),
        )
        recommended_tools = self._merge_unique(
            structured_recommended_tools,
            tool_context.get("recommended_tools", []),
        )
        knowledge_queries = self._merge_unique(
            self._normalize_list(structured.get("knowledge_queries"), knowledge_queries),
            knowledge_queries,
        )

        trace = self._normalize_trace(tool_context.get("trace", []))
        if rag_references:
            trace.append(
                TraceEntry(
                    stage="rag",
                    message=f"retrieved {len(rag_references)} alert knowledge references",
                    tags=self._trace_tags(request),
                )
            )
        trace.append(
            TraceEntry(
                stage="alert_analysis",
                message="generated structured alert analysis",
                tags=self._trace_tags(request),
            )
        )

        return AlertAnalysisResult(
            summary=self._normalize_text(structured.get("summary"), fallback["summary"]),
            severity_assessment=self._normalize_text(
                structured.get("severity_assessment"),
                fallback["severity_assessment"],
            ),
            possible_causes=self._normalize_list(
                structured.get("possible_causes"),
                fallback["possible_causes"],
            ),
            suggested_actions=suggested_actions,
            recommended_tools=recommended_tools,
            knowledge_queries=knowledge_queries,
            workflow="router_alert_analysis",
            confidence=0.9 if (request.severity.strip().upper() or "P3") in {"P0", "P1"} else 0.6,
            source="python-ai-runtime-router-alert-agent",
            generated_at=datetime.now(UTC).isoformat(),
            trace=trace,
        )

    def _fallback_payload(self, request: runtime_pb2.AnalyzeAlertRequest) -> dict[str, object]:
        severity = request.severity.strip().upper() or "P3"
        return {
            "summary": (
                f"{request.service or 'unknown-service'} in {request.environment or 'unknown-env'} has a {severity} "
                f"alert: {request.title or request.summary}."
            ),
            "severity_assessment": self._severity_assessment(severity),
            "possible_causes": self._possible_causes(request, severity),
            "suggested_actions": self._suggested_actions(request),
            "recommended_tools": self._recommended_tools(severity),
            "knowledge_queries": self._knowledge_queries(request),
        }

    def _build_prompt(
        self,
        request: runtime_pb2.AnalyzeAlertRequest,
        fallback: dict[str, object],
        tool_context: dict[str, object],
        rag_references: list[RAGReference],
    ) -> str:
        rag_snippets = " ".join(reference.chunk for reference in rag_references[:2])
        tool_names = ", ".join(tool_context.get("recommended_tools", []))
        return (
            "Return JSON with summary, severity_assessment, possible_causes, suggested_actions, "
            "recommended_tools, knowledge_queries. "
            f"Alert: service={request.service}, env={request.environment}, severity={request.severity}, "
            f"title={request.title}, summary={request.summary}, description={request.description}. "
            f"Labels={list(request.labels)}. "
            f"Fallback={fallback}. "
            f"Recommended tools={tool_names}. "
            f"Knowledge context={rag_snippets}."
        )

    def _tool_context(self, request: runtime_pb2.AnalyzeAlertRequest, recommended_tools: list[str]) -> dict[str, Any]:
        if self.tool_agent is None or not hasattr(self.tool_agent, "enrich_alert"):
            return {}
        context = self.tool_agent.enrich_alert(request, list(recommended_tools))
        if not isinstance(context, dict):
            return {}
        return context

    def _retrieve_rag_references(self, knowledge_queries: list[str]) -> list[RAGReference]:
        if self.rag_service is None or not hasattr(self.rag_service, "retrieve_alert_context"):
            return []
        references = self.rag_service.retrieve_alert_context(knowledge_queries, limit=2)
        if isinstance(references, dict):
            references = references.get("references", [])
        result: list[RAGReference] = []
        for item in references or []:
            if isinstance(item, RAGReference):
                result.append(item)
                continue
            if isinstance(item, dict):
                try:
                    result.append(
                        RAGReference(
                            document_id=str(item.get("document_id", "")),
                            document_title=str(item.get("document_title", "")),
                            category=str(item.get("category", "")),
                            chunk_index=int(item.get("chunk_index", 0)),
                            chunk=str(item.get("chunk", "")),
                            score=float(item.get("score", 0.0)),
                            lexical_score=float(item.get("lexical_score", 0.0)),
                            semantic_score=float(item.get("semantic_score", 0.0)),
                            boost_score=float(item.get("boost_score", 0.0)),
                            match_reasons=[str(reason) for reason in item.get("match_reasons", [])],
                        )
                    )
                except (TypeError, ValueError):
                    continue
        return result

    def _rag_suggested_actions(self, references: list[RAGReference]) -> list[str]:
        actions: list[str] = []
        for reference in references[:2]:
            if reference.document_title:
                actions.append(f"Open {reference.document_title} and verify the matching mitigation steps.")
        return actions

    def _normalize_text(self, value: object, fallback: str) -> str:
        if isinstance(value, str) and value.strip():
            return value.strip()
        return fallback

    def _normalize_list(self, value: object, fallback: list[str]) -> list[str]:
        if not isinstance(value, list):
            return list(fallback)
        result = [str(item).strip() for item in value if str(item).strip()]
        return result or list(fallback)

    def _normalize_trace(self, value: object) -> list[TraceEntry]:
        if not isinstance(value, list):
            return []
        return [item for item in value if isinstance(item, TraceEntry)]

    def _merge_unique(self, *groups: object) -> list[str]:
        seen: set[str] = set()
        result: list[str] = []
        for group in groups:
            if not isinstance(group, list):
                continue
            for item in group:
                normalized = str(item).strip()
                if not normalized or normalized in seen:
                    continue
                seen.add(normalized)
                result.append(normalized)
        result.sort()
        return result

    def _trace_tags(self, request: runtime_pb2.AnalyzeAlertRequest) -> list[str]:
        return [tag for tag in [request.metadata.request_id, request.alert_id, request.service] if tag]

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
