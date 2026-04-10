from __future__ import annotations

import json
from dataclasses import dataclass
from urllib.error import HTTPError, URLError
from urllib.request import Request, urlopen


@dataclass(slots=True)
class GoToolGateway:
    base_url: str
    shared_secret: str
    timeout_seconds: int = 8

    def execute_tool(
        self,
        *,
        user_id: str,
        user_roles: list[str],
        tool_name: str,
        parameters: dict[str, object],
    ) -> dict[str, object]:
        payload = json.dumps(
            {
                "user_id": user_id,
                "user_roles": list(user_roles),
                "parameters": dict(parameters),
            },
            ensure_ascii=True,
            sort_keys=True,
        ).encode("utf-8")
        request = Request(
            url=f"{self.base_url.rstrip('/')}/internal/ai/tools/{tool_name}/call",
            data=payload,
            headers={
                "Content-Type": "application/json",
                "X-OnCall-Runtime-Secret": self.shared_secret,
            },
            method="POST",
        )
        try:
            with urlopen(request, timeout=self.timeout_seconds) as response:
                data = json.loads(response.read().decode("utf-8"))
        except HTTPError as exc:
            message = self._read_error_message(exc)
            raise RuntimeError(message) from exc
        except URLError as exc:
            raise RuntimeError(str(exc.reason)) from exc

        if data.get("code") != "OK":
            raise RuntimeError(str(data.get("message") or "tool execution failed"))
        envelope = data.get("data") or {}
        return {
            "status": "success",
            "result": envelope.get("result"),
        }

    def _read_error_message(self, exc: HTTPError) -> str:
        try:
            payload = json.loads(exc.read().decode("utf-8"))
        except Exception:
            return str(exc.reason)
        return str(payload.get("message") or exc.reason)
