from __future__ import annotations

from app.agents.alert_analysis_agent import AlertAnalysisAgent
from app.graphs.state import GraphRunner


def build_alert_analysis_graph(agent: AlertAnalysisAgent | None = None) -> GraphRunner:
    alert_agent = agent or AlertAnalysisAgent()
    return GraphRunner(name="alert_analysis", _invoke=alert_agent.analyze)
