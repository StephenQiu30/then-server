# Then Server 项目协作规范

本文件适用于 `then-server` 仓库。`backend/` 的 Go/Gin 细则以 [backend/AGENTS.md](backend/AGENTS.md) 为准。

## 优先级与仓库边界

执行顺序为：用户当前要求 → 当前目录的 `AGENTS.md` → 已批准 Design/PRD/Plan → 代码和测试事实 → README。

`then-server` 保存服务端代码和产品文档；iOS 位于同级独立仓库 `../then-app`。未明确需要跨端变更时不修改另一个仓库。

```text
then-server/
├── AGENTS.md
├── README.md
├── CONTRIBUTING.md
├── design.md
├── docker-compose.yml
├── docker-compose-env.yml
├── .env.example
├── .github/workflows/
├── backend/
└── docs/
```

不为模板创建空目录，不新增第二个 Go module、重复的 `src/server` 包装层或独立 Swagger 服务。

## 当前事实源

- 技术版本和启用阶段：[`docs/design/01-技术选型.md`](docs/design/01-技术选型.md)。
- Go/Gin 结构和 SOP：[`backend/AGENTS.md`](backend/AGENTS.md) 与 [`docs/design/02-后端架构.md`](docs/design/02-后端架构.md)。
- HTTP 接口：`backend/internal/transport` 的 Huma operation 与 Go 类型 tag；运行中的 `/openapi.json` 是 Swagger/Umi 消费入口。
- 数据库结构：`backend/internal/repository` 的 GORM record 与集中 `AutoMigrate`。
- 执行状态和证据：对应的 `docs/plan/` 与 `docs/acceptance/`。

## SOP

1. 先读相关代码、Design、PRD、Plan/checklist、Acceptance 和 Git 状态。
2. 用户可见行为、API、数据或架构变化时，先把决定同步到 Design/PRD 和单切片 Plan。
3. 实现最小端到端路径，不预建未来包、表、接口、兼容层或脚本。
4. 用可观察行为验证修复；数据库变化必须在真实 PostgreSQL 上验证。
5. 回写 checklist 与 Acceptance，准确说明未运行的远程 CI、生产和设备验收。

文件和包使用简短的语义化名称。不得提交 `.env`、凭据、本地数据、日志、缓存、DerivedData 或临时生成物。提交与推送只在用户要求时执行，并只暂存当前任务文件。
