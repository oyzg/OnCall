from fastapi import FastAPI

from app.api.routes import router
from app.core.config import get_settings
from app.core.exceptions import AppError, app_error_handler, unhandled_exception_handler
from app.core.logging import setup_logging

settings = get_settings()
logger = setup_logging(settings.log_level)

app = FastAPI(title="AI OnCall Python AI Service")
app.add_exception_handler(AppError, app_error_handler)
app.add_exception_handler(Exception, unhandled_exception_handler)
app.include_router(router)

logger.info("python ai service initialized")
