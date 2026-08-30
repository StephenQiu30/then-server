# 于是 OOTD 项目协作规范

## 适用范围

本文件位于仓库根目录，规则适用于仓库内全部文件。若子目录新增更具体的 `AGENTS.md`，可以补充本文件，但不得降低安全、隐私、数据正确性、可访问性和测试要求。

## 产品与技术方向

“于是”当前是一款仅面向 iOS 的 C 端 OOTD 穿搭产品。仓库名为 `then`，用户可见名称暂时保留“于是”，Xcode target 与 Swift module 使用 `ThenApp`。

当前产品围绕以下闭环建设：

- 通过一张本人 OOTD 照或手工方式建立数字形象与首批衣物。
- 以低录入成本逐步形成个人数字衣橱。
- 基于真实拥有且当前可穿的衣物生成结构化、可解释、可局部调整的推荐。
- 用户主动发起静态 AI 试穿；动态预览必须通过独立质量、成本、隐私和性能门禁。
- 保存计划与实际穿搭，并用穿后反馈持续改善推荐。

当前产品需求事实源是 `docs/prd/10-OOTD产品需求.md`。旧记账、日历和出行 PRD、03–09 号设计、旧计划、旧验收及相应 iOS 实现只用于解释历史代码和数据；不得继续扩展，也不得在没有数据保留决策时零散删除。

固定技术方向：

- iOS：Xcode 26.6、Swift 6.3.3、最低 iOS 18，所有产品页面使用 SwiftUI + Observation。
- 客户端数据：GRDB 7.11.1 + 系统 SQLite，本地优先、离线可用；媒体字节使用受保护文件，不存 SQLite BLOB。
- API：REST + JSON，以 `backend/openapi.yaml` 的 OpenAPI 3.1.2 为唯一契约。
- 后端：Go 1.26.5、Gin、GORM v2 Generics、PostgreSQL 18、Atlas versioned SQL。
- 异步与媒体：RabbitMQ + PostgreSQL Outbox/Inbox、受限 Redis、私有 S3-compatible 对象存储、受控 FFmpeg worker。
- 运行形态：一个 Go module、一个二进制、一个 OCI 镜像；通过 `APP_ROLE=api|worker|all` 选择角色，生产 API 与 worker 可独立进程部署。

精确版本、启用阶段、禁止项和重新评估条件以 `docs/design/01-技术选型.md` 为唯一事实源。Feature 不得自行引入同类替代框架。

## 目标仓库结构

以下结构随实施计划逐步落地，不表示所有条目当前都已存在。不要为了填满结构创建空文件或空目录。

```text
.
├── AGENTS.md
├── README.md
├── CONTRIBUTING.md
├── scripts/
│   ├── validate-commit-message.sh
│   ├── validate-ios-architecture.sh
│   └── verify-toolchain.sh
├── ios/
│   ├── README.md
│   ├── ThenApp/
│   │   ├── openapi.yaml -> ../../backend/openapi.yaml
│   │   ├── openapi-generator-config.yaml
│   │   ├── Localizable.xcstrings
│   │   ├── InfoPlist.xcstrings
│   │   ├── App/
│   │   ├── Core/
│   │   ├── Features/
│   │   ├── Services/
│   │   └── Data/
│   ├── ThenAppTests/
│   └── ThenAppUITests/
├── backend/
│   ├── README.md
│   ├── go.mod
│   ├── go.sum
│   ├── main.go
│   ├── openapi.yaml
│   ├── atlas.hcl
│   ├── migrations/
│   ├── internal/
│   │   ├── model/
│   │   ├── service/
│   │   ├── repository/
│   │   ├── transport/
│   │   ├── worker/
│   │   └── platform/
│   └── tests/
└── docs/
    ├── prd/
    ├── design/
    ├── plan/
    └── acceptance/
```

`backend/migrations/*.sql` 与 `atlas.sum` 是 PostgreSQL schema 的唯一事实源；不得恢复第二份 `schema.sql`。固定文件名与详细文档目录规则见下文。

