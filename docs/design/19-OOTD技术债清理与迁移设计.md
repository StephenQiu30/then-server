# OOTD 技术债清理与迁移设计

## 状态

已批准，2026-08-30。关联 [`../prd/10-OOTD产品需求.md`](../prd/10-OOTD产品需求.md)、[`01-技术选型.md`](01-技术选型.md)、[`02-后端架构.md`](02-后端架构.md) 与 [`../plan/10-OOTD产品实施计划.md`](../plan/10-OOTD产品实施计划.md)。

本文只负责旧生活管理实现到 OOTD 产品基线的清理、冻结和迁移，不重复定义具体 OOTD 功能。详细历史仍由 Git 保存，不通过删除历史决策掩盖产品转向。

## 目标与约束

目标：

- 项目只保留一套当前产品事实源和一套当前技术基线。
- 明确固定 SwiftUI，不因旧页面存在而引入第二套 UI 架构。
- 删除可以证明无业务价值且没有数据风险的工程空壳和悬空引用。
- 将旧业务代码从“继续维护”切换为“冻结待成组迁移”。
- 在处理真实用户数据、GRDB migration、资源和测试前建立可回滚边界。

约束：

- 当前大量 iOS 源码、工程和测试在工作区中仍是未跟踪文件，不能把“未提交”误判为“冗余”。
- 旧 Swift 文件全部仍被 Xcode 工程引用，没有可以单文件安全删除的孤立生产源码。
- 是否存在已发布版本与真实本地数据尚未确认；在该决策关闭前不得修改或删除既有 migration identifier。
- 本轮固定架构与治理，不伪造未使用的 Go 依赖、业务表、空目录或 OOTD 功能完成状态。

## 债务分类

| 类别 | 现状 | 决策 |
| --- | --- | --- |
| 产品事实源 | 旧生活管理 PRD 与 OOTD 设计同时被描述为当前 | PRD 10 成为当前需求；旧 PRD 03 与 03–09 号设计仅作历史参考 |
| 技术事实源 | chi/pgx/schema.sql/no MQ 与 Gin/GORM/Atlas/RabbitMQ 并存 | 01 与 02 号设计统一为唯一当前基线 |
| iOS UI | 工程已使用 SwiftUI，但规范只说“主要使用” | 固定所有产品页面使用 SwiftUI + Observation；UIKit 仅系统桥接 |
| Xcode 工程 | 存在 SDK 绝对路径 framework 与悬空 plist 引用 | 直接删除无业务语义的引用，并用构建验证 |
| 服务端 schema | 空 `backend/schema.sql` 仍被称为事实源 | 删除空壳，改为 Atlas versioned migration 目录 |
| 旧业务 Feature | Ledger、Calendar、Travel、Life、Today、Profile 仍组成五 Tab | 冻结；数据策略批准后按完整垂直切片成组移除 |
| App 依赖容器 | `AppEnvironment` 聚合二十余项依赖，多页面接收完整容器 | OOTD Feature 只注入精确依赖；旧容器随旧 Feature 迁移收缩 |
| 启动路径 | 数据库与 Keychain 同步初始化位于 App 初始化 | OOTD shell 落地时改为可观测 bootstrap 状态与可恢复失败页 |
| 资源与发布 | 旧权限、本地化、Info.plist 和测试仍围绕旧业务，缺少正式 App Icon | 新 OOTD 资源建立后再成组移除旧声明；App Icon 是发布门槛 |
| 校验脚本 | 旧 P0 scope/motion/privacy/localization 门禁会阻止或误判 OOTD | 退出当前发布门槛；先启用 SwiftUI 架构守卫，再按 OOTD 数据流重写专用门禁 |

## 已完成的安全清理

以下项目没有用户数据语义，可以在本轮直接清理：

1. 从 Xcode 工程移除硬编码到特定 `iPhoneOS26.0.sdk` 的 `Foundation.framework` 引用及只承载该引用的空分组。系统 framework 由 SDK 和 linker 正常解析，不需要绝对路径引用。
2. 移除错误指向 `ThenApp/App/Info.plist` 的悬空 PBX 文件引用；构建继续使用实际 `ThenApp/Info.plist`。
3. 删除没有领域 DDL 的 `backend/schema.sql` 空壳，创建 `backend/migrations/README.md` 固定 Atlas versioned SQL 规则。
4. 将根规范、README、后端 README 和 OpenAPI 元信息切换为 OOTD。
5. 新增 `scripts/validate-ios-architecture.sh`，阻断替代 UI/状态/持久化框架、工程设置漂移和越界 UIKit 页面。

这些清理的回滚只需要恢复工程引用或空壳文件，但没有这样做的技术理由；如果旧分支仍依赖它们，应在旧分支独立维护，不把过时基线带回当前分支。

## 暂缓删除的历史实现

### iOS Feature 与服务

以下内容必须作为一个迁移集合处理，而不是按文件名猜测冗余：

- `Features/Ledger`、`Calendar`、`Travel`、`Life`、`Today`、`Profile` 及其 RootView 导航。
- 对应 Domain model、Use Case、Repository 协议和 GRDB Record。
- EventKit、Core Location、MapKit、OCR、导航、旧通知与导出服务。
- `Localizable.xcstrings`、`InfoPlist.xcstrings`、Info.plist 权限和 background mode 中只服务旧功能的条目。
- 对应 Swift Testing、XCUITest、启动参数和测试 fixture。

