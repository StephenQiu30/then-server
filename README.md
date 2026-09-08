# 于是

“于是”正在从历史个人生活管理实现切换为一款仅面向 iOS 的 C 端 OOTD 穿搭产品。首版围绕可调 3D 模板角色、直接换装、真实衣橱、可解释推荐及实际穿着反馈形成离线闭环；SwiftUI 原生 Liquid Glass，最低 iOS 26。云端 AI 试穿和多设备同步分期启用，后端技术与编码规范预先固定。

## 当前状态

| 范围 | 状态 |
| --- | --- |
| OOTD 产品需求 | 10 号产品总纲与 11–19 号单功能 PRD 已批准或按功能门禁批准 |
| 产品与技术设计 | 01–12 号设计为当前基线；13 号保留三维研究及待 POC 参数，首版范围已回写 PRD 11/12 与 Design 04/05 |
| iOS 工程 | 现有工程可构建且已使用 SwiftUI + Observation；页面仍是旧生活管理实现，已冻结待成组迁移 |
| OOTD Feature | M0 已建立 `11-01 照片输入与质量门` 隔离 POC 和 `19-01 历史发布事实与数据盘点` 只读计划；两者均未开始产品功能实现 |
| Go 后端 | 技术与架构已固定，尚无运行时代码或业务 migration |
| 旧生活管理代码 | 仅作历史与数据迁移参考，不再扩展；真实用户数据策略确认前不零散删除 |

## 固定技术栈

- iOS：Xcode 26.6、Swift 6.3.3、最低 iOS 26、SwiftUI + Observation、Swift Concurrency；必要的高级动态效果可在获批 POC 后使用本地锁版 Three.js/WebKit renderer，产品页面仍全部由 SwiftUI 承担。
- 本地数据：GRDB 7.11.1 + SQLite；结构化数据本地优先，媒体保存在受保护的私有文件目录。
- API：REST + JSON、OpenAPI 3.1.2；Apple Swift OpenAPI Generator 生成 iOS Client。
- 后端：Go 1.26.5、Gin、GORM v2 Generics、PostgreSQL 18、Atlas versioned SQL。
- 异步与媒体：RabbitMQ + Outbox/Inbox、受限 Redis、私有 S3-compatible 对象存储、受控 FFmpeg worker。
- 运行形态：一个 Go module、一个二进制与一个镜像，`APP_ROLE=api|worker|all`。

精确版本、分阶段启用边界和禁止项以 [`docs/design/01-技术选型.md`](docs/design/01-技术选型.md) 为唯一事实源；后端职责见 [`docs/design/02-后端架构.md`](docs/design/02-后端架构.md)。

## 产品结构

首版 iOS 使用三个一级入口：

- 今日：输入场景，查看并调整当天搭配。
- 衣橱：添加、确认、管理衣物与素材质量。
- 穿搭簿：保存计划、实际穿着、收藏与反馈。

数字形象、隐私、账号和数据删除从头像入口进入。静态 AI 试穿由用户主动触发；推荐、保存和反馈不依赖生成成功。动态预览只有通过质量、成本、隐私和性能门禁后才启用。

## 目录

- `ios/`：SwiftUI 客户端、GRDB 数据层、系统能力适配与测试。
- `backend/`：Go 后端、OpenAPI 唯一契约与 Atlas migration 目录。
- `docs/prd/`：产品需求与范围。
- `docs/design/`：一个功能一个 design，以及架构与隐私决策。
- `docs/plan/`：产品级实施计划，以及统一范围契约、任务与证据的单切片执行计划。
- `docs/acceptance/`：可执行验收标准与证据要求。
- `scripts/`：工具链、提交和架构守卫。

核心入口：

| 文件 | 用途 |
| --- | --- |
| [`AGENTS.md`](AGENTS.md) | 全仓库当前产品、架构、隐私、数据与测试规范 |
| [`docs/README.md`](docs/README.md) | 当前文档索引与历史文档边界 |
| [`docs/prd/README.md`](docs/prd/README.md) | 产品总纲与 11–19 号单功能需求索引 |
| [`docs/plan/README.md`](docs/plan/README.md) | 产品级实施计划与单切片执行计划的准入、编号、状态和模板 |
| [`docs/plan/11-01-照片输入与质量门执行计划.md`](docs/plan/11-01-照片输入与质量门执行计划.md) | 首个统一契约、任务与证据的隔离 POC 执行计划 |
| [`docs/plan/19-01-历史发布事实与数据盘点执行计划.md`](docs/plan/19-01-历史发布事实与数据盘点执行计划.md) | 旧发布事实和历史数据类别的只读 Go/No-Go 计划 |
| [`docs/acceptance/README.md`](docs/acceptance/README.md) | 10 号系统验收与 11–19 号单功能验收索引 |
| [`docs/design/12-OOTD技术债清理与迁移设计.md`](docs/design/12-OOTD技术债清理与迁移设计.md) | 旧实现冻结、清理分组、数据决策与回滚 |
| [`backend/openapi.yaml`](backend/openapi.yaml) | iOS 与 Go 共用的唯一接口契约 |
| [`backend/migrations/README.md`](backend/migrations/README.md) | PostgreSQL schema 管理规则 |
| [`scripts/validate-ios-architecture.sh`](scripts/validate-ios-architecture.sh) | SwiftUI、Observation、GRDB 与工程设置守卫 |

## 本地校验

开始工作前：

```bash
scripts/verify-toolchain.sh
scripts/validate-ios-architecture.sh
```

iOS 通用模拟器构建：

```bash
xcodebuild \
  -project ios/ThenApp.xcodeproj \
  -scheme ThenApp \
  -configuration Debug \
  -destination 'generic/platform=iOS Simulator' \
  -skipPackagePluginValidation \
  build
```

Go 运行时代码建立后执行：

```bash
cd backend
go test ./...
go vet ./...
```

旧 `validate-p0-scope.sh` 及旧生活管理 motion/privacy/localization 结果仅用于历史实现，不能作为 OOTD 发布门槛。

## 协作

开始贡献前阅读 [`AGENTS.md`](AGENTS.md) 与 [`CONTRIBUTING.md`](CONTRIBUTING.md)，检查工作区已有修改，并按 PRD → design → execution plan → implementation → acceptance 推进。切片排入近期产品计划后、编码前创建一份统一范围契约、任务和证据的 `FF-SS` 执行计划；不要恢复 `backend/schema.sql`，不要在生产使用 GORM `AutoMigrate`，也不要为清理旧界面而零散破坏现有 GRDB migration 或测试夹具。
