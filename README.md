# 于是

“于是”是一款面向 iOS 的个人生活管理 App，聚焦记账、日历事件、出行计划、导航衔接与生活记录关联。

- 代码仓库：`then`
- 项目/App 名称：“于是”
- iOS 技术栈：Xcode + Swift + SwiftUI
- 后端技术栈：Go 单 module + PostgreSQL
- 接口契约：OpenAPI + Swagger UI

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
| `backend/openapi.yaml` | Swagger UI 和 iOS Client 生成共用的唯一接口契约。 |
| `backend/schema.sql` | PostgreSQL 数据库结构的唯一定义文件。 |
| `ios/ThenApp/openapi-generator-config.yaml` | Swift OpenAPI Client 生成配置。 |
| `docs/design/backend-architecture.md` | Go 单 module、单进程与纯 struct 架构决策。 |

## 当前状态

项目处于需求与架构准备阶段，已确定 iOS 原生客户端、Go 单 module 后端、OpenAPI 契约和集中数据库 schema 的基础规范。Xcode 工程、Go 程序入口和业务数据表尚未创建。

开始贡献前请阅读 [`AGENTS.md`](AGENTS.md) 和 [`CONTRIBUTING.md`](CONTRIBUTING.md)。
