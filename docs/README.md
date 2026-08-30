# 项目文档

项目文档按目的分为四类：

- [`prd/README.md`](prd/README.md)：产品需求与范围。
- [`design/README.md`](design/README.md)：产品设计、技术设计和架构决策。
- [`plan/README.md`](plan/README.md)：产品级实施计划，以及统一契约、任务、依赖与证据的单切片执行计划。
- [`acceptance/README.md`](acceptance/README.md)：可执行验收标准和证据。

一个功能从提出到交付，遵循 PRD → design → execution plan → implementation → acceptance。主要功能各自维护一份 PRD 和 design；切片排入近期产品计划后、编码前创建一份 `FF-SS` 执行计划，在同一文件中固定范围契约、任务与完成证据。当前产品是 OOTD；旧记账、日历与出行文档只用于解释历史实现和数据迁移，不再指导新功能。

## 当前事实源

产品总纲：

- [`prd/10-OOTD产品需求.md`](prd/10-OOTD产品需求.md)：共同愿景、产品组合边界、跨功能规则与指标。

单功能需求与设计：

| 功能 | PRD | design |
| --- | --- | --- |
| 数字形象与照片采集 | [`prd/11-数字形象与照片采集需求.md`](prd/11-数字形象与照片采集需求.md) | [`design/04-数字形象与照片采集设计.md`](design/04-数字形象与照片采集设计.md) |
| 数字衣橱与衣物录入 | [`prd/12-数字衣橱与衣物录入需求.md`](prd/12-数字衣橱与衣物录入需求.md) | [`design/05-数字衣橱与衣物录入设计.md`](design/05-数字衣橱与衣物录入设计.md) |
| 穿搭推荐 | [`prd/13-穿搭推荐需求.md`](prd/13-穿搭推荐需求.md) | [`design/06-穿搭推荐设计.md`](design/06-穿搭推荐设计.md) |
| AI 虚拟试穿 | [`prd/14-AI虚拟试穿需求.md`](prd/14-AI虚拟试穿需求.md) | [`design/07-AI虚拟试穿设计.md`](design/07-AI虚拟试穿设计.md) |
| 动态预览 | [`prd/15-动态预览需求.md`](prd/15-动态预览需求.md) | [`design/08-动态预览设计.md`](design/08-动态预览设计.md) |
| 穿搭记录与反馈 | [`prd/16-穿搭记录与反馈需求.md`](prd/16-穿搭记录与反馈需求.md) | [`design/09-穿搭记录与反馈设计.md`](design/09-穿搭记录与反馈设计.md) |
| 云端生成与任务管理 | [`prd/17-云端生成与任务管理需求.md`](prd/17-云端生成与任务管理需求.md) | [`design/10-OOTD服务端与异步任务设计.md`](design/10-OOTD服务端与异步任务设计.md) |
| 隐私与数据控制 | [`prd/18-隐私与数据控制需求.md`](prd/18-隐私与数据控制需求.md) | [`design/11-OOTD权限隐私与安全设计.md`](design/11-OOTD权限隐私与安全设计.md) |
| 历史数据迁移 | [`prd/19-历史数据迁移需求.md`](prd/19-历史数据迁移需求.md) | [`design/12-OOTD技术债清理与迁移设计.md`](design/12-OOTD技术债清理与迁移设计.md) |

技术与总体架构：

- [`design/01-技术选型.md`](design/01-技术选型.md)：SwiftUI、条件 Three.js/WebKit renderer、Go、数据、消息队列、媒体和精确版本的唯一事实源。
- [`design/02-后端架构.md`](design/02-后端架构.md)：模块化单体、API/worker 角色、GORM/SQL、事务与部署边界。
- [`design/03-OOTD产品总体设计.md`](design/03-OOTD产品总体设计.md)：OOTD 产品承诺、信息架构和端到端闭环。

计划与验收：

- [`plan/README.md`](plan/README.md)：产品级实施计划与单切片执行计划的准入、编号、状态和模板。
- [`plan/10-OOTD产品实施计划.md`](plan/10-OOTD产品实施计划.md)：从基线收口、本地核心到云端增强与发布的阶段计划。
- [`plan/11-01-照片输入与质量门执行计划.md`](plan/11-01-照片输入与质量门执行计划.md)：首个统一范围契约、任务与证据的隔离 POC 执行计划；生产照片入口保持关闭。
- [`plan/19-01-历史发布事实与数据盘点执行计划.md`](plan/19-01-历史发布事实与数据盘点执行计划.md)：只读盘点发布事实、历史数据类别和后续 Go/No-Go，不授权迁移或删除。
- [`acceptance/README.md`](acceptance/README.md)：10 号系统组合验收与 11–19 号单功能验收的索引和证据规则。
- [`acceptance/10-OOTD产品系统验收.md`](acceptance/10-OOTD产品系统验收.md)：本地核心、受控静态试穿与动态实验的跨功能系统验收。

## 历史参考

以下文档描述 2026-08-30 产品转向前的个人生活管理实现：

- `prd/03-最小可行产品需求.md`
- `plan/01-技术基线实施计划.md` 与 `plan/03-最小可行产品实施计划.md`
- `acceptance/01-技术基线验收.md` 与 `acceptance/03-最小可行产品系统验收.md`

旧 `design/03-系统总体设计.md` 至 `design/09-权限隐私与安全设计.md` 已从工作树删除，详细内容仍可在 Git 历史中审计；当前 [`design/12-OOTD技术债清理与迁移设计.md`](design/12-OOTD技术债清理与迁移设计.md) 承接旧数据盘点、保留和安全清理边界。其余历史文档不再是当前事实源。涉及旧 GRDB 数据、权限、资源或测试时可以查阅，但任何继续实现都必须先转写到当前 OOTD PRD/design；进入实施时再生成单切片执行计划，并纳入当前产品计划与 acceptance。

## 命名与编号

- 每个文档目录直接使用 `README.md` 作为入口，不创建编号目录文档。
- 主题文档使用 `01` 至 `99` 的两位编号与中文名称，例如 `10-OOTD产品需求.md`。
- PRD 使用稳定的 10–19 号产品编号；design 独立按 01–12 连续编号，并通过本页表格显式映射，不要求两类文档同号。
- 单切片执行计划使用 `plan/FF-SS-中文名称执行计划.md`；`FF` 对应单功能 PRD 编号，`SS` 在该功能内递增，因此 PRD 11 对应 design 04 的首个切片仍为 `11-01`。
- 工具要求的 `README.md`、`AGENTS.md`、`openapi.yaml`、`atlas.hcl` 与 `atlas.sum` 等固定文件名不翻译、不复制。
- 已发布文档不为排序随意改号；替代关系写在当前入口和新文档状态中。
