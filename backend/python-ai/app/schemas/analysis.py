from pydantic import BaseModel, Field


class AlertAnalysisRequest(BaseModel):
    alert_id: str = Field(default="")
    title: str
    service: str
    environment: str
    severity: str
    source: str
    summary: str = Field(default="")
    description: str = Field(default="")
    labels: dict[str, str] = Field(default_factory=dict)
    triggered_at: str = Field(default="")
    linked_session_id: str = Field(default="")


class AlertAnalysisResponse(BaseModel):
    status: str = "ready"
    summary: str
    severity_assessment: str
    possible_causes: list[str] = Field(default_factory=list)
    suggested_actions: list[str] = Field(default_factory=list)
    recommended_tools: list[str] = Field(default_factory=list)
    knowledge_queries: list[str] = Field(default_factory=list)
    workflow: str = ""
    confidence: str = "medium"
    source: str = "python-ai-alert-analysis"
    generated_at: str
    error: str = ""
