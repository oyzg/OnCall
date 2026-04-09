from __future__ import annotations

from dataclasses import dataclass

from app.core.config import AppSettings, get_settings


@dataclass(slots=True)
class OpenAICompatibleClient:
    base_url: str
    api_key: str
    model: str
    timeout_seconds: int = 15

    def complete(self, prompt: str) -> str:
        prompt = prompt.strip()
        if not prompt:
            return ""
        return f"[{self.model}] {prompt}"

    def chat(self, messages: list[str]) -> str:
        cleaned = " ".join(message.strip() for message in messages if message.strip())
        return self.complete(cleaned)


def build_openai_compatible_client(settings: AppSettings | None = None) -> OpenAICompatibleClient:
    current = settings or get_settings()
    return OpenAICompatibleClient(
        base_url=current.openai_base_url,
        api_key=current.openai_api_key,
        model=current.runtime_api_model,
        timeout_seconds=current.runtime_api_timeout_seconds,
    )
