# AI OnCall RAG Upgrade Design

## 1. 背景

当前 AI OnCall 已经具备知识上传、文档切片、检索测试和引用展示的基本链路，主流程可演示、可联调，但 RAG 能力仍处于“证明产品链路成立”的阶段，而不是“真实可用的检索子系统”阶段。

现状问题主要有四类：

1. 检索仍带有明显的本地规则匹配特征，真实 semantic recall 能力不足。
2. 检索结果的可解释性和可诊断性还不够完整，难以快速判断效果问题出在哪一层。
3. 索引构建、一致性、失败恢复和降级能力较弱，更接近 demo 方案。
4. 简历和面试层面虽然能说明“做了知识检索”，但还不足以支撑“做过工程化 RAG 系统”的表达。

因此，本次升级的目标不是简单增加几个模型或中间件名词，而是把现有知识检索能力升级成一个“分层清晰、主链路真实、问题可查、面试可讲”的 RAG 子系统。

## 2. 目标

本次 RAG 升级目标分为三个层次：

### 2.1 业务目标

- 提升知识问答、告警排障、工具辅助说明等场景下的知识召回能力。
- 让值班类问题不再只依赖精确关键词匹配，而能覆盖同义表达、模糊描述和上下文化提问。
- 为后续告警分析、对话增强和工具推荐提供更可信的知识支撑。

### 2.2 工程目标

- 将检索从本地 fallback 逻辑升级为真实的 hybrid retrieval。
- 将检索过程做成可解释、可诊断、可降级的工程链路。
- 将索引流程做成具备状态管理和失败恢复能力的子系统，而不是一次性同步处理逻辑。

### 2.3 展示目标

- 让项目在简历上可以明确写出 `Elasticsearch + Milvus + OpenAI-compatible embeddings + hybrid retrieval`。
- 让面试时能够讲清楚为什么这样分阶段推进、每一步遇到了什么问题、如何解决。
- 让项目整体从“AI demo”更进一步，成为“有完整 AI 检索工程思路的全栈项目”。

## 3. 非目标

本轮升级刻意不把范围扩到以下事项：

- 不在第一阶段引入复杂 rerank 模型或 cross-encoder。
- 不把重点放在多租户、多团队权限隔离上。
- 不优先引入复杂异步队列编排或 Kafka 消费链路。
- 不直接把 RAG 升级绑定到完整 Agent 工作流。
- 不追求生产级 SLA，只追求“结构正确、链路真实、可演示、可解释”。

这些能力可以作为后续增强项，但不应该阻塞主链路升级。

## 4. 现有架构与改造基线

当前系统中与 RAG 相关的主要路径如下：

- Go 业务层通过统一 retrieval service 发起检索请求。
- Python AI 服务负责更接近 AI 能力层的检索实现。
- 文档上传和知识管理由 Go 业务侧负责。
- 前端知识库页面负责发起检索测试和展示引用结果。

关键代码位置：

- [backend/go-api/internal/ai/retrieval/service.go](/Users/ouyangzhenguang/project/OnCall/backend/go-api/internal/ai/retrieval/service.go)
- [backend/go-api/internal/ai/gateway/client.go](/Users/ouyangzhenguang/project/OnCall/backend/go-api/internal/ai/gateway/client.go)
- [backend/go-api/internal/knowledge/application/service.go](/Users/ouyangzhenguang/project/OnCall/backend/go-api/internal/knowledge/application/service.go)
- [backend/python-ai/app/services/hybrid_rag.py](/Users/ouyangzhenguang/project/OnCall/backend/python-ai/app/services/hybrid_rag.py)
- [backend/python-ai/app/core/config.py](/Users/ouyangzhenguang/project/OnCall/backend/python-ai/app/core/config.py)
- [frontend/web/src/pages/KnowledgePage.vue](/Users/ouyangzhenguang/project/OnCall/frontend/web/src/pages/KnowledgePage.vue)

整体设计原则保持不变：

