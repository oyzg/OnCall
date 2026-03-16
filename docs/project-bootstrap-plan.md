# AI OnCall 平台项目初始化方案

## 1. 文档目的

本文档用于指导 AI OnCall 平台阶段 1 的项目初始化工作，明确 monorepo 目录结构、各子项目边界、基础配置组织方式、proto 目录规划、脚本规划和初始化顺序。

本文档的目标不是讨论完整业务设计，而是回答一个问题：仓库应该如何从空状态进入可开发状态。

## 2. 初始化目标

阶段 1 初始化需要达到以下目标：

1. 建立统一 monorepo 结构。
2. 建立前端 Vue 项目骨架。
3. 建立 Go 核心业务后端骨架。
4. 建立 Python AI 服务骨架。
5. 建立 gRPC proto 目录与生成物约定。
6. 建立本地开发依赖与启动方式。
7. 建立统一配置、脚本和文档入口。

## 3. 初始化原则

### 3.1 单仓管理

所有代码放在同一个仓库中，便于统一管理前端、Go 服务、Python AI 服务、proto 文件和部署配置。

### 3.2 目录清晰

目录结构按职责拆分，不混放不同语言和不同服务的代码。

### 3.3 先骨架后实现

初始化阶段先完成目录、配置、启动入口和基本联通，不追求业务功能完整。

### 3.4 可扩展

目录结构要支持后续继续增加 Worker、脚本、部署配置和测试目录。

## 4. 推荐 monorepo 结构

建议仓库采用如下结构：

```text
OnCall/
  README.md
  .gitignore
  .editorconfig
  .env.example
  docs/
  deploy/
    docker-compose/
      docker-compose.dev.yml
    k8s/
  proto/
    ai/
    common/
    buf.yaml
    buf.gen.yaml
  scripts/
    dev/
    build/
    proto/
    db/
  frontend/
    web/
  backend/
    go-api/
    python-ai/
  internal-tools/
  testdata/
```

## 5. 顶层目录职责

## 5.1 `docs/`

存放需求、架构、计划、数据库设计、API 设计等项目文档。

## 5.2 `deploy/`

存放部署相关配置。

建议继续拆分为：

- `deploy/docker-compose/`
- `deploy/k8s/`

## 5.3 `proto/`

存放 Go 与 Python AI 服务之间的 gRPC 协议定义。

建议分类：

- `proto/ai/`
  AI 服务相关协议
- `proto/common/`
  公共消息结构

## 5.4 `scripts/`

存放项目脚本。

建议分类：

- `scripts/dev/`
  本地开发启动脚本
- `scripts/build/`
  构建脚本
- `scripts/proto/`
  proto 生成脚本
- `scripts/db/`
  数据库初始化或迁移脚本

## 5.5 `frontend/`

存放前端项目。

第一阶段只建立一个 Web 前端：

- `frontend/web/`

## 5.6 `backend/`

存放所有后端项目。

建议拆分为：

- `backend/go-api/`
  Go 核心业务后端
- `backend/python-ai/`
  Python AI 服务

## 5.7 `internal-tools/`

预留给项目内部辅助工具或后续代码生成工具，不要求阶段 1 必须使用。

## 5.8 `testdata/`

存放本地开发或演示用的测试数据、示例文档和示例请求数据。

## 6. 前端项目初始化方案

前端项目路径建议为：

```text
frontend/web/
```

建议初始结构：

```text
frontend/web/
  public/
  src/
    assets/
    components/
    composables/
    features/
    layouts/
    pages/
    router/
    services/
    stores/
    styles/
    types/
    utils/
  index.html
  package.json
  tsconfig.json
  vite.config.ts
  .env.example
```

### 前端初始化阶段需要具备的内容

- Vue 3 + TypeScript + Vite 基础工程
- Vue Router
- Pinia
- Element Plus
- 页面基础布局
- 路由骨架
- 登录态存储骨架
- API 请求封装骨架

## 7. Go 核心业务后端初始化方案

Go 项目路径建议为：

```text
backend/go-api/
```

建议初始结构：

```text
backend/go-api/
  cmd/
    server/
      main.go
  configs/
  internal/
    auth/
    user/
    session/
    alert/
    knowledge/
    tool/
    audit/
    config/
    ai/
      eino/
      gateway/
      orchestrator/
      retrieval/
      analyzer/
    platform/
      middleware/
      db/
      cache/
      queue/
      storage/
      search/
      vector/
      grpcclient/
      observability/
  pkg/
    logger/
    response/
    errors/
    config/
    utils/
  migrations/
  go.mod
  Makefile
  .env.example
```

### Go 初始化阶段需要具备的内容

- Gin 启动入口
- 基础路由
- 健康检查接口
- 配置加载
- 日志组件
- 数据库连接骨架
- Redis 连接骨架
- gRPC client 目录
- AI 编排目录和 Eino 目录骨架

