# 于是项目协作规范

## 适用范围

本文件位于仓库根目录，规则适用于仓库内全部文件。若某个子目录未来新增更具体的 `AGENTS.md`，则子目录规则可以补充本文件，但不得降低这里规定的安全、隐私、数据正确性和测试要求。

## 产品与技术方向

于是是一款仅面向 iOS 的个人生活管理 App。仓库名为 `then`，用户可见项目名与 App 显示名为“于是”，内部 Xcode target 和 Swift module 使用 `ThenApp`。当前核心能力是：

- 记账和消费复盘。
- 系统日历事件同步与出发提醒。
- 导航 App 唤起、出行计划和实际行程记录。
- 行程、事件和消费之间的关联。

当前范围明确不实现预算、结转或共享预算、票据原图/附件、长期原始轨迹或轨迹云备份、国内地图适配、Time Sensitive/Critical Alerts，以及订阅、付费、免费容量或权益分层。不得为这些能力预建入口、表、接口或 SDK；未来若要恢复，必须先批准独立 PRD。

已确定的技术方向：

- iOS：Xcode 26.6、Swift 6.3.3、最低 iOS 18、SwiftUI + Observation，必要时桥接 UIKit。
- 客户端数据：GRDB 7.11.1 + 系统 SQLite，本地优先、离线可用；P0 为单设备本地模式，增量同步进入 P1。
- P1 后端：Go 1.26.5 单 module、单进程、单部署单元，使用 `net/http` + chi 与 pgxpool。
- API：REST + JSON，以 OpenAPI 3.1.2 文档为唯一契约。
- 服务端数据库：PostgreSQL 18，结构由单一 `backend/schema.sql` 声明。
- 系统能力：EventKit、Core Location、MapKit、UserNotifications、Vision/VisionKit；P0 不依赖第三方地图 SDK、云 OCR 或聚合推送 SDK。
- 部署：P1 优先使用中国大陆地域的托管计算、PostgreSQL 和密钥管理服务；当前不使用对象存储。

精确依赖、排除项、待业务决策与升级规则以 `docs/design/01-技术选型.md` 为准，不得在 Feature 实现中自行引入同类替代框架。

## 目标仓库结构

以下结构随实施计划逐步落地，不表示所有条目当前都已存在。当前状态以文件系统、README 和验收文档为准。

```text
.
├── AGENTS.md
├── README.md
├── CONTRIBUTING.md
├── .gitignore
├── .xcode-version
├── .githooks/
│   └── commit-msg
├── scripts/
│   ├── validate-commit-message.sh
│   └── verify-toolchain.sh
├── ios/
│   ├── README.md
│   ├── ThenApp/
│   │   ├── openapi.yaml -> ../../backend/openapi.yaml
│   │   ├── openapi-generator-config.yaml
│   │   ├── App/
│   │   ├── Core/
│   │   ├── Features/
│   │   ├── Services/
│   │   └── Data/
│   └── ThenAppTests/
├── backend/
│   ├── README.md
│   ├── go.mod
│   ├── go.sum
│   ├── main.go
│   ├── openapi.yaml
│   ├── schema.sql
│   ├── internal/
│   │   ├── model/
│   │   ├── service/
│   │   ├── repository/
│   │   ├── transport/
│   │   └── platform/
│   └── tests/
└── docs/
    ├── README.md
    ├── prd/
    │   ├── README.md
    │   └── 03-最小可行产品需求.md
    ├── design/
    │   ├── README.md
    │   ├── 01-技术选型.md
    │   ├── 02-后端架构.md
    │   ├── 03-系统总体设计.md
    │   ├── 04-信息架构与用户流程.md
    │   ├── 05-领域模型与数据设计.md
    │   ├── 06-记账系统设计.md
    │   ├── 07-日历与出行系统设计.md
    │   ├── 08-同步与接口设计.md
    │   └── 09-权限隐私与安全设计.md
    ├── plan/
    │   ├── README.md
    │   ├── 01-技术基线实施计划.md
    │   └── 03-最小可行产品实施计划.md
    └── acceptance/
        ├── README.md
        ├── 01-技术基线验收.md
        └── 03-最小可行产品系统验收.md
```

不要为了填满结构而创建无用空文件。新增目录时，应同时放入真实代码、文档或用途说明；文档目录直接使用 `README.md` 作为入口，不创建编号目录文档。

## 信息源优先级

出现冲突时，按以下顺序判断：

