# 项目文档

更新：2026-09-16。仅维护当前有效设计、需求、执行计划和验收；已退役功能、被替代的技术方案和重复实验记录已清理。

## 从这里开始

| 需要了解 | 入口 |
| --- | --- |
| 当前后端需求与数据设计 | [领域](design/21-后端产品边界与领域设计.md)、[数据库](design/22-后端数据库设计.md)、[API](design/23-后端接口设计.md)、[审核计划](plan/19-01-后端需求与数据设计审核计划.md) |
| 整体进度、阻断与下一步 | [产品实施计划](plan/10-OOTD产品实施计划.md) |
| 完整产品与分期 | [Design 20](design/20-OOTD完整产品能力与阶段架构设计.md)、[PRD 10](prd/10-OOTD产品需求.md) |
| 当前技术栈与变更规则 | [Design 01](design/01-技术选型.md) |
| 后端目录、规范与 SOP | [Design 02](design/02-后端架构.md)、[后端规范](../backend/AGENTS.md) |
| Woo 页面与研究 | [UI 设计](design/17-Woo页面与三维穿搭UI设计.md)、[Woo 证据](design/18-Woo立体数字衣橱技术路线研究与决策.md)、[市场研究](design/19-数字衣橱与虚拟试穿竞品研究.md) |
| Three.js / GLB 与资产交付 | [当前研究与选型](design/threejs-avatar-research.md)、[灵动计划](plan/11-04-Threejs角色灵动反馈执行计划.md)、[三维闭环规格](plan/11-05-三维人物与造型闭环执行计划.md) |
| 已测结果与发布缺口 | [系统验收](acceptance/10-OOTD产品系统验收.md) |
| 启动服务 / Swagger | [后端 README](../backend/README.md) |

## 功能追踪

| 功能 | 设计 | 需求 | 验收 |
| --- | --- | --- | --- |
| 每日记录与图文社区 | [Design 21](design/21-后端产品边界与领域设计.md) | [PRD 19](prd/19-每日记录与穿搭社区需求.md) | [Acceptance 19](acceptance/19-每日记录与穿搭社区验收.md) |
| 人物与可选照片 | [Design 04](design/04-数字形象与照片采集设计.md) | [PRD 11](prd/11-数字形象与照片采集需求.md) | [Acceptance 11](acceptance/11-数字形象与照片采集验收.md) |
| 内置衣物与真实衣橱 | [Design 05](design/05-数字衣橱与衣物录入设计.md) | [PRD 12](prd/12-数字衣橱与衣物录入需求.md) | [Acceptance 12](acceptance/12-数字衣橱与衣物录入验收.md) |
| 推荐 | [Design 06](design/06-穿搭推荐设计.md) | [PRD 13](prd/13-穿搭推荐需求.md) | [Acceptance 13](acceptance/13-穿搭推荐验收.md) |
| 本人 AI 试穿 | [Design 07](design/07-AI虚拟试穿设计.md) | [PRD 14](prd/14-AI虚拟试穿需求.md) | [Acceptance 14](acceptance/14-AI虚拟试穿验收.md) |
| Create 360° | [Design 08](design/08-动态预览设计.md) | [PRD 15](prd/15-动态预览需求.md) | [Acceptance 15](acceptance/15-动态预览验收.md) |
| 计划、实际穿着与反馈 | [Design 09](design/09-穿搭记录与反馈设计.md) | [PRD 16](prd/16-穿搭记录与反馈需求.md) | [Acceptance 16](acceptance/16-穿搭记录与反馈验收.md) |
| 账号与云任务 | [Design 10](design/10-OOTD服务端与异步任务设计.md)、[账号](design/15-账号认证与账户数据设计.md) | [PRD 17](prd/17-云端生成与任务管理需求.md) | [Acceptance 17](acceptance/17-云端生成与任务管理验收.md) |
| 隐私与数据控制 | [Design 11](design/11-OOTD权限隐私与安全设计.md) | [PRD 18](prd/18-隐私与数据控制需求.md) | [Acceptance 18](acceptance/18-隐私与数据控制验收.md) |

## 维护规则

[Design 索引](design/README.md) → [PRD 索引](prd/README.md) → [Plan 与 SOP](plan/README.md) → [Acceptance 索引](acceptance/README.md)。

Design 管方案，PRD 管行为，单切片 Plan 合并 spec/checklist，Acceptance 管实际结果。直接更新现行章节，不新增一份状态清单或追加互相矛盾的“最新结论”。删除被替代文档时同步引用；有效需求、尚未完成的门禁和必要失败证据保留。文件名保持语义化，不为编号连续而重命名已有有效文件。
