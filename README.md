# then-server

“于是”OOTD 的 Go 服务端与中央产品文档仓库，位于 `Then/then-server`；客户端是同级独立 [then-app](../then-app/README.md)。

## 当前实现

Go + Gin/Huma + GORM/AutoMigrate + PostgreSQL 已实现健康检查、注册/登录、Cookie 会话、本人账户 CRUD、本人成年声明、结构化衣橱、账号穿搭计划、实际穿着、私人日记，以及社区帖子审核治理。运行时 OpenAPI 是 Swagger 与 Umi 请求生成的唯一输入。目录与规范见 [后端架构](docs/design/02-后端架构.md)，精确版本见 [技术选型](docs/design/01-技术选型.md)。

日常使用本机已安装 PostgreSQL、Redis、RabbitMQ、MinIO；当前 API 以 PostgreSQL 保存账号、会话、声明、结构化衣橱、计划、实际穿着、私人日记、社区审核治理和媒体事实，Redis 只保存认证限流短期计数。RabbitMQ 与 MinIO 已用于显式开启的私有图片开发闭环，默认仍关闭。`frontend` 已建立 Next.js App Router/TypeScript 基础工程与运行时 OpenAPI 生成客户端，业务页面尚未实现；App 云接入、生成和同步未完成。

## 目录与入口

| 位置 | 内容 |
| --- | --- |
| [PROJECT.md](PROJECT.md) | 产品边界、shadcn/ui + Radix 前端设计实现规范、前后端目录与验收要求 |
| [DESIGN.md](DESIGN.md) | App 与 Web 唯一视觉和交互设计标准；与 `then-app/DESIGN.md` 保持一致 |
| [backend](backend/README.md) | Go 运行说明、内嵌 Swagger、schema 和独立测试 |
| [frontend](frontend/README.md) | Next.js App Router Web 基础工程、Umi OpenAPI 与 Axios 请求层 |
| [docs](docs/README.md) | 当前 Design → PRD → Plan → Acceptance |
| [产品计划](docs/plan/10-OOTD产品实施计划.md) | 全部切片的当前状态、缺口与下一步 |
| [系统验收](docs/acceptance/10-OOTD产品系统验收.md) | 局部证据与完整产品的验收边界 |
| [docker-compose.yml](docker-compose.yml) / [docker-compose-env.yml](docker-compose-env.yml) | 明确需要隔离环境时使用，非日常默认启动 |
| [AGENTS.md](AGENTS.md) / [CONTRIBUTING.md](CONTRIBUTING.md) | 开发和提交规范 |

不维护手写或服务端物化的 OpenAPI 文件、Atlas 迁移账本或旧项目兼容目录。前端生成客户端只由运行时 `/openapi.json` 刷新。当前业务未进入实现的模块不建空壳。静态内部素材见 [assets](frontend/assets/README.md)。

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
(cd backend && go test -race -tags=services ./tests/... -count=1)
```

后端镜像使用 `docker build --tag then-server:local backend` 构建。Swagger 由 Go API 从 Huma operation 与类型标签实时生成，不需要独立 Swagger 容器或 `go generate`。确需隔离依赖时再使用 `docker compose --profile isolated-env up --detach --wait`；Compose 不是日常开发前置，也不代表生产部署已完成。

前端校验：

```bash
cd frontend
npm install
npm run lint
npm run test
npm run typecheck
npm run format:check
npm run build
```

GitHub Actions 的 `Go quality`、`PostgreSQL integration` 与 `OCI container` 是每次 push/PR 的必要服务端门禁。需要本机 MinIO、Redis 和 RabbitMQ 的 `services` 测试仍按对应验收显式运行。

验证命令直接维护在 README/CI；不新增脚本入口。



## 协作

先读所属 Design、PRD 和切片 spec/checklist，再实现最小闭环并回写 Acceptance。当前技术路径是 SwiftUI 页面 + Three.js/GLB，不使用 Blender。文档原地更新，删除被替代内容，保留未完成门禁与必要证据；提交/推送按用户明确要求执行。
