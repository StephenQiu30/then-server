# 运行时OpenAPI与GORM Schema收敛执行计划

## 状态

- 契约状态：`approved`，依据用户 2026-09-14 最新要求。
- 执行状态：`completed`。
- 前提：当前开发阶段不保留历史数据库数据。

## 目标

在不增加新服务和生成脚本的前提下，把后端收敛为：

```text
Gin/Huma 声明 -> 运行时 OpenAPI -> Swagger / Umi
GORM record -> AutoMigrate -> PostgreSQL schema
```

移除重复事实源 `generate_openapi.go`、仓库内 `openapi.yaml`、`atlas.hcl`、SQL migration 与 checksum。

## 追踪关系

- Design：[技术选型](../design/01-技术选型.md#2026-09-14后端最小技术栈定案)、[后端架构](../design/02-后端架构.md)。
- PRD：[接口协作需求](../prd/17-云端生成与任务管理需求.md#2026-09-12接口协作需求)。
- Acceptance：[后端收敛验收](../acceptance/17-云端生成与任务管理验收.md#2026-09-14运行时openapi与gorm-schema收敛验收)。

## SMART范围

- Specific：接口只由 Huma operation/Go tag 声明，schema 只由 Repository GORM record 声明。
- Measurable：8 个 operation 的运行时契约可校验；空 PostgreSQL 能由实际二进制自动建表并启动；账号唯一约束、事务和级联删除通过。
- Achievable：沿用现有 Gin、Huma、GORM 和测试体系，不引入新框架。
- Relevant：满足 Swagger/Umi 生成，同时删除 Atlas 和物化 OpenAPI 的维护成本。
- Time-bound：本切片完成代码、CI、规范与真实 PostgreSQL 验证；不创建 `frontend/` 页面。

## Spec

1. `/openapi.json` 和 `/openapi.yaml` 必须由当前进程注册的 Huma API 运行时生成，Swagger 读取 `/openapi.json`。
2. 仓库不得包含 OpenAPI 物化工具或生成 YAML/JSON；CI 不执行 `go generate` 漂移检查。
3. operationId、请求/响应、错误状态和 Cookie security 只在 transport 声明。
4. GORM record 必须明确账号表的列类型、非空、唯一、检查约束、索引和用户删除级联。
5. `repository.Migrate` 集中执行 `AutoMigrate`，Main 在 HTTP 监听前调用；失败时进程不报告启动成功。
6. 当前不保留 Atlas、SQL migration、checksum、旧 schema 探测、回填或双写。
7. Redis、RabbitMQ、MinIO 不因 schema 调整进入当前 API 运行时。
8. App 当前没有实际网络调用时，移除依赖物化契约的空 transport target、生成插件和未使用依赖；未来接入另立执行计划。

## Checklist

- [x] GitHub/Firecrawl 检查 CloudWeGo、Gin/GORM 项目、GORM migration 与 Swagger 注解方案。
- [x] 删除 `generate_openapi.go` 与仓库内 `openapi.yaml`。
- [x] 删除 `atlas.hcl`、账号 SQL migration 与 `atlas.sum`。
- [x] 在 GORM record 上声明 PostgreSQL schema 约束。
- [x] 新增集中 `repository.Migrate` 并接入 Main 启动顺序。
- [x] 契约测试直接调用运行时 OpenAPI 生成，不读取磁盘产物。
- [x] 进程测试验证运行时 JSON/YAML，不比较仓库文件。
- [x] CI 删除生成产物漂移步骤。
- [x] 根规范、后端规范、Design、PRD、README 与 Acceptance 同步。
- [x] 移除 App 中未使用的 ThenTransport、OpenAPI 符号链接、生成配置和 Swift OpenAPI 依赖；CI 不再检出服务端仓库参与 App 构建。
- [x] `go test ./... -count=1`、`go vet ./...` 通过。
- [x] `go test -race ./... -count=1` 通过。
- [x] 本机 services 测试通过，覆盖 GORM schema、唯一约束、事务与级联删除。
- [x] Testcontainers integration 通过，覆盖空库迁移、无建表权限失败、实际 binary、健康、断连恢复和 SIGTERM。
- [x] Xcode 工程仅保留实际 target/GRDB 依赖；Debug Simulator build 与非 UI 测试通过。
- [ ] 远程 GitHub Actions：本轮未提交，因此尚未触发。
- [ ] 前端项目和 Umi 生成文件：按用户要求暂不创建 `frontend/`。

## 非目标

- 不切换到 Swaggo。它依赖 CLI 并生成 `docs.go/json/yaml`，与删除物化产物的要求冲突。
- 不复制 CloudWeGo 的 Hertz/Kitex、微服务、IDL 或生成体系。
- 不为未来数据保留预建生产迁移平台。
- 不修改业务 API 语义、账号 Cookie 或当前中间件启用范围。

## 完成判定

代码、规范与运行证据都只保留两份事实源：transport 的运行时接口声明和 repository 的 GORM schema 声明。普通测试、真实本机服务和隔离 PostgreSQL 进程测试通过后，本切片为 `completed`。远程 CI、生产数据迁移和前端页面仍按各自切片验收。
