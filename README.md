# then-server

`then-server` 是“于是”OOTD 的 Go 服务端与产品级契约仓库。按 2026-09-14 用户决定，本地统一放在 `Then/` 父目录下，服务端位于 `Then/then-server`，iOS 位于 `Then/then-app`；两个子目录是独立 Git 项目。本仓库继续拥有唯一 OpenAPI、数据库迁移、后端运行代码、可选开发依赖编排和 Design → PRD → Plan → Acceptance 文档。

## 当前状态

| 范围 | 状态 |
| --- | --- |
| OOTD 产品需求 | 10 号产品总纲与 11–19 号单功能 PRD 已批准或按功能门禁批准 |
| 产品与技术设计 | 01–12 号设计为当前基线；13 号保留三维研究及待 POC 参数，首版范围已回写 PRD 11/12 与 Design 04/05 |
| iOS 工程 | 位于独立 `then-app`；已切换今日/衣橱/穿搭簿三个 SwiftUI 入口，生成 Client 使用 ThenTransport |
| OOTD Feature | 已接无图/单件图衣橱、16-01 本地穿搭计划与日期回看；快照、编辑/取消/删除和含图组合已实现，验收继续。实际穿着、反馈、受控 AI 与真实 3D 分片推进 |
| Go 后端 | Gin 健康/数据库运行、容器与内嵌 Swagger 已实现；账号注册、Cookie 会话与本人账户 API 已通过单元、竞态及 PostgreSQL 集成测试 |
| Web 前端 | Design 16 与 17-13 草案已固定；按用户要求暂停实现，当前不创建 `frontend/` 代码目录 |
| 旧生活管理代码 | 用户确认仅开发阶段且无需保留旧数据，已成组退役；不建设历史兼容层 |

## 固定技术栈

- iOS：Xcode 26.6、Swift 6.3.3、最低 iOS 26、SwiftUI + Observation、Swift Concurrency；必要的高级动态效果可在获批 POC 后使用本地锁版 Three.js/WebKit renderer，产品页面仍全部由 SwiftUI 承担。
- 本地数据：GRDB 7.11.1 + SQLite；结构化数据本地优先，媒体保存在受保护的私有文件目录。
- API：REST + JSON、OpenAPI 3.1.2；Apple Swift OpenAPI Generator 生成 iOS Client。
- 后端：Go 1.26.5、Gin、GORM v2 Generics、PostgreSQL 18、Atlas versioned SQL。
- Web：规划使用 React 19.3.0、TypeScript 6.0.3、Vite 8.3.0；从唯一 OpenAPI 生成请求 SDK，当前尚未开工。
- 异步与媒体：RabbitMQ + Outbox/Inbox、受限 Redis、私有 S3-compatible 对象存储、受控 FFmpeg worker。
- 运行形态：一个 Go module、一个二进制与一个镜像，`APP_ROLE=api|worker|all`。

精确版本、分阶段启用边界和禁止项以 [`docs/design/01-技术选型.md`](docs/design/01-技术选型.md) 为唯一事实源；后端职责见 [`docs/design/02-后端架构.md`](docs/design/02-后端架构.md)。
后端 MVP 分期与 test 边界见 [`docs/design/14-后端MVP与测试边界设计.md`](docs/design/14-后端MVP与测试边界设计.md)。

## 产品结构

首版 iOS 使用三个一级入口：

- 今日：输入场景，查看并调整当天搭配。
- 衣橱：添加、确认、管理衣物与素材质量。
- 穿搭簿：保存计划、实际穿着、收藏与反馈。

数字形象、隐私、账号和数据删除从头像入口进入。静态 AI 试穿由用户主动触发；推荐、保存和反馈不依赖生成成功。动态预览只有通过质量、成本、隐私和性能门禁后才启用。

## 目录

