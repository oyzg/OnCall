from typing import Any

from pydantic import BaseModel, Field


class Envelope(BaseModel):
    code: str = Field(default="OK")
    message: str = Field(default="success")
    data: Any | None = None


def success(data: Any | None = None) -> dict[str, Any]:
    return Envelope(data=data).model_dump()
