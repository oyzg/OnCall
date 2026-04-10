from __future__ import annotations

import json
import unittest

from app.grpc.mappers import bootstrap_proto_modules

bootstrap_proto_modules()

from app.agents.tool_agent import ToolAgent
from app.gen.proto.ai import runtime_pb2
from app.gen.proto.common import metadata_pb2


class FakeLLMClient:
    def complete(self, prompt: str) -> str:
        return f"llm:{prompt}"


class FakeToolGateway:
    def __init__(self) -> None:
        self.calls: list[tuple[str, str, list[str], dict[str, object]]] = []

    def execute_tool(
        self,
        *,
        user_id: str,
        user_roles: list[str],
        tool_name: str,
        parameters: dict[str, object],
    ) -> dict[str, object]:
        self.calls.append((user_id, tool_name, list(user_roles), dict(parameters)))
        return {
            "status": "success",
            "result": {
                "service": "payment-api",
                "environment": "prod",
                "open_alerts": 2,
                "risk": "degraded",
            },
            "duration_ms": 12,
        }


class ToolAgentTest(unittest.TestCase):
    def test_execute_calls_gateway_and_returns_real_tool_metadata(self) -> None:
        gateway = FakeToolGateway()
        agent = ToolAgent(llm_client=FakeLLMClient(), tool_gateway=gateway)

        result = agent.execute(
            runtime_pb2.RunConversationTurnRequest(
                metadata=metadata_pb2.RequestMetadata(
                    request_id="req-tool-1",
                    user_id="user-1",
                    session_id="session-1",
                ),
                session_id="session-1",
                user_id="user-1",
                user_roles=["ops"],
                message="Please check payment-api status in prod with a tool",
                allowed_tools=["service_status"],
                retrieval_limit=3,
            )
        )

        self.assertEqual("tool", result.route)
        self.assertEqual("ready", result.status)
        self.assertEqual(1, len(gateway.calls))
        self.assertEqual("user-1", gateway.calls[0][0])
        self.assertEqual("service_status", gateway.calls[0][1])
        self.assertEqual(["ops"], gateway.calls[0][2])
        self.assertEqual("payment-api", gateway.calls[0][3]["service"])
        self.assertEqual("prod", gateway.calls[0][3]["environment"])
        self.assertTrue(result.tool_calls)
        self.assertEqual("service_status", result.tool_calls[0].name)
        self.assertEqual("success", result.tool_calls[0].outcome)
        self.assertIn("payment-api", result.tool_calls[0].arguments_json)
        self.assertIn("payment-api", result.answer)
        self.assertTrue(result.trace)
        self.assertIn("tool", [entry.stage for entry in result.trace])

    def test_execute_marks_failed_tool_calls_when_gateway_errors(self) -> None:
        class FailingToolGateway:
            def execute_tool(self, *, user_id: str, user_roles: list[str], tool_name: str, parameters: dict[str, object]):
                del user_id, user_roles, tool_name, parameters
                raise RuntimeError("tool gateway unavailable")

        agent = ToolAgent(llm_client=FakeLLMClient(), tool_gateway=FailingToolGateway())

        result = agent.execute(
            runtime_pb2.RunConversationTurnRequest(
                metadata=metadata_pb2.RequestMetadata(
                    request_id="req-tool-2",
                    user_id="user-2",
                    session_id="session-2",
                ),
                session_id="session-2",
                user_id="user-2",
                user_roles=["ops"],
                message="Run service status for checkout-api",
                allowed_tools=["service_status"],
                retrieval_limit=3,
            )
        )

        self.assertEqual("tool", result.route)
        self.assertEqual("failed", result.status)
        self.assertEqual(1, len(result.tool_calls))
        self.assertEqual("failed", result.tool_calls[0].outcome)
        self.assertIn("tool gateway unavailable", result.error)
        self.assertIn("tool gateway unavailable", result.answer)
        self.assertTrue(any(entry.severity == "error" for entry in result.trace))
        parsed = json.loads(result.tool_calls[0].arguments_json)
        self.assertEqual("checkout-api", parsed["service"])


if __name__ == "__main__":
    unittest.main()
