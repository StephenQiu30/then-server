# Go/Gin 后端工程规范

本文件是 `backend/` 的直接工程规范。架构理由见 [`docs/design/02-后端架构.md`](../docs/design/02-后端架构.md)，精确依赖版本见 [`docs/design/01-技术选型.md`](../docs/design/01-技术选型.md)。

前后端共同的产品边界、目录与接口合同见 [PROJECT.md](../PROJECT.md)；本文件维护 Go 侧执行细则，两者必须保持一致。

## 当前工程形态

- 一个 Go module、一个 `main.go` 命令、一个二进制和一个 OCI 镜像；`APP_ROLE=api|worker|all` 选择已实现角色。
- 不增加没有真实交付程序的命令目录、微服务、依赖注入框架或通用 BaseRepository。
- PostgreSQL 由 GORM 访问；当前开发阶段没有历史数据，进程启动时调用 GORM `AutoMigrate` 对齐表结构。
- Gin 承担 HTTP 运行时，Huma operation 与 Go struct tag 是接口声明源；OpenAPI 在运行时生成，不提交 YAML/JSON 物化文件。
- Swagger UI 读取同一进程的 `/openapi.json`；Umi 也从该地址生成请求代码。

## 目录

```text
backend/
├── main.go                         # 唯一薄入口，不再套 cmd 子目录
├── go.mod / go.sum                 # 唯一 Go module
├── internal/
│   ├── application/                # 按业务能力拆分；模型、规则、用例和消费端口共属一个 package
│   │   ├── account/
│   │   ├── privacy/
│   │   ├── wardrobe/
│   │   ├── outfitplan/
│   │   ├── wearevent/
│   │   ├── outfitfeedback/
│   │   ├── diary/
│   │   ├── community/
│   │   ├── media/
│   │   └── eventworker/
│   ├── adapter/
│   │   ├── httpapi/                # 按能力 *_contract / *_routes / *_handlers；router/health/errors/openapi 分责
│   │   ├── postgres/               # GORM record、迁移、查询与事务
│   │   ├── objectstore/            # MinIO 私有对象适配器
│   │   └── messagequeue/           # Kafka 持久消息适配器
│   ├── bootstrap/                  # 配置、具体依赖组装和进程生命周期
│   └── platform/
│       ├── config/                 # 类型化配置
│       ├── database/               # PostgreSQL/GORM 连接与探测
│       ├── httpserver/             # HTTP 生命周期
│       └── ratelimit/              # Redis 认证限流、探测与连接生命周期
├── tests/                          # 跨包测试；根目录不直接放 Go 文件
│   ├── services/                   # 本机真实依赖，build tag: services
│   ├── integration/                # 实际进程与隔离容器，build tag: integration
│   ├── container/                  # 已构建 OCI 镜像，build tag: container
│   └── internal/                   # 仅供以上测试共享的夹具
├── architecture_test.go            # 目录、入口、层间依赖与 HTTP 文件职责
├── Dockerfile / .dockerignore
└── .env.example
```

只按已经进入实现的业务能力创建语义包，例如 `application/account`、`adapter/postgres`、`adapter/httpapi`。不创建 `common`、`utils`、`manager` 等无明确所有权的收容目录。

package 内继续按真实能力拆语义文件。HTTP 的 contract、route registration、handler 按能力分文件；PostgreSQL 的 records、commands/queries、moderation、notifications 等按事务所有权分文件。只有职责形成可独立导入和测试的依赖边界时才增加子 package，不能为套用 Java MVC 或缩短单个文件机械复制目录树。完整 SOP 见 [Design 24](../docs/design/24-Go后端工程结构研究与规范.md)。

## 依赖方向

```text
main.go -> bootstrap
bootstrap -> application, adapter, platform
adapter -> application
application/<feature> -> 仅明确批准的更基础业务包
```

| 包 | 负责 | 禁止 |
| --- | --- | --- |
| `main.go` | 创建 logger 并委托 bootstrap | 业务规则、数据库和 HTTP 组装 |
| `bootstrap` | 配置、具体依赖组装、启动迁移、信号与关闭 | 业务规则、SQL、HTTP DTO |
| `application/<feature>` | 同一业务能力的模型、错误、规则、用例和自己消费的最小端口 | `gin.Context`、GORM record、SQL、外部 SDK；禁止重新建立全局 domain/model 大包 |
| `adapter/postgres` | GORM record、AutoMigrate、参数化查询、事务、领域映射 | Gin/Huma DTO、HTTP 状态码 |
| `adapter/httpapi` | 路由、输入校验、会话 Cookie、错误与状态映射 | GORM、业务 SQL、业务事务 |
| `adapter/objectstore` | MinIO 私有桶、versioning、签名 PUT、固定版本读写与全版本删除 | HTTP DTO、业务状态事务 |
| `adapter/messagequeue` | Kafka topic、acks=all、成功后同步提交 offset | 业务数据库、媒体正文 |
| `application/eventworker` | Outbox relay、JPEG 有界检查/重编码、删除与清扫编排 | Gin/Huma、GORM、具体 SDK |
| `platform` | 数据库和 HTTP 等技术资源的连接与生命周期 | 用户权限和业务状态规则 |

