# OOTD Backend

## 本地数据库与中间件

普通开发直接使用本机已经安装并启动的服务，不要求 Docker：

```sh
brew services start postgresql@18
brew services start minio
brew services start redis
brew services start rabbitmq
(cd backend && go test -race -tags=services ./tests -count=1)
```

服务测试默认连接 PostgreSQL `127.0.0.1:5432/postgres`、MinIO `127.0.0.1:9000`、Redis `127.0.0.1:6379` 的 DB 15 与 RabbitMQ `127.0.0.1:5672`，适配 Homebrew 默认开发安装。已有自定义账号或端口时，通过 `THEN_TEST_DATABASE_URL`、`THEN_TEST_MINIO_ENDPOINT`、`THEN_TEST_MINIO_ACCESS_KEY`、`THEN_TEST_MINIO_SECRET_KEY`、`THEN_TEST_REDIS_ADDR`、`THEN_TEST_REDIS_PASSWORD`、`THEN_TEST_REDIS_DB`、`THEN_TEST_RABBITMQ_URL` 覆盖；测试只允许 loopback、只读取进程环境、不解析项目 `.env`，并且不会输出连接密钥。

| 服务 | 本机入口 | 用途 |
| --- | --- | --- |
| PostgreSQL | 127.0.0.1:5432 | API 数据库与事务验证 |
| MinIO | S3 127.0.0.1:9000 | 私有对象开发验证 |
| Redis | 127.0.0.1:6379 | 可失效缓存/TTL 验证 |
| RabbitMQ | AMQP 127.0.0.1:5672 | quorum/确认/重投递验证 |

这些地址只用于 loopback 合成数据开发，不是生产应用权限模型。自定义凭据由本机服务管理，不能复制到源码或日志。

根 `docker-compose.yml` 与 `docker-compose-env.yml` 只保留一个显式的 `isolated-env` 备用环境。确需隔离时才复制 `.env.example`、设置随机凭据并运行 `docker compose --profile isolated-env up --detach --wait`；它使用 18432/18900/18379/18672 等备用端口，不会替代本机默认开发服务。`docker build --tag then-backend:local backend` 仍可独立验证后端镜像。

