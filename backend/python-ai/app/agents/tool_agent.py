from __future__ import annotations

from dataclasses import dataclass
from datetime import UTC, datetime
import json
import re
from typing import Any

from app.core.config import get_settings
from app.grpc.mappers import ConversationTurnResult, ToolCallEntry, TraceEntry, bootstrap_proto_modules
from app.llm.openai_compatible import OpenAICompatibleClient, build_openai_compatible_client
from app.services.go_tool_gateway import GoToolGateway

bootstrap_proto_modules()

from app.gen.proto.ai import runtime_pb2


@dataclass(slots=True)
class ToolAgent:
    llm_client: OpenAICompatibleClient | None = None
    tool_gateway: Any | None = None

    def __post_init__(self) -> None:
        if self.llm_client is None:
            self.llm_client = build_openai_compatible_client()
        if self.tool_gateway is None:
            settings = get_settings()
            self.tool_gateway = GoToolGateway(
                base_url=settings.go_api_base_url,
                shared_secret=settings.runtime_shared_secret,
                timeout_seconds=settings.runtime_api_timeout_seconds,
            )

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

    def select_tool(self, request: runtime_pb2.RunConversationTurnRequest) -> str:
        return self._select_tool(request)

    def execute(self, request: runtime_pb2.RunConversationTurnRequest) -> ConversationTurnResult:
        selected_tool = self._select_tool(request)
        if not selected_tool:
            return ConversationTurnResult(
                answer="No allowed tools were available for this request.",
                route="tool",
                status="failed",
                error="no allowed tools were available",
                trace=[
                    TraceEntry(
                        stage="tool",
                        message="no allowed tools were available",
                        severity="error",
                        timestamp=datetime.now(UTC).isoformat(),
                        tags=[request.metadata.request_id, request.session_id],
                    )
                ],
            )

        return self.execute_selected_tool(request, selected_tool)

    def execute_selected_tool(
        self,
        request: runtime_pb2.RunConversationTurnRequest,
        tool_name: str,
    ) -> ConversationTurnResult:
        parameters = self._build_parameters(tool_name, request)
        return self._execute_named_tool(
            user_id=request.user_id,
            user_roles=list(request.user_roles),
            tool_name=tool_name,
            parameters=parameters,
            route="tool",
            trace_tags=[request.metadata.request_id, request.session_id, tool_name],
        )

    def execute_alert_tools(
        self,
        request: runtime_pb2.AnalyzeAlertRequest,
        recommended_tools: list[str],
        *,
        limit: int = 2,
    ) -> dict[str, Any]:
        normalized_tools = self._normalize_alert_tools(request, recommended_tools)
        tool_calls: list[ToolCallEntry] = []
        trace: list[TraceEntry] = []
        suggested_actions: list[str] = []

        for tool_name in normalized_tools[:limit]:
            result = self._execute_named_tool(
                user_id=request.user_id,
                user_roles=list(request.user_roles),
                tool_name=tool_name,
                parameters=self._build_alert_parameters(tool_name, request),
                route="alert_analysis",
                trace_tags=[request.metadata.request_id, request.alert_id, request.service, tool_name],
            )
            tool_calls.extend(result.tool_calls)
            trace.extend(result.trace)
            for tool_call in result.tool_calls:
                if tool_call.summary:
                    suggested_actions.append(f"Use {tool_name}: {tool_call.summary}")

        return {
            "tool_calls": tool_calls,
            "suggested_actions": suggested_actions,
            "trace": trace,
        }

    def _execute_named_tool(
        self,
        *,
        user_id: str,
        user_roles: list[str],
        tool_name: str,
        parameters: dict[str, object],
        route: str,
        trace_tags: list[str],
    ) -> ConversationTurnResult:
        arguments_json = json.dumps(parameters, ensure_ascii=True, sort_keys=True)
        trace = [
            TraceEntry(
                stage="tool",
                message=f"selected {tool_name} for execution",
                timestamp=datetime.now(UTC).isoformat(),
                tags=[tag for tag in trace_tags if tag],
            )
        ]

        try:
            gateway_result = self.tool_gateway.execute_tool(
                user_id=user_id,
                user_roles=user_roles,
                tool_name=tool_name,
                parameters=parameters,
            )
        except Exception as exc:
            message = str(exc)
            trace.append(
                TraceEntry(
                    stage="tool",
                    message=f"{tool_name} execution failed: {message}",
                    severity="error",
                    timestamp=datetime.now(UTC).isoformat(),
                    tags=[tag for tag in trace_tags if tag],
                )
            )
            return ConversationTurnResult(
                answer=f"Tool execution failed for {tool_name}: {message}",
                tool_calls=[
                    ToolCallEntry(
                        name=tool_name,
                        arguments_json=arguments_json,
                        outcome="failed",
                        summary=message,
                    )
                ],
                route=route,
                status="failed",
                error=message,
                trace=trace,
            )

        result = gateway_result.get("result")
        summary = self._summarize_result(tool_name, result)
        trace.append(
            TraceEntry(
                stage="tool",
                message=f"executed {tool_name} successfully",
                timestamp=datetime.now(UTC).isoformat(),
                tags=[tag for tag in trace_tags if tag],
            )
        )
        return ConversationTurnResult(
            answer=summary,
            tool_calls=[
                ToolCallEntry(
                    name=tool_name,
                    arguments_json=arguments_json,
                    outcome=str(gateway_result.get("status") or "success"),
                    summary=summary,
                )
            ],
            route=route,
            status="ready",
            trace=trace,
        )

    def _select_tool(self, request: runtime_pb2.RunConversationTurnRequest) -> str:
        allowed = [tool.strip() for tool in request.allowed_tools if tool.strip()]
        if not allowed:
            return ""

        message = request.message.strip().lower()
        if "knowledge_search" in allowed and any(keyword in message for keyword in ("knowledge", "runbook", "doc", "search")):
            return "knowledge_search"
        if "service_status" in allowed and any(keyword in message for keyword in ("status", "health", "service", "degraded", "check")):
            return "service_status"
        if "recent_alerts" in allowed and any(keyword in message for keyword in ("alert", "alerts", "recent")):
            return "recent_alerts"
        if "platform_overview" in allowed and any(keyword in message for keyword in ("platform", "overview")):
            return "platform_overview"
        return allowed[0]

    def _build_parameters(self, tool_name: str, request: runtime_pb2.RunConversationTurnRequest) -> dict[str, object]:
        if tool_name == "knowledge_search":
            return {"query": request.message.strip(), "limit": request.retrieval_limit or 3}
        if tool_name == "service_status":
            params: dict[str, object] = {}
            service = self._extract_service_name(request)
            if service:
                params["service"] = service
            environment = self._extract_environment(request)
            if environment:
                params["environment"] = environment
            return params
        if tool_name == "recent_alerts":
            params = {"limit": 5}
            service = self._extract_service_name(request)
            if service:
                params["service"] = service
            status = self._extract_alert_status(request)
            if status:
                params["status"] = status
            return params
        return {}

    def _build_alert_parameters(
        self,
        tool_name: str,
        request: runtime_pb2.AnalyzeAlertRequest,
    ) -> dict[str, object]:
        if tool_name == "knowledge_search":
            query = " ".join(part for part in [request.service, request.title or request.summary] if part).strip()
            return {"query": query, "limit": 3}
        if tool_name == "service_status":
            params: dict[str, object] = {}
            if request.service.strip():
                params["service"] = request.service.strip()
            if request.environment.strip():
                params["environment"] = request.environment.strip()
            return params
        if tool_name == "recent_alerts":
            params = {"limit": 5}
            if request.service.strip():
                params["service"] = request.service.strip()
            params["status"] = "open"
            return params
        if tool_name == "platform_overview":
            params = {}
            if request.environment.strip():
                params["environment"] = request.environment.strip()
            return params
        return {}

    def _extract_service_name(self, request: runtime_pb2.RunConversationTurnRequest) -> str:
        if request.HasField("linked_alert") and request.linked_alert.service.strip():
            return request.linked_alert.service.strip()
        for token in re.findall(r"[a-z0-9][a-z0-9_-]*", request.message.strip().lower()):
            if token in {"service", "status", "tool", "run", "check"}:
                continue
            if ("-" in token or "_" in token) and token.endswith(("service", "api")):
                return token
        for token in re.findall(r"[a-z0-9][a-z0-9_-]*", request.message.strip().lower()):
            if token in {"service", "status", "tool", "run", "check"}:
                continue
            if token.endswith(("service", "api")):
                return token
        return ""

    def _extract_environment(self, request: runtime_pb2.RunConversationTurnRequest) -> str:
        if request.HasField("linked_alert") and request.linked_alert.environment.strip():
            return request.linked_alert.environment.strip()
        lowered = request.message.strip().lower()
        for environment in ("prod", "production", "staging", "test", "dev"):
            if environment in lowered:
                return "prod" if environment == "production" else environment
        return ""

    def _extract_alert_status(self, request: runtime_pb2.RunConversationTurnRequest) -> str:
        lowered = request.message.strip().lower()
        for status in ("open", "resolved", "investigating", "acknowledged"):
            if status in lowered:
                return status
        return ""

    def _summarize_result(self, tool_name: str, result: object) -> str:
        if isinstance(result, dict):
            if tool_name == "service_status":
                service = result.get("service") or "the service"
                environment = result.get("environment") or "the current environment"
                risk = result.get("risk") or result.get("status") or "unknown"
                return f"Executed service_status for {service} in {environment}. Current risk is {risk}."
            if tool_name == "platform_overview":
                environment = result.get("environment") or "the current environment"
                risk = result.get("risk") or result.get("status") or "unknown"
                return f"Executed platform_overview for {environment}. Current platform risk is {risk}."
            if tool_name == "knowledge_search":
                answer = result.get("answer")
                if isinstance(answer, str) and answer.strip():
                    return answer
            if tool_name == "recent_alerts":
                items = result.get("alerts") or result.get("items")
                if isinstance(items, list):
                    return f"Executed recent_alerts and found {len(items)} matching alerts."
        return f"Executed {tool_name} successfully."

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