## 8. Python AI 服务初始化方案

Python AI 服务路径建议为：

```text
backend/python-ai/
```

建议初始结构：

```text
backend/python-ai/
  app/
    api/
    chains/
    graphs/
    tools/
    rag/
    services/
    schemas/
    clients/
    core/
  tests/
  pyproject.toml
  uv.lock
  .env.example
  README.md
```

### Python AI 初始化阶段需要具备的内容

- FastAPI 启动入口
- 健康检查接口
- 基础路由骨架
- LangChain 目录骨架
- LangGraph 目录骨架
- gRPC server 或网关接入位置
- 配置加载和日志骨架

## 9. Proto 初始化方案

proto 路径建议为：

```text
proto/
```

建议初始结构：

```text
proto/
  ai/
    chat.proto
    alert.proto
    rag.proto
  common/
    pagination.proto
    metadata.proto
  buf.yaml
  buf.gen.yaml
```

### Proto 初始化阶段需要明确的内容

- 统一包名规范
- 统一 service 命名规范
- Go 生成路径
- Python 生成路径
- proto 生成脚本入口

### 初始建议

阶段 1 先定义最基础的 AI 服务协议，例如：

- `ChatService`
- `AlertAnalysisService`
- `RAGService`

## 10. 部署与本地依赖初始化方案

阶段 1 本地开发依赖建议通过 Docker Compose 管理，配置文件放在：

```text
deploy/docker-compose/docker-compose.dev.yml
```

建议第一阶段纳入以下依赖：

- MySQL
- Redis
- Kafka
- Elasticsearch
- Milvus

### 第一阶段可选依赖

- MinIO
  用于对象存储模拟

## 11. 环境变量组织方案

建议采用以下约定：

- 仓库根目录放置统一 `.env.example`
- 每个子项目各自保留一份 `.env.example`

例如：

- `/Users/ouyangzhenguang/project/OnCall/.env.example`
- `/Users/ouyangzhenguang/project/OnCall/frontend/web/.env.example`
- `/Users/ouyangzhenguang/project/OnCall/backend/go-api/.env.example`
- `/Users/ouyangzhenguang/project/OnCall/backend/python-ai/.env.example`

### 根目录环境变量

用于描述跨项目公共配置，例如：

- 端口约定
- 数据库地址
- Redis 地址
- Kafka 地址
- Elasticsearch 地址
- Milvus 地址

### 子项目环境变量

用于描述各项目私有配置，例如：

- 前端 API Base URL
- Go 服务 JWT 密钥
- Python AI 服务模型密钥

## 12. 脚本组织方案

建议通过 `scripts/` 目录统一管理常用脚本。

### 阶段 1 建议具备的脚本

- 启动本地依赖
- 启动前端
- 启动 Go 服务
- 启动 Python AI 服务
- 生成 proto 代码

建议脚本入口如下：

```text
scripts/dev/up.sh
scripts/dev/web.sh
scripts/dev/go-api.sh
scripts/dev/python-ai.sh
scripts/proto/gen.sh
```

## 13. README 初始化方案

阶段 1 的 README 不需要写完整业务说明，但至少应覆盖：

1. 项目简介
2. 仓库结构
3. 技术栈
4. 本地依赖
5. 本地启动方式
6. 子项目说明

## 14. 阶段 1 初始化顺序

建议按以下顺序执行：

1. 建立顶层目录结构
2. 初始化前端项目
3. 初始化 Go 后端项目
4. 初始化 Python AI 服务项目
5. 建立 proto 目录与基础 proto 文件
6. 建立 Docker Compose 开发依赖
7. 建立 `.env.example`
8. 建立基础启动脚本
9. 更新 README

## 15. 阶段 1 完成标准

当以下条件满足时，可认为阶段 1 完成：

1. 仓库目录结构已固定
2. 前端项目可启动
3. Go 后端项目可启动
4. Python AI 服务可启动
5. Docker Compose 可启动基础依赖
6. proto 目录和生成规则已建立
7. README 能指导新用户完成本地启动

## 16. 阶段 1 之后的衔接

完成初始化后，建议立即进入以下工作：

1. Go 后端基础能力建设
2. Python AI 服务基础能力建设
3. 前端基础能力建设

也就是说，项目初始化方案完成后，不需要再停下来补大量文档，而是可以直接进入编码。

## 17. 当前结论

阶段 1 的核心不是“实现业务功能”，而是把仓库变成一个真正可运行、可扩展、可协作的工程骨架。

只要按本文档完成：

- monorepo 结构
- Vue 前端骨架
- Go 后端骨架
- Python AI 服务骨架
- proto 骨架
- Docker Compose 基础依赖

项目就从文档设计阶段正式进入开发阶段。
