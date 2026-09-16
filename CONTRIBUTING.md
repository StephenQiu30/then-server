# 贡献指南

感谢参与“于是”项目。提交代码或文档前，必须先阅读根目录的 [`AGENTS.md`](AGENTS.md)。

## 工作流程

1. 先阅读任务对应的单功能 PRD、design、产品级计划和验收文档；进入实现的切片还必须阅读已批准的同编号执行计划。
2. 使用 `go version`，以及任务实际涉及的 `node --version` 与 `npm --version`，确认本机工具链和 Design 01 固定基线一致；iOS 工具链在独立 `then-app` 核对。
3. 从 `main` 创建短周期分支，推荐使用 `<type>/<short-name>`，例如 `feat/wardrobe-import`、`fix/tryon-retry` 或 `docs/commit-convention`。
4. 使用小而聚焦的提交，提交标题和 Pull Request 标题必须遵循下方 Git 提交规范。
5. 提交 Pull Request 前运行与风险匹配的测试和生成检查。
6. Pull Request 说明应包含变更内容、选择原因、验证方式、用户影响与已知限制。

## 后端开发入口

开始后端编码前，按 [目录职责](docs/design/02-后端架构.md#目录)、[服务规范](docs/design/02-后端架构.md#依赖方向)与[开发交付 SOP](docs/design/02-后端架构.md#开发sop)核对切片。依赖精确版本仍归 Design 01；不另建第二份技术规范或重复任务清单。SOP 中未建立的流水线需在对应切片补齐，不能写作已通过。

## Git 提交规范

### 标题格式

所有准备进入 `main` 的提交标题和 Pull Request 标题必须使用：

```text
type(scope): subject
```

- `type` 和 `scope` 使用小写英文。
- `scope` 必填，只能使用小写字母、数字和连字符；不得省略括号或填写多个 scope。
- 冒号后必须有一个半角空格。
- `subject` 推荐使用简洁的中文动宾短语，说明实际变化，不写句号，不使用“更新代码”“修复问题”等模糊描述。
- 标题建议不超过 72 个字符；一个提交只处理一个可独立说明和回滚的变化。

### Type

| Type | 使用场景 |
| --- | --- |
| `feat` | 新增或扩展用户可感知的能力。 |
| `fix` | 修复缺陷或错误行为。 |
| `docs` | 只修改文档。 |
| `refactor` | 不增加功能、不修复缺陷的代码重构。 |
| `perf` | 性能优化。 |
| `test` | 新增或调整测试。 |
| `build` | 构建系统、依赖或打包配置。 |
| `ci` | 持续集成和自动化流程。 |
| `chore` | 不属于以上类别的仓库维护。 |
| `style` | 不改变行为的格式、空白或命名整理。 |
| `revert` | 回退已有提交。 |

不得自创同义 type，例如 `feature`、`bugfix`、`update` 或 `hotfix`。

### Scope

优先使用以下 scope：

| 类别 | Scope |
| --- | --- |
| 技术边界 | `ios`、`backend`、`openapi`、`db`、`docs`、`repo`、`ci`、`deps`、`security` |
| 业务领域 | `avatar`、`wardrobe`、`recommendation`、`tryon`、`outfit`、`media`、`auth` |

- 只影响单个平台时使用技术 scope，例如 `fix(ios)`。
- 同时影响 iOS、后端和契约的完整业务变化，使用业务 scope，例如 `feat(tryon)`。
- 纯 OpenAPI 契约变化使用 `openapi`；数据库结构变化使用 `db`。
- 确需新增 scope 时，使用可长期复用的英文小写 kebab-case，并在本文件中补充定义。
- 如果无法选出唯一 scope，优先拆分提交；确实不可拆分的仓库级调整使用 `repo`，不得使用 `all` 或逗号分隔多个 scope。

### 正确示例

```text
feat(wardrobe): 新增手工录入衣物入口
fix(tryon): 避免迟到结果覆盖取消状态
docs(repo): 补充 Git 提交规范
refactor(backend): 明确生成任务事务边界
build(deps): 固定 GRDB 依赖版本
revert(backend): 回退生成任务幂等改动
```

以下标题不合规：

```text
feat: 新增衣物入口                 # 缺少 scope
Feat(ios): 新增页面                # type 不是小写
feat(ios):新增页面                 # 冒号后缺少空格
update code                        # 缺少完整结构且描述模糊
fix(ios,backend): 修复同步         # 包含多个 scope
```

### 正文、关联项与破坏性变更

- 标题与正文之间空一行；正文说明为什么修改、关键约束和不明显的取舍，不重复罗列代码。
- Issue 使用 `Refs: #123`；需要合并后自动关闭时使用 `Closes: #123`。
- 破坏性变更仍保持标准标题，并在页脚使用 `BREAKING CHANGE: 影响与迁移方式`；必须说明数据影响、必要变更与回滚；当前开发阶段不增加历史兼容层。
- `revert` 提交正文应记录被回退的 commit SHA 和原因。
- `fixup!`、`squash!` 和 Git 自动生成的 Merge 标题不得进入 `main`，合并前必须整理历史。

### 本地校验

提交前使用 `git log -1 --format=%s` 复核标题，并按本文件的格式、type、scope 与长度规则检查。仓库不保留本地脚本或 Git hook；未来 CI 应直接实现相同规则并覆盖 Pull Request 范围内的提交。

## 文档和契约

- 用户可见项目名使用“于是”；本仓库名使用 `then-server`；iOS 仓库名使用 `then-app`，技术标识使用 `ThenApp`；本地统一放在 `Then/` 父目录。
- 工具链、依赖、供应商或最低系统版本变更必须先更新 `docs/design/01-技术选型.md` 并说明迁移与回滚。
- 接口变更必须先修改 Huma operation、请求/响应类型与 tag，并验证运行时 `/openapi.json`；不得提交第二份 OpenAPI 契约。
- Web 接口调用必须由同一 OpenAPI 生成到 `frontend/src/api/`，不得手写第二份请求模型。
- 当前无历史数据的开发 schema 只由 Repository 内 GORM record 定义，并由 Main 启动时集中 `AutoMigrate`；需要保留数据或进入生产前另立版本化迁移计划。
- 产品或架构行为变化时，同步更新 `docs/` 中的对应文档。
- 交付按 design → PRD → execution plan → implementation → acceptance 推进。执行计划仅为已排期的可独立交付切片创建，并在同一文件中统一范围契约、任务、依赖与完成证据；不得用任务表反向替代产品或设计决策。

## 质量和安全

- 不提交密钥、令牌、生产连接串或真实用户数据。
- iOS 修改应通过构建和相关测试；Go 修改应通过 `gofmt`、`go test ./...` 和已配置的静态检查。
- iOS 修改至少直接运行相关 `xcodebuild build` 与 `xcodebuild test`；工程、target 和并发隔离边界同时通过 Xcode 构建验证。
- iOS 测试不能只依据 xcodebuild 退出码：使用 `-resultBundlePath` 保存独立结果包，再通过 `xcrun xcresulttool get test-results summary --path <结果包路径>` 核对注册数、通过数、失败、跳过与预期失败。筛选测试只能证明所选范围，物理设备限制必须单独记录。
- 人物与衣物图片、生成结果、穿着规律和认证信息按敏感数据处理，遵循最小收集、最短保留、目的分离和可验证删除原则。


文档整理应删除被替代内容并验证本地文件/章节链接，保留有效需求 ID 和当前验收边界。纯文档变更只运行文档与差异检查，不声称新增功能测试通过。