## 信息源优先级

出现冲突时按以下顺序判断：

1. 用户当前明确要求。
2. 根目录及当前目录链上的 `AGENTS.md`。
3. `docs/prd/10-OOTD产品需求.md`。
4. `docs/design/01-技术选型.md`。
5. `docs/design/02-后端架构.md` 与 10–19 号 OOTD 设计。
6. `backend/openapi.yaml`。
7. 当前 OOTD 实施计划与验收文档。
8. 现有代码和测试体现的行为。

旧生活管理文档和代码不具有当前产品行为的优先级。发现当前事实源之间冲突时不得静默选择；会改变产品行为、数据模型、隐私承诺或兼容性的冲突必须先记录并请求确认。

## 开始任务前

1. 阅读本文件和任务涉及目录的说明文件。
2. 运行 `scripts/verify-toolchain.sh`；版本不一致时停止，不使用未批准的替代工具链。
3. 阅读对应 PRD、设计、实施计划和验收标准。
4. 检查工作区已有修改，不覆盖或回滚无关改动。
5. 确认改动是否影响 iOS、后端、OpenAPI、本地 migration、服务端 migration、媒体生命周期和文档。
6. 对照片采集、Vision 质量门、AI Provider、对象存储、队列恢复、删除链和最低设备性能先做隔离 POC。

## Git 提交规范

- 准备进入 `main` 的提交标题和 Pull Request 标题使用 `type(scope): subject`；scope 必填，冒号后保留一个半角空格。
- type 只允许 `feat`、`fix`、`docs`、`refactor`、`perf`、`test`、`build`、`ci`、`chore`、`style` 和 `revert`。
- scope 使用小写英文、数字和连字符；优先使用 `ios`、`backend`、`openapi`、`db`、`docs`、`repo`、`ci`、`deps`、`security` 或明确业务域。
- 每个提交只包含一个可独立说明和回滚的变化；破坏性变更在页脚使用 `BREAKING CHANGE:`，说明兼容、迁移与回滚。
- 完整规则以 `CONTRIBUTING.md` 为准；首次克隆后运行 `git config --local core.hooksPath .githooks`。

## iOS 开发规范

### SwiftUI 架构

- 所有产品页面、导航、Tab、sheet、表单和状态展示固定使用 SwiftUI；App 生命周期使用 SwiftUI `App`。
- UIKit 只允许通过 `UIViewRepresentable`、`UIViewControllerRepresentable` 或服务适配器封装缺少合适 SwiftUI 接口的系统控制器。UIKit 不承担产品页面、全局导航、领域状态或业务规则。
- 界面状态使用 Observation 与 `@Observable`；不新增 `ObservableObject`、`@Published`、`@StateObject` 或 Combine 全局状态流。
- 首版只使用一个生产 `ThenApp` Swift module，加 `ThenAppTests` 与 `ThenAppUITests`；Feature 先按目录和协议隔离。
- 采用 feature-first + MVVM + Repository。View 只负责展示和用户事件，不直接访问 GRDB、文件系统、Photos、Vision、网络或供应商 SDK。
- ViewModel 通过初始化器接收完成当前用例所需的精确依赖，不新增包含全 App 服务的巨型环境对象或隐藏全局单例。
- 系统能力通过协议封装，例如 `PhotoPickerService`、`CameraService`、`ImageAnalysisService`、`MediaStore`、`RecommendationService`、`TryOnService` 和 `NotificationService`。

现有 Ledger、Calendar、Travel、Life、Today 与 Profile 目录属于历史实现。数据保留策略批准前冻结，不在其中增加 OOTD 功能；迁移时按 Feature、数据库 migration、资源与测试一起成组处理。

### Swift 代码

