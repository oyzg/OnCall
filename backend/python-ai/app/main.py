from fastapi import FastAPI

from app.api.routes import router

app = FastAPI(title="AI OnCall Python AI Service")
app.include_router(router)

