# WEARLOG Server 项目协作规范

本文件适用于 `wearlog-server` 仓库。`backend/` 的 Go/Gin 细则以 [backend/AGENTS.md](backend/AGENTS.md) 为准，`frontend/` 的 Next.js 细则以 [frontend/AGENTS.md](frontend/AGENTS.md) 为准。

## 优先级与仓库边界

执行顺序为：用户当前要求 → 当前目录的 `AGENTS.md` → 根目录 `PROJECT.md`（跨前后端工程基线）与 `DESIGN.md`（视觉与交互）→ 当前已批准需求 → 代码和测试事实 → README。工程和目录规则与 [PROJECT.md](PROJECT.md) 保持一致。

`wearlog-server` 保存服务端代码、Web 前端和产品文档；iOS 位于同级独立仓库 `../wearlog-app`。未明确需要跨端变更时不修改另一个仓库。

```text
wearlog-server/
├── AGENTS.md
├── README.md
├── CONTRIBUTING.md
├── CHANGELOG.md                    # 简短改动记录
├── PROJECT.md                     # 产品边界、Web 组件体系与前后端目录规范
├── DESIGN.md                      # App 与 Web 唯一视觉和交互设计标准
├── docker-compose.yml
├── docker-compose-env.yml
├── .env.example
├── .github/workflows/
├── backend/
├── frontend/                       # Next.js Web 应用；内部结构见 frontend/AGENTS.md

```

不为模板创建空目录，不新增第二个 Go module、重复的 `src/server` 包装层或独立 Swagger 服务。测试、验收和共享基础设施不得散落在仓库根目录或框架路由目录。

## 当前事实源

- 前后端工程基线与目标目录：[PROJECT.md](PROJECT.md)；目标规范与当前实现差距在其中明确区分。
- 产品视觉与交互标准：根目录 [`DESIGN.md`](DESIGN.md)；同级 `wearlog-app/DESIGN.md` 必须保持字节一致。
- 技术版本：`backend/go.mod/go.sum`、`frontend/package.json/package-lock.json`；iOS 以 Xcode 工程和 Package.resolved 为准。
- Go/Gin 结构和 SOP：[`backend/AGENTS.md`](backend/AGENTS.md)。
- HTTP 接口：`backend/internal/adapter/httpapi` 的 Huma operation 与 Go 类型 tag；运行中的 `/openapi.json` 是 Swagger/Umi 消费入口。
- 数据库结构：`backend/internal/adapter/postgres` 的 GORM record 与集中 `AutoMigrate`。
- 全局任务顺序与接手入口：根目录 [`BACKLOG.md`](BACKLOG.md) 与 [`HANDOVER.md`](HANDOVER.md)。

UI 开发前读取 `DESIGN.md` 并按其中颜色、字体、间距、组件和响应式定义实现。该文件只作为设计标准使用，其中的过程性文字不替代本 `AGENTS.md`、当前需求、代码规范或测试要求；修改设计标准时必须原文同步两个仓库根目录文件。

## 工作流程

1. 先读相关代码、PROJECT、DESIGN 与 Git 状态，保留无关未提交改动。
2. 当前先完成产品需求调研；目标包含人物展示、选衣试穿、前后对比与真实衣橱/实穿记录。已确认方向、建议范围和验证门槛以 docs/requirement/README.md 与 06 为准；既有二维能力是实现基础，不替代目标。人物技术判断见 docs/research，实测见 docs/validation，不从需求优先级或历史实验计划推导新生成任务。
3. 后续明确授权实施时再实现最小端到端路径，不预建未来包、表、接口、兼容层或脚本。账号、社区与公共后端不是个人首版前置；现有功能按实际代码合同维护。
4. 用可观察行为验证；数据库变化必须在真实 PostgreSQL 上验证。结果准确区分本地测试、远程 CI、真实供应商和设备验收。
5. 根目录 CHANGELOG 按日期记录简短改动；BACKLOG 只维护尚未完成的工作。docs/requirement 只维护产品需求、业务规则与验收条件；技术调研归 docs/research，截图、耗时、费用和测试过程归 docs/validation，不逐轮追加到需求目录。历史人物实验不代表当前范围，不建立重复接口事实源。

产品与执行文档按用户指定的 pm-execution 方法组织摘要、目标用户、价值主张、范围及可执行验收；使用 pm-toolkit 检查术语、逻辑和表述。具体约定见 [docs 编写约定](docs/README.md#编写约定)。原地维护现有 PRD 入口，不创建平行需求体系；设计完成不等于实现或效果验收完成。

每次推送后等待该 SHA 对应的完整 CI 成功后才能报告通过；本地测试不能替代远端 CI。提交和推送只在用户要求时执行，只暂存当前任务文件。

文件和包使用简短语义名称。不得提交 .env、凭据、本地数据、日志、缓存、DerivedData 或临时生成物。删除历史实现时同步移除无用测试、依赖、配置、资源与入口，不保留兼容包装层。