- 使用 Swift Concurrency 与结构化并发；UI 状态更新明确运行在 Main Actor。
- App/UI target 使用 Swift 6 language mode、Complete Strict Concurrency、Approachable Concurrency 与 Main Actor 默认隔离。
- 不启动没有所有者、无法取消或依赖 View 生命周期保存结果的 `Task`。
- 禁止业务代码使用 `try!`、强制解包和无说明的 `fatalError`；错误映射为可测试领域错误并提供恢复路径。
- 优先使用值类型与不可变状态；共享可变状态必须有 actor 或明确隔离策略。
- 用户可见文本进入 String Catalog，不在 View 中散落不可本地化文案。
- 使用语义颜色、Dynamic Type、VoiceOver 标签、足够点击区域和系统控件；自定义动效必须支持 Reduce Motion 静态降级。

### 本地数据与媒体

- 衣橱浏览、手工编辑、基础推荐、穿搭计划、实际穿着与反馈在无网络时可用。
- 本地数据库固定使用 GRDB + 系统 SQLite，以 `DatabasePool`、WAL、显式事务和集中 `DatabaseMigrator` 管理；不得混用 SwiftData、Core Data 或 Realm。
- 已发布 migration identifier 不修改；结构修正追加新 migration，并测试历史 schema 与历史数据升级。
- GRDB Record 只存在于 Data 层；View、ViewModel 和领域层不得依赖 GRDB 类型。
- 媒体文件不存 SQLite BLOB。数据库只保存稳定资产 ID、相对路径、用途、质量、版本、哈希、血缘和生命周期状态。
- 人物原图、净化图、衣物图、试穿结果、动态帧和缩略图分用途管理；删除由资产血缘驱动，不依赖页面生命周期。
- 敏感令牌存 Keychain；本地数据库与媒体使用合适 Data Protection，不存入 `UserDefaults`、源码或日志。
- 优先使用 `PhotosPicker` 获得用户主动选择的图片；定制相机才使用 AVFoundation。上传前移除 EXIF、GPS、原文件名和无关区域。

## Go 后端规范

### 架构与运行形态

- 后端只使用一个 `go.mod`、一个 `main.go`、一个二进制与一个 OCI 镜像，不创建独立 module 或微服务仓库。
- 同一二进制支持 `APP_ROLE=api|worker|all`。本地和集成测试可用 `all`；生产默认用同一镜像分别运行 API 与 worker。
- 首版不引入微服务、Kubernetes、Kafka、服务网格、分布式事务或提前分库分表。
- HTTP 固定使用 Gin。使用 `gin.New()` 并显式注册 recovery、追踪、日志、限流、认证、幂等、授权和 OpenAPI 校验等中间件。
- Handler 只处理协议转换、认证上下文和响应；业务规则位于 Service；数据库访问位于 Repository；消息消费位于 worker。
- 领域对象是纯 Go struct，不依赖 `gin.Context`、`http.Request`、`gorm.DB`、AMQP Delivery、数据库连接或供应商 DTO。
- 所有 I/O、数据库、队列和供应商调用传递 `context.Context`；所有后台循环支持取消、超时和优雅退出。

### GORM 与 SQL

- GORM v2 Generics 用于 CRUD、简单关联、稳定过滤和普通事务写入；Repository 是唯一数据访问边界。
- Service 不直接调用 GORM；GORM model 不进入 transport、service 或领域层。
- 复杂推荐、CTE、窗口函数、Outbox 领取和执行计划敏感查询使用 Repository 内的命名、参数化 raw SQL；必要时使用 pgx 专有能力。
- 禁止在共享开发、测试、预发或生产调用 `AutoMigrate`。禁止无界 `Preload`、隐式 association cascade、传统 `Save`、字符串拼接排序和隐藏跨聚合副作用的 hook。
- 一个业务动作涉及多表、幂等结果、配额和 Outbox 时必须使用同一个 PostgreSQL 事务。

### 迁移、队列与媒体

