# AI OnCall 开放自治型 Agent 面试讲解

## 1. 一句话定位

这个项目里的 Agent 不是简单的“大模型问答接口”，而是一个独立的 Python AI runtime：

**Go 负责稳定业务边界，Python 负责开放自治型 Agent 的计划、工具调用、观察、反思和结果收敛。**

当前实现更准确地说是：

**Plan-and-Execute 外层规划 + ReAct 内层执行器。**

外层 Agent 会先生成排障计划，内层执行器会在每一步里选择只读工具、执行、观察结果，并决定是否继续或收敛。它不是无限制自治系统，默认最多 6 步，并且写操作必须由用户确认后才执行。

## 2. 为什么不是只调一次大模型

单次模型调用很难解释和治理：

- 它不知道什么时候该查知识库，什么时候该查服务状态。
- 它无法把“计划、工具调用、观察、反思、待确认动作”拆开给前端展示。
- 它很难控制副作用，容易把建议和执行混在一起。
- 它不方便做 trace、审计、fallback 和跨语言协议约束。

这个项目把模型推理拆成一个可观测的自治循环：

1. Router 判断进入告警分析还是排障问答。
2. Autonomous Agent 生成初始计划。
3. ReAct executor 调用只读工具。
4. Agent 根据观察结果反思和收敛。
5. Agent 返回 answer / analysis、tool calls、agent plan、pending actions。
6. Go 持久化消息、告警分析和待确认动作。
7. 用户确认后，Go 才执行写操作并写审计。

## 3. 项目里 Agent 用在哪里

目前覆盖两个主入口：

- `AnalyzeAlert`：告警页生成 AI 分析。
- `RunConversationTurn`：聊天页排障问答。

告警分析链路：

1. 前端调用 Go 告警分析接口。
2. Go 通过 gRPC 调 Python runtime。
3. Python `router_graph` 路由到 `alert_analysis`。
4. `alert_analysis_graph` 进入自治 Agent runner。
5. Agent 生成计划并调用 `service_status`、`recent_alerts`、`knowledge_search`、`platform_overview` 等只读工具。
6. Agent 生成结构化分析、执行计划、工具观察和待确认动作。
7. Go 保存分析结果，并把 pending actions 写入 `agent_actions`。

聊天问答链路：

1. 前端通过 Go SSE 发送消息。
2. Go 保存 user message 和 assistant placeholder。
3. Go 通过 gRPC 调 Python runtime。
4. Python `router_graph` 路由到 `chat_qa` 或 `alert_analysis`。
5. `chat_qa_graph` 进入自治 Agent runner。
6. Agent 自主选择只读工具并生成回答。
7. Go 保存 assistant message 的 answer、references、tool calls、trace、agent plan、pending actions。
8. 前端展示计划时间线和待确认动作。

## 4. 架构分层

前端层：

- Vue 3 页面。
- 聊天页和告警页展示回答、引用、工具调用、trace、agent plan、pending actions。
- 用户可以点击确认执行 Agent 建议的写动作。

Go 业务层：

- 系统唯一业务入口。
- 负责鉴权、会话、告警、工具、审计、持久化和 SSE。
- 新增 `agent_actions` 持久化待确认动作。
- 确认接口是 `POST /api/v1/agent-actions/:actionID/confirm`。

Python AI runtime 层：

- 承接模型、RAG、工具编排和 Agent loop。
- 对外暴露 gRPC：`AnalyzeAlert`、`RunConversationTurn`、`Health`。
- 使用 LangGraph `StateGraph` 承接 runtime graph。
- 默认启用自治 runner，异常时 fallback 到旧固定 workflow。

## 5. 当前关键实现

Python 侧：

- [autonomous_agent.py](/Users/ouyangzhenguang/project/OnCall/backend/python-ai/app/agents/autonomous_agent.py)：`AutonomousAgentRunner`，执行 plan、act、observe、reflect、finalize。
- [chat_qa_graph.py](/Users/ouyangzhenguang/project/OnCall/backend/python-ai/app/graphs/chat_qa_graph.py)：聊天 graph 默认调用自治 runner，异常时 fallback。
- [alert_analysis_graph.py](/Users/ouyangzhenguang/project/OnCall/backend/python-ai/app/graphs/alert_analysis_graph.py)：告警 graph 默认调用自治 runner，异常时 fallback。
- [runtime_service.py](/Users/ouyangzhenguang/project/OnCall/backend/python-ai/app/grpc/services/runtime_service.py)：gRPC facade，透传 agent plan 和 pending actions。

协议与 Go 侧：