1. 用户当前明确要求。
2. 根目录及当前目录链上的 `AGENTS.md`。
3. `docs/prd/` 中已批准的产品需求。
4. `docs/design/01-技术选型.md` 中已确认的技术基线。
5. `docs/design/` 中其他已记录的架构与设计决策。
6. `backend/openapi.yaml` 中的接口契约。
7. `docs/plan/` 中的实施计划。
8. 现有代码和测试体现的行为。

发现这些来源互相矛盾时，不得静默选择。应在相关文档中记录差异，并在会改变产品行为、数据模型或兼容性的情况下先请求确认。

## 开始任务前

1. 阅读本文件和任务涉及目录内的说明文件。
2. 运行 `scripts/verify-toolchain.sh`；版本不一致时先停止，不使用未批准的替代工具链。
3. 阅读对应 PRD、设计文档、实施计划和验收标准。
4. 检查工作区已有修改，不覆盖或回滚无关改动。
5. 确认本次改动影响 iOS、后端、API、数据库 schema 和文档中的哪些部分。
6. 对高风险能力先做最小技术验证，包括日历权限、后台定位、通知、导航调起、账务事务和离线同步。

## Git 提交规范

- 准备进入 `main` 的提交标题和 Pull Request 标题统一使用 `type(scope): subject`；scope 必填，冒号后保留一个半角空格。
- type 只允许 `feat`、`fix`、`docs`、`refactor`、`perf`、`test`、`build`、`ci`、`chore`、`style` 和 `revert`。
- scope 使用小写英文、数字和连字符；优先使用 `ios`、`backend`、`openapi`、`db`、`docs`、`repo`、`ci`、`deps`、`security` 或明确的业务领域名称。
- subject 使用明确、简洁的动作描述；禁止无 scope、多个 scope、模糊描述或 Git 自动生成标题进入 `main`。
- 每个提交只包含一个可独立说明和回滚的变化；无关修改必须拆分。
- 破坏性变更在页脚使用 `BREAKING CHANGE:`，并说明兼容、迁移与回滚方案。
- 完整 type、scope、正文和示例以 `CONTRIBUTING.md` 为准。首次克隆后运行 `git config --local core.hooksPath .githooks` 启用仓库 hook；校验逻辑只维护在 `scripts/validate-commit-message.sh`。

## iOS 开发规范

### 架构

- P0 只使用一个 `ThenApp` Swift module 加测试 target；Feature 先按目录和协议隔离，不拆成内部 framework 或 Swift Package。
- 采用 feature-first 组织方式，每个功能拥有自己的 View、ViewModel、Domain Model 和 Repository 接口。
- SwiftUI + Observation 负责主要 UI 和状态观察；只有系统或第三方能力缺少合适 SwiftUI 接口时才桥接 UIKit。
- ViewModel 使用 `@Observable`；不引入 RxSwift，也不把 Combine 作为全局状态管理框架。
- View 只负责展示和用户交互，不直接访问数据库、网络、EventKit、Core Location 或 UserNotifications。
- 系统能力必须通过协议封装，例如 `CalendarService`、`ReminderService`、`NavigationService`、`LocationService`、`OCRService` 和 `SyncService`。
- 依赖通过初始化器或明确的环境容器注入，避免隐藏的全局单例。

### Swift 代码

- 使用 Swift Concurrency 和结构化并发；UI 状态更新应明确运行在主 Actor。
- App/UI target 使用 Swift 6 language mode、Complete Strict Concurrency、Approachable Concurrency 和 Main Actor 默认隔离。
- 禁止在业务代码中使用 `try!`、强制解包和无说明的 `fatalError`。
- 错误必须转换为可测试的领域错误，并为用户提供可恢复路径。
- 优先使用值类型和不可变状态；共享可变状态必须有明确隔离策略。
- 用户可见文本必须支持本地化，不在 View 中散落硬编码文案。
- 使用语义颜色、Dynamic Type、VoiceOver 标签和系统控件，遵循 iOS Human Interface Guidelines。

### 本地数据和权限

