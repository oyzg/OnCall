from fastapi import APIRouter, Depends

from app.api.deps import get_app_settings, get_chain_registry, get_graph_registry
from app.chains.base import ChainRegistry
from app.core.config import AppSettings
from app.graphs.base import GraphRegistry
from app.schemas.response import success
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
            "chain_registry": chains.name,
            "graph_registry": graphs.name,
        }
    )
