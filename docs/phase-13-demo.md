# 阶段 13 部署与演示环境说明

## 1. 本阶段目标

阶段 13 的目标是把项目从“本地开发工程”推进到“可直接演示的部署形态”。

当前这一轮已经补齐：

- 三端 Dockerfile
- 演示专用 Docker Compose
- 演示启动/停止脚本
- 演示数据导入脚本
- 演示数据样例

## 2. 新增文件

容器化：

- [backend/go-api/Dockerfile](/Users/ouyangzhenguang/project/OnCall/backend/go-api/Dockerfile)
- [backend/python-ai/Dockerfile](/Users/ouyangzhenguang/project/OnCall/backend/python-ai/Dockerfile)
- [frontend/web/Dockerfile](/Users/ouyangzhenguang/project/OnCall/frontend/web/Dockerfile)
- [frontend/web/nginx.conf](/Users/ouyangzhenguang/project/OnCall/frontend/web/nginx.conf)

演示编排：

- [docker-compose.demo.yml](/Users/ouyangzhenguang/project/OnCall/deploy/docker-compose/docker-compose.demo.yml)

演示脚本：

- [up.sh](/Users/ouyangzhenguang/project/OnCall/scripts/demo/up.sh)
- [down.sh](/Users/ouyangzhenguang/project/OnCall/scripts/demo/down.sh)
- [load-demo-data.sh](/Users/ouyangzhenguang/project/OnCall/scripts/demo/load-demo-data.sh)

演示样例：

- [user-service-runbook.txt](/Users/ouyangzhenguang/project/OnCall/examples/demo/knowledge/user-service-runbook.txt)
- [payment-api-latency.json](/Users/ouyangzhenguang/project/OnCall/examples/demo/alerts/payment-api-latency.json)

## 3. 演示环境启动方式

启动演示环境：

```bash
./scripts/demo/up.sh
```

导入演示数据：

```bash
./scripts/demo/load-demo-data.sh
```

停止演示环境：

```bash
./scripts/demo/down.sh
```

## 4. 演示入口

- Web 控制台: `http://127.0.0.1:5173`
- Go API 健康检查: `http://127.0.0.1:8080/healthz`
- Python AI 健康检查: `http://127.0.0.1:8000/healthz`
- MinIO Console: `http://127.0.0.1:9001`

## 5. 演示账号

- `admin / OnCallAdmin2026!`
- `ops / OnCallOps2026!`

## 6. 推荐演示路径

1. 登录工作台，确认系统健康和 AI 服务状态。
2. 进入知识库页，查看导入的 runbook 文档。
3. 进入告警中心，选择种子告警或导入的演示告警，生成 AI 分析。
4. 从告警详情进入排障会话，发送消息验证对话链路。
5. 进入工具中心，执行 `service_status` 和 `knowledge_search`。
6. 进入审计页，查看刚才的登录、工具调用和告警分析轨迹。

## 7. 当前边界

1. 演示环境当前以 `Docker Compose` 为主，不是 Kubernetes 部署。
2. Python AI 分析仍使用规则/Prompt 结构，不是正式模型推理。
3. RAG 仍是预切片文本检索，不是最终版混合检索。
4. 演示环境重点是“可启动、可演示、路径完整”，不是生产级部署。