- `design.md`：全仓库通用页面设计与交付规范，固定自指定上游提交；产品范围、技术栈和功能行为仍由 `AGENTS.md` 与 `docs/` 对应事实源定义。
- `backend/`：Go 后端、OpenAPI 唯一契约与 Atlas migration 目录。
- `frontend/`：待 17-13 批准后创建的注册、登录与本人账户 CRUD 响应式 Web 应用。
- `docker-compose.yml`：可选隔离环境的标准 Compose 入口。
- `docker-compose-env.yml`：可选的 PostgreSQL、MinIO、Redis、RabbitMQ 容器配置；日常开发使用本机已安装服务。
- `.env.example`：仅供可选隔离环境使用的配置格式。
- `docs/prd/`：产品需求与范围。
- `docs/design/`：一个功能一个 design，以及架构与隐私决策。
- `docs/plan/`：产品级实施计划，以及统一范围契约、任务与证据的单切片执行计划。
- `docs/acceptance/`：可执行验收标准与证据要求。

核心入口：

| 文件 | 用途 |
| --- | --- |
| [`AGENTS.md`](AGENTS.md) | 全仓库当前产品、架构、隐私、数据与测试规范 |
| [`design.md`](design.md) | 通用页面视觉、组件、交互与交付质量规范 |
| [`then-app`](https://github.com/StephenQiu30/then-app) | 独立 SwiftUI 客户端、GRDB 本地数据与 iOS 测试 |
| [`docs/README.md`](docs/README.md) | 当前文档索引与历史文档边界 |
| [`docs/prd/README.md`](docs/prd/README.md) | 产品总纲与 11–19 号单功能需求索引 |
| [`docs/plan/README.md`](docs/plan/README.md) | 产品级实施计划与单切片执行计划的准入、编号、状态和模板 |
| [`docs/plan/11-01-照片输入与质量门执行计划.md`](docs/plan/11-01-照片输入与质量门执行计划.md) | 首个统一契约、任务与证据的隔离 POC 执行计划 |
| [19-02 开发基线与旧实现清理](docs/plan/19-02-开发基线与旧实现清理执行计划.md) | 用户确认无旧数据保留后的清理与验收；原 19-01 已替代 |
| [`docs/acceptance/README.md`](docs/acceptance/README.md) | 10 号系统验收与 11–19 号单功能验收索引 |
| [`docs/design/12-OOTD技术债清理与迁移设计.md`](docs/design/12-OOTD技术债清理与迁移设计.md) | 当前开发基线、清理分组与未来数据保护 |
| [`backend/openapi.yaml`](backend/openapi.yaml) | iOS、Web 与 Go 共用的唯一接口契约 |
| [`docker-compose.yml`](docker-compose.yml) | 默认 Compose 入口，统一包含开发环境配置 |
| [`docker-compose-env.yml`](docker-compose-env.yml) | 固定镜像、回环端口、命名卷和健康检查的开发依赖 |

根目录 [`design.md`](design.md) 固定自 [StephenQiu30/video-server 提交 `3f95c3d`](https://github.com/StephenQiu30/video-server/blob/3f95c3de67a89ae8988724b0a51a436768ff75e9/design.md)。固定版本的 Git blob SHA 为 `65ca7f18fd4d24ae0b60f91ee582a34aefcda1a8`，SHA-256 为 `fb4ac9a5f11c56a2788ecd69acbaf9654f27f2b1d7a216892497e0480dddcc1d`；更新时必须重新固定上游提交并评审差异。

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

后端镜像使用 `docker build --tag then-server:local backend` 构建。Swagger 由 Go API 内嵌并读取同一份 OpenAPI，不需要独立 Swagger 容器。确需隔离依赖时再使用 `docker compose --profile isolated-env up --detach --wait`；Compose 不是日常开发前置，也不代表生产部署已完成。

GitHub Actions 的 `Go quality`、`PostgreSQL integration` 与 `OCI container` 是每次 push/PR 的必要服务端门禁。需要本机 MinIO、Redis 和 RabbitMQ 的 `services` 测试仍按对应验收显式运行。

旧生活管理脚本随 19-02 退役；历史验收只在 Git/旧文档中保留。

## 协作

开始贡献前阅读 [`AGENTS.md`](AGENTS.md) 与 [`CONTRIBUTING.md`](CONTRIBUTING.md)，检查工作区已有修改，并按 design → PRD → execution plan → implementation → acceptance 推进。切片排入近期产品计划后、编码前创建一份统一范围契约、任务和证据的 `FF-SS` 执行计划；不要恢复 `backend/schema.sql`，不要在生产使用 GORM `AutoMigrate`。涉及 iOS 时在 `then-app` 独立提交，并让 App CI 从本仓库唯一契约生成和编译请求代码。
