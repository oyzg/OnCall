from fastapi import APIRouter

router = APIRouter()


@router.get("/healthz")
def healthz() -> dict[str, str]:
    return {"service": "python-ai", "status": "ok"}


@router.get("/")
def root() -> dict[str, str]:
    return {"message": "AI OnCall Python AI bootstrap is ready"}
