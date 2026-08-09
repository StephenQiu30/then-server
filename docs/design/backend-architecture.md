# 后端架构决策

## 状态

已确认，2026-08-09。

## 决策

- 后端使用一个 Go module、一个 `main.go`、一个进程和一个部署单元。
- 不创建 `api/` 目录，不创建独立 API、Worker 或迁移入口。
- HTTP 服务与必要的后台任务共用同一进程生命周期。
- 后端接口对象采用 POJO-like 纯 Go struct，不持有 HTTP、路由器、数据库连接或框架容器。
- `backend/openapi.yaml` 是 Swagger 和 iOS Client 生成的唯一契约；Go Handler 与 request/response struct 手写并使用契约测试校验。
- `backend/schema.sql` 集中保存 PostgreSQL 全部结构定义，不拆分 migrations 或分功能 schema 文件。

## 原因

当前产品处于早期，一体化结构能减少部署、调试、数据一致性和工具链成本，同时保留必要的内部分层。

## 约束与重新评估

- 单一 `schema.sql` 适合尚未有持久生产数据的阶段。首次对真实生产数据做不兼容结构变更前，必须重新确认增量迁移、审计和回滚策略。
- 只有后台任务已明确影响 HTTP 服务稳定性或扩容曲线显著不同时，才重新评估独立进程。
- 如果手写 Go Handler 与 OpenAPI 长期发生漂移，应先增强契约测试，而不是默认引入第二套 API 模型。
