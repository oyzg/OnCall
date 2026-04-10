from __future__ import annotations

from langgraph.graph import END, START, StateGraph

from app.agents.router_agent import RouterAgent
from app.graphs.state import GraphRunner, RouterGraphState


def build_router_graph(agent: RouterAgent | None = None) -> GraphRunner:
    router = agent or RouterAgent()

    def route_request(state: RouterGraphState) -> RouterGraphState:
        decision = router.route(state["request"])
        return {
            "route": decision.route,
            "reason": decision.reason,
            "needs_rag": decision.needs_rag,
            "needs_tooling": decision.needs_tooling,
            "trace": decision.trace,
            "decision": decision,
        }

    graph = StateGraph(RouterGraphState)
    graph.add_node("route_request", route_request)
    graph.add_edge(START, "route_request")
    graph.add_edge("route_request", END)
    compiled = graph.compile()

    return GraphRunner(
        name="router",
        _invoke=lambda request: compiled.invoke({"request": request})["decision"],
    )
