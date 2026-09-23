# OOTD Backend

这是“于是”当前的 Go/Gin 模块化单体。`main.go` 通过 `internal/bootstrap` 组装 Gin、Huma、GORM/PostgreSQL、Redis、MinIO、Kafka 和进程生命周期；账号、会话、本人成年声明、结构化衣橱、账号穿搭计划、账号实际穿着、私人穿搭日记与日历、社区帖子审核治理，以及私有图片上传/检查/删除 API 已经实现。

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

进程连接数据库后会在监听端口前执行 GORM `AutoMigrate`。当前处于无历史数据的开发阶段，数据库结构由 [`internal/adapter/postgres`](internal/adapter/postgres) 的 GORM record 统一声明；项目不维护 Atlas 配置或 SQL migration。需要破坏性调整时更新 record 并重建本地开发库。

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

`frontend` 通过 `@umijs/openapi` 直接读取开发 API 的 `/openapi.json`，生成到 `frontend/src/api/`。`API_DOCS_ENABLED` 默认关闭，开启时只允许回环监听；页面提供筛选、operationId、请求耗时和同源 Try it out。

## 账号 API

- `POST /auth/registrations`
- `POST /auth/sessions`
- `DELETE /auth/session`
- `GET /users/me`
- `PATCH /users/me`
- `DELETE /users/me`
- `POST /auth/email-verifications`
- `POST /auth/email-verifications/confirm`
- `POST /auth/password-resets`
- `POST /auth/password-resets/confirm`

浏览器会话使用 HttpOnly、SameSite=Strict Cookie。本机回环开发可设置 `SESSION_COOKIE_SECURE=false`；非回环监听必须使用安全 Cookie。注册按直连源 IP 每小时 5 次、登录每 15 分钟 10 次，Redis 原子计数超限返回 429 与 `Retry-After`；Redis 不可用时认证失败关闭且 readiness 返回 503。

账号邮件功能需要在运行环境同时配置 `ACCOUNT_MAIL_FROM`（完整 163 邮箱）、`ACCOUNT_MAIL_AUTH_CODE`（网易客户端授权码）、`ACCOUNT_MAIL_KEY`（至少 32 随机字节的无填充 base64url）和 `ACCOUNT_MAIL_LINK_BASE`（HTTPS 应用入口）。`ACCOUNT_MAIL_SMTP_ADDR` 默认为 `smtp.163.com:465`，只使用验证证书的 TLS；缺少邮件配置时四个新端点返回 503，不会发送。授权码、挑战密钥不写入仓库或日志。验证/找回后端已支持合成环境；正式送达、App/Web 链接入口、可信代理/边缘防护、地域与生产审计仍是公开注册门禁。

## 隐私前置 API

- `GET /privacy/self-adult-declaration`
- `PUT /privacy/self-adult-declaration`
- `DELETE /privacy/self-adult-declaration`

该 API 只记录当前账号对 `self-adult-v1` 的确认或撤回，不收集出生日期或证件。它不是第三方 AI 逐次同意。

## 结构化衣橱 API

- `POST /wardrobe/items`
- `GET /wardrobe/items`
- `GET /wardrobe/items/{item_id}`
- `GET /wardrobe/items/{item_id}/deletion-impact`
- `PUT /wardrobe/items/{item_id}`
- `DELETE /wardrobe/items/{item_id}`

当前运行时 OpenAPI 文档版本为 0.19.0，共 106 个 operation，覆盖账号邮件、衣橱、计划、实际事件、反馈统计、私人日记与社区互动治理，并保持业务路径无版本前缀。本人资源更新必须提交 `expected_revision`；公开帖子只包含批准版本的公开字段，来源日记、媒体 ID、owner、对象 key、对象 version 和同意记录不进入公开响应，HttpOnly 会话 Cookie 不进入生成客户端参数。

## 账号穿搭计划 API

- `POST /outfit-plans`
- `GET /outfit-plans`
- `GET /outfit-plans/{plan_id}`
- `PUT /outfit-plans/{plan_id}`
- `POST /outfit-plans/{plan_id}/cancel`
- `DELETE /outfit-plans/{plan_id}`