- Go 继续承担业务边界和统一对外 API。
- Python 继续承担 AI 检索逻辑和 AI 能力适配。
- Go 不直接依赖底层向量库和搜索引擎细节。
- 前端只感知“检索能力”和“诊断结果”，不感知底层检索实现。

## 5. 分阶段升级方案

### 5.1 阶段一：真实混合检索落地

#### 目标

将当前 RAG 从本地规则检索升级为真实的 lexical + semantic 双路召回。

#### 交付物

- 接入 OpenAI-compatible embedding 服务。
- 文档切片后同步写入 Elasticsearch。
- 文档向量同步写入 Milvus。
- 查询时分别执行 lexical recall 和 semantic recall。
- Python AI 服务完成 first-stage fusion。
- Go 侧保留统一 retrieval service 和 remote fallback 逻辑。

#### 关键问题

1. `embedding model` 与 `Milvus collection schema` 维度不一致。
2. Elasticsearch 和 Milvus 双写成功率不同，容易出现索引不一致。
3. 中文、英文、错误码、服务名混合文本下，纯关键词召回不稳定。
4. 小规模数据集下，向量召回优势不明显，演示效果可能不足。

#### 解决方向

1. 在配置中显式固定 embedding model 和 dimension，并在启动或索引阶段校验。
2. 在索引响应中返回 `embedding_backend`、`lexical_backend`、`vector_backend`，允许前端和调试链路感知真实状态。
3. 先采用“简单分词 + query rewrite + hybrid fusion”策略，不在首阶段过度优化 analyzer。
4. 提前准备同义表达、模糊表达类测试文档和查询样例，突出 semantic recall 的价值。

#### 面试表达

- “我先把知识检索从本地规则匹配升级为 hybrid retrieval，因为运维问答里的表达非常不标准，只靠关键词很难稳定命中。”
- “我把 lexical 和 semantic 两路召回分开设计，这样后续替换搜索和向量底座时，不会影响上层业务接口。”

### 5.2 阶段二：检索诊断与可解释性增强

#### 目标

让 RAG 不再是黑盒结果，而是能解释“为什么召回这些片段”的链路。

#### 交付物

- 输出 `rewritten_query`、`query_terms`、`expanded_terms`。
- 输出 `lexical_score`、`semantic_score`、`boost_score`。
- 输出 `match_reasons` 说明命中原因。
- 在前端展示引用来源、召回方式和诊断信息。
- 在 health / dashboard 页面展示依赖可用性。

#### 关键问题

1. 命中了结果，但用户和开发者不知道排序原因。
2. 检索返回空结果时，无法快速区分“没有命中”还是“依赖不可用”。
3. 项目完成后，如果面试官追问“你怎么评估升级效果”，缺少证据。

#### 解决方向

1. 将分数拆解成多字段返回，而不是只返回一个总分。
2. 明确区分“零命中”、“依赖降级”、“部分 backend 可用”三类状态。
3. 准备最小评测集，记录升级前后 top-k 命中效果。
4. 前端默认展示简版信息，展开时展示详细诊断，避免界面噪音过大。

#### 面试表达

- “我没有把 RAG 当成黑盒接口，我把中间诊断结果透出来，让检索效果问题可以被定位。”
- “这样做之后，我能判断问题出在 query rewrite、lexical recall 还是 semantic recall，而不是只能盲调参数。”

### 5.3 阶段三：索引链路工程化

#### 目标

将知识索引从“同步处理逻辑”升级为“具备状态管理和失败恢复能力的工程链路”。

#### 交付物

- 文档状态扩展为 `uploaded / indexing / ready / failed`。
- 支持索引失败重试。
- 删除文档时同步删除 ES / Milvus 索引。
- 支持索引不一致修复入口。
- 增加 embedding 调用、索引写入、检索调用的超时、重试与降级策略。

#### 关键问题

1. 上传文档时同步做 embedding 和索引，接口容易变慢甚至超时。
2. ES 成功、Milvus 失败时，系统表面看似 ready，实际检索不完整。
3. 外部 embedding 服务波动会放大成整条知识链路不稳定。

#### 解决方向

