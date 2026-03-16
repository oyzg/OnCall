from __future__ import annotations

from datetime import UTC, datetime

from langchain_core.prompts import ChatPromptTemplate

from app.schemas.analysis import AlertAnalysisRequest, AlertAnalysisResponse


class AlertAnalysisService:
    def __init__(self) -> None:
        self.prompt = ChatPromptTemplate.from_messages(
            [
                (
                    "system",
                    "You are an on-call incident analysis assistant. Summarize alerts, infer likely causes, and suggest next actions.",
                ),
                (
                    "human",
                    (
                        "Alert title: {title}\n"
                        "Service: {service}\n"
                        "Environment: {environment}\n"
                        "Severity: {severity}\n"
                        "Source: {source}\n"
                        "Summary: {summary}\n"
                        "Description: {description}\n"
                        "Labels: {labels}\n"
                        "Linked session: {linked_session_id}\n"
                    ),
                ),
            ]
        )

    def analyze_alert(self, request: AlertAnalysisRequest) -> AlertAnalysisResponse:
        rendered_prompt = self.prompt.invoke(
            {
                "title": request.title,
                "service": request.service,
                "environment": request.environment,
                "severity": request.severity,
                "source": request.source,
                "summary": request.summary,
                "description": request.description,
                "labels": request.labels,
                "linked_session_id": request.linked_session_id or "none",
            }
        )

        context_text = "\n".join(message.content for message in rendered_prompt.messages)
        return AlertAnalysisResponse(
            summary=self._build_summary(request),
            severity_assessment=self._severity_assessment(request),
            possible_causes=self._infer_causes(request, context_text),
            suggested_actions=self._infer_actions(request),
            recommended_tools=self._recommend_tools(request),
            knowledge_queries=self._knowledge_queries(request),
            workflow=self._workflow_name(request),
            confidence=self._confidence(request),
            source="python-ai-langchain-rule-analysis",
            generated_at=datetime.now(UTC).isoformat(),
        )

    def _build_summary(self, request: AlertAnalysisRequest) -> str:
        if request.severity in {"P0", "P1"}:
            return (
                f"{request.service} 在 {request.environment} 环境出现高优先级告警，"
                "建议立即围绕依赖链路、资源负载和最近变更展开排查。"
            )
        return (
            f"{request.service} 当前存在 {request.severity} 级别告警，"
            "建议先结合近期告警、知识 SOP 和服务状态做快速定位。"
        )

    def _severity_assessment(self, request: AlertAnalysisRequest) -> str:
        if request.severity == "P0":
            return "P0 代表系统级故障风险，通常需要立即升级处理并同步相关负责人。"
        if request.severity == "P1":
            return "P1 已具备明显用户影响或核心链路风险，应优先检查下游依赖、数据库和发布变更。"
        if request.severity == "P2":
            return "P2 更适合先在值班侧完成限定范围排查，再决定是否升级。"
        return "P3 以观察和补充上下文为主，可结合知识库和历史案例快速排除。"

    def _infer_causes(self, request: AlertAnalysisRequest, context_text: str) -> list[str]:
        causes: list[str] = []
        text = f"{request.title} {request.summary} {request.description} {context_text}".lower()

        if "latency" in text or "延迟" in text or "timeout" in text:
            causes.append("接口延迟升高，可能由下游依赖抖动、数据库慢查询或连接池耗尽导致。")
        if "error" in text or "5xx" in text or "错误" in text:
            causes.append("错误率异常，可能与近期发布、配置变更、依赖服务异常或流量激增有关。")
        if "cpu" in text or "memory" in text or "load" in text:
            causes.append("资源利用率异常，建议优先检查容器/实例资源和扩缩容策略。")
        if not causes:
            causes.extend(
                [
                    "最近的发布、配置变更或灰度流量可能触发了当前告警。",
                    "下游依赖抖动或服务自身负载上升，导致指标持续恶化。",
                ]
            )
        return causes[:3]

    def _infer_actions(self, request: AlertAnalysisRequest) -> list[str]:
        actions = [
            f"先查看 {request.service} 的服务状态、实例资源和最近 30 分钟内相关告警。",
            "核对最近一次发布、配置变更和依赖服务健康状况。",
        ]
        if request.linked_session_id:
            actions.append("基于已关联排障会话继续记录排查结论，并同步关键操作时间线。")
        else:
            actions.append("创建排障会话，沉淀处理过程并继续联动工具中心。")
        return actions

    def _recommend_tools(self, request: AlertAnalysisRequest) -> list[str]:
        tools = ["service_status", "recent_alerts", "knowledge_search"]
        if request.severity in {"P0", "P1"}:
            tools.append("platform_overview")
        return tools

    def _knowledge_queries(self, request: AlertAnalysisRequest) -> list[str]:
        queries = [
            f"{request.service} {request.title}",
            f"{request.service} {request.summary}",
        ]
        for key, value in list(request.labels.items())[:2]:
            queries.append(f"{request.service} {key} {value}")
        return queries[:4]

    def _workflow_name(self, request: AlertAnalysisRequest) -> str:
        if request.severity in {"P0", "P1"}:
            return "langgraph_incident_escalation"
        return "langgraph_standard_alert_triage"

    def _confidence(self, request: AlertAnalysisRequest) -> str:
        if request.severity in {"P0", "P1"}:
            return "high"
        return "medium"