根 `architecture_test.go` 检查单 module、唯一根入口与源码位置，阻止核心包反向依赖、跨 adapter 直接导入和生产导入测试包，校验业务白名单及测试 suite 布局。`main.go` 只允许 log/slog、os 和 bootstrap；HTTP routes 不容纳 DTO/receiver 方法，contract 不容纳 Handler、消费方接口或非 schema 函数。新增依赖方向前先更新 Design 与测试，不用全局 service locator 绕过组装。

## Gin 与接口契约

- 使用 `gin.New()` 并显式注册中间件；受信代理、404、405、panic 恢复和日志行为必须可测试。
- 业务路由使用无版本前缀的语义根路径并通过 Huma operation 注册。每个 operation 声明稳定 `operationId`、method、path、tag、成功状态和预期错误。
- 请求/响应字段只在 httpapi DTO 上使用 `json`、校验和文档 tag；Handler 将它们转换为领域输入。
- 修改接口后运行 httpapi 契约测试，再实际访问 `/openapi.json` 验证 Swagger 或 Umi 消费。禁止手写或提交第二份接口 YAML/JSON。
- 文档默认关闭；本地显式开启时只允许回环监听。Swagger UI 只是阅读与调试入口，生成器读取 `/openapi.json`。

## GORM 与 PostgreSQL

- GORM record 只放在 `adapter/postgres`。表名、列类型、非空、唯一、索引、检查约束和关联删除通过 record tag 明确声明。
- `postgres.Migrate` 是当前唯一 schema 初始化入口，集中调用 `AutoMigrate`；bootstrap 必须在监听端口前执行并在失败时停止启动。
- 当前没有历史数据，不维护 Atlas 配置、SQL migration 目录或迁移兼容分支。需要破坏性改表时直接更新 record，并重建本地开发库。
- AutoMigrate 不删除废弃列。项目进入需要保留数据的部署阶段前，必须重新评审版本化迁移方案，不能把当前开发约定直接当作生产数据升级方案。
- 业务查询使用 GORM Generics 或 Repository 内的参数化 SQL；禁止拼接用户条件、无界 `Preload` 和依赖隐式 hook 承担核心规则。
- 跨表写入放在显式事务中；唯一约束、外键和检查约束必须由 PostgreSQL 集成测试证明。

## Go 与测试

- 遵循 `gofmt`、Go Code Review Comments 和标准库错误语义。错误可用 `errors.Is/As` 判断，不记录凭据、Token、数据库 URL 或用户媒体内容。
- API、Service、Repository I/O 传递 `context.Context`；构造函数拒绝无效依赖，资源由创建者逆序有界关闭。
- 包内 `_test.go` 验证纯逻辑、包级协作和 HTTP/OpenAPI 契约。Go 工具链不会把它们编入生产二进制，因此这不是测试代码与业务代码混编。
- `tests/services`、`tests/integration`、`tests/container` 分别承载本机真实依赖、实际进程、OCI 镜像验收；禁止把不同层级重新放回 `tests/` 根目录。`tests/internal` 只放测试层共享资源所有权代码。
- 最小检查：`gofmt -l .`、`go mod verify`、`go vet ./...`、`go test ./... -count=1`、`go test -race ./... -count=1`。
- 涉及 GORM/PostgreSQL、Redis、MinIO 或 Kafka 运行时依赖时增加 `go test -race -tags=services ./tests/... -count=1` 和 `go test -race -tags=integration ./tests/... -count=1`；涉及镜像时再运行 `go test -race -tags=container ./tests/... -count=1`。

## 完成条件

代码、Design、PRD、单切片 Plan 和 Acceptance 必须表达同一实现；依赖方向、运行时 OpenAPI、GORM schema、真实数据库行为、Redis 原子限流及相关 CI 检查均通过。验证范围要明确区分本地单测、真实服务、容器、远程 CI 和生产验收。

## 当前文件职责实施

[17-29](../docs/plan/17-29-Backend目录与HTTP文件职责规范化.md) 将账户、公开资料、隐私、衣橱、计划、实际穿着、日记与媒体统一为 `*_contract.go`、`*_routes.go`、`*_handlers.go`；社区的消费端口也归 handlers。DTO 与 schema 方法归 contract，operation 注册归 routes，消费接口/Handler/构造/映射归 handlers。`router.go` 只做 HTTP 组装；健康探测归 `health.go`，错误与请求关联归 `errors.go`，OpenAPI 规范化/序列化归 `openapi.go`，静态 Swagger 归既有 `docs.go`。不为这些文件新建嵌套 package。
