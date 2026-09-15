# OOTD Backend

这是“于是”当前的 Go/Gin 模块化单体。一个 `main.go` 负责组装 Gin、Huma、GORM/PostgreSQL、Redis、MinIO、RabbitMQ 和进程生命周期；账号、会话、本人成年声明、带用户确认属性的结构化衣橱 CRUD，以及合成本人照片的私有上传/检查/删除 API 已经实现。

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

## 隐私前置 API

- `GET /v1/privacy/self-adult-declaration`
- `PUT /v1/privacy/self-adult-declaration`
- `DELETE /v1/privacy/self-adult-declaration`

该 API 只记录当前账号对 `self-adult-v1` 的确认或撤回，不收集出生日期或证件。它不是第三方 AI 逐次同意。

## 结构化衣橱 API

- `POST /v1/wardrobe/items`
- `GET /v1/wardrobe/items`
- `GET /v1/wardrobe/items/{item_id}`
- `PUT /v1/wardrobe/items/{item_id}`
- `DELETE /v1/wardrobe/items/{item_id}`

OpenAPI 0.9.0 在无图最小结构上增加正式度、保暖感受、雨天和步行适用四项 nullable 用户确认属性。POST/PUT 必须提交 `attributes` 对象；空项表示未知，非空响应携带 `user_confirmed`，请求不能提交来源。会话决定 owner；属性参与幂等比较和完整 revision 更新。App 尚未接入主动同步，衣物图片、增量墓碑和多设备合并不在本切片。

## 合成本人照片开发闭环

运行时 OpenAPI 包含 8 个私有媒体 operation：同意创建/查询/撤回、上传意图、finalize、媒体状态、删除和删除状态。开发入口必须显式设置 `MEDIA_DEVELOPMENT_ENABLED=true`，并只接受回环 HTTP、MinIO 与 RabbitMQ；因此真实用户照片和生产流量无法通过这组配置误开启。

MinIO 使用 `raw-private` 与 `derived-private` 私有版本桶；RabbitMQ worker 从 PostgreSQL Outbox 取得事件，以 Inbox 和条件状态更新保证重复投递不产生第二份业务效果。输入只接受 12 MiB/24 MP 以内的单帧 JPEG，worker 固定对象 version ID、复算 SHA-256、解码后重编码并记录派生关系。删除先在事务内 tombstone，再删除原始对象全部版本和派生对象。

本机合成数据运行示例需要 `.env.example` 中的 PostgreSQL、Redis、MinIO 和 RabbitMQ 参数，并使用 `APP_ROLE=all`。`api` 与 `worker` 可由同一二进制分别运行。

## 本机中间件验证

Redis 只保存认证限流的短期计数；PostgreSQL 是账户、同意、媒体状态、Outbox 和 Inbox 的事实源。RabbitMQ 与 MinIO 已进入获批的合成照片开发闭环。默认测试连接本机 loopback；需要时用 `THEN_TEST_*` 环境变量覆盖本机端口和账号。

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
│   ├── objectstore/
│   ├── messagequeue/
│   ├── worker/
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
