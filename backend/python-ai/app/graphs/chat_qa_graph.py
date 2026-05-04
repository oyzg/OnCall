from __future__ import annotations

from langgraph.graph import END, START, StateGraph

from app.agents.autonomous_agent import AutonomousAgentRunner
from app.agents.chat_qa_agent import ChatQAAgent
from app.graphs.state import ChatQAGraphState, GraphRunner


def build_chat_qa_graph(agent: ChatQAAgent | None = None) -> GraphRunner:
    chat_agent = agent or ChatQAAgent()
    autonomous_runner = AutonomousAgentRunner()

    def run_autonomous_chat(state: ChatQAGraphState) -> ChatQAGraphState:
        try:
            return {"result": autonomous_runner.run_chat(chat_agent, state["request"])}
        except Exception:
            return finalize_legacy_chat(execute_legacy_tool(decide_legacy_tool(retrieve_legacy_context(prepare_legacy_context(state)))))

    def prepare_legacy_context(state: ChatQAGraphState) -> ChatQAGraphState:
        return {"history": chat_agent.prepare_chat_context(state["request"]), "trace": []}

    def retrieve_legacy_context(state: ChatQAGraphState) -> ChatQAGraphState:
        citations, trace_entries = chat_agent.retrieve_chat_context(state["request"])
        trace = list(state.get("trace", []))
        trace.extend(trace_entries)
        return {**state, "citations": citations, "trace": trace}

    def decide_legacy_tool(state: ChatQAGraphState) -> ChatQAGraphState:
        return {**state, "tool_name": chat_agent.decide_chat_tool(state["request"])}

    def execute_legacy_tool(state: ChatQAGraphState) -> ChatQAGraphState:
        if not state.get("tool_name"):
            return state
        tool_result = chat_agent.execute_chat_tool(state["request"], state.get("tool_name", ""))
        trace = list(state.get("trace", []))
        trace.extend(tool_result.trace)
        return {
            **state,
            "tool_calls": list(tool_result.tool_calls),
            "tool_summaries": [
                tool_call.summary for tool_call in tool_result.tool_calls if tool_call.summary.strip()
            ],
            "trace": trace,
        }

    def finalize_legacy_chat(state: ChatQAGraphState) -> ChatQAGraphState:
        result = chat_agent.finalize_chat_answer(
            state["request"],
            list(state.get("history", [])),
            list(state.get("citations", [])),
            list(state.get("tool_calls", [])),
            list(state.get("trace", [])),
        )
        return {"result": result}

    graph = StateGraph(ChatQAGraphState)
    graph.add_node("run_autonomous_chat", run_autonomous_chat)
    graph.add_edge(START, "run_autonomous_chat")
    graph.add_edge("run_autonomous_chat", END)
    compiled = graph.compile()

    return GraphRunner(
        name="chat_qa",
        _invoke=lambda request: compiled.invoke({"request": request})["result"],
    )
