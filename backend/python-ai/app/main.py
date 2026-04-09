from contextlib import asynccontextmanager

from fastapi import FastAPI

from app.core.config import get_settings
from app.core.exceptions import AppError, app_error_handler, unhandled_exception_handler
from app.core.logging import setup_logging
from app.grpc.server import create_server
from app.schemas.response import success
from app.services.health import build_health_report

settings = get_settings()
logger = setup_logging(settings.log_level)

@asynccontextmanager
async def lifespan(app: FastAPI):
    grpc_server = create_server(settings=settings, bind=True)
    grpc_server.start()
    app.state.grpc_server = grpc_server
    try:
        yield
    finally:
        grpc_server.stop(grace=0)


app = FastAPI(title="AI OnCall Python AI Service", lifespan=lifespan)
app.add_exception_handler(AppError, app_error_handler)
app.add_exception_handler(Exception, unhandled_exception_handler)


@app.get("/healthz")
def healthz() -> dict:
    return success(build_health_report(settings).model_dump())


@app.get("/")
def root() -> dict:
    return success(
        {
            "message": "AI OnCall Python AI base server is ready",
            "service": settings.app_name,
            "env": settings.app_env,
        }
    )

logger.info("python ai service initialized")