- [runtime.proto](/Users/ouyangzhenguang/project/OnCall/proto/ai/runtime.proto)：新增 `AgentPlanStep` 和 `PendingAgentAction`。
- [client.go](/Users/ouyangzhenguang/project/OnCall/backend/go-api/internal/ai/gateway/client.go)：gRPC 响应映射到 Go DTO。
- [agentaction](/Users/ouyangzhenguang/project/OnCall/backend/go-api/internal/agentaction)：新增待确认动作领域模型、持久化和确认执行。

前端侧：

- [ChatPage.vue](/Users/ouyangzhenguang/project/OnCall/frontend/web/src/pages/ChatPage.vue)：展示会话 Agent plan 和 pending actions。
- [AlertsPage.vue](/Users/ouyangzhenguang/project/OnCall/frontend/web/src/pages/AlertsPage.vue)：展示告警分析 Agent plan 和 pending actions。
- [api.ts](/Users/ouyangzhenguang/project/OnCall/frontend/web/src/services/api.ts)：新增类型和确认动作 API。

## 6. 权限与安全边界

这套系统是开放自治型 Agent，但不是无边界自治：

- 读工具可以自动执行。
- 写动作只生成 pending action。
- 用户点击确认后，Go 侧才执行写操作。
- 默认最大 6 步，避免无限循环和工具风暴。
- 所有工具调用和确认动作都进入 trace / tool calls / audit。
- Python runtime 异常时，Go 侧保留 fallback 分析路径。

第一版可确认执行的写动作包括：

- `update_alert_status`
- `link_or_create_session`
- `append_alert_record`

## 7. 面试 30 秒讲法

我在项目里做的不是单次 LLM 调用，而是一个开放自治型 Agent runtime。Go 继续负责业务入口、鉴权、会话、告警、审计和持久化；Python 负责 Plan-and-Execute + ReAct 的自治循环。Agent 会先规划排障步骤，再自动调用只读工具观察系统状态，最后生成回答、工具调用记录、执行计划和待确认动作。写操作不会自动执行，而是落成 `agent_actions`，由用户确认后 Go 执行并写审计。

## 8. 面试 2 分钟讲法

这个项目里，我把 AI 能力从 Go 业务层拆成独立 Python runtime。Python 侧不是简单 prompt，而是 Router + Autonomous Agent runner：Router 判断请求是告警分析还是聊天问答；业务 graph 默认进入自治 runner；runner 按 Plan-and-Execute 生成计划，再用 ReAct 方式在每一步选择工具、执行、观察和反思。

只读工具，比如知识检索、服务状态、近期告警、平台概览，可以由 Agent 自动调用。涉及写操作时，例如更新告警状态、关联排障会话、追加处置记录，Agent 只生成待确认动作，不直接修改业务数据。Go 侧新增 `agent_actions` 持久化这些动作，并提供确认接口。前端可以展示 Agent 的计划时间线、工具观察和确认按钮。

这样做的价值是：AI 推理链路可解释，工具调用可观测，副作用可控，Go 和 Python 的边界清晰，后续扩展新工具或新 Agent 场景也不会污染核心业务层。

## 9. 高频追问

### Q1：为什么不用纯 ReAct？

纯 ReAct 容易边想边做，排障过程不够可解释。我这里用外层 Plan-and-Execute 先给出排障计划，再让内层 ReAct 执行具体工具步骤，兼顾自治性和可观测性。

### Q2：为什么不用纯 Plan-and-Execute？

纯 Plan-and-Execute 对新观察不够灵活。排障里工具结果可能改变下一步判断，所以内层需要 ReAct 的 observe / reflect 能力。

### Q3：这算开放自治型 Agent 吗？

算。它能自主生成计划、选择工具、观察结果、反思并收敛输出。但它不是无限制自治，工程上保留了最大步数、只读自动执行、写动作用户确认、trace 和 fallback。

### Q4：写操作为什么不让 Agent 自动执行？

OnCall 场景里写操作会影响告警状态和处置记录，必须可审计、可追责。Agent 可以建议，但最终执行权留给用户和 Go 业务层。

### Q5：怎么证明不是只调了一个模型？

可以从三个层面证明：

- 协议上返回 `agent_plan`、`pending_actions`、`tool_calls`、`trace`。
- 代码上有 `AutonomousAgentRunner` 和独立的 `agentaction` 模块。
- 页面上能看到计划时间线、工具观察和待确认动作。

### Q6：如果 Python runtime 挂了怎么办？

Go 侧仍然保留 fallback 路径，告警分析不会完全不可用。自治 Agent 是 AI 能力层，不是整个业务系统的单点主脑。

## 10. 总结

这套 Agent 的核心价值不是“用了 Agent 这个词”，而是把排障中的模型推理、知识检索、工具调用、观察反思、待确认动作和审计闭环做成了一个可解释、可扩展、可回滚的工程系统。
