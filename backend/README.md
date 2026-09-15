# OOTD Backend

这是“于是”当前的 Go/Gin 模块化单体。一个 `main.go` 负责组装 Gin、Huma、GORM/PostgreSQL、Redis 认证限流和进程生命周期；账号注册、登录、退出及本人账户查询/修改/删除已经实现。

## 本地运行

普通开发使用本机 PostgreSQL 18 与 Redis，不要求 Docker，也不会自动读取 `.env`：

```sh
brew services start postgresql@18
brew services start redis
cd backend
DATABASE_URL='postgres://127.0.0.1/postgres?sslmode=disable' \
REDIS_URL='redis://127.0.0.1:6379/0' \
go run .
```

进程连接数据库后会在监听端口前执行 GORM `AutoMigrate`。当前处于无历史数据的开发阶段，数据库结构由 [`internal/repository`](internal/repository) 的 GORM record 统一声明；项目不维护 Atlas 配置或 SQL migration。需要破坏性调整时更新 record 并重建本地开发库。

## Swagger 与 OpenAPI

```sh
API_DOCS_ENABLED=true \
DATABASE_URL='postgres://127.0.0.1/postgres?sslmode=disable' \
REDIS_URL='redis://127.0.0.1:6379/0' \
go run .
```

默认入口：

- [Swagger 文档](http://127.0.0.1:8080/docs/)
- [OpenAPI JSON](http://127.0.0.1:8080/openapi.json)
- [OpenAPI YAML](http://127.0.0.1:8080/openapi.yaml)

Huma operation、请求/响应结构和字段 tag 是唯一接口声明。API 启动时从这些声明生成并校验 OpenAPI 3.1.2，Swagger UI 和两个契约地址都读取同一个运行时对象；仓库不保存生成 YAML/JSON，也不运行独立 Swagger 容器。

未来 `frontend` 通过 `@umijs/openapi` 直接读取开发 API 的 `/openapi.json`，生成到前端自己的 generated 目录。`API_DOCS_ENABLED` 默认关闭，开启时只允许回环监听；页面提供筛选、operationId、请求耗时和同源 Try it out。

## 账号 API

- `POST /v1/auth/registrations`
- `POST /v1/auth/sessions`
- `DELETE /v1/auth/session`
- `GET /v1/users/me`
- `PATCH /v1/users/me`
- `DELETE /v1/users/me`

浏览器会话使用 HttpOnly、SameSite=Strict Cookie。本机回环开发可设置 `SESSION_COOKIE_SECURE=false`；非回环监听必须使用安全 Cookie。注册按直连源 IP 每小时 5 次、登录每 15 分钟 10 次，Redis 原子计数超限返回 429 与 `Retry-After`；Redis 不可用时认证失败关闭且 readiness 返回 503。邮件验证、找回密码、可信代理/边缘防护和生产审计尚未完成，因此当前端点只用于开发 MVP。

## 本机中间件验证

Redis 已进入 API 运行时，只保存认证限流的短期计数；账户与会话事实仍在 PostgreSQL。RabbitMQ 和 MinIO 当前只用于开发协议测试。默认测试连接本机 loopback；需要时用 `THEN_TEST_*` 环境变量覆盖本机端口和账号。

```sh
brew services start minio
brew services start redis
brew services start rabbitmq
go test -race -tags=services ./tests -count=1 -v
```

根 `docker-compose-env.yml` 只是显式选择的隔离备用环境。本机服务可用时不启动 Compose 依赖。

## 结构

```text
backend/
├── main.go
├── internal/
│   ├── model/
│   ├── service/
│   ├── repository/
│   ├── transport/
│   └── platform/
├── tests/
├── Dockerfile
└── .env.example
```

详细职责、依赖方向和完成标准见 [AGENTS.md](AGENTS.md)；产品架构和 SOP 见 [Design 02](../docs/design/02-后端架构.md)。

## 验证

```sh
go mod verify
go vet ./...
go test ./... -count=1
go test -race ./... -count=1
go test -race -tags=services ./tests -count=1 -v
go test -race -tags=integration ./tests -count=1 -v
```

`services` 使用已经启动的本机 PostgreSQL/MinIO/Redis/RabbitMQ；`integration` 使用 Testcontainers 创建隔离 PostgreSQL 18 与 Redis，并验证实际二进制从空库迁移、启动、认证限流、两项依赖断连恢复和 SIGTERM 退出。镜像验证另见 [17-07](../docs/plan/17-07-后端容器构建与运行验证执行计划.md)。