请求只提交计划日期、IANA 时区、可选摘要及有序的衣物 ID/revision；名称、类别、可用状态和确认属性由服务端在同一 PostgreSQL 事务中生成快照。创建按客户端 UUID 幂等，更新/状态/删除使用 revision，永久删除写入 tombstone 防止迟到请求复活。计划保存不会自行创建实际穿着；App 主动同步、反馈、推荐与提醒仍未启用。

## 账号实际穿着 API

- `POST /wear-events`
- `GET /wear-events`
- `GET /wear-events/{wear_event_id}`
- `PUT /wear-events/{wear_event_id}`
- `DELETE /wear-events/{wear_event_id}`
- `POST /outfit-plans/{plan_id}/not-worn`
- `POST /outfit-plans/{plan_id}/restore`

实际事件只接受衣物 ID/revision 和用户明确确认，服务端生成快照。同日高度相似记录返回当前候选供再次确认；保存与待洗状态、计划 completed 状态在同一 PostgreSQL 事务完成。纠正/删除使用 revision，永久删除写 tombstone；删除最后一条关联事件后计划恢复 active。

## 合成本人照片开发闭环

运行时 OpenAPI 包含 8 个私有媒体 operation：同意创建/查询/撤回、上传意图、finalize、媒体状态、删除和删除状态。开发入口必须显式设置 `MEDIA_DEVELOPMENT_ENABLED=true`，并只接受回环 HTTP、MinIO 与 Kafka；因此真实用户照片和生产流量无法通过这组配置误开启。

MinIO 使用 `raw-private` 与 `derived-private` 私有版本桶；Kafka worker 从 PostgreSQL Outbox 取得事件，以 Inbox 和条件状态更新保证重复投递不产生第二份业务效果。输入只接受 12 MiB/24 MP 以内的单帧 JPEG，worker 固定对象 version ID、复算 SHA-256、解码后重编码并记录派生关系。删除先在事务内 tombstone，再删除原始对象全部版本和派生对象。

本机合成数据运行示例需要 `.env.example` 中的 PostgreSQL、Redis、MinIO 和 Kafka 参数，并使用 `APP_ROLE=all`。`api` 与 `worker` 可由同一二进制分别运行。

## 私人穿搭日记与月日历

- `POST /diary-entries`
- `GET /diary-entries`
- `GET /diary-entries/{entry_id}`
- `PUT /diary-entries/{entry_id}`
- `GET /diary-entries/{entry_id}/deletion-impact`
- `DELETE /diary-entries/{entry_id}`
- `GET /calendar?month=YYYY-MM`

日记允许纯文字、普通私有图片或两者组合，可选关联本人的计划和同日实际穿着。普通日记 JPEG 复用媒体净化链但不要求本人照片同意；只有 ready 的 owner 媒体可关联。日记保留原本地日期和 IANA 时区，可同日多条；未来日期、跨账号关联和旧 revision 被拒绝。删除日记保留媒体，删除计划或实际事件只解除关联，月日历分别返回计划、实际穿着和日记数量。

## 社区帖子审核与治理

作者可创建私人草稿、提交人工审核、撤回和删除；公开详情与图片只读取 active 作者当前批准的不可变版本。社区图片使用独立 `community_publish` 用途并从 MinIO 固定 derived version 返回。moderator/admin 可审核、处理举报和下架，admin 还可封禁或恢复账号；封禁会在同一事务撤销全部会话。Feed、搜索、评论、赞藏关注、屏蔽、通知与申诉已实现后端开发接口；客户端和运营发布验收仍按对应切片执行。

## 本机中间件验证

Redis 只保存认证限流的短期计数；PostgreSQL 是账户、同意、媒体状态、Outbox 和 Inbox 的事实源。Kafka 与 MinIO 已进入获批的合成照片开发闭环。默认测试连接本机 loopback；需要时用 `THEN_TEST_*` 环境变量覆盖本机端口和账号。

```sh
brew services start minio
brew services start redis
brew services start kafka
go test -race -tags=services ./tests/... -count=1 -v
```

根 `docker-compose-env.yml` 只是显式选择的隔离备用环境。本机服务可用时不启动 Compose 依赖。

## 结构