- 记账、读取已缓存日程和结束行程在无网络时仍应可用。
- 本地数据库固定使用 GRDB + 系统 SQLite，以 `DatabasePool`、WAL、显式事务和集中 `DatabaseMigrator` 管理；不得混用 SwiftData、Core Data 或 Realm。
- GRDB Record 只存在于 Data 层，View、ViewModel 和领域层不得依赖 GRDB 类型。
- 敏感令牌和密钥存入 Keychain，不存入 `UserDefaults`、源码或日志。
- 日历、通知、相册和定位权限必须在对应功能首次使用时按需申请。
- 后台定位仅在用户明确开始记录行程后启用，并提供明显状态和停止入口。
- 原始日历正文、参会人和精确轨迹遵循最小化采集原则。
- 高频定位点先在原生层可靠落盘；用户确认摘要或丢弃行程后原子删除，不进入同步层，也不依赖页面或 ViewModel 生命周期保存轨迹。

## Go 后端规范

### 架构

以下后端规范用于 P1 账号与多设备同步；P0 不依赖 Go 服务运行，也不得展示不可用的云入口。

- 后端只使用一个 `go.mod`、一个 `main.go`、一个进程和一个部署单元；不得创建 `cmd/api`、`cmd/worker`、`cmd/migrate` 或其他独立服务入口。
- 单 module 不等于把所有代码写入一个文件；内部只按 `model`、`service`、`repository`、`transport` 和 `platform` 做必要分层。
- 首版不引入微服务、Kubernetes、Kafka 或分布式事务。
- HTTP 层保持轻量，固定使用标准库 `net/http` 与 chi；数据库连接固定使用 pgxpool。
- Handler 只处理协议转换、认证上下文和响应；业务规则位于 Service/Use Case；SQL 位于 Repository。
- 核心账务查询使用 `pgx` 和 Repository 中的显式 SQL，不引入会要求分散查询文件的代码生成器，也不使用隐藏查询与事务边界的重型 ORM。
- HTTP 服务、定时任务和 Outbox 消费都由同一 `main.go` 启动，共用统一的 `context.Context`、健康检查和优雅退出生命周期；不创建独立 Worker 进程。

### POJO 原则在 Go 中的落地

- Go 没有 Java POJO 类型体系；本项目将 POJO 理解为“纯 Go struct”：对象只表达数据与必要的领域不变量。
- `model` 中的 struct 不依赖 `chi`、`http.Request`、`pgx.Rows`、数据库连接或全局容器，不继承框架基类，不实现 Active Record。
- 后端 request/response 使用手写纯 Go struct，只在 transport 边界使用 `json` 等必要标签；Handler 负责将其转换为领域 struct，Service 不接收 HTTP 或框架特有类型。
- 验证、鉴权、持久化和序列化是边界责任，不把这些框架行为塞进数据对象。
- 只在语义确实不同时增加映射类型，不为每一层机械复制一份 DTO。

### Go 代码

- 所有代码必须通过 `gofmt`；提交前运行 `go test ./...` 和项目已配置的静态检查。
- 所有 I/O、数据库和外部服务调用都接收并传递 `context.Context`。
- 错误使用 `%w` 保留错误链；不得依赖字符串匹配判断错误类型。
- 不启动无法停止或没有所有者的 goroutine；后台任务必须支持取消、超时和优雅退出。
- 配置来自环境或密钥服务；不得提交密钥、生产连接串或真实用户数据。
- 使用结构化日志，但禁止记录访问令牌、手机号、金额明细、日历标题、精确地址和原始轨迹。

### 数据库和异步任务

- `backend/schema.sql` 是 PostgreSQL 结构的唯一事实来源。P1 首版服务端只使用 Atlas Community 可声明管理的 enum、table、column、constraint、index 和 comment。
- 不创建 `db/migrations/`、`db/queries/` 或其他分散 schema 定义。Repository 中的业务查询不得重复声明表结构。
- schema 变更直接修改该文件，在对已有数据执行前必须在 `docs/plan/` 记录数据转换、回滚和验证方案。不允许应用在未审核的情况下自动修改生产 schema。
- 空库、本地和测试使用 `psql --single-transaction` 初始化；已有库必须使用专用数据库，以 Atlas Community `--schema public --dry-run` 生成差异并人工评审。任何 `DROP` 默认阻断，生产禁止 `--auto-approve`。
- P1 首版服务端不使用 extension、function、trigger、RLS 或 seed DML；如业务需要，必须重新评估 Atlas 版本或批准版本化迁移，不得用 `psql` 冒充已有生产库的增量升级器。
- 一个业务动作涉及多张表时必须使用数据库事务。
- 业务写入和需要异步处理的事件应在同一事务中写入 Outbox。
- P1 使用 PostgreSQL jobs/outbox，由同一进程内的可取消后台循环处理；只有真实瓶颈出现后才重新评估独立进程、Redis 或消息队列。
- 当前不提供票据附件、原始轨迹上传或对象存储；不得预建相关表、接口、SDK 或清理任务。

