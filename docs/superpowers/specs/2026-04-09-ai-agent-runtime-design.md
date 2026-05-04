# AI OnCall AI Agent Runtime Design

## 1. 背景

当前 AI OnCall 已经具备可演示的主链路：

- Go 后端承接登录、会话、知识库、告警、工具、审计
- Python AI 服务承接告警分析和 RAG 检索
- 前端可以完成会话问答、告警分析、知识检索测试和工具调用展示

但现状仍然停留在 MVP 级 AI 形态：

1. 告警分析主要是规则化逻辑，不是真实模型推理。
2. 对话问答主要依赖 Go 侧检索结果拼接，不是真正的 AI workflow。
3. Go 与 Python 之间仍然是 HTTP 边界，尚未落地设计文档中规划的 gRPC 协作。
4. LangChain / LangGraph 目录已经预留，但没有形成真正的 Agent runtime。

因此，这一轮不是“给现有接口换一个模型”这么简单，而是把 Python AI 服务升级为真正的 AI runtime：

- 使用 OpenAI-compatible 模型
- 使用 gRPC 替代主链路 HTTP 通信
- 使用 LangGraph 组织 Agent workflow
- 用多个专职 Agent 替代当前规则流

## 2. 目标

### 2.1 业务目标

- 让告警分析输出来自真实模型和真实上下文增强链路，而不是纯规则兜底。
- 让会话问答具备基于知识检索和工具调用的真实 AI 回答能力。
- 保持现有前端页面和 Go 业务接口尽量稳定，不因为 AI 升级而推翻业务层。

### 2.2 架构目标

- 将 Go 与 Python AI 服务的主链路通信切换为 gRPC。
- 在 Python AI 服务中落地 LangGraph runtime。
- 采用多 Agent 协作方案，但严格限制每个 Agent 的职责边界。
- 保持 Go 为业务主入口，Python 为 AI 能力边界，不让 LangGraph 侵入 Go 业务模块。

### 2.3 展示目标

- 让项目可以明确表达为：
  `Vue 3 + Go + Python + gRPC + LangGraph + OpenAI-compatible + RAG + Tool Agent`
- 能在面试中解释清楚：
  - 为什么不把 Agent 放到 Go 里
  - 为什么 Router Agent 和 Tool Agent 要分开
  - 为什么先保留 Go 侧 SSE，而不急着做 Python 真流式

## 3. 非目标

本轮刻意不做以下事项：

- 不做前端 UI 重构
- 不做完整用户中心和 RBAC 重构
- 不把工具中心升级为外部系统适配平台
- 不在第一版做 Python 到前端的 token 级真流式输出
- 不把所有 AI 场景都接成多 Agent，只覆盖告警分析和会话问答两条主链路
- 不追求一步到位的复杂多 Agent 自主协作

## 4. 方案选择

本轮最终采用激进方案：

- 真实模型接入
- gRPC 替换主链路 HTTP
- LangGraph 落地
- 多 Agent 协作

但为了控制复杂度，首版只开放两个业务入口：

- `AnalyzeAlert`
- `RunConversationTurn`

也就是说，系统内部是多 Agent runtime，系统对外仍是两个稳定业务能力。

## 5. 总体架构

### 5.1 Go 侧职责

Go 仍然负责：

- 鉴权
- 会话与消息持久化
- 知识元数据与文档生命周期
- 告警管理与状态流转
- 工具定义与调用日志
- 审计记录

Go 不负责：

- LangGraph graph 执行
- Agent 路由决策
- 模型推理

Go 对 Python AI 的调用统一收口到 AI gateway / orchestrator。

### 5.2 Python 侧职责

Python AI 服务升级为真正的 AI runtime，负责：

- OpenAI-compatible 模型接入
- LangGraph graph 组织
- Router Agent 路由
- Alert Analysis Agent 执行
- Chat QA Agent 执行
- Tool Agent 执行
- RAG adapter、tool adapter 等 AI 边界能力

### 5.3 通信边界

Go 与 Python 主链路通信统一改为 gRPC：

- Go 只关心稳定业务协议
- Python 内部的 graph state、node 拆分、agent 决策不直接暴露给 Go

FastAPI HTTP 继续保留，但降级为：

- 健康检查
- 本地调试
- 回归验证入口

## 6. Agent 设计

### 6.1 Router Agent

职责：

- 根据请求类型和上下文决定路由方向
- 判断是否需要 RAG
- 判断是否需要工具调用
- 输出 route decision 和原因

输出示例：

- `route=alert_analysis`
- `route=chat_qa`
- `needs_rag=true`
- `needs_tooling=false`

