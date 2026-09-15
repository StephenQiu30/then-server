# then-server

“于是”OOTD 的 Go 服务端与中央产品文档仓库，位于 `Then/then-server`；客户端是同级独立 [then-app](../then-app/README.md)。

## 当前实现

Go + Gin/Huma + GORM/AutoMigrate + PostgreSQL 已实现健康检查、注册/登录、Cookie 会话、本人账户 CRUD 与本人成年声明的查询/确认/撤回。运行时 OpenAPI 是 Swagger 与未来 Umi 请求生成的唯一输入。目录与规范见 [后端架构](docs/design/02-后端架构.md)，精确版本见 [技术选型](docs/design/01-技术选型.md)。

日常使用本机已安装 PostgreSQL、Redis、RabbitMQ、MinIO；当前 API 以 PostgreSQL 保存账号、会话、声明、结构化衣橱和合成照片媒体事实，Redis 只保存认证限流短期计数。RabbitMQ 与 MinIO 已用于显式开启的合成照片开发闭环，默认仍关闭。`frontend` 按用户要求暂停实现；App 云接入、生成和同步未完成。

## 目录与入口

| 位置 | 内容 |
| --- | --- |
| [design.md](design.md) | 根目录产品设计规范 |
| [backend](backend/README.md) | Go 运行说明、内嵌 Swagger、schema 和独立测试 |
| [docs](docs/README.md) | 当前 Design → PRD → Plan → Acceptance |
| [产品计划](docs/plan/10-OOTD产品实施计划.md) | 全部切片的当前状态、缺口与下一步 |
| [系统验收](docs/acceptance/10-OOTD产品系统验收.md) | 局部证据与完整产品的验收边界 |
| [docker-compose.yml](docker-compose.yml) / [docker-compose-env.yml](docker-compose-env.yml) | 明确需要隔离环境时使用，非日常默认启动 |
| [AGENTS.md](AGENTS.md) / [CONTRIBUTING.md](CONTRIBUTING.md) | 开发和提交规范 |

不维护 OpenAPI 物化文件、Atlas 迁移账本、生成脚本或旧项目兼容目录。当前业务未进入实现的模块不建空壳。静态内部素材见 [assets](assets/README.md)。

## 本地校验

开始工作前核对服务端固定工具链：

```bash
go version
```

Go 校验：

```bash
cd backend
go test ./...
go vet ./...
```

本机开发环境：

```bash
brew services start postgresql@18
brew services start minio
brew services start redis
brew services start rabbitmq
(cd backend && go test -race -tags=services ./tests -count=1)
```

后端镜像使用 `docker build --tag then-server:local backend` 构建。Swagger 由 Go API 从 Huma operation 与类型标签实时生成，不需要独立 Swagger 容器或 `go generate`。确需隔离依赖时再使用 `docker compose --profile isolated-env up --detach --wait`；Compose 不是日常开发前置，也不代表生产部署已完成。

GitHub Actions 的 `Go quality`、`PostgreSQL integration` 与 `OCI container` 是每次 push/PR 的必要服务端门禁。需要本机 MinIO、Redis 和 RabbitMQ 的 `services` 测试仍按对应验收显式运行。

验证命令直接维护在 README/CI；不新增脚本入口。



## 协作

先读所属 Design、PRD 和切片 spec/checklist，再实现最小闭环并回写 Acceptance。当前技术路径是 SwiftUI 页面 + Three.js/GLB，不使用 Blender。文档原地更新，删除被替代内容，保留未完成门禁与必要证据；提交/推送按用户明确要求执行。