处理前必须生成引用清单和数据清单。一个 Feature 只有在页面、领域、数据、服务、权限、资源和测试都没有剩余消费者时才算移除完成。

### GRDB 数据

既有八个 migration 当前视为不可变历史。必须先选择以下产品策略之一：

| 场景 | 处理策略 |
| --- | --- |
| 从未对真实用户发布 | 在保留工作区快照后，可为 OOTD 创建全新数据库 baseline；旧 migration 与代码在同一变更中删除 |
| 已发布且旧数据仍需访问 | 保留旧表只读能力，提供结构化导出；OOTD 使用新增表与版本，不复用旧业务表 |
| 已发布但产品决定清空 | 先获得明确用户同意、发布迁移说明和导出路径，再执行带证据的清理 migration |
| 无法确认是否发布 | 按已发布处理，不删除、不改写、不自动重建数据库 |

不得通过删除 App、换 Bundle ID、`eraseDatabaseOnSchemaChange` 或启动时重建来绕过数据决策。

## 目标 iOS 迁移形态

迁移完成后的根结构只保留三个一级 Tab：今日、衣橱、穿搭簿。数字形象、账号、隐私与删除从头像入口进入。

```text
ThenApp
├── App
│   ├── ThenApp.swift
│   ├── AppBootstrap.swift
│   └── RootView.swift
├── Features
│   ├── Today
│   ├── Wardrobe
│   ├── OutfitBook
│   ├── Avatar
│   ├── Recommendation
│   └── TryOn
├── Core
├── Data
└── Services
```

约束：

- Root shell 先落地空状态和离线手工路径，再接照片、推荐和生成。
- `AppBootstrap` 显式表达 `starting`、`ready`、`recoverableFailure`，不在 `ThenApp.init` 同步执行可能阻塞首屏的数据库和 Keychain 工作。
- 每个 ViewModel 只接收当前 Feature 的 Use Case 或 Repository，不接收完整 AppEnvironment。
- OOTD 数据表与媒体目录先通过 design 与 migration 测试，再接页面；不得让 View 创建 schema 或决定文件保留。

## 目标后端迁移形态

- 当前没有 Go 运行时代码，因此不为了表示“选过技术”向 `go.mod` 添加未使用依赖。
- 第一项后端垂直切片从配置、健康检查、数据库、Atlas baseline、认证上下文和错误模型开始。
- 只有首个云端生成 POC 获批时才实际配置 RabbitMQ、Redis、对象存储和 Provider；版本仍按 01 号设计固定。
- PostgreSQL migration 建立后，应用和 GORM model 不得另行声明或自动修改 schema。
- API/worker 使用同一 module、main 和镜像；生产进程分角色不等于拆微服务。

## 分阶段清理顺序

### 阶段 A：事实源收口

- PRD 10、01/02 与 10–19 号设计进入当前索引。
- 旧 PRD、03–09 号设计、旧计划与验收增加历史状态提示并退出当前索引。
- 根规范、README、iOS/backend README 与 OpenAPI 元信息一致。

### 阶段 B：建立 OOTD shell

- 新 RootView 使用三个 Tab，并完成 SwiftUI 空状态、无障碍和本地化。
- 建立精确依赖 bootstrap，不复用旧巨型环境容器承载新功能。
- 新增 OOTD GRDB baseline 或增量 migration；取决于旧数据决策。

### 阶段 C：垂直切片替换

按“数字形象与采集 → 衣橱 → 推荐 → 记录反馈 → 静态试穿 → 条件动态预览”的顺序实现。每完成一个切片，先证明没有旧依赖再清除对应历史代码。

### 阶段 D：历史数据与资源收尾

- 执行已批准的保留、导出或删除策略。
- 移除旧 Feature、服务、权限、字符串、测试和脚本。
- 清理 AppEnvironment、同步启动、超大文件与缺失 App Icon 等工程债。
- 在干净克隆、历史数据库升级样本和真机上完成发布验收。

## 失败与回滚

- OOTD shell 发布前始终保留上一可运行提交或 release tag；不在同一发布中同时执行不可逆数据删除和大规模 UI 替换。
- schema 变更采用追加与 expand/contract；App 降级时旧版本仍能忽略新表或新字段。
- 媒体目录迁移先复制并校验 manifest，再切换引用，最后在保留窗口后清理旧副本。
- 如果新启动流程失败，显示无敏感数据的恢复页并保留数据库；不得自动清空数据。
- 供应商、队列或对象存储失败不回滚用户已确认的衣橱与搭配，只使云端增强降级。

## 验证策略

- 静态：根文档索引无双事实源，活动文档不再把旧产品称为当前；`git diff --check` 通过。
- iOS：架构守卫、`plutil`、Swift Package 锁、通用模拟器 clean build 通过。
- 数据：历史每个 GRDB schema fixture 能升级或按已批准策略导出；没有未经同意的删除。
- 后端：Atlas checksum、空库迁移、真实 PostgreSQL 升级和 drift 检查通过；GORM `AutoMigrate` 不存在。
- 端到端：三个 OOTD Tab、无照片路径、离线路径、照片权限撤回、生成失败和删除链具有可重复验收证据。

## 明确非目标

- 本轮不实现 OOTD 页面、数据库表、后端运行时或第三方 AI 集成。
- 本轮不删除整套旧 iOS 业务代码或重写既有 GRDB migration。
- 本轮不选择具体云厂商和 AI Provider。
- 本轮不把历史自动化结果冒充为 OOTD 发布证据。
