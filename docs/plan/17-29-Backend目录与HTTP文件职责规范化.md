# Backend 目录与 HTTP 文件职责规范化

## 状态与范围

- 2026-09-22 用户要求 backend 同样落实目录与文件规范；契约 approved，实施 completed（本地实现与验证）。交付版本以 Git 提交记录为准，远端验证以对应 Actions 结果为准。
- 依据：[PROJECT](../../PROJECT.md#6-规范化后端目录与服务规则)、[Design 24](../design/24-Go后端工程结构研究与规范.md)、[backend/AGENTS](../../backend/AGENTS.md)。
- 单 module、根 main.go、application/adapter/platform/bootstrap 和测试分层保持不变；不创建入口包装、空目录或并行 MVC 结构。

## 文件所有权与交付合同

| 文件 | 内容、输入输出与边界 |
| --- | --- |
| `httpapi/<resource>_contract.go` | HTTP DTO、校验/schema 方法、Huma 输入输出；保留原字段/tag/类型名 |
| `httpapi/<resource>_routes.go` | operation 注册、鉴权标记和错误状态声明；保留路径/operationId/注册次序 |
| `httpapi/<resource>_handlers.go` | Handler、消费方接口、构造、调用 application、领域映射与错误处理；保持副作用和失败语义 |
| `httpapi/router.go` | Gin/Huma 与各能力的装配、HTTP 生命周期接入 |
| `httpapi/health.go` | 健康 DTO、探测 operation、请求检查与不可用反馈 |
| `httpapi/errors.go` | 统一错误 DTO、状态/错误映射与请求关联上下文 |
| `httpapi/openapi.go` | 同一 operation 对象的规范化、序列化和校验 |
| `architecture_test.go` | 继续在 module 根执行目录、入口、跨适配器及测试包依赖约束 |

账户、公开资料、隐私、衣橱、计划、实际穿着、日记、媒体按上述 HTTP 文件职责收敛，与现有社区模块一致。所有文件仍属同一 httpapi package，不改变构造接口或共享事务。PostgreSQL 与 worker 已有清晰归属，本片不机械拆包。

## Checklist

- [x] 清点当前目录、用户未提交修改，明确文件职责与边界。
- [x] 保存临时声明/生成合同基线；整个 HTTP 包 447 个非 import 声明 token 流一致，生成 JSON 字节一致。
- [x] 加强根入口、单 module、源码归属、跨 adapter 及 HTTP 文件职责约束；额外根入口、跨适配器导入、嵌套 module、routes 中 DTO 四项违规样本均被拦截并已删除。
- [x] gofmt、mod verify、vet、unit、完整 race、实际进程 integration 验证通过。
- [x] 同步 PROJECT/AGENTS/README/Design/索引及 [Acceptance 17](../acceptance/17-云端生成与任务管理验收.md#17-29-backend-目录与-http-文件职责规范化验收)。

## 回退与验收边界

本片没有数据库、API 或 Kafka 合同变化；只回退文件拆分及检查增量即可，不触及数据和前端改动。合同快照仅保存到临时目录，不进入仓库。真实进程验证使用已有隔离合成数据套件；远端 CI、镜像与生产发布是否运行需单独记录。