- `backend/migrations/*.sql` 与 `atlas.sum` 是服务端 schema 的唯一事实源；应用身份没有生产 DDL 权限。
- migration 必须评审锁级别、表重写、索引、回填、兼容窗口、恢复点和回滚；破坏性变更采用 expand/contract。
- 第一个云端 AI 生成能力上线时使用 RabbitMQ durable quorum queue。业务事务先写 Outbox，由 relay 使用 publisher confirm 发布。
- consumer 使用 Inbox、数据库租约、fencing token 与幂等 Provider key。系统承诺至少一次投递与业务效果幂等，不宣称跨系统恰好一次。
- Redis 仅用于短 TTL 缓存、限流、SSE 状态通知和可重建协调数据；不得作为账号、任务、配额、同意、删除或媒体状态事实源，也不得代替 RabbitMQ。
- 私有 S3-compatible 对象存储保存媒体字节；消息不得携带图片、签名 URL、令牌或敏感正文。
- FFmpeg 只运行版本固定、资源受限的参数模板，不接受用户或供应商文本拼接命令。
- 日志使用结构化 `slog`，trace/metrics 使用 OpenTelemetry；禁止记录令牌、人物或衣物图片 URL、用户提示词、供应商正文和可还原个人习惯的敏感内容。

## API 契约规范

- `backend/openapi.yaml` 是 iOS、Go 后端和 Swagger UI 的唯一接口契约。不得复制第二份 YAML/JSON、使用 Swagger 注解生成契约或手写 iOS transport DTO。
- 契约固定 OpenAPI 3.1.2；公开业务接口使用 `/v1`。每个 operation 必须有全局唯一、稳定、可读的 `operationId`。
- 修改顺序：先改 OpenAPI 并校验，再生成并编译 iOS Client，手写 Go Handler 与纯 struct，最后更新契约测试和示例。
- iOS 生成代码只存在 DerivedData；`ios/ThenApp/openapi.yaml` 必须保持指向 `backend/openapi.yaml` 的符号链接。
- 请求与响应 schema 明确 required、可空性、枚举、格式、单位和示例；不得用无约束 object 代替稳定结构。
- 创建、上传 finalize、生成、取消、删除、同步和第三方回调支持幂等键；列表优先使用稳定游标。
- 长任务返回 `202 Accepted`、稳定 job ID、状态 URL 和建议轮询间隔。
- 错误响应包含稳定错误码、用户安全信息、`request_id` 与可重试标志，不泄露堆栈和供应商正文。
- Swagger UI 生产默认关闭；确需开启时必须经过认证和网络限制，并关闭持久化授权信息。

## OOTD 领域数据规范

- 用户、衣物、搭配、穿着记录、媒体资产、生成任务和同意记录使用全局唯一 ID；不得以文件名、图片哈希、Photos identifier 或本地自增 ID 作为跨端业务标识。
- 服务端时间保存为 UTC，展示语义保留原始时区；排序与同步不只依赖设备时间。
- 衣物当前状态与历史搭配快照分离；归档、待洗或借出不能改写已经保存的历史穿搭。
- AI 识别属性默认是建议；用户确认值与来源、置信度、模型版本分开保存。
- 推荐先执行硬约束再排序，不为凑足结果放宽用户明确要求，也不创建用户未拥有的衣物。
- 推荐、静态试穿、动态预览和真实穿着是不同实体；视觉生成结果不得当作尺码、面料、体型或真实穿着事实。
- 媒体资产保存 owner、用途、版本、派生自、保留期和删除状态。删除源资产时必须遍历派生结果、缓存、供应商副本和后续清理任务。
- 所有服务端查询从认证上下文确定 owner，并显式限定 `user_id`；客户端不得声明可信 owner。

## 安全与隐私规范

