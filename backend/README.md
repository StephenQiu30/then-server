# Backend

“于是”后端使用 Go 单 module、单进程架构：一个 `go.mod`、一个 `main.go` 和一个部署单元。HTTP 服务、定时任务和 Outbox 处理共用同一进程生命周期，不存在独立 API、Worker 或迁移程序。

## 核心文件

| 文件 | 用途 |
| --- | --- |
| `main.go` | 唯一进程入口，尚未创建。 |
| `openapi.yaml` | Swagger UI、Go 服务端和 iOS Client 共用的唯一接口契约。 |
| `schema.sql` | PostgreSQL 全部结构的唯一定义文件。 |

## 内部分层

- `internal/model`：纯 Go struct 和必要的领域不变量。
- `internal/service`：用例编排与业务规则。
- `internal/repository`：PostgreSQL 访问和事务。
- `internal/transport`：HTTP 协议、鉴权上下文和响应映射。
- `internal/platform`：日志、配置、时钟、对象存储和外部服务适配。

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

`schema.sql` 集中定义 PostgreSQL extension、type、table、constraint、index、trigger 和必要的 seed data。不创建 `db/migrations` 或 `db/queries` 目录。

schema 变更需要先审查数据兼容性，并在 `docs/plan/` 记录对已有数据的转换、验证和回滚方案。应用启动时不得未经审核自动修改生产库。

## 预定技术栈

- `net/http` + `chi`
- OpenAPI + Swagger UI
- PostgreSQL + `pgx`
- PostgreSQL Outbox/Jobs（同进程后台循环）
- 私有对象存储
- OpenTelemetry

创建 Go module 后，在此补充本地启动、环境变量、schema 应用、测试和部署命令。
