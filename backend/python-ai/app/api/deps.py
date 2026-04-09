from functools import lru_cache

from app.chains.base import ChainRegistry, build_chain_registry
from app.core.config import AppSettings, get_settings
from app.graphs.base import GraphRegistry, build_graph_registry
from app.services.alert_analysis import AlertAnalysisService
from app.services.hybrid_rag import HybridRAGService, build_rag_service


@lru_cache
def get_app_settings() -> AppSettings:
    return get_settings()


@lru_cache
def get_chain_registry() -> ChainRegistry:
    return build_chain_registry()


@lru_cache
def get_graph_registry() -> GraphRegistry:
    return build_graph_registry()


@lru_cache
def get_alert_analysis_service() -> AlertAnalysisService:
    return AlertAnalysisService()


@lru_cache
def get_rag_service() -> HybridRAGService:
    return build_rag_service(get_app_settings())
