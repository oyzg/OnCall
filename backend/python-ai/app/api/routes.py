from fastapi import APIRouter, Depends

from app.api.deps import (
    get_alert_analysis_service,
    get_app_settings,
    get_chain_registry,
    get_graph_registry,
    get_rag_service,
)
from app.chains.base import ChainRegistry
from app.core.config import AppSettings
from app.graphs.base import GraphRegistry
from app.schemas.analysis import AlertAnalysisRequest
from app.schemas.rag import RAGDeleteRequest, RAGIndexRequest, RAGRetrieveRequest
from app.schemas.response import success
from app.services.alert_analysis import AlertAnalysisService
from app.services.hybrid_rag import HybridRAGService
from app.services.health import build_health_report

router = APIRouter()


@router.get("/healthz")
def healthz(settings: AppSettings = Depends(get_app_settings)) -> dict:
    return success(build_health_report(settings).model_dump())


@router.get("/")
def root(
    settings: AppSettings = Depends(get_app_settings),
    chains: ChainRegistry = Depends(get_chain_registry),
    graphs: GraphRegistry = Depends(get_graph_registry),
) -> dict:
    return success(
        {
            "message": "AI OnCall Python AI base server is ready",
            "service": settings.app_name,
            "env": settings.app_env,
            "runtime_model": settings.runtime_api_model,
            "grpc_endpoint": f"{settings.grpc_host}:{settings.grpc_port}",
            "chain_registry": chains.name,
            "graph_registry": graphs.name,
        }
    )


@router.post("/api/v1/analysis/alert")
def analyze_alert(
    payload: AlertAnalysisRequest,
    service: AlertAnalysisService = Depends(get_alert_analysis_service),
) -> dict:
    return success(service.analyze_alert(payload).model_dump())


@router.post("/api/v1/rag/index")
def index_rag_document(
    payload: RAGIndexRequest,
    service: HybridRAGService = Depends(get_rag_service),
) -> dict:
    return success(service.index_document(payload).model_dump())


@router.post("/api/v1/rag/delete")
def delete_rag_document(
    payload: RAGDeleteRequest,
    service: HybridRAGService = Depends(get_rag_service),
) -> dict:
    return success(service.delete_document(payload).model_dump())


@router.post("/api/v1/rag/retrieve")
def retrieve_rag(
    payload: RAGRetrieveRequest,
    service: HybridRAGService = Depends(get_rag_service),
) -> dict:
    return success(service.retrieve(payload).model_dump())
