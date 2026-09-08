# OOTD Backend

OOTD 后端采用 Go 1.26.5 模块化单体：一个 `go.mod`、一个 `main.go`、一个 OCI 镜像。相同二进制通过 `APP_ROLE=api|worker|all` 运行 Gin API 或异步 worker；生产可以分别扩缩，但不拆业务微服务。

## 固定技术栈

2026-09-08 已固定后端设计规范及启用边界。首版先交付本地 3D 穿搭闭环，后端留作后续云能力；当前没有 Go runtime，不启动生产云服务，不为本地角色调参/换装建立 API，也不复制衣橱到服务端。

- Gin `v1.12.0`
- GORM `v1.31.2` Generics + PostgreSQL driver `v1.6.2`
- PostgreSQL 18；Atlas `v1.3.0` versioned SQL migrations
- RabbitMQ `4.3.5` quorum queues + `amqp091-go v1.14.0`
- Redis 8.10 + `go-redis/v9 v9.22.0`
- 私有 S3-compatible 对象存储
- OpenAPI 3.1.2、kin-openapi `v0.149.0`
- OpenTelemetry Go `v1.46.0`、otelgin `v0.71.0`
- Testcontainers for Go `v0.44.0`
- FFmpeg `8.1.2`

精确边界和版本事实源见 [`../docs/design/01-技术选型.md`](../docs/design/01-技术选型.md)，模块与进程设计见 [`../docs/design/02-后端架构.md`](../docs/design/02-后端架构.md)，异步细节见 [`../docs/design/10-OOTD服务端与异步任务设计.md`](../docs/design/10-OOTD服务端与异步任务设计.md)。

## 当前文件

| 文件 | 用途 |
| --- | --- |
| `go.mod` | 唯一 Go module 与 Go toolchain；运行时代码建立后再由实际 import 固定依赖。 |
| `openapi.yaml` | iOS Client、Go contract test 和 Swagger UI 共用的唯一契约。 |
| `migrations/` | Atlas versioned SQL 事实源；当前等待首个已批准数据模型。 |

旧的空 `schema.sql` 已删除，不能与 migration 目录并行恢复。GORM model 是运行时映射，不是生产 schema 管理器。

## 目标结构

```text
backend/
├── main.go
├── go.mod
├── go.sum
├── openapi.yaml
├── atlas.hcl
├── migrations/
└── internal/
    ├── model/
    ├── service/
    ├── repository/
    ├── transport/
    ├── worker/
    └── platform/
```

不要为了填满结构创建空目录。第一项后端实现应先完成鉴权、配置、健康检查、数据库连接和 migration 验证，再增加媒体与生成任务。

## 编码前必须阅读

1. [PRD 10 功能清单](../docs/prd/10-OOTD产品需求.md#编码前固定的功能清单)：首版、分期与非目标。
2. [Design 01 技术冻结](../docs/design/01-技术选型.md#编码前技术冻结与启用界限)：固定组件、精确版本和启用阶段。
3. [Design 02 编码规范](../docs/design/02-后端架构.md#编码前固定的模块职责)：模块所有权、依赖/事务、HTTP/数据、故障与合入规则。
4. [Design 10 云任务](../docs/design/10-OOTD服务端与异步任务设计.md)：认证/幂等顺序、队列/租约、上传/删除生命周期。
5. [编码准入登记](../docs/plan/10-OOTD产品实施计划.md#编码前决策与准入登记)：对应切片的未决项与批准前置。

技术规范固定后，仍须为近期真实用例完成 OpenAPI、字段约束、迁移、依赖锁、测试和切片契约；不能仅根据本 README 建空服务或声称后端完成。

## 核心约束

- Handler 只处理 HTTP；Service 承担业务规则；Repository 独占 GORM/raw SQL。
- GORM 使用 Generics API，生产禁止 `AutoMigrate`。
- 跨表业务写入、幂等、配额和 Outbox 在同一个 PostgreSQL 事务中提交。
- RabbitMQ 消息不携带图片、签名 URL、令牌或敏感正文。
- Redis 不是业务事实源，也不是任务队列。
- API 与 worker 的所有 I/O 都传递 `context.Context`，支持超时、取消和优雅退出。
- 日志不得记录人物/衣物图片 URL、访问令牌、用户提示词或供应商正文。

当前尚无 Go 运行时代码，因此没有把未使用依赖伪造进 `go.mod`。实现某组件时必须按技术基线精确加入依赖并提交 `go.sum`，随后运行 `go test ./...`、`go vet ./...` 和集成测试。
