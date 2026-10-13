# wearlog-server

“衣序”OOTD 的 Go 服务端与 Web 仓库，位于 `Wearlog/wearlog-server`；客户端是同级独立 [wearlog-app](../wearlog-app/README.md)。

## 当前产品设计

当前先明确产品需求：以人物展示、选衣试穿、前后对比为核心，连接真实衣橱、保存搭配与实穿记录。研究草案、已确认方向及待验证项见 [需求入口](docs/requirement/README.md)，工程规范见 [PROJECT.md](PROJECT.md)。现有二维能力与固定示例试衣间不代表真实照片试穿或连续环绕已完成；证据见 [文档入口](docs/README.md)。

账号、公共后端、社区、同步和三人物六套内容不作个人首版前置。新存储/API 方案尚未迁移；已有工程维护仍按原代码合同与证据认领。

## 当前实现

Go + Gin/Huma + GORM/AutoMigrate + PostgreSQL 已实现健康检查、注册/登录、Cookie 会话、本人账户 CRUD、本人成年声明、结构化衣橱、账号穿搭计划、实际穿着、私人日记、服务端增量同步，以及社区帖子审核治理。运行时 OpenAPI 是 Swagger 与 Umi 请求生成的唯一输入。目录与规范见 [后端 AGENTS](backend/AGENTS.md)，精确版本以依赖锁文件为准。

日常使用本机已安装 PostgreSQL、Redis、Kafka、MinIO；当前 API 以 PostgreSQL 保存账号、会话、声明、结构化衣橱、计划、实际穿着、私人日记、增量变更、社区审核治理和媒体事实，Redis 只保存认证限流短期计数。Kafka 与 MinIO 已用于显式开启的私有图片开发闭环，默认仍关闭。`frontend` 已建立 Next.js App Router/TypeScript、shadcn/ui + Radix 基础组件、统一设计 token 与运行时 OpenAPI 生成客户端，已有账户、衣橱、计划、实际及推荐页面；实际能力以当前代码和本次验证为准。App 云接入与主动同步未完成，后端 fixture 不代表真实生成通过。

## 目录与入口

| 位置                                                                                        | 内容                                                                |
| ------------------------------------------------------------------------------------------- | ------------------------------------------------------------------- |
| [PROJECT.md](PROJECT.md)                                                                    | 产品边界、shadcn/ui + Radix 前端设计实现规范、前后端目录与验收要求  |
| [DESIGN.md](DESIGN.md)                                                                      | App 与 Web 唯一视觉和交互设计标准；与 `wearlog-app/DESIGN.md` 保持一致 |
| [backend](backend/README.md)                                                                | Go 运行说明、内嵌 Swagger、schema 和独立测试                        |
| [frontend](frontend/README.md)                                                              | Next.js App Router Web 基础工程、Umi OpenAPI 与 Axios 请求层        |
| [BACKLOG.md](BACKLOG.md)                                                                    | 全局任务顺序、依赖、checklist 与验收出口                            |
| [HANDOVER.md](HANDOVER.md)                                                                  | 当前接手入口与实施规则                                              |
| [CHANGELOG.md](CHANGELOG.md)                                                                | 按日期记录的简短改动摘要                                            |
| [docker-compose.yml](docker-compose.yml) / [docker-compose-env.yml](docker-compose-env.yml) | 明确需要隔离环境时使用，非日常默认启动                              |
| [AGENTS.md](AGENTS.md) / [CONTRIBUTING.md](CONTRIBUTING.md)                                 | 开发和提交规范                                                      |

不维护手写或服务端物化的 OpenAPI 文件、Atlas 迁移账本或旧项目兼容目录。前端生成客户端只由运行时 `/openapi.json` 刷新。当前业务未进入实现的模块不建空壳。

## 本地校验

二维衣橱、平铺单件替换和计划/实穿统计的当前实现与实测见 [二维最小交付](docs/validation/25-二维衣橱与搭配最小交付.md)。支持照片上传、补换图、本地抠图和简单裁剪，原文件与透明展示图分别保留，实测及限制见 [本地抠图与裁剪](docs/validation/27-本地抠图与简单裁剪最小交付.md)。

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
brew services start kafka
(cd backend && go test -race -tags=services ./tests/... -count=1)
```

后端镜像使用 `docker build --tag wearlog-server:local backend` 构建。Swagger 由 Go API 从 Huma operation 与类型标签实时生成，不需要独立 Swagger 容器或 `go generate`。确需隔离依赖时再使用 `docker compose --profile isolated-env up --detach --wait`；Compose 不是日常开发前置，也不代表生产部署已完成。

前端校验：

```bash
cd frontend
npm ci
npm run lint
npm run test
npm run typecheck
npm run format:check
npm run build
npm run test:e2e
```

## CI 功能门禁

每次 push/PR 都运行真实行为回归：

- `Go quality` 执行 race-enabled 单元/架构测试，以及连接隔离 PostgreSQL、Redis、MinIO、Kafka 的 `services` 套件。
- `PostgreSQL integration` 构建并启动实际后端进程，验证真实依赖上的注册、媒体和任务路径；`OCI container` 则验证构建后的生产镜像在受限运行条件下仍能启动和处理请求。
- `Frontend quality` 执行单元/API 合同测试，并在生产构建上用 Chromium 操作注册流程，覆盖首页导航、字段校验、提交请求和账户页状态；浏览器测试用合成 API 响应隔离前端行为，真实后端结果由服务/进程集成套件负责。
- gofmt、ESLint、Prettier、类型检查、构建和生产依赖审计保留为辅助工程门禁，不能替代上面的运行时行为测试。

CI 使用短期隔离服务和合成账号/素材。MinIO 测试镜像由工作流从固定上游源码提交构建，避免依赖当前拒绝匿名拉取的 Quay 镜像；服务行为仍由同一版本的 MinIO 实际执行。

验证命令直接维护在 README/CI；不新增脚本入口。

## 协作

先核对当前需求、PROJECT、DESIGN 与相关代码，再实现最小闭环并验证；准确说明尚未完成的真实供应商、费用和设备验证。提交与推送按用户明确要求执行。
