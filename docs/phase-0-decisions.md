# AI OnCall 平台阶段 0 决策记录

## 1. 文档目的

本文档用于记录阶段 0 已确认的关键架构与技术选型决策，作为后续阶段 1 项目初始化和后续开发的固定基线。

## 2. 阶段 0 结论

阶段 0 的目标是冻结系统范围、总体架构和核心技术选型。到当前为止，阶段 0 可视为完成。

## 3. 已确认决策

## 3.1 产品与架构定位

- 系统定位为全栈 `AI OnCall 平台`
- 系统不是单一聊天机器人，而是包含对话、告警、知识库、工具调用和审计能力的业务平台
- 架构采用多语言协作模式
- `Go` 负责核心业务后端
- `Python` 负责 AI 应用服务

## 3.2 前端技术栈

- 前端框架：`Vue 3`
- 语言：`TypeScript`
- 构建工具：`Vite`
- UI 组件库：`Element Plus`
- 路由：`Vue Router`
- 状态管理：`Pinia`

## 3.3 Go 核心业务后端技术栈

- 语言：`Go`
- Web 框架：`Gin`
- ORM：`GORM`
- 鉴权方式：`JWT`
- 流式响应：`SSE`

## 3.4 Python AI 服务技术栈

- Web 框架：`FastAPI`
- LLM 应用框架：`LangChain`
- Agent 编排框架：`LangGraph`

## 3.5 AI 编排决策

- Go 侧采用 `Eino` 作为 AI 编排入口和适配层
- Python 侧采用 `LangChain + LangGraph` 承载复杂 AI 工作流
- Go 业务模块不直接依赖 `LangChain` 或 `LangGraph`
- Python AI 服务不负责用户、权限、会话主数据和告警主数据

## 3.6 Agent 架构决策

- 采用 `多 Agent` 方向设计
- 通过统一路由/编排层决定请求应进入哪个 Agent
- Agent 属于 AI 能力层的一部分，不是整个系统的架构主体

## 3.7 存储与基础设施选型

- 关系数据库：`MySQL`
- 缓存：`Redis`
- 消息队列：`Kafka`
- 搜索引擎：`Elasticsearch`
- 向量数据库：`Milvus`
- 文件存储：对象存储方案，阶段 1 允许先用本地或兼容方案

## 3.8 服务间通信决策

- Go 核心业务后端与 Python AI 服务之间采用 `gRPC` 通信

选择 gRPC 的原因：

- 接口定义更稳定，便于前后期演进
- 类型约束更强，适合双后端协作
- 对流式和多服务调用场景更友好

## 4. 当前边界约束

为保证后续实现不跑偏，明确以下边界：

1. 业务主入口在 Go 后端，不在 Python AI 服务。
2. 用户、权限、会话、告警、知识元数据和审计主数据由 Go 后端负责。
3. Python AI 服务负责模型调用、RAG、工具推理和 Agent 工作流。
4. Go 与 Python 之间通过 gRPC 通信，不直接共享业务逻辑。
5. Eino 只用于 Go 侧 AI 编排层。
6. LangChain 和 LangGraph 只用于 Python AI 服务内部。
7. 前端只调用 Go 后端公开接口，不直接调用 Python AI 服务。

## 5. 阶段 1 前仍待明确的事项

以下事项不影响进入阶段 1，但建议尽快补齐：

1. 默认模型供应商与默认模型名称
2. gRPC 服务拆分方式
3. Milvus 与 Elasticsearch 的本地开发部署细节
4. 首批工具清单
5. 首批 Agent 清单

## 6. 关联文档

- [需求文档](/Users/ouyangzhenguang/project/OnCall/docs/requirements.md)
- [业务模块拆分文档](/Users/ouyangzhenguang/project/OnCall/docs/module-design.md)
- [系统架构设计文档](/Users/ouyangzhenguang/project/OnCall/docs/system-architecture.md)
- [开发计划文档](/Users/ouyangzhenguang/project/OnCall/docs/development-plan.md)

## 7. 当前结论

阶段 0 已形成稳定基线：

- 前端：`Vue 3`
- 核心后端：`Go + Gin + GORM`
- AI 服务：`Python + FastAPI + LangChain + LangGraph`
- Go 侧 AI 编排：`Eino`
- 搜索：`Elasticsearch`
- 向量检索：`Milvus`
- 服务间通信：`gRPC`

基于以上决策，可以进入阶段 1 的项目初始化工作。
