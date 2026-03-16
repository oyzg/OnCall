from app.core.config import AppSettings
from app.schemas.health import ComponentStatus, HealthReport


def build_health_report(settings: AppSettings) -> HealthReport:
    components = [
        ComponentStatus(name="langchain", status="up"),
        ComponentStatus(name="langgraph", status="up"),
        ComponentStatus(name="elasticsearch", status="configured", detail=settings.elasticsearch_url),
        ComponentStatus(name="milvus", status="configured", detail=settings.milvus_address),
    ]

    return HealthReport(
        service=settings.app_name,
        env=settings.app_env,
        status="ok",
        components=components,
    )
