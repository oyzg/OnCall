from __future__ import annotations

from langgraph.graph import END, START, StateGraph

from app.agents.alert_analysis_agent import AlertAnalysisAgent
from app.graphs.state import AlertAnalysisGraphState, GraphRunner


def build_alert_analysis_graph(agent: AlertAnalysisAgent | None = None) -> GraphRunner:
    alert_agent = agent or AlertAnalysisAgent()

    def prepare_alert_context(state: AlertAnalysisGraphState) -> AlertAnalysisGraphState:
        fallback = alert_agent.prepare_alert_context(state["request"])
        return {
            "fallback": fallback,
            "knowledge_queries": list(fallback.get("knowledge_queries", [])),
            "trace": [],
        }

    def retrieve_alert_context(state: AlertAnalysisGraphState) -> AlertAnalysisGraphState:
        rag_references, trace_entries = alert_agent.retrieve_alert_context(
            state["request"],
            list(state.get("knowledge_queries", [])),
        )
        trace = list(state.get("trace", []))
        trace.extend(trace_entries)
        return {"rag_references": rag_references, "trace": trace}

    def decide_alert_tools(state: AlertAnalysisGraphState) -> AlertAnalysisGraphState:
        decision = alert_agent.decide_alert_tools(
            state["request"],
            dict(state.get("fallback", {})),
            list(state.get("rag_references", [])),
            list(state.get("knowledge_queries", [])),
        )
        trace = list(state.get("trace", []))
        trace.extend(list(decision.get("trace", [])))
        return {
            "structured": dict(decision["structured"]),
            "recommended_tools": list(decision["recommended_tools"]),
            "knowledge_queries": list(decision["knowledge_queries"]),
            "suggested_actions": list(decision["suggested_actions"]),
            "trace": trace,
        }

    def route_after_tool_decision(state: AlertAnalysisGraphState) -> str:
        return "execute_alert_tools" if state.get("recommended_tools") else "finalize_alert_analysis"

    def execute_alert_tools(state: AlertAnalysisGraphState) -> AlertAnalysisGraphState:
        execution = alert_agent.execute_alert_tools(
            state["request"],
            list(state.get("recommended_tools", [])),
        )
        trace = list(state.get("trace", []))
        trace.extend(list(execution.get("trace", [])))
        return {
            "tool_suggested_actions": list(execution.get("suggested_actions", [])),
            "trace": trace,
        }

    def finalize_alert_analysis(state: AlertAnalysisGraphState) -> AlertAnalysisGraphState:
        result = alert_agent.finalize_alert_analysis(
            request=state["request"],
            fallback=dict(state.get("fallback", {})),
            structured=dict(state.get("structured", {})),
            recommended_tools=list(state.get("recommended_tools", [])),
            knowledge_queries=list(state.get("knowledge_queries", [])),
            suggested_actions=list(state.get("suggested_actions", [])),
            trace=list(state.get("trace", [])),
            tool_suggested_actions=list(state.get("tool_suggested_actions", [])),
        )
        return {"result": result}

    graph = StateGraph(AlertAnalysisGraphState)
    graph.add_node("prepare_alert_context", prepare_alert_context)
    graph.add_node("retrieve_alert_context", retrieve_alert_context)
    graph.add_node("decide_alert_tools", decide_alert_tools)
    graph.add_node("execute_alert_tools", execute_alert_tools)
    graph.add_node("finalize_alert_analysis", finalize_alert_analysis)
    graph.add_edge(START, "prepare_alert_context")
    graph.add_edge("prepare_alert_context", "retrieve_alert_context")
    graph.add_edge("retrieve_alert_context", "decide_alert_tools")
    graph.add_conditional_edges(
        "decide_alert_tools",
        route_after_tool_decision,
        {
            "execute_alert_tools": "execute_alert_tools",
            "finalize_alert_analysis": "finalize_alert_analysis",
        },
    )
    graph.add_edge("execute_alert_tools", "finalize_alert_analysis")
    graph.add_edge("finalize_alert_analysis", END)
    compiled = graph.compile()

    return GraphRunner(
        name="alert_analysis",
        _invoke=lambda request: compiled.invoke({"request": request})["result"],
    )
