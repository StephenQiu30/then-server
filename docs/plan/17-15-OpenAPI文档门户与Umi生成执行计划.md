# 运行时OpenAPI与GORM Schema收敛执行计划

## 状态

- 契约状态：`approved`，依据用户 2026-09-14 最新要求。
- 执行状态：`completed`（2026-09-16 无路径版本前缀跟进已完成）。
- 前提：当前开发阶段不保留历史数据库数据。

## 目标

在不增加新服务和生成脚本的前提下，把后端收敛为：

```text
Gin/Huma 声明 -> 运行时 OpenAPI -> Swagger / Umi
GORM record -> AutoMigrate -> PostgreSQL schema
```

移除重复事实源 `generate_openapi.go`、仓库内 `openapi.yaml`、`atlas.hcl`、SQL migration 与 checksum。

## 追踪关系

- Design：[技术选型](../design/01-技术选型.md)、[后端架构](../design/02-后端架构.md)。
- PRD：[接口协作需求](../prd/17-云端生成与任务管理需求.md)。
- Acceptance：[后端收敛验收](../acceptance/17-云端生成与任务管理验收.md#2026-09-14运行时openapi与gorm-schema收敛验收)。

## SMART范围

- Specific：接口只由 Huma operation/Go tag 声明，schema 只由 Repository GORM record 声明。
- Measurable：运行时全部 operation 的契约可校验；空 PostgreSQL 能由实际二进制自动建表并启动；账号、衣橱与媒体约束、事务和级联关系通过。
- Achievable：沿用现有 Gin、Huma、GORM 和测试体系，不引入新框架。
- Relevant：满足 Swagger/Umi 生成，同时删除 Atlas 和物化 OpenAPI 的维护成本。
- Time-bound：本切片完成代码、CI、规范与真实 PostgreSQL 验证；不创建 `frontend/` 页面。

## Spec

1. `/openapi.json` 和 `/openapi.yaml` 必须由当前进程注册的 Huma API 运行时生成，Swagger 读取 `/openapi.json`。
2. 仓库不得包含 OpenAPI 物化工具或生成 YAML/JSON；CI 不执行 `go generate` 漂移检查。
3. operationId、请求/响应、错误状态和 Cookie security 只在 transport 声明。
4. GORM record 必须明确账号表的列类型、非空、唯一、检查约束、索引和用户删除级联。
5. `postgres.Migrate` 集中执行 `AutoMigrate`，bootstrap 在 HTTP 监听前调用；失败时进程不报告启动成功。
6. 当前不保留 Atlas、SQL migration、checksum、旧 schema 探测、回填或双写。
7. Redis 只因已批准的认证限流进入当前 API 运行时；RabbitMQ、MinIO 不因 schema 调整或未来规划提前进入生产 binary。
8. App 当前没有实际网络调用时，移除依赖物化契约的空 transport target、生成插件和未使用依赖；未来接入另立执行计划。
9. 公开 API 直接使用语义根路径（例如 `/auth`、`/users`、`/health`），不在 Axios、Next.js rewrites、Huma operation 或 Cookie Path 中配置路径版本；当前尚无生产消费者，不保留旧前缀兼容路由。

## Checklist

- [x] GitHub/Firecrawl 检查 CloudWeGo、Gin/GORM 项目、GORM migration 与 Swagger 注解方案。
- [x] 删除 `generate_openapi.go` 与仓库内 `openapi.yaml`。
- [x] 删除 `atlas.hcl`、账号 SQL migration 与 `atlas.sum`。
- [x] 在 GORM record 上声明 PostgreSQL schema 约束。
- [x] 新增集中 `postgres.Migrate` 并接入 bootstrap 启动顺序。
- [x] 契约测试直接调用运行时 OpenAPI 生成，不读取磁盘产物。
- [x] Swagger UI 覆盖当前 GET/POST/PUT/DELETE/PATCH；禁用外部 validator、查询覆盖与授权持久化。实际浏览器展开 PUT 声明接口后可编辑请求并显示 Execute。
- [x] 进程测试验证运行时 JSON/YAML，不比较仓库文件。
- [x] CI 删除生成产物漂移步骤。
- [x] 根规范、后端规范、Design、PRD、README 与 Acceptance 同步。
- [x] Design 10/14 删除物化规格、固定 SQL migration job 和已移除 Swift Client 的旧要求；当前开发统一为 GORM AutoMigrate、运行时 OpenAPI 与实际消费者按需生成。
- [x] 移除 App 中未使用的 ThenTransport、OpenAPI 符号链接、生成配置和 Swift OpenAPI 依赖；CI 不再检出服务端仓库参与 App 构建。
- [x] `go test ./... -count=1`、`go vet ./...` 通过。
- [x] `go test -race ./... -count=1` 通过。
- [x] 本机 services 测试通过，覆盖 GORM schema、唯一约束、事务与级联删除。
- [x] Testcontainers integration 通过，覆盖空库迁移、无建表权限失败、实际 binary、健康、断连恢复和 SIGTERM。
- [x] Xcode 工程仅保留实际 target/GRDB 依赖；Debug Simulator build 与非 UI 测试通过。
- [x] 远程 GitHub Actions：服务端 `01937cc` 的 `34860127582` 与 App `8afd8f6` 的 `34860139428` 均成功；后续 main 服务端 `6aba3af` 的 `34952510997`、App `26cc8f4` 的对应 CI 继续成功。
- [x] 顶层 `frontend/` 已按用户后续要求创建，Umi 从运行时 OpenAPI 生成请求文件。
- [x] 2026-09-16 跟进：移除前后端路径版本前缀，Cookie Path 收敛为 `/`，升级 API 文档版本并重新生成 Umi 客户端。
- [x] 2026-09-16 跟进：Axios 官方源码对照后的统一 `request.ts`、前端请求测试、Go 契约/单元/race、真实运行时 OpenAPI 与前端 lint/typecheck/build 全部通过。
- [x] 2026-09-16 跟进：Umi 生成目录迁移为 `frontend/src/api/`，从当时真实命令入口的 `/openapi.json` 生成 API 0.13.0 / 41 个 operation，并继续只通过 `src/lib/api/request.ts` 发送请求；当前入口已统一为 `cmd/then-server/main.go`。

## 非目标

- 不切换到 Swaggo。它依赖 CLI 并生成 `docs.go/json/yaml`，与删除物化产物的要求冲突。
- 不复制 CloudWeGo 的 Hertz/Kitex、微服务、IDL 或生成体系。
- 不为未来数据保留预建生产迁移平台。
- 除已批准的路径去版本化和 Cookie Path 收敛外，不修改业务 API 语义或当前中间件启用范围。

## 完成判定

代码、规范与运行证据都只保留两份事实源：HTTP adapter 的运行时接口声明和 PostgreSQL adapter 的 GORM schema 声明。当前本机运行时 OpenAPI 为 API 0.14.0 / 48 个 operation，路径无版本前缀；19-03 变更后已从实际 `/openapi.json` 重新生成 `src/api/` 下的 diary client，并通过 TypeScript、请求层测试和生产构建。本切片保持 `completed`；远程 CI 和生产数据迁移仍按各自切片验收。