1. 先把同步流程改造成带状态机和可重试的“伪异步”链路，必要时再接入真正异步任务。
2. 在文档状态中记录索引失败原因，并提供重试入口。
3. 制定清晰降级策略：
   - Milvus 不可用时退回 lexical recall。
   - embedding 不可用时暂停新索引，但保留旧索引查询。
   - Python AI 不可用时 Go 侧返回明确错误与降级信息。

#### 面试表达

- “真正难的不是把向量搜起来，而是多依赖系统之间的一致性和失败恢复。”
- “我后期重点补的是状态机、降级和重试，这些决定系统是不是一个可维护的 RAG 子系统。”

### 5.4 阶段四：高级增强项

#### 目标

在主链路稳定后，再增加更高级的检索质量能力和系统完整度。

#### 可选能力

- 引入 rerank model 或 cross-encoder reranking。
- 基于服务、环境、文档类型做 metadata filtering。
- 基于会话上下文做 query rewrite。
- 支持 chunk stitching、parent-child retrieval。
- 增加固定评测脚本和 regression report。

#### 关键问题

1. rerank 能提升相关性，但会显著增加延迟和复杂度。
2. query rewrite 容易把用户问题改偏。
3. metadata filter 过严会导致 recall 下降。

#### 解决方向

1. 所有高级能力都设计为可配置开关。
2. 永远保留原 query 和 rewritten query，避免无法回溯。
3. 评测同时看 precision 与 recall，不只看单个案例。

#### 面试表达

- “我没有一开始就堆高级模型，而是先把主链路做稳，再做质量优化。这是有意识的复杂度控制。”

## 6. 路线优先级

推荐执行顺序如下：

1. 阶段一：真实混合检索落地
2. 阶段二：检索诊断与可解释性增强
3. 阶段三：索引链路工程化
4. 阶段四：高级增强项

这样安排的原因是：

- 第一阶段先把“是否是真 RAG”这个核心问题解决。
- 第二阶段把结果变得可解释、可展示、可证明。
- 第三阶段再把链路做稳，补工程性。
- 第四阶段作为拔高项，不阻塞主线。

## 7. 风险与取舍

### 7.1 为什么不一步到位做生产级 RAG

因为这是简历项目，最重要的是用有限时间构建“真实、完整、能讲清楚的技术故事”，而不是在所有高级能力上浅尝辄止。

如果一开始就把范围拉到：

- 异步索引
- rerank
- 多租户权限
- 全量评测平台
- 复杂缓存体系

那么主链路很容易变得过重，导致项目最终只能讲很多名词，而讲不清楚核心设计和实际问题。

### 7.2 为什么保留 Go + Python 分工

因为这是项目当前最清晰、也最适合展示工程边界的结构：

- Go 负责业务接口、权限边界和知识元数据管理。
- Python 负责 embedding、hybrid recall、vector/search integration 和 AI 侧逻辑。

这种边界有利于面试时说明“业务系统”和“AI 子系统”的职责分离。

## 8. 简历与面试表达建议

### 8.1 简历版本

可将本轮升级总结为：

> 将知识检索从本地规则匹配升级为基于 Elasticsearch、Milvus 和 OpenAI-compatible embeddings 的混合检索系统，设计召回融合、诊断字段、失败降级与索引状态管理机制，增强了知识问答与告警排障场景下的可解释性和工程完整度。

### 8.2 面试版本

建议按以下顺序讲：

1. 先讲问题
   - 关键词检索不足以覆盖值班场景中的模糊表达和同义表达。
2. 再讲方案
   - 增加 lexical recall + semantic recall，并通过统一接口对外暴露。
3. 再讲难点
   - 多依赖系统的一致性、失败恢复、效果诊断。
4. 最后讲取舍
   - 先把主链路做真，再做可解释和工程化，最后才做高级 rerank。

## 9. 下一步

在该设计确认后，下一步应进入实施计划阶段，优先为“阶段一：真实混合检索落地”编写详细实现计划，包括：

- 具体文件改动范围
- 每个任务的测试策略
- 依赖配置方式
- 联调验证命令
- 每个小步骤的预期风险与完成标准