**MinIO 官方服务端已归档，不再维护；当前锁定历史发行版仅用于隔离开发。生产发行版、维护支持和安全修复方案尚未确定。** 选型来源、镜像/SDK 版本与限制归 [Design 01](../docs/design/01-技术选型.md#数据库与中间件接入决策)，spec/checklist 归 [17-10](../docs/plan/17-10-数据库与中间件开发环境执行计划.md)。四项协议测试通过不代表业务任务 Outbox/Inbox、取消/删除或用户媒体链路完成。

## Swagger接口文档

设置好本地 PostgreSQL 18 的 `DATABASE_URL` 后，在 `backend/` 目录运行：

```sh
API_DOCS_ENABLED=true go run .
```

默认地址为 [Swagger UI](http://127.0.0.1:8080/docs/) 和 [原始契约](http://127.0.0.1:8080/openapi.yaml)；自定义 HTTP_ADDR 时使用对应端口。页面、脚本、样式、许可证与唯一 OpenAPI 都随 Go 二进制嵌入，页面展示 API 实际校验的契约版本。修改源契约后需要重建/重启后端。

`API_DOCS_ENABLED` 默认 false，仅接受 true/false，启用时必须绑定回环 IP；关闭时所有文档路径返回 404。当前只提供本机开发文档，Try it out、外部 validator、查询配置覆盖与鉴权持久化关闭，生产文档未启用。API 仍按既有语义要求数据库可用才能启动。

Swift 生成不要求 Docker、文档页面或 API 运行，直接读取仓库唯一 YAML；见 [iOS README](../app/README.md#openapi请求代码生成)。独立 Swagger Compose 和管理脚本已退役，不提供旧 18108 端口或脚本兼容入口。当前 spec/checklist 和证据见 [17-09](../docs/plan/17-09-后端内嵌接口文档执行计划.md)。

## 后端基线

OOTD 后端采用 Go 1.26.5 模块化单体：一个 `go.mod`、一个 `main.go`、一个 OCI 镜像。目标是相同二进制通过 `APP_ROLE=api|worker|all` 运行 Gin API 或异步 worker；当前仅实现 api，worker/all 明确拒绝启动，本地 OCI 构建与运行验证已落地，生产发布尚未完成。生产按角色部署，不拆业务微服务。

## 固定技术栈

当前保留小型 Go 单体，按实际代码区分组件：

| 状态 | 组件 | 本阶段用途 |
| --- | --- | --- |
| 已接入 | Gin、GORM/PostgreSQL、kin-openapi、标准库 slog/context/config | API 启动、两个健康接口、数据库连接和内嵌 Swagger；尚无用户云业务 |
| 已接入开发验证 | Go testing/httptest、Testcontainers/Moby | 后两者用于带标签的数据库/镜像集成测试，不是 API 的 Docker 运行依赖 |
| 首个业务 schema 时 | Atlas versioned SQL | 先核定发行版/许可和所需命令，不预建空 migration 或默认依赖 Pro |
| 已接入开发验证 | RabbitMQ | 17-10 验证 quorum、confirm、ack/requeue；业务 Outbox/Inbox 尚待任务切片 |
| 已接入开发验证 | Redis、MinIO | TTL、鉴权、私有对象读写删除；MinIO 历史镜像只用于合成数据开发 |
| 明确需要时 | FFmpeg、OTel exporter | 视频处理及生产观测；不作为本地三维前置 |

精确版本和开源复用决策统一见 [Design 01](../docs/design/01-技术选型.md#当前最小技术栈与开源复用)，不维护第二份可能被误当成“全部已运行”的版本清单。当前三维开发只需 iOS 本地应用；后端开发环境由 17-10 提供 PG/MinIO/Redis/RabbitMQ，API 当前只连接 PG，其余由真实协议测试消费。API 启动要求数据库的现有语义保留，生成 Swift 请求文件则不需要它们运行。

不增加微服务、BFF、通用 BaseRepository、自动依赖注入框架、另一套任务队列或独立 Swagger 服务。角色/衣物固定资产随 App 提供，不为内置目录创建远程 Catalog 或上传接口。

## 当前文件

| 文件 | 用途 |
| --- | --- |
| `go.mod` | 唯一 Go module 与 Go toolchain，实际依赖由 go.sum 锁定。 |
| `openapi.yaml` | iOS Client、Go contract test 和 Swagger UI 共用的唯一契约。 |

首个服务端业务 schema 获批时再创建 `migrations/*.sql`、`atlas.sum` 与 `atlas.hcl`；当前不保留空目录或说明文件。

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

1. [PRD 10 功能清单](../docs/prd/10-OOTD产品需求.md#当前功能清单2026-09-13更新)：首版、分期与非目标。
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

具体范围和交付门禁见 [17-01 执行计划](../docs/plan/17-01-后端服务启动与健康契约执行计划.md)。用户已批准仅编译生成代码的 ThenTransport 技术模块，Swift Client 与正式 App 构建及相关测试已通过；ThenApp/UI 继续 MainActor。17-01 当前待提交交付，完整云业务与生产验收分别按所属切片推进。

2026-09-08 运行基线补充：监听异常返回前会关闭活动连接；实际 binary 已验证缺少 DATABASE_URL、worker/all 未实现、错误数据库凭据和监听端口占用均非零退出，错误输出不包含数据库 URL/密码。测试与限制见 [运行验收记录](../docs/acceptance/17-云端生成与任务管理验收.md#17-01-运行异常清理与启动失败补充验证)。

## 后端统一验证与容器构建

测试与生产进程边界：包级 unit/contract 使用同目录 `_test.go`；`backend/tests` 是独立黑盒测试包，只有显式 build tag 才加载本机服务、Testcontainers 或镜像测试依赖。它不是运行时微服务，不增加第二个 Go module 或 `APP_ROLE=test`。详细矩阵见 [Design 14](../docs/design/14-后端MVP与测试边界设计.md#独立测试矩阵)。

在仓库根目录执行：

```sh
cd backend
go test ./...
go vet ./...
go test -race ./...
go test -race -tags=integration ./tests -count=1
cd ..
docker build -t then-backend:local backend
```

在 backend 目录验证上述实际本地镜像（需要 Docker）：

```sh
THEN_BACKEND_TEST_IMAGE=then-backend:local go test -race -tags=container ./tests -count=1 -v
```

镜像非 root，无 shell；默认监听仍为 127.0.0.1:8080。容器需要对外监听时显式设置 HTTP_ADDR=0.0.0.0:8080，并限制宿主发布地址/访问网络。DATABASE_URL 由运行环境提供，非 loopback 数据库要求 verify-full。测试使用隔离共享网络，不作为生产 TLS 部署样板。

这些后端命令不要求 Xcode，也不替代 OpenAPI/Swift 的完整契约验收。容器测试验证只读根文件系统、资源限制、健康接口、SIGTERM 和未实现角色拒绝；当前仅验证 linux/arm64。详情见 [17-07](../docs/plan/17-07-后端容器构建与运行验证执行计划.md)。
