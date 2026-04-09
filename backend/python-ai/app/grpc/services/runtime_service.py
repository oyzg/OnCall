from __future__ import annotations

from app.agents.alert_analysis_agent import AlertAnalysisAgent
from app.agents.chat_qa_agent import ChatQAAgent
from app.agents.router_agent import RouterAgent
from app.agents.tool_agent import ToolAgent
from app.grpc.mappers import (
    ConversationTurnResult,
    HealthResult,
    alert_result_to_proto,
    conversation_request_to_alert_request,
    conversation_result_to_proto,
    health_result_to_proto,
)
from app.graphs.alert_analysis_graph import build_alert_analysis_graph
from app.graphs.chat_qa_graph import build_chat_qa_graph
from app.graphs.router_graph import build_router_graph
from app.llm.openai_compatible import OpenAICompatibleClient, build_openai_compatible_client

from app.gen.proto.ai import runtime_pb2, runtime_pb2_grpc


class RuntimeService(runtime_pb2_grpc.RuntimeServiceServicer):
    def __init__(
        self,
        *,
        router_agent: RouterAgent | None = None,
        alert_analysis_agent: AlertAnalysisAgent | None = None,
        chat_qa_agent: ChatQAAgent | None = None,
        tool_agent: ToolAgent | None = None,
        llm_client: OpenAICompatibleClient | None = None,
    ) -> None:
        self.llm_client = llm_client or build_openai_compatible_client()
        self.router_agent = router_agent or RouterAgent()
        self.alert_analysis_agent = alert_analysis_agent or AlertAnalysisAgent(llm_client=self.llm_client)
        self.chat_qa_agent = chat_qa_agent or ChatQAAgent(llm_client=self.llm_client)
        self.tool_agent = tool_agent or ToolAgent(llm_client=self.llm_client)
        self.router_graph = build_router_graph(self.router_agent)
        self.alert_analysis_graph = build_alert_analysis_graph(self.alert_analysis_agent)
        self.chat_qa_graph = build_chat_qa_graph(self.chat_qa_agent)

    def AnalyzeAlert(self, request: runtime_pb2.AnalyzeAlertRequest, context=None) -> runtime_pb2.AnalyzeAlertResponse:
        result = self.alert_analysis_graph.invoke(request)
        return alert_result_to_proto(result)

    def RunConversationTurn(
        self,
        request: runtime_pb2.RunConversationTurnRequest,
        context=None,
    ) -> runtime_pb2.RunConversationTurnResponse:
        decision = self.router_graph.invoke(request)
        if decision.route == "alert_analysis":
            alert_result = self.alert_analysis_graph.invoke(conversation_request_to_alert_request(request))
            result = ConversationTurnResult(
                status=alert_result.status,
                answer=alert_result.summary,
                route=decision.route,
                trace=decision.trace + alert_result.trace,
            )
            return conversation_result_to_proto(result)

        if decision.route == "tool":
            tool_result = self.tool_agent.execute(request)
            tool_result.trace = decision.trace + tool_result.trace
            tool_result.route = decision.route
            return conversation_result_to_proto(tool_result)

        chat_result = self.chat_qa_graph.invoke(request)
        chat_result.trace = decision.trace + chat_result.trace
        chat_result.route = decision.route
        return conversation_result_to_proto(chat_result)

    def Health(self, request: runtime_pb2.HealthRequest, context=None) -> runtime_pb2.HealthResponse:
        result = HealthResult(
            status="ok",
            service="python-ai",
            version="0.1.0",
            trace=[],
        )
        return health_result_to_proto(result)
