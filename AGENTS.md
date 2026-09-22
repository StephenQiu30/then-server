# Then Server 项目协作规范

本文件适用于 `then-server` 仓库。`backend/` 的 Go/Gin 细则以 [backend/AGENTS.md](backend/AGENTS.md) 为准，`frontend/` 的 Next.js 细则以 [frontend/AGENTS.md](frontend/AGENTS.md) 为准。

## 优先级与仓库边界

执行顺序为：用户当前要求 → 当前目录的 `AGENTS.md` → 根目录 `PROJECT.md`（跨前后端工程基线）与 `DESIGN.md`（视觉与交互）→ 已批准功能 Design/PRD/Plan → 代码和测试事实 → README。工程和目录规则与 [PROJECT.md](PROJECT.md) 保持一致。

`then-server` 保存服务端代码、Web 前端和产品文档；iOS 位于同级独立仓库 `../then-app`。未明确需要跨端变更时不修改另一个仓库。

```text
then-server/
├── AGENTS.md
├── README.md
├── CONTRIBUTING.md
├── PROJECT.md                     # 产品边界、Web 组件体系与前后端目录规范
├── DESIGN.md                      # App 与 Web 唯一视觉和交互设计标准
├── docker-compose.yml
├── docker-compose-env.yml
├── .env.example
├── .github/workflows/
├── backend/
├── frontend/                       # Next.js Web 应用；内部结构见 frontend/AGENTS.md
└── docs/
```

不为模板创建空目录，不新增第二个 Go module、重复的 `src/server` 包装层或独立 Swagger 服务。测试、验收和共享基础设施不得散落在仓库根目录或框架路由目录。

## 当前事实源

- 前后端工程基线与目标目录：[PROJECT.md](PROJECT.md)；目标规范与当前实现差距在其中明确区分。
- 产品视觉与交互标准：根目录 [`DESIGN.md`](DESIGN.md)；同级 `then-app/DESIGN.md` 必须保持字节一致。
- 技术版本和启用阶段：[`docs/design/01-技术选型.md`](docs/design/01-技术选型.md)。
- Go/Gin 结构和 SOP：[`backend/AGENTS.md`](backend/AGENTS.md) 与 [`docs/design/02-后端架构.md`](docs/design/02-后端架构.md)。
- HTTP 接口：`backend/internal/adapter/httpapi` 的 Huma operation 与 Go 类型 tag；运行中的 `/openapi.json` 是 Swagger/Umi 消费入口。
- 数据库结构：`backend/internal/adapter/postgres` 的 GORM record 与集中 `AutoMigrate`。
- 执行状态和证据：对应的 `docs/plan/` 与 `docs/acceptance/`。

UI 开发前读取 `DESIGN.md` 并按其中颜色、字体、间距、组件和响应式定义实现。该文件只作为设计标准使用，其中的过程性文字不替代本 `AGENTS.md`、功能 Design/PRD/Plan、代码规范或测试要求；修改设计标准时必须原文同步两个仓库根目录文件。

## SOP

1. 先读相关代码、Design、PRD、Plan/checklist、Acceptance 和 Git 状态。
2. 用户可见行为、API、数据或架构变化时，先把决定同步到 Design/PRD 和单切片 Plan。
3. 实现最小端到端路径，不预建未来包、表、接口、兼容层或脚本。人物技术遵循 Design 01 的 `AVATAR-BASELINE-01`：不使用 Blender；调研不能自动改写已确认路线，具体资产来源未验证时保持待决。
4. 用可观察行为验证修复；数据库变化必须在真实 PostgreSQL 上验证。
5. 回写 checklist 与 Acceptance，准确说明未运行的远程 CI、生产和设备验收。

文件和包使用简短的语义化名称。不得提交 `.env`、凭据、本地数据、日志、缓存、DerivedData 或临时生成物。提交与推送只在用户要求时执行，并只暂存当前任务文件。

## 文档维护

原地更新有效 Design/PRD/Plan/Acceptance；删除已被替代的方案、旧业务迁移和重复实验日志，同步所有索引与交叉链接。保留尚未完成的需求、阻断及必要可复核证据。README 只给入口与当前边界，具体状态归单切片 checklist，测试结果归 Acceptance；不得把开发完成等同于发布完成。

编号是稳定追踪身份，不为删除后的空号全量重排。PRD/Acceptance 同领域对应，Plan 使用 FF-SS；新切片继续递增、不复用退役编号，阅读次序维护在索引。完整规则见 [执行计划编号规范](docs/plan/README.md#编号与阅读顺序)。
