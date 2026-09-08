# OOTD Backend

OOTD 后端采用 Go 1.26.5 模块化单体：一个 `go.mod`、一个 `main.go`、一个 OCI 镜像。目标是相同二进制通过 `APP_ROLE=api|worker|all` 运行 Gin API 或异步 worker；当前仅实现 api，worker/all 明确拒绝启动，本地 OCI 构建与运行验证已落地，生产发布尚未完成。生产按角色部署，不拆业务微服务。

## 固定技术栈

2026-09-08 已固定后端设计规范及启用边界。首版先交付本地 3D 穿搭闭环，后端留作后续云能力；当前已开始 17-01 API 运行基线实现，不启动生产云服务，不为本地角色调参/换装建立 API，也不复制衣橱到服务端。

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
| `go.mod` | 唯一 Go module 与 Go toolchain，实际依赖由 go.sum 锁定。 |
| `openapi.yaml` | iOS Client、Go contract test 和 Swagger UI 共用的唯一契约。 |
| `migrations/` | Atlas versioned SQL 事实源；当前等待首个已批准数据模型。 |

旧的空 `schema.sql` 已删除，不能与 migration 目录并行恢复。GORM model 是运行时映射，不是生产 schema 管理器。

## 目录与规范入口

仓库顶层为 `backend/` 和 `app/`。后端入口保留根 `main.go`，只按真实职责增加 `internal` 包，不建项目包装层或空业务目录。

| 需要回答的问题 | 唯一规范入口 |
| --- | --- |
| 用哪些技术、精确版本与何时启用 | [Design 01 技术选型](../docs/design/01-技术选型.md#后端选型执行决策与当前状态) |
| 每个已有文件的功能、输入输出及边界 | [现有文件实现契约](../docs/design/02-后端架构.md#现有文件的实现契约) |
| 后续目录包含哪些功能、创建前要做什么 | [功能与文件落点](../docs/design/02-后端架构.md#后续目录的功能与文件落点) |
| 文件放哪里、各层负责什么 | [Design 02 目录职责](../docs/design/02-后端架构.md#目标目录) |
| 依赖、错误、事务、配置与服务代码如何写 | [Design 02 服务规范](../docs/design/02-后端架构.md#服务代码规范) |
| 从需求到实现、验证、发布如何推进 | [Design 02 开发交付 SOP](../docs/design/02-后端架构.md#后端开发与交付-sop) |
| 当前先做什么、什么仍阻断 | [17-01 执行计划](../docs/plan/17-01-后端服务启动与健康契约执行计划.md) |

model/service/repository/worker 在真实业务进入切片后按需建立；当前 platform/config、database、httpserver 和 transport 已有实际运行职责。首份业务 schema 才创建 SQL 与 atlas.hcl，17-07 已加入 Dockerfile，不为填满架构图创建占位代码。

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

当前只实现 API 角色；worker/all 明确拒绝启动，异步与云业务未启用。实现某组件时必须按技术基线精确加入依赖并提交 `go.sum`，随后运行 `go test ./...`、`go vet ./...` 和集成测试。

## 本地运行与验证

将 `.env.example` 的配置按实际隔离数据库环境设置到进程环境，然后在 `backend/` 执行 `go run .`。程序不自动加载或执行环境文件。PostgreSQL 必须为 major 18；远端连接要求 `sslmode=verify-full`，仅 loopback 开发连接允许 `disable`。

- `GET /v1/health/live`：进程存活，不访问数据库。
- `GET /v1/health/ready`：数据库可连接为 200，故障或退出中为 503；不代表业务 schema 或云功能就绪。
- `go test ./...`、`go vet ./...`、`go test -race ./...`：单元和 HTTP 契约验证。
- `go test -race -tags=integration ./tests -v`：需要 Docker，自动创建并清理固定 digest 的 PostgreSQL 18.4 容器，验证断连恢复。

具体范围和交付门禁见 [17-01 执行计划](../docs/plan/17-01-后端服务启动与健康契约执行计划.md)。当前 Swift 生成 Client 的默认 MainActor 隔离冲突仍阻断该切片完整交付；没有新增包装模块或更改 UI 并发规则。

2026-09-08 运行基线补充：监听异常返回前会关闭活动连接；实际 binary 已验证缺少 DATABASE_URL、worker/all 未实现、错误数据库凭据和监听端口占用均非零退出，错误输出不包含数据库 URL/密码。测试与限制见 [运行验收记录](../docs/acceptance/17-云端生成与任务管理验收.md#17-01-运行异常清理与启动失败补充验证)。

## 后端统一验证与容器构建

在仓库根目录执行：

```sh
scripts/validate-backend.sh integration
docker build -t then-backend:local backend
```

在 backend 目录验证上述实际本地镜像（需要 Docker）：

```sh
THEN_BACKEND_TEST_IMAGE=then-backend:local go test -race -tags=container ./tests -count=1 -v
```

镜像非 root，无 shell；默认监听仍为 127.0.0.1:8080。容器需要对外监听时显式设置 HTTP_ADDR=0.0.0.0:8080，并限制宿主发布地址/访问网络。DATABASE_URL 由运行环境提供，非 loopback 数据库要求 verify-full。测试使用隔离共享网络，不作为生产 TLS 部署样板。

统一脚本不要求 Xcode，也不替代 OpenAPI/Swift 的完整契约验收。容器测试验证只读根文件系统、资源限制、健康接口、SIGTERM 和未实现角色拒绝；当前仅验证 linux/arm64。详情见 [17-07](../docs/plan/17-07-后端容器构建与运行验证执行计划.md)。