## API 契约规范

- `backend/openapi.yaml` 是 iOS、Go 后端和 Swagger UI 的唯一事实来源。不得创建 `api/` 目录、独立 API 进程、第二份 JSON/YAML、Swagger 注解或手写 iOS DTO。
- 契约固定使用 OpenAPI 3.1.2；升级 OpenAPI minor 版本必须验证 Swift 生成器、Swagger UI、kin-openapi、Spectral 和 oasdiff 的兼容性。
- 本项目中“Swagger 接口文档”指 Swagger UI 对这份 OpenAPI 契约的可视化和调试界面，不是另一份契约。
- 修改接口时按以下顺序进行：修改 OpenAPI、校验契约、生成 iOS Client、手写 Go Handler 与纯 struct、更新契约测试和 Swagger 示例。
- Go 后端不从 OpenAPI 生成 server、DTO 或额外 API 文件，而是使用手写纯 Go struct 和 Handler；iOS 端使用 Apple Swift OpenAPI Generator 生成 types 和 client。
- iOS 生成代码由 Xcode Build Tool Plugin 在构建时放入 DerivedData，不提交、不手工修改。
- `ios/ThenApp/openapi.yaml` 必须保持为指向 `backend/openapi.yaml` 的符号链接，仅用于满足 Xcode 插件的文件发现要求；链接和生成器配置都必须加入 ThenApp target 的 Compile Sources，不得将契约复制到 iOS 目录或同时再加一份 external reference。
- 每个 operation 都必须有全局唯一、可读且稳定的 `operationId`，因为它会成为 Swift 生成方法名和 Go 契约测试映射的稳定标识。
- 请求与响应 schema 必须明确 `required`、可空性、枚举、格式、单位和示例；不得使用无约束的 object 代替稳定数据结构。
- Go Handler 将 HTTP request/response struct 与领域 struct 明确转换；iOS Repository/Service 应把生成的 transport DTO 转换为领域模型，View 与 ViewModel 不直接依赖生成 Client。
- HTTP 层必须根据 OpenAPI 契约编写 contract test，校验路由、参数、状态码和 JSON schema；业务约束仍在 Service/领域层校验。
- 后端在开发和测试环境提供 `/swagger/` 交互文档与 `/swagger/openapi.yaml` 部署契约；运行时直接嵌入并提供 `backend/openapi.yaml`，不生成或复制第二份契约。
- Swagger UI 生产环境默认关闭；若业务确需开放，必须经过身份认证、网络限制并关闭持久化授权信息。
- 所有公开业务接口对外使用 `/v1` 版本前缀。OpenAPI 已在 `servers.url` 中定义 `/v1`，`paths` 不得重复填写该前缀。
- 创建、导入、同步和第三方回调接口必须支持幂等键。
- 分页优先使用游标，不使用依赖不稳定排序的页码分页。
- 错误响应应包含稳定错误码、用户安全的信息和请求追踪 ID，不泄露内部堆栈。
- 破坏性变更必须提供迁移路径或新版本接口，不得无提示修改既有字段语义。

## 领域数据规范

- 标识符由客户端或服务端生成全局唯一 ID；不得依赖本地自增 ID 做跨设备同步。
- 时间在服务端统一保存为 UTC，并在事件模型中保留原始时区信息。
- 金额使用整数最小货币单位与 ISO 币种代码，禁止使用浮点数。
- 账务至少区分 `transaction` 与 `posting`，转账、退款和冲销不得伪装成普通支出。
- 用户已确认的账务记录不得被外部同步静默覆盖。
- 导入记录保存 `source + external_id`；没有外部 ID 时使用稳定指纹并进入待确认流程。
- 每次客户端变更携带 `mutation_id`；服务端保存幂等结果。
- 增量同步使用服务端单调游标、版本号和 tombstone，不以设备时间作为唯一顺序依据。
- 日历事件、出行计划、实际行程和交易记录是不同实体，不得混为一张记录。
- 位置数据必须显式保存坐标系；当前 MapKit/Core Location 数据统一为 WGS-84，不接入或混用 GCJ-02、BD-09 供应商数据。
- 导航估价只能作为预计费用，不能自动成为实际账单。

