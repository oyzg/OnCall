from __future__ import annotations

from app.agents.router_agent import RouterAgent
from app.graphs.state import GraphRunner


def build_router_graph(agent: RouterAgent | None = None) -> GraphRunner:
    router = agent or RouterAgent()
    return GraphRunner(name="router", _invoke=router.route)
