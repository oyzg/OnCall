from __future__ import annotations

from app.agents.chat_qa_agent import ChatQAAgent
from app.graphs.state import GraphRunner


def build_chat_qa_graph(agent: ChatQAAgent | None = None) -> GraphRunner:
    chat_agent = agent or ChatQAAgent()
    return GraphRunner(name="chat_qa", _invoke=chat_agent.answer)
