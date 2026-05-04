from __future__ import annotations

from langgraph.graph import END, START, StateGraph

from app.agents.autonomous_agent import AutonomousAgentRunner
from app.agents.alert_analysis_agent import AlertAnalysisAgent
from app.graphs.state import AlertAnalysisGraphState, GraphRunner


def build_alert_analysis_graph(agent: AlertAnalysisAgent | None = None) -> GraphRunner:
    alert_agent = agent or AlertAnalysisAgent()
    autonomous_runner = AutonomousAgentRunner()

    def run_autonomous_alert(state: AlertAnalysisGraphState) -> AlertAnalysisGraphState:
        try:
            return {"result": autonomous_runner.run_alert(alert_agent, state["request"])}
        except Exception:
            prepared = prepare_legacy_context(state)
            retrieved = retrieve_legacy_context(prepared)
            decided = decide_legacy_tools(retrieved)
            executed = execute_legacy_tools(decided) if decided.get("recommended_tools") else decided
            return finalize_legacy_alert(executed)

    def prepare_legacy_context(state: AlertAnalysisGraphState) -> AlertAnalysisGraphState:
        fallback = alert_agent.prepare_alert_context(state["request"])
        return {
            **state,
            "fallback": fallback,
            "knowledge_queries": list(fallback.get("knowledge_queries", [])),
            "trace": [],
        }

    def retrieve_legacy_context(state: AlertAnalysisGraphState) -> AlertAnalysisGraphState:
        rag_references, trace_entries = alert_agent.retrieve_alert_context(
            state["request"],
            list(state.get("knowledge_queries", [])),
        )
        trace = list(state.get("trace", []))
        trace.extend(trace_entries)
        return {**state, "rag_references": rag_references, "trace": trace}

    def decide_legacy_tools(state: AlertAnalysisGraphState) -> AlertAnalysisGraphState:
        decision = alert_agent.decide_alert_tools(
            state["request"],
            dict(state.get("fallback", {})),
            list(state.get("rag_references", [])),
            list(state.get("knowledge_queries", [])),
        )
        trace = list(state.get("trace", []))
        trace.extend(list(decision.get("trace", [])))
        return {
            **state,
            "structured": dict(decision["structured"]),
            "recommended_tools": list(decision["recommended_tools"]),
            "knowledge_queries": list(decision["knowledge_queries"]),
            "suggested_actions": list(decision["suggested_actions"]),
            "trace": trace,
        }

    def execute_legacy_tools(state: AlertAnalysisGraphState) -> AlertAnalysisGraphState:
        execution = alert_agent.execute_alert_tools(
            state["request"],
            list(state.get("recommended_tools", [])),
        )
        trace = list(state.get("trace", []))
        trace.extend(list(execution.get("trace", [])))
        return {
            **state,
            "tool_calls": list(execution.get("tool_calls", [])),
            "tool_suggested_actions": list(execution.get("suggested_actions", [])),
            "trace": trace,
        }

    def finalize_legacy_alert(state: AlertAnalysisGraphState) -> AlertAnalysisGraphState:
        result = alert_agent.finalize_alert_analysis(
            request=state["request"],
            fallback=dict(state.get("fallback", {})),
            structured=dict(state.get("structured", {})),
            recommended_tools=list(state.get("recommended_tools", [])),
            knowledge_queries=list(state.get("knowledge_queries", [])),
            suggested_actions=list(state.get("suggested_actions", [])),
            trace=list(state.get("trace", [])),
            tool_suggested_actions=list(state.get("tool_suggested_actions", [])),
            tool_calls=list(state.get("tool_calls", [])),
        )
        return {"result": result}

    graph = StateGraph(AlertAnalysisGraphState)
    graph.add_node("run_autonomous_alert", run_autonomous_alert)
    graph.add_edge(START, "run_autonomous_alert")
    graph.add_edge("run_autonomous_alert", END)
    compiled = graph.compile()

    return GraphRunner(
        name="alert_analysis",
        _invoke=lambda request: compiled.invoke({"request": request})["result"],
    )
