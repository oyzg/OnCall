from pydantic import BaseModel


class ComponentStatus(BaseModel):
    name: str
    status: str
    detail: str | None = None


class HealthReport(BaseModel):
    service: str
    env: str
    status: str
    components: list[ComponentStatus]