约束：

- 不直接输出最终业务结果
- 不直接执行工具
- 只做路由和策略决策

### 6.2 Alert Analysis Agent

职责：

- 针对单条告警生成结构化分析结果
- 可按需调用 RAG 和 Tool Agent 补充上下文

输出字段必须兼容现有前端：

- `summary`
- `severity_assessment`
- `possible_causes`
- `suggested_actions`
- `recommended_tools`
- `knowledge_queries`
- `workflow`
- `confidence`
- `source`

约束：

- 不承接自由聊天
- 以结构化输出为主

### 6.3 Chat QA Agent

职责：

- 处理排障问答、知识查询、上下文追问
- 决定是否走 RAG
- 决定是否调用 Tool Agent
- 综合结果生成最终回答

输出：

- `answer`
- `citations`
- `tool_calls`
- `status`

约束：

- 第一版只读，不直接修改业务状态
- 最终消息持久化仍由 Go 完成

### 6.4 Tool Agent

职责：

- 统一封装现有工具调用
- 管理工具参数校验、执行、错误归一化和结果回填

第一版只接现有内建工具：

- `service_status`
- `recent_alerts`
- `knowledge_search`
- `platform_overview`

约束：

- Tool Agent 不生成最终业务答案
- 只负责执行工具并返回结构化结果

## 7. LangGraph 设计

### 7.1 Graph 切分

第一版使用三类 graph：

- `router_graph`
- `alert_analysis_graph`
- `chat_qa_graph`

其中：

- `router_graph` 负责 route decision
- `alert_analysis_graph` 负责告警分析链路
- `chat_qa_graph` 负责问答链路

### 7.2 State 设计

统一基础 state，避免不同 graph 完全割裂：

- `request_id`
- `request_type`
- `user_id`
- `user_roles`
- `session_id`
- `alert_context`
- `messages`
- `route`
- `retrieval_result`
- `tool_results`
- `draft_answer`
- `final_output`
- `errors`
- `trace`

### 7.3 首版控制原则

虽然逻辑上是多 Agent 协作，但首版执行上尽量保持“受控 workflow”，不做完全开放式 agent-to-agent 自由循环。

也就是说：

- Router 决定路径
- Alert / Chat Agent 调用 RAG 或 Tool Agent
- 最终由当前主 Agent 统一收敛输出

不允许无限递归调用和开放式自治。

## 8. gRPC 协议设计

### 8.1 对外主 RPC

首版只开放两个主 RPC：

- `AnalyzeAlert`
- `RunConversationTurn`

可选辅助 RPC：

- `Health`
- `DebugRouteDecision` 仅开发态使用

### 8.2 AnalyzeAlert

请求字段建议包含：

- `alert_id`
- `title`
- `service`
- `environment`
- `severity`
- `source`
- `summary`
- `description`
- `labels`
- `triggered_at`
- `linked_session_id`
- `user_id`
- `user_roles`

响应字段：

- `status`
- `summary`
- `severity_assessment`
- `possible_causes`
- `suggested_actions`
- `recommended_tools`
- `knowledge_queries`
- `workflow`
- `confidence`
- `source`
- `generated_at`
- `error`
- `trace`

### 8.3 RunConversationTurn

请求字段建议包含：

- `session_id`
- `user_id`
- `user_roles`
- `message`
- `history`
- `linked_alert`
- `allowed_tools`
- `retrieval_limit`

响应字段：

- `answer`
- `citations`
- `tool_calls`
- `route`
- `status`
- `error`
- `trace`

## 9. 数据流设计

### 9.1 告警分析链路

1. Go `alert` 模块接收分析请求
2. Go gateway 通过 gRPC 调 Python `AnalyzeAlert`
3. Python 进入 `router_graph`
4. Router 决定进入 `alert_analysis_graph`
5. Alert Analysis Agent 按需调用 RAG / Tool Agent
6. Python 返回结构化分析结果
7. Go 写回告警详情并记录审计

### 9.2 会话问答链路

1. Go `session` 模块接收用户消息
2. Go 先落 user message
3. Go gateway 通过 gRPC 调 Python `RunConversationTurn`
4. Python 进入 `router_graph`
5. Router 决定进入 `chat_qa_graph`
6. Chat QA Agent 按需调用 RAG / Tool Agent
7. Python 返回完整 answer、citations、tool_calls
8. Go 将 answer 按当前方式切片，通过 SSE 发给前端
9. Go 持久化 assistant message

### 9.3 为什么保留 Go 侧 SSE

本轮不做 Python 到前端的真流式，原因是：