```text
backend/
├── main.go
├── internal/
│   ├── application/
│   ├── adapter/
│   │   ├── httpapi/
│   │   ├── postgres/
│   │   ├── objectstore/
│   │   └── messagequeue/
│   ├── bootstrap/
│   └── platform/
├── tests/
│   ├── services/       # 本机 PostgreSQL/Redis/MinIO/Kafka
│   ├── integration/    # 实际二进制与 Testcontainers
│   ├── container/      # 已构建 OCI 镜像
│   └── internal/       # 测试专用共享夹具
├── architecture_test.go # 目录、入口、依赖与 HTTP 文件职责
├── Dockerfile
└── .env.example
```

详细职责、依赖方向和完成标准见 [AGENTS.md](AGENTS.md)；产品架构和 SOP 见 [Design 02](../docs/design/02-后端架构.md)。

### HTTP 文件归属

[17-29](../docs/plan/17-29-Backend目录与HTTP文件职责规范化.md) 已落实同一 httpapi package 内的文件职责：

```text
internal/adapter/httpapi/
├── <resource>_contract.go   DTO、tag、schema 方法
├── <resource>_routes.go     Huma operation 注册与状态声明
├── <resource>_handlers.go   消费端口、Handler、应用调用与映射
├── router.go               Gin/Huma 与业务路由组装
├── health.go               存活、就绪与探测
├── errors.go               全局错误与请求关联
├── openapi.go              运行时规范化、序列化与校验
├── docs.go / swaggerui/    内嵌文档资源
└── *_test.go               原有包内协议与行为测试
```

resource 包括 account、profile、privacy、wardrobe、outfit_plan、wear_event、diary、media、community 和 community_social。保留根 main.go 和现有 application/postgres 事务归属，不增加入口包装或镜像目录。

## 验证

```sh
go mod verify
go vet ./...
go test ./... -count=1
go test -race ./... -count=1
go test -race -tags=services ./tests/... -count=1 -v
go test -race -tags=integration ./tests/... -count=1 -v
```

`services` 使用已经启动的本机 PostgreSQL/MinIO/Redis/Kafka；`integration` 使用 Testcontainers 创建隔离 PostgreSQL 18、Redis、MinIO 与 Kafka，验证实际二进制从空库迁移、启动、认证限流、依赖断连恢复、合成照片上传/检查/删除和 SIGTERM 退出。镜像验证另见 [17-07](../docs/plan/17-07-后端容器构建与运行验证执行计划.md)。

## Kafka 本地消息合同

配置 `KAFKA_BROKERS=127.0.0.1:9092`、`KAFKA_TOPIC_PREFIX=then`；隔离 Compose 使用 `127.0.0.1:18992`。`RABBITMQ_URL` 已退役。broker 与媒体开关继续仅用于 loopback 合成数据开发，系统中其他项目的 RabbitMQ 不受影响。

franz-go 使用幂等生产和 `acks=all`，Outbox 在确认后标记 published。单次 broker produce 请求为 10 秒，记录交付预算为 30 秒，以容纳元数据发现与重试；超时退出 relay，未确认 Outbox 留待重启。三个 topic 为 `<prefix>.media-check`、`<prefix>.media-delete`、`<prefix>.community-notification`，对应消费组为 `<topic>.worker`，key 为 aggregate ID。仅创建这三个开发 topic（单分区、单副本、7 天保留），不依赖自动创建任意 topic。

消费者关闭自动提交，一次处理一条，业务成功后同步提交 offset；失败最多尝试三次，每次最长 30 秒，间隔 250/500ms，耗尽后退出并保留 offset。非法记录停止消费并保留位置供检查，不静默跳过。消费者处理/提交期间阻止 rebalance，随后释放；worker 取消后等待全部协程退出，再关闭资源。

services 使用 `THEN_TEST_KAFKA_BROKERS` 和每次运行独立 prefix，清理只作用于该次 topic/消费组。端到端业务幂等由 PostgreSQL Inbox/状态约束承担；单机 acks=all 不证明生产高可用。迁移、回退与生产门禁见 [17-26](../docs/plan/17-26-Kafka与工程规范化执行计划.md)。