## 安全与隐私规范

- 财务、日历和精确位置按敏感数据处理，遵循最小收集、最短保留、可解释、可撤回和可删除原则。
- 网络传输使用 TLS；数据库和备份启用静态加密。
- 高敏感字段使用 KMS 管理的字段级或信封式加密。
- 授权判断必须在服务端执行，数据库查询始终限定当前用户。P1 首版不使用 RLS；如后续需要将其作为纵深防御，必须先更新 schema 工具与迁移决策。
- 生产日志、分析事件和推送载荷中不得包含敏感正文。
- 删除功能必须覆盖主数据、派生数据和后续清理队列；OCR 图片与原始轨迹按临时数据规则在会话或摘要确认后清理。
- 不使用真实用户账单、日历或轨迹作为测试夹具。

## 文档规范

### `docs/prd/`

存放产品需求。文档至少包含：背景、目标用户、问题、目标、非目标、范围、用户故事、业务规则、指标、依赖与风险。

### `docs/design/`

存放产品设计和技术设计，包括信息架构、用户流程、状态设计、数据模型、API 设计、架构决策和隐私设计。重要取舍要写明备选方案与选择原因。

### `docs/plan/`

存放实施计划，包括阶段、任务拆分、依赖、风险、验证方式和状态。计划状态只使用：`pending`、`in_progress`、`completed`、`blocked`。

### `docs/acceptance/`

存放可执行验收标准和验收结果。每条需求应包含前置条件、操作步骤、期望结果、边界场景和证据。不得用“功能正常”代替可验证条件。

### 通用规则

- 除各目录入口 `README.md` 外，`docs/` 下的人类文档使用 Markdown，文件名统一采用“二位编号-中文名称.md”，例如 `01-日历出行同步.md`；编号不足两位时必须补零，名称不使用空格。
- `README.md` 只用于目录入口、模板和编写规范；主题文档按 `01` 至 `99` 顺序编号。新增文档优先使用下一个未占用编号，已发布文档不因排序变化随意改号。
- 同一个功能在 PRD、设计、计划和验收文档中尽量沿用相同主题编号与中文主题词，例如 `03-出行需求.md`、`03-出行设计.md`、`03-出行实施计划.md`、`03-出行验收.md`。
- 工具、构建系统或平台要求的固定文件名不翻译、不编号，包括 `README.md`、`AGENTS.md`、`CONTRIBUTING.md`、`go.mod`、`openapi.yaml`、`schema.sql` 及源代码文件；不得为了满足中文命名复制出第二份配置或契约。
- 需求、架构、接口或验收行为变化时，相关文档必须与代码在同一改动中更新。
- 工具链、依赖或供应商边界变化时，必须同步更新 `docs/design/01-技术选型.md`；重大替换需要说明迁移与回滚。
- 不删除历史决策来掩盖变更；使用 Git 历史，并在当前文档记录替代关系。
- 文档中不得包含密钥、生产数据或真实用户敏感信息。

## 测试与验证

- iOS：为领域逻辑、同步、记账计算和权限状态编写单元测试；核心用户旅程增加 UI 测试。
- Go：为领域服务编写单元测试；数据库 schema、事务、幂等和同步使用真实 PostgreSQL 集成测试。
- API：OpenAPI 契约必须通过校验，iOS 生成代码应能成功编译，Go 手写 Handler 必须通过契约测试。
- API 契约变更时，CI 必须检查 OpenAPI 语法/风格、破坏性变更、Go 契约测试，以及 iOS Build Tool Plugin 能否重新生成并编译 Client。
- OpenAPI 风格使用 Spectral，破坏性变更使用 oasdiff；版本以技术基线为准。
- 高风险能力必须在真机验证，包括日历变化、锁屏通知、导航 App 缺失、权限撤回、后台定位、断网恢复和系统杀进程。
- 修复缺陷时应先添加能够复现问题的测试，或在无法自动化时写入验收文档并说明原因。

## 完成标准

任务只有在以下条件满足时才算完成：

1. 实现符合已批准的 PRD、设计与 API 契约。
2. 相关测试通过，并完成与风险相称的真机或集成验证。
3. 没有提交密钥、生成缓存、真实用户数据或无关文件。
4. 相关 PRD、设计、计划和验收文档已更新。
5. 数据迁移、回滚或兼容方案已记录。
6. 已说明仍存在的限制、风险和后续工作。
