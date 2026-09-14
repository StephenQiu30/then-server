# Go/Gin 后端工程规范

本文件是 `backend/` 的直接工程规范。架构理由见 [`docs/design/02-后端架构.md`](../docs/design/02-后端架构.md)，精确依赖版本见 [`docs/design/01-技术选型.md`](../docs/design/01-技术选型.md)。

## 当前工程形态

- 一个 Go module、一个根 `main.go`、一个 API 二进制和一个 OCI 镜像。
- 当前只有 API 命令，不增加空的 `cmd/`、微服务、依赖注入框架或通用 BaseRepository。
- PostgreSQL 由 GORM 访问；当前开发阶段没有历史数据，进程启动时调用 GORM `AutoMigrate` 对齐表结构。
- Gin 承担 HTTP 运行时，Huma operation 与 Go struct tag 是接口声明源；OpenAPI 在运行时生成，不提交 YAML/JSON 物化文件。
- Swagger UI 读取同一进程的 `/openapi.json`；Umi 也从该地址生成请求代码。

## 目录

```text
backend/
├── main.go                         # 配置、依赖组装、迁移、启停
├── go.mod / go.sum                 # 唯一 Go module
├── internal/
│   ├── model/                      # 纯领域类型与错误
│   ├── service/                    # 用例和业务规则
│   ├── repository/                 # GORM record、迁移、查询与事务
│   ├── transport/                  # Gin/Huma 路由、DTO 与错误映射
│   │   └── swaggerui/              # 内嵌 Swagger UI 固定资源
│   └── platform/
│       ├── config/                 # 类型化配置
│       ├── database/               # PostgreSQL/GORM 连接与探测
│       └── httpserver/             # HTTP 生命周期
├── tests/                          # 真实依赖、进程和镜像测试
├── Dockerfile / .dockerignore
└── .env.example
```

只按已经进入实现的业务能力创建语义文件，例如 `service/account.go`、`repository/account.go`、`transport/account.go`。不创建 `common`、`utils`、`manager` 等无明确所有权的收容目录。

## 依赖方向

```text
main -> transport, service, repository, platform
transport -> model
service -> model
repository -> model
```

| 包 | 负责 | 禁止 |
| --- | --- | --- |
| `main` | 配置、具体依赖组装、启动迁移、信号与关闭 | 业务规则、SQL、HTTP DTO |
| `model` | 领域值、不变量、领域错误 | Gin、Huma、GORM、SQL、外部 SDK |
| `service` | 用例规则；定义自己需要的最小 Repository 接口 | `gin.Context`、GORM record、SQL |
| `repository` | GORM record、AutoMigrate、参数化查询、事务、领域映射 | Gin/Huma DTO、HTTP 状态码 |
| `transport` | 路由、输入校验、会话 Cookie、错误与状态映射 | GORM、业务 SQL、业务事务 |
| `platform` | 数据库和 HTTP 等技术资源的连接与生命周期 | 用户权限和业务状态规则 |

根 `architecture_test.go` 负责阻止核心包反向依赖。新增依赖方向前先更新 Design 与测试，不用全局 service locator 绕过组装。

## Gin 与接口契约

- 使用 `gin.New()` 并显式注册中间件；受信代理、404、405、panic 恢复和日志行为必须可测试。
- `/v1` 业务路由通过 Huma operation 注册。每个 operation 声明稳定 `operationId`、method、path、tag、成功状态和预期错误。
- 请求/响应字段只在 transport DTO 上使用 `json`、校验和文档 tag；Handler 将它们转换为领域输入。
- 修改接口后运行 transport 契约测试，再实际访问 `/openapi.json` 验证 Swagger 或 Umi 消费。禁止手写或提交第二份接口 YAML/JSON。
- 文档默认关闭；本地显式开启时只允许回环监听。Swagger UI 只是阅读与调试入口，生成器读取 `/openapi.json`。

## GORM 与 PostgreSQL

- GORM record 只放在 Repository。表名、列类型、非空、唯一、索引、检查约束和关联删除通过 record tag 明确声明。
- `repository.Migrate` 是当前唯一 schema 初始化入口，集中调用 `AutoMigrate`；`main` 必须在监听端口前执行并在失败时停止启动。
- 当前没有历史数据，不维护 Atlas 配置、SQL migration 目录或迁移兼容分支。需要破坏性改表时直接更新 record，并重建本地开发库。
- AutoMigrate 不删除废弃列。项目进入需要保留数据的部署阶段前，必须重新评审版本化迁移方案，不能把当前开发约定直接当作生产数据升级方案。
- 业务查询使用 GORM Generics 或 Repository 内的参数化 SQL；禁止拼接用户条件、无界 `Preload` 和依赖隐式 hook 承担核心规则。
- 跨表写入放在显式事务中；唯一约束、外键和检查约束必须由 PostgreSQL 集成测试证明。

## Go 与测试

- 遵循 `gofmt`、Go Code Review Comments 和标准库错误语义。错误可用 `errors.Is/As` 判断，不记录凭据、Token、数据库 URL 或用户媒体内容。
- API、Service、Repository I/O 传递 `context.Context`；构造函数拒绝无效依赖，资源由创建者逆序有界关闭。
- 包内 `_test.go` 验证纯逻辑和 HTTP 契约；`tests/` 只放真实 PostgreSQL、本机中间件、实际进程和镜像测试，它不是 test 微服务。
- 最小检查：`gofmt -l .`、`go mod verify`、`go vet ./...`、`go test ./... -count=1`、`go test -race ./... -count=1`。
- 涉及 GORM schema 或 PostgreSQL 时增加 `go test -race -tags=services ./tests -count=1` 和 `go test -race -tags=integration ./tests -count=1`；涉及镜像时再运行 container 测试。

## 完成条件

代码、Design、PRD、单切片 Plan 和 Acceptance 必须表达同一实现；依赖方向、运行时 OpenAPI、GORM schema、真实数据库行为及相关 CI 检查均通过。验证范围要明确区分本地单测、真实服务、容器、远程 CI 和生产验收。
