from __future__ import annotations

from dataclasses import dataclass
from datetime import UTC, datetime
import json
from typing import Any

from app.grpc.mappers import (
    AgentPlanStepEntry,
    AlertAnalysisResult,
    CitationEntry,
    ConversationTurnResult,
    PendingAgentActionEntry,
    ToolCallEntry,
    TraceEntry,
)

READ_TOOLS = ("knowledge_search", "service_status", "recent_alerts", "platform_overview")


@dataclass(slots=True)
class AutonomousAgentRunner:
    max_steps: int = 6

    def run_chat(self, agent: Any, request: Any) -> ConversationTurnResult:
        history = agent.prepare_chat_context(request)
        citations, trace = agent.retrieve_chat_context(request)
        plan: list[AgentPlanStepEntry] = [
            self._step("step-1", "plan", "Create an investigation plan for the current conversation turn."),
        ]
        tool_calls: list[ToolCallEntry] = []

        for tool_name in self._chat_tools(request):
            if len(plan) >= self.max_steps - 1:
                break
            tool_result = agent.execute_chat_tool(request, tool_name)
            tool_calls.extend(tool_result.tool_calls)
            trace.extend(tool_result.trace)
            plan.append(
                self._step(
                    f"step-{len(plan) + 1}",
                    "act",
                    f"Execute {tool_name} and observe the result.",
                    tool_name=tool_name,
                    observation=self._tool_observation(tool_result.tool_calls),
                )
            )

        plan.append(
            self._step(
                f"step-{len(plan) + 1}",
                "reflect",
                "Reflect on gathered evidence and decide whether a user-confirmed write action is needed.",
            )
        )
        pending_actions = self._chat_pending_actions(request, tool_calls)
        trace.append(
            TraceEntry(
                stage="agent",
                message=f"autonomous chat agent completed {len(plan)} plan steps",
                timestamp=self._now(),
                tags=[request.metadata.request_id, request.session_id],
            )
        )
        result = agent.finalize_chat_answer(request, history, citations, tool_calls, trace)
        result.agent_plan = plan[: self.max_steps]
        result.pending_actions = pending_actions
        result.route = "chat_qa"
        return result

    def run_alert(self, agent: Any, request: Any) -> AlertAnalysisResult:
        fallback = agent.prepare_alert_context(request)
        queries = list(fallback.get("knowledge_queries", []))
        rag_references, trace = agent.retrieve_alert_context(request, queries)
        observations = list(getattr(request, "action_observations", []))
        plan: list[AgentPlanStepEntry] = []
        for observation in observations:
            plan.append(
                self._step(
                    f"step-{len(plan) + 1}",
                    "observe",
                    f"Observe confirmed action {observation.action_type}.",
                    observation=self._action_observation(observation),
                )
            )
        plan.append(
            self._step(
                f"step-{len(plan) + 1}",
                "plan",
                "Replan from confirmed action results." if observations else "Create an autonomous alert investigation plan.",
            )
        )
        if observations:
            trace.append(
                TraceEntry(
                    stage="agent",
                    message=f"observed {len(observations)} confirmed agent action(s)",
                    timestamp=self._now(),
                    tags=[request.metadata.request_id, request.alert_id],
                )
            )

        decision = agent.decide_alert_tools(request, fallback, rag_references, queries)
        trace.extend(decision["trace"])
        tool_calls: list[ToolCallEntry] = []
        tool_suggested_actions: list[str] = []
        for tool_name in self._alert_tools(decision.get("recommended_tools", [])):
            if len(plan) >= self.max_steps - 1:
                break
            execution = agent.execute_alert_tools(request, [tool_name])
            trace.extend(execution.get("trace", []))
            calls = list(execution.get("tool_calls", []))
            tool_calls.extend(calls)
            tool_suggested_actions.extend(list(execution.get("suggested_actions", [])))
            plan.append(
                self._step(
                    f"step-{len(plan) + 1}",
                    "act",
                    f"Execute {tool_name} and observe the alert diagnostic result.",
                    tool_name=tool_name,
                    observation=self._tool_observation(calls),
                )
            )

        plan.append(
            self._step(
                f"step-{len(plan) + 1}",
                "reflect",
                "Reflect on diagnostics and prepare confirmable remediation actions.",
            )
        )
        pending_actions = self._alert_pending_actions(request, tool_calls)
        trace.append(
            TraceEntry(
                stage="agent",
                message=f"autonomous alert agent completed {len(plan)} plan steps",
                timestamp=self._now(),
                tags=[request.metadata.request_id, request.alert_id],
            )
        )
        result = agent.finalize_alert_analysis(
            request=request,
            fallback=fallback,
            structured=dict(decision["structured"]),
            recommended_tools=list(decision["recommended_tools"]),
            knowledge_queries=list(decision["knowledge_queries"]),
            suggested_actions=list(decision["suggested_actions"]),
            trace=trace,
            tool_suggested_actions=tool_suggested_actions,
            tool_calls=tool_calls,
        )
        result.agent_plan = plan[: self.max_steps]
        result.pending_actions = pending_actions
        result.workflow = "autonomous_plan_react_alert_analysis"
        result.source = "python-ai-runtime-autonomous-agent"
        return result

    def _chat_tools(self, request: Any) -> list[str]:
        allowed = [tool.strip() for tool in request.allowed_tools if tool.strip()]
        if not allowed:
            allowed = ["knowledge_search"]
        return self._unique([tool for tool in allowed if tool in READ_TOOLS])

    def _alert_tools(self, recommended_tools: object) -> list[str]:
        candidates = recommended_tools if isinstance(recommended_tools, list) else []
        return self._unique([str(tool).strip() for tool in candidates if str(tool).strip() in READ_TOOLS])

    def _chat_pending_actions(self, request: Any, tool_calls: list[ToolCallEntry]) -> list[PendingAgentActionEntry]:
        alert_id = request.linked_alert.alert_id if request.HasField("linked_alert") else ""
        if not alert_id:
            return []
        message = request.message.lower()
        if "ack" in message or "acknowledge" in message:
            status = "acknowledged"
        elif "resolve" in message:
            status = "resolved"
        else:
            status = "investigating"
        actions = [
            self._pending_action(
                "update_alert_status",
                "Move alert to investigation state",
                "Requires user confirmation before changing alert status.",
                {"alert_id": alert_id, "status": status, "comment": self._action_comment(tool_calls)},
                "medium",
            )
        ]
        if alert_id:
            actions.append(
                self._pending_action(
                    "append_alert_record",
                    "Record autonomous investigation note",
                    "Append the agent investigation summary to the alert handling record.",
                    {"alert_id": alert_id, "comment": self._action_comment(tool_calls)},
                    "low",
                )
            )
        return actions

    def _alert_pending_actions(self, request: Any, tool_calls: list[ToolCallEntry]) -> list[PendingAgentActionEntry]:
        executed_types = {
            observation.action_type
            for observation in getattr(request, "action_observations", [])
            if observation.status == "executed"
        }
        candidates = [
            (
                1,
                "update_alert_status",
                "Move alert to investigating",
                "The autonomous agent found enough signal to start active investigation.",
                {"alert_id": request.alert_id, "status": "investigating", "comment": self._action_comment(tool_calls)},
                "medium",
            ),
            (
                2,
                "append_alert_record",
                "Record autonomous investigation note",
                "Append the tool observations to the alert handling timeline.",
                {"alert_id": request.alert_id, "comment": self._action_comment(tool_calls)},
                "low",
            ),
        ]
        if not request.linked_session_id:
            candidates.append(
                (
                    3,
                    "link_or_create_session",
                    "Create and link an investigation session",
                    "Create a troubleshooting session for this alert.",
                    {"alert_id": request.alert_id},
                    "low",
                )
            )
        actions: list[PendingAgentActionEntry] = []
        for index, action_type, title, description, arguments, risk_level in candidates:
            if action_type in executed_types:
                continue
            actions.append(
                self._pending_action(
                    action_type,
                    title,
                    description,
                    arguments,
                    risk_level,
                    action_id=self._alert_action_id(request.alert_id, action_type, index),
                )
            )
        return actions

    def _pending_action(
        self,
        action_type: str,
        title: str,
        description: str,
        arguments: dict[str, object],
        risk_level: str,
        action_id: str = "",
    ) -> PendingAgentActionEntry:
        return PendingAgentActionEntry(
            action_id=action_id or f"pending-{action_type}",
            action_type=action_type,
            status="pending",
            title=title,
            description=description,
            arguments_json=json.dumps(arguments, ensure_ascii=True, sort_keys=True),
            risk_level=risk_level,
        )

    def _step(
        self,
        step_id: str,
        phase: str,
        description: str,
        *,
        tool_name: str = "",
        observation: str = "",
    ) -> AgentPlanStepEntry:
        return AgentPlanStepEntry(
            step_id=step_id,
            phase=phase,
            description=description,
            tool_name=tool_name,
            observation=observation,
            status="completed",
        )

    def _tool_observation(self, tool_calls: list[ToolCallEntry]) -> str:
        summaries = [call.summary for call in tool_calls if call.summary]
        return " | ".join(summaries) if summaries else "Tool returned no notable observation."

    def _action_comment(self, tool_calls: list[ToolCallEntry]) -> str:
        observation = self._tool_observation(tool_calls)
        return f"Autonomous agent recommendation based on: {observation}"

    def _action_observation(self, observation: Any) -> str:
        result = getattr(observation, "result_json", "") or getattr(observation, "error", "")
        return f"{observation.action_id} {observation.status}: {result or observation.title}"

    def _alert_action_id(self, alert_id: str, action_type: str, index: int) -> str:
        raw = f"agent_alert_{alert_id}_{action_type}_{index}"
        return "".join(char if char.isalnum() else "_" for char in raw)

    def _unique(self, items: list[str]) -> list[str]:
        result: list[str] = []
        seen: set[str] = set()
        for item in items:
            if not item or item in seen:
                continue
            seen.add(item)
            result.append(item)
        return result

    def _now(self) -> str:
        return datetime.now(UTC).isoformat()
