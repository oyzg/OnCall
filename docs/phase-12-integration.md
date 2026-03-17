# 阶段 12 联调与测试记录

## 1. 本阶段目标

阶段 12 关注“主链路是否真的连起来”。本轮不再单独补业务功能，而是验证登录、会话、知识检索、告警分析、工具调用和审计查询是否能串成完整流程。

## 2. 自动化联调覆盖

已新增后端联调测试：

- [server_integration_test.go](/Users/ouyangzhenguang/project/OnCall/backend/go-api/internal/platform/httpserver/server_integration_test.go)

当前覆盖的链路如下：

1. 登录链路
- 错误密码登录应返回 `401`
- 正确账号登录应返回 JWT

2. 对话链路
- 成功创建会话
- 成功调用 SSE 消息接口
- 返回 `event: done`

3. 知识入库与检索链路
- 上传文本知识文档
- 等待文档状态变为 `ready`
- 调用 RAG 检索接口并返回引用片段

4. 告警分析链路
- 读取种子告警
- 调用告警 AI 分析接口
- 通过 mock Python AI 服务返回结构化分析结果

5. 工具调用链路
- 调用 `knowledge_search`
- 返回检索答案

6. 审计与运营链路
- 查询审计统计接口
- 查询审计日志接口
- 验证审计中包含 `auth` 和 `tool` 等核心分类

## 3. 本轮验证命令

后端：

```bash
cd /Users/ouyangzhenguang/project/OnCall/backend/go-api
env GOPROXY=https://proxy.golang.org,direct \
  GOCACHE=/Users/ouyangzhenguang/project/OnCall/tmp/go-build \
  GOMODCACHE=/Users/ouyangzhenguang/project/OnCall/tmp/go-mod-cache \
  go test ./...
```

前端：

```bash
cd /Users/ouyangzhenguang/project/OnCall/frontend/web
npm run build
```

## 4. 当前已知问题

1. Python AI 分析仍是 `HTTP + mock/规则分析` 形态
- 当前没有切到最终的 `gRPC + 真实模型调用`
- 但接口边界和联调主链已经成立

2. RAG 仍不是最终版
- 当前是预切片文本检索
- 尚未接入 `Embedding + Milvus + Elasticsearch hybrid recall + rerank`

3. 前端产物体积偏大
- `vite build` 仍会提示 chunk size warning
- 不影响运行，但后续需要按路由或模块拆包

4. 审计日志当前为本地文件持久化
- 已满足开发和演示联调
- 后续可以切到正式数据库表

## 5. 阶段结论

当前阶段 12 的第一轮目标已达到：

- 登录链路可验证
- 对话链路可验证
- 知识入库与检索链路可验证
- 告警分析链路可验证
- 工具调用链路可验证
- 审计查询链路可验证

这意味着系统已经具备“可联调、可回归、可演示”的基础测试底座，可以继续进入阶段 13 或继续深化现有链路。
