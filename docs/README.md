# 项目文档

项目文档按目的分为四类：

- [`prd/README.md`](prd/README.md)：产品需求与范围。
- [`design/README.md`](design/README.md)：产品设计、技术设计和架构决策。
- [`plan/README.md`](plan/README.md)：实施阶段、依赖、风险和进度。
- [`acceptance/README.md`](acceptance/README.md)：可执行验收标准和证据。

一个功能从提出到交付，应能沿相同主题找到 PRD、design、plan 和 acceptance。当前产品是 OOTD；旧记账、日历与出行文档只用于解释历史实现和数据迁移，不再指导新功能。

## 当前事实源

产品需求：

- [`prd/10-OOTD产品需求.md`](prd/10-OOTD产品需求.md)：当前唯一已批准产品范围、用户故事、业务规则与指标。

技术与总体架构：

- [`design/01-技术选型.md`](design/01-技术选型.md)：SwiftUI、Go、数据、消息队列、媒体和精确版本的唯一事实源。
- [`design/02-后端架构.md`](design/02-后端架构.md)：模块化单体、API/worker 角色、GORM/SQL、事务与部署边界。
- [`design/10-OOTD产品总体设计.md`](design/10-OOTD产品总体设计.md)：OOTD 产品承诺、信息架构和端到端闭环。

功能设计：

- [`design/11-数字形象与照片采集设计.md`](design/11-数字形象与照片采集设计.md)
- [`design/12-数字衣橱与衣物录入设计.md`](design/12-数字衣橱与衣物录入设计.md)
- [`design/13-穿搭推荐设计.md`](design/13-穿搭推荐设计.md)
- [`design/14-AI虚拟试穿设计.md`](design/14-AI虚拟试穿设计.md)
- [`design/15-动态预览设计.md`](design/15-动态预览设计.md)
- [`design/16-穿搭记录与反馈设计.md`](design/16-穿搭记录与反馈设计.md)
- [`design/17-OOTD服务端与异步任务设计.md`](design/17-OOTD服务端与异步任务设计.md)
- [`design/18-OOTD权限隐私与安全设计.md`](design/18-OOTD权限隐私与安全设计.md)
- [`design/19-OOTD技术债清理与迁移设计.md`](design/19-OOTD技术债清理与迁移设计.md)

实施与验收：

- [`plan/10-OOTD产品实施计划.md`](plan/10-OOTD产品实施计划.md)：从基线收口、本地核心到云端增强与发布的阶段计划。
- [`acceptance/10-OOTD产品系统验收.md`](acceptance/10-OOTD产品系统验收.md)：SwiftUI、衣橱、推荐、生成、迁移、删除和发布的可执行标准。

## 历史参考

以下文档描述 2026-08-30 产品转向前的个人生活管理实现：

- `prd/03-最小可行产品需求.md`
- `design/03-系统总体设计.md` 至 `design/09-权限隐私与安全设计.md`
- `plan/01-技术基线实施计划.md` 与 `plan/03-最小可行产品实施计划.md`
- `acceptance/01-技术基线验收.md` 与 `acceptance/03-最小可行产品系统验收.md`

它们不再是当前事实源。涉及旧 GRDB 数据、权限、资源或测试时可以查阅，但任何继续实现都必须先转写到当前 OOTD PRD/design/plan/acceptance。

## 命名与编号

- 每个文档目录直接使用 `README.md` 作为入口，不创建编号目录文档。
- 主题文档使用 `01` 至 `99` 的两位编号与中文名称，例如 `10-OOTD产品需求.md`。
- 同一主题跨 PRD、design、plan 与 acceptance 时尽量沿用编号和中文主题词。
- 工具要求的 `README.md`、`AGENTS.md`、`openapi.yaml`、`atlas.hcl` 与 `atlas.sum` 等固定文件名不翻译、不复制。
- 已发布文档不为排序随意改号；替代关系写在当前入口和新文档状态中。