- 前端当前已稳定接入 Go SSE
- 先把 gRPC 和 Agent runtime 稳定起来更重要
- Go 伪流式足以满足当前演示和主链路需求

后续如需升级，可在 Python 增加 streaming RPC，再由 Go 做桥接。

## 10. 代码结构设计

### 10.1 Go 侧

重点改造：

- [backend/go-api/internal/ai/gateway/client.go](/Users/ouyangzhenguang/project/OnCall/backend/go-api/internal/ai/gateway/client.go)
- [backend/go-api/internal/ai/eino/orchestrator.go](/Users/ouyangzhenguang/project/OnCall/backend/go-api/internal/ai/eino/orchestrator.go)
- [backend/go-api/internal/ai/analyzer/service.go](/Users/ouyangzhenguang/project/OnCall/backend/go-api/internal/ai/analyzer/service.go)
- [backend/go-api/internal/session/api/handler.go](/Users/ouyangzhenguang/project/OnCall/backend/go-api/internal/session/api/handler.go)

### 10.2 Proto 层

新增：

- `proto/ai/runtime.proto`

用于定义：

- alert analysis request / response
- conversation turn request / response
- citations
- tool calls
- trace events

### 10.3 Python 侧

建议新增目录：

- `backend/python-ai/app/grpc/`
- `backend/python-ai/app/llm/`
- `backend/python-ai/app/agents/`
- `backend/python-ai/app/graphs/`
- `backend/python-ai/app/adapters/`

保留并复用：

- [backend/python-ai/app/services/hybrid_rag.py](/Users/ouyangzhenguang/project/OnCall/backend/python-ai/app/services/hybrid_rag.py)
  作为 RAG 底层能力

降级旧实现：

- [backend/python-ai/app/services/alert_analysis.py](/Users/ouyangzhenguang/project/OnCall/backend/python-ai/app/services/alert_analysis.py)
  作为迁移参考，不继续堆新逻辑

## 11. 分阶段交付顺序

### Phase 1：协议和骨架

交付：

- `runtime.proto`
- Go/Python gRPC stub
- Python gRPC server
- Go gRPC gateway
- 主链路从 HTTP 切到 gRPC

目标：

- 即便先返回 mock，也要让新边界跑通

### Phase 2：Alert Analysis Agent

交付：

- Router Agent
- Alert Analysis Agent
- Tool Agent 最小版
- `AnalyzeAlert` 接真实模型

目标：

- 先完成结构化输出最清晰的一条链路

### Phase 3：Chat QA Agent

交付：

- Chat QA Agent
- RAG adapter 接入 graph
- `RunConversationTurn` 输出 answer / citations / tool_calls

目标：

- 用 Python AI runtime 替换 Go 本地检索拼答

### Phase 4：路由增强与稳定化

交付：

- Router 决策增强
- trace 可观测性增强
- prompt 与 structured output 稳定化

目标：

- 提升可解释性和调试效率

## 12. 测试策略

### 12.1 Go 侧

- gateway / orchestrator 单测
- 告警分析链路集成测试
- 会话 SSE 链路集成测试

### 12.2 Python 侧

- proto mapper 单测
- model client 单测
- Router Agent 单测
- Alert Analysis Agent 单测
- Chat QA Agent 单测
- graph 流程测试

### 12.3 联调测试

- Go 调 Python gRPC 的端到端测试
- 告警分析真实输出字段兼容测试
- 会话问答 citations / tool_calls 兼容测试

## 13. 风险与取舍

### 13.1 为什么不是先做单 Agent

因为本轮目标已经明确为多 Agent runtime，如果退回单 Agent，会和本轮目标不一致。

但为了控制复杂度，首版仍然用“多 Agent + 受控 graph”而不是“完全自治多 Agent”。

### 13.2 为什么不把 Agent 放到 Go

因为：

- Go 是业务边界层，不适合承载 LangGraph runtime
- Python 更适合承接模型、prompt、tool calling 和 graph workflow
- 把 Agent 留在 Python，更符合当前项目的多语言分层设计

### 13.3 为什么保留现有前端形态

因为这轮核心是 AI runtime 重构，不是 UI 重构。保持前端稳定可以让改造集中在后端边界和 Python AI 服务内部。

## 14. 预期结果

完成本轮后，项目将从“带 AI 功能的业务平台 MVP”升级为“具备独立 AI runtime 的全栈平台”：

- Go 管业务
- Python 管 AI runtime
- gRPC 管跨语言边界
- LangGraph 管多 Agent workflow
- OpenAI-compatible 模型提供真实推理能力

这会显著提升项目的技术完整度、可讲性和后续扩展空间。
