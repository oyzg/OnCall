from __future__ import annotations

from langgraph.graph import END, START, StateGraph

from app.agents.chat_qa_agent import ChatQAAgent
from app.graphs.state import ChatQAGraphState, GraphRunner


def build_chat_qa_graph(agent: ChatQAAgent | None = None) -> GraphRunner:
    chat_agent = agent or ChatQAAgent()

    def prepare_chat_context(state: ChatQAGraphState) -> ChatQAGraphState:
        return {"history": chat_agent.prepare_chat_context(state["request"]), "trace": []}

    def retrieve_chat_context(state: ChatQAGraphState) -> ChatQAGraphState:
        citations, trace_entries = chat_agent.retrieve_chat_context(state["request"])
        trace = list(state.get("trace", []))
        trace.extend(trace_entries)
        return {"citations": citations, "trace": trace}

    def decide_chat_tools(state: ChatQAGraphState) -> ChatQAGraphState:
        return {"tool_name": chat_agent.decide_chat_tool(state["request"])}

    def route_after_tool_decision(state: ChatQAGraphState) -> str:
        return "execute_chat_tools" if state.get("tool_name") else "finalize_chat_answer"

    def execute_chat_tools(state: ChatQAGraphState) -> ChatQAGraphState:
        tool_result = chat_agent.execute_chat_tool(state["request"], state.get("tool_name", ""))
        trace = list(state.get("trace", []))
        trace.extend(tool_result.trace)
        return {
            "tool_calls": list(tool_result.tool_calls),
            "tool_summaries": [
                tool_call.summary for tool_call in tool_result.tool_calls if tool_call.summary.strip()
            ],
            "trace": trace,
        }

    def finalize_chat_answer(state: ChatQAGraphState) -> ChatQAGraphState:
        result = chat_agent.finalize_chat_answer(
            state["request"],
            list(state.get("history", [])),
            list(state.get("citations", [])),
            list(state.get("tool_calls", [])),
            list(state.get("trace", [])),
        )
        return {"result": result}

    graph = StateGraph(ChatQAGraphState)
    graph.add_node("prepare_chat_context", prepare_chat_context)
    graph.add_node("retrieve_chat_context", retrieve_chat_context)
    graph.add_node("decide_chat_tools", decide_chat_tools)
    graph.add_node("execute_chat_tools", execute_chat_tools)
    graph.add_node("finalize_chat_answer", finalize_chat_answer)
    graph.add_edge(START, "prepare_chat_context")
    graph.add_edge("prepare_chat_context", "retrieve_chat_context")
    graph.add_edge("retrieve_chat_context", "decide_chat_tools")
    graph.add_conditional_edges(
        "decide_chat_tools",
        route_after_tool_decision,
        {
            "execute_chat_tools": "execute_chat_tools",
            "finalize_chat_answer": "finalize_chat_answer",
        },
    )
    graph.add_edge("execute_chat_tools", "finalize_chat_answer")
    graph.add_edge("finalize_chat_answer", END)
    compiled = graph.compile()

    return GraphRunner(
        name="chat_qa",
        _invoke=lambda request: compiled.invoke({"request": request})["result"],
    )
