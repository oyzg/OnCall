from app.core.config import AppSettings
from app.schemas.health import ComponentStatus, HealthReport
from app.services.hybrid_rag import HybridRAGService


def build_health_report(settings: AppSettings) -> HealthReport:
    runtime_status, runtime_detail = check_runtime_model(settings)
    embedding_status, embedding_detail = check_embedding(settings)
    elasticsearch_status, elasticsearch_detail = check_elasticsearch(settings)
    milvus_status, milvus_detail = check_milvus(settings)

    components = [
        ComponentStatus(name="fastapi_http", status="up", detail=f"port={settings.http_port}"),
        ComponentStatus(
            name="grpc_runtime",
            status="up",
            detail=f"{settings.grpc_host}:{settings.grpc_port}",
        ),
        ComponentStatus(name="runtime_model", status=runtime_status, detail=runtime_detail),
        ComponentStatus(name="embedding_model", status=embedding_status, detail=embedding_detail),
        ComponentStatus(name="elasticsearch", status=elasticsearch_status, detail=elasticsearch_detail),
        ComponentStatus(name="milvus", status=milvus_status, detail=milvus_detail),
    ]

    return HealthReport(
        service=settings.app_name,
        env=settings.app_env,
        status=overall_status([item.status for item in components]),
        components=components,
    )


def check_runtime_model(settings: AppSettings) -> tuple[str, str]:
    if settings.openai_api_key and settings.openai_base_url and settings.runtime_api_model:
        return "up", f"{settings.runtime_api_model} via {settings.openai_base_url.rstrip('/')}"
    if settings.runtime_api_model:
        return "fallback", f"{settings.runtime_api_model} (stub fallback: missing OPENAI_API_KEY)"
    return "down", "missing runtime model configuration"


def check_embedding(settings: AppSettings) -> tuple[str, str]:
    return HybridRAGService(settings).embedding_health()


def check_elasticsearch(settings: AppSettings) -> tuple[str, str]:
    import json
    from urllib.error import URLError
    from urllib.request import urlopen

    try:
        with urlopen(settings.elasticsearch_url, timeout=2) as response:
            payload = json.loads(response.read().decode("utf-8"))
            return "up", payload.get("cluster_name", settings.elasticsearch_url)
    except URLError as exc:
        return "down", str(exc.reason)
    except Exception as exc:
        return "down", str(exc)


def check_milvus(settings: AppSettings) -> tuple[str, str]:
    try:
        from pymilvus import connections, utility

        host, port = settings.milvus_address.split(":", 1)
        alias = "healthcheck"
        connections.connect(alias=alias, host=host, port=port)
        utility.list_collections(using=alias)
        connections.disconnect(alias)
        return "up", settings.milvus_address
    except Exception as exc:
        return "fallback", f"local_fallback ({exc})"


def overall_status(statuses: list[str]) -> str:
    if any(status == "down" for status in statuses):
        return "degraded"
    if any(status == "fallback" for status in statuses):
        return "degraded"
    return "ok"
