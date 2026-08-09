# Backend

“于是”的 P1 账号与同步后端使用 Go 1.26.5 单 module、单进程架构：一个 `go.mod`、一个 `main.go` 和一个部署单元。HTTP 服务、定时任务和 Outbox 处理共用同一进程生命周期，不存在独立 API、Worker 或迁移程序。P0 是单设备本地 App，不依赖本服务。当前 `go.mod` 已创建，`main.go` 尚未创建。

## 核心文件

| 文件 | 用途 |
| --- | --- |
| `go.mod` | 唯一 Go module，language 1.26.0、toolchain go1.26.5。 |
| `main.go` | 唯一进程入口，尚未创建。 |
| `openapi.yaml` | OpenAPI 3.1.2 契约，供 Swagger UI、Go 服务端和 iOS Client 共用。 |
| `schema.sql` | PostgreSQL 全部结构的唯一定义文件。 |

## 内部分层

- `internal/model`：纯 Go struct 和必要的领域不变量。
- `internal/service`：用例编排与业务规则。
- `internal/repository`：PostgreSQL 访问和事务。
- `internal/transport`：HTTP 协议、鉴权上下文和响应映射。
- `internal/platform`：日志、配置、时钟和外部服务适配。

这些只是同一 Go module 中的必要分层，不是独立业务模块或可单独部署的服务。

## POJO-like 纯 Go struct

Go 没有 Java POJO 的继承模型，本项目用纯 Go struct 实现同样的简单对象原则：

- struct 不持有 HTTP request、router、数据库连接或全局容器。
- 数据对象不负责查库、认证、网络请求或框架生命周期。
- HTTP request/response 使用手写纯 Go struct，transport 将其转换为领域 struct，不泄漏到 Service。
- 只在语义不同时创建额外映射类型，避免无意义的 DTO 重复。

## Swagger/OpenAPI

Swagger UI 只展示 `openapi.yaml`，不另外维护 Swagger 注解或第二份契约。后端创建 HTTP 入口后提供：

- `GET /swagger/`：开发和测试环境中的 Swagger UI。
- `GET /swagger/openapi.yaml`：通过 Go `embed` 直接提供当前部署的 `openapi.yaml`。

Go 后端不从 OpenAPI 生成 server、DTO 或额外 API 文件。Handler 与 request/response struct 手写，并通过 contract test 保证与 `openapi.yaml` 一致。只有 iOS 客户端从这份契约生成代码。

## 数据库

`schema.sql` 集中定义 PostgreSQL 结构，不创建 `db/migrations` 或 `db/queries` 目录。P1 首版服务端使用 Atlas Community 可管理的 enum、table、column、constraint、index 和 comment，不使用 extension、function、trigger、RLS 或 seed DML。

schema 变更需要先审查数据兼容性，并在 `docs/plan/` 记录对已有数据的转换、验证和回滚方案。应用启动时不得未经审核自动修改生产库。

空库、本地与测试使用 `psql --single-transaction` 初始化。已有环境使用专用数据库，先以 Atlas Community 1.3.0 和 `--schema public --dry-run` 计算差异并人工评审；任何 `DROP` 默认阻断，生产禁止 `--auto-approve`。详细命令和限制见 [`../docs/design/01-技术选型.md`](../docs/design/01-技术选型.md)。

## 固定技术栈

- Go 1.26.5，`net/http` + chi v5.3.1
- OpenAPI 3.1.2 + Swagger UI v5.32.12
- PostgreSQL major 18（本地/CI `postgres:18.4`）+ pgx/pgxpool v5.10.0
- kin-openapi v0.146.0 contract test；不生成 Go server/DTO
- PostgreSQL Outbox/Jobs（同进程后台循环）
- `log/slog` JSON + OpenTelemetry Go v1.45.0/otelhttp v0.70.0
- Testcontainers for Go v0.44.0 + `postgres:18.4`

创建 Go 程序入口后，在此补充本地启动、环境变量、测试和部署命令。完整选型见 [`../docs/design/01-技术选型.md`](../docs/design/01-技术选型.md)。
