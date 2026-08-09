# 于是

“于是”是一款面向 iOS 的个人生活管理 App，聚焦记账、日历事件、出行计划、导航衔接与生活记录关联。

- 代码仓库：`then`
- 项目/App 名称：“于是”
- iOS 技术栈：Xcode 26.6 + Swift 6.3.3 + SwiftUI/Observation + GRDB，最低 iOS 18
- P1 后端技术栈：Go 1.26.5 单 module + chi/pgx + PostgreSQL 18
- 接口契约：OpenAPI 3.1.2 + Swagger UI

## 目录

- `ios/`：iOS 客户端。
- `backend/`：Go 后端、OpenAPI 契约与集中数据库 schema。
- `docs/prd/`：产品需求。
- `docs/design/`：产品与技术设计。
- `docs/plan/`：实施计划。
- `docs/acceptance/`：验收标准与结果。

## 核心文件

| 文件 | 说明 |
| --- | --- |
| `AGENTS.md` | 全仓库架构、开发、隐私、API、数据库与验收规范。 |
| `CONTRIBUTING.md` | 贡献流程、分支与提交要求。 |
| `.githooks/commit-msg` | Git 提交标题的本地 hook 入口。 |
| `scripts/validate-commit-message.sh` | `type(scope): subject` 提交格式的统一校验脚本。 |
| `scripts/verify-toolchain.sh` | 强制检查本机 Xcode、Swift、Go 与仓库固定版本。 |
| `backend/openapi.yaml` | Swagger UI 和 iOS Client 生成共用的唯一接口契约。 |
| `backend/schema.sql` | PostgreSQL 数据库结构的唯一定义文件。 |
| `backend/go.mod` | 后端唯一 Go module 与固定 Go toolchain。 |
| `ios/ThenApp/openapi-generator-config.yaml` | Swift OpenAPI Client 生成配置。 |
| `docs/README.md` | 项目文档总目录与分类入口。 |
| `docs/prd/03-最小可行产品需求.md` | MVP 产品范围、业务规则、指标和已关闭范围决策。 |
| `docs/design/01-技术选型.md` | 固定版本、架构选择、排除项与待业务决策。 |
| `docs/design/02-后端架构.md` | Go 单 module、单进程与纯 struct 架构决策。 |
| `docs/design/03-系统总体设计.md` | 记账、日历、出行、同步与隐私的系统总蓝图。 |

## 当前状态

项目已完成技术基线和 MVP 全面系统设计初稿，P0 固定为单设备本地模式，账号与 Go 同步后端进入 P1。预算、长期原始轨迹、票据原图、Time Sensitive、国内地图适配和商业化均已从当前实现范围移除。产品范围见 [`docs/prd/03-最小可行产品需求.md`](docs/prd/03-最小可行产品需求.md)，系统总蓝图见 [`docs/design/03-系统总体设计.md`](docs/design/03-系统总体设计.md)，实施顺序见 [`docs/plan/03-最小可行产品实施计划.md`](docs/plan/03-最小可行产品实施计划.md)。Xcode 工程、Go 程序入口、业务 API 和业务数据表尚未创建。

开始贡献前请阅读 [`AGENTS.md`](AGENTS.md) 和 [`CONTRIBUTING.md`](CONTRIBUTING.md)，并运行 `scripts/verify-toolchain.sh`。
