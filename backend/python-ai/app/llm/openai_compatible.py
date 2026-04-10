from __future__ import annotations

from dataclasses import dataclass
import json
from typing import Any, Mapping
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen

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
        return self.chat([{"role": "user", "content": prompt}])

    def chat(self, messages: list[str] | list[dict[str, str]]) -> str:
        normalized_messages = _normalize_messages(messages)
        if not normalized_messages:
            return ""

        if not self.api_key.strip() or not self.base_url.strip():
            return _fallback_text(self.model, normalized_messages)

        endpoint = self.base_url.rstrip("/")
        if not endpoint.endswith("/chat/completions"):
            endpoint = f"{endpoint}/chat/completions"

        payload = {
            "model": self.model,
            "messages": normalized_messages,
        }
        headers = {
            "Content-Type": "application/json",
            "Authorization": f"Bearer {self.api_key}",
        }
        request = Request(
            url=endpoint,
            method="POST",
            data=json.dumps(payload, ensure_ascii=False).encode("utf-8"),
            headers=headers,
        )

        try:
            with urlopen(request, timeout=self.timeout_seconds) as response:
                raw = json.loads(response.read().decode("utf-8"))
        except (HTTPError, URLError, TimeoutError, OSError, json.JSONDecodeError):
            return _fallback_text(self.model, normalized_messages)

        try:
            content = raw["choices"][0]["message"]["content"]
        except (KeyError, IndexError, TypeError):
            return _fallback_text(self.model, normalized_messages)
        if not isinstance(content, str) or not content.strip():
            return _fallback_text(self.model, normalized_messages)
        return content.strip()

    def complete_structured(self, *, prompt: str, fallback: Mapping[str, Any]) -> dict[str, Any]:
        raw = self.complete(prompt).strip()
        if not raw:
            return dict(fallback)

        parsed = _parse_json_object(raw)
        if parsed is None:
            return dict(fallback)
        return {**dict(fallback), **parsed}


def _parse_json_object(raw: str) -> dict[str, Any] | None:
    candidates = [raw]
    if "{" in raw and "}" in raw:
        start = raw.find("{")
        end = raw.rfind("}")
        if start >= 0 and end > start:
            candidates.append(raw[start : end + 1])

    for candidate in candidates:
        try:
            parsed = json.loads(candidate)
        except json.JSONDecodeError:
            continue
        if isinstance(parsed, dict):
            return parsed
    return None


def _normalize_messages(messages: list[str] | list[dict[str, str]]) -> list[dict[str, str]]:
    normalized: list[dict[str, str]] = []
    for message in messages:
        if isinstance(message, dict):
            role = str(message.get("role", "user")).strip() or "user"
            content = str(message.get("content", "")).strip()
        else:
            role = "user"
            content = str(message).strip()
        if content:
            normalized.append({"role": role, "content": content})
    return normalized


def _fallback_text(model: str, messages: list[dict[str, str]]) -> str:
    cleaned_parts = [message["content"].strip() for message in messages if message["content"].strip()]
    if not cleaned_parts:
        return ""
    return f"[{model}] {' '.join(cleaned_parts)}"


def build_openai_compatible_client(settings: AppSettings | None = None) -> OpenAICompatibleClient:
    current = settings or get_settings()
    return OpenAICompatibleClient(
        base_url=current.openai_base_url,
        api_key=current.openai_api_key,
        model=current.runtime_api_model,
        timeout_seconds=current.runtime_api_timeout_seconds,
    )
