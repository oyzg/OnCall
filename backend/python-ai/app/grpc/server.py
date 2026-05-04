from __future__ import annotations

from concurrent.futures import ThreadPoolExecutor

import grpc

from app.core.config import AppSettings, get_settings
from app.grpc.services.runtime_service import RuntimeService

from app.gen.proto.ai import runtime_pb2_grpc


def create_server(
    settings: AppSettings | None = None,
    service: RuntimeService | None = None,
    *,
    bind: bool = True,
) -> grpc.Server:
    current = settings or get_settings()
    server = grpc.server(ThreadPoolExecutor(max_workers=4))
    runtime_pb2_grpc.add_RuntimeServiceServicer_to_server(service or RuntimeService(settings=current), server)
    if bind:
        address = f"{current.grpc_host}:{current.grpc_port}"
        bound_port = server.add_insecure_port(address)
        if not bound_port:
            raise RuntimeError(f"Failed to bind gRPC server to {address}")
    return server


def serve(settings: AppSettings | None = None) -> grpc.Server:
    server = create_server(settings=settings, bind=True)
    server.start()
    return server