- 人物照片、衣物照片、生成结果、穿着规律、认证信息和精确上下文按敏感数据处理，遵循最小收集、最短保留、可解释、可撤回和可删除原则。
- 只允许年满 18 周岁的用户上传本人照片；不接受他人、名人、未成年人或来源不明的人物照片。
- 本地处理、单次云生成、多设备同步、产品分析和模型训练是不同目的，必须分别评审与同意；生产数据固定不得用于训练。
- 云端上传前说明用途、处理方、地域、保留期、删除方式和不上传的替代路径。取消任务不等于供应商已停止，产品状态必须如实表达。
- 传输使用 TLS；数据库、对象存储和备份启用静态加密；高敏感字段使用 KMS 管理的字段级或信封加密。
- 授权必须在服务端执行；生产日志、分析事件、trace 和通知载荷不得包含敏感正文或可访问资产 URL。
- 删除覆盖主数据、派生资产、对象版本、缓存、队列、供应商副本和备份窗口，并提供可审计结果。
- 不使用真实用户照片、穿着记录、令牌或生产数据作为测试夹具。

## 文档规范

### 目录职责

- `docs/prd/`：背景、目标用户、问题、目标、非目标、范围、用户故事、业务规则、指标、依赖与风险。
- `docs/design/`：信息架构、用户流程、状态、数据模型、API、架构决策、隐私与失败降级。一个主要功能一个 design。
- `docs/plan/`：阶段、任务、依赖、风险、验证和状态。状态只使用 `pending`、`in_progress`、`completed`、`blocked`。
- `docs/acceptance/`：前置条件、操作步骤、期望结果、边界场景和证据；不得用“功能正常”替代可验证条件。

### 通用规则

- 除目录入口 `README.md` 外，`docs/` 人类文档使用 Markdown，文件名为“二位编号-中文名称.md”；编号不足两位补零，名称不使用空格。
- 同一主题在 PRD、设计、计划和验收尽量沿用相同编号与主题词。
- 工具或平台要求的固定文件名不翻译、不编号，包括 `README.md`、`AGENTS.md`、`CONTRIBUTING.md`、`go.mod`、`openapi.yaml`、`atlas.hcl`、`atlas.sum` 和源码文件。
- 需求、架构、接口、隐私或验收行为变化时，同一改动更新相关文档。依赖或供应商边界变化必须更新 `docs/design/01-技术选型.md`。
- 不删除历史决策掩盖变更；在当前索引和替代文档中记录历史状态，详细内容由 Git 保存。
- 文档不得包含密钥、生产数据、真实用户敏感信息或依赖短期 `/tmp` 路径的长期证据。

## 测试与验证

- iOS：为领域规则、推荐、状态机、GRDB migration、媒体生命周期与 ViewModel 编写 Swift Testing；核心旅程增加 XCUITest。
- Go：为 Service 编写单元测试；Repository、事务、迁移、幂等和 Outbox 使用真实 PostgreSQL；worker 使用真实 RabbitMQ/Redis 的容器集成测试。
- API：OpenAPI 通过语法、风格与破坏性变更检查；iOS 生成 Client 可编译；Go Handler 通过契约测试。
- 高风险能力必须真机或隔离环境验证，包括照片权限撤回、低内存、后台恢复、Reduce Motion、VoiceOver、离线恢复、供应商超时、重复消息、迟到结果、删除竞态和失败清理。
- 修复缺陷时优先添加复现测试；无法自动化时写入验收文档并说明原因。
- 当前 iOS 架构改动至少运行 `scripts/validate-ios-architecture.sh` 与相关 `xcodebuild`；旧 `validate-p0-scope.sh`、旧生活管理 motion/privacy/localization 结果不构成 OOTD 发布证据。

## 完成标准

任务只有在以下条件满足时才算完成：

1. 实现符合已批准 OOTD PRD、技术基线、功能设计与 OpenAPI 契约。
2. 相关测试通过，并完成与风险相称的真机或集成验证。
3. 没有提交密钥、缓存、生成产物、真实用户数据或无关文件。
4. 相关 PRD、设计、计划和验收文档已同步更新。
5. 数据迁移、媒体删除、回滚和兼容方案已记录并可验证。
6. 历史生活管理代码只按批准的迁移分组处理，没有零散破坏未提交代码或现有用户数据。
7. 已说明仍存在的限制、风险和后续工作。
