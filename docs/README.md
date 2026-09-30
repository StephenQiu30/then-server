# 项目文档

更新：2026-09-30。当前需求为本人自用的 Woo 风格虚拟人偶每日穿搭 App。设计去重后，产品、技术、研究成本各有单一入口；既有服务端/Web合同及原执行证据单独保留，不作个人App前置。

## 从这里开始

| 需要了解 | 入口 |
| --- | --- |
| 当前需求和范围 | [PRD10](prd/10-OOTD产品需求.md)、[需求索引](prd/README.md) |
| 页面、数据与运行设计 | [Design03](design/03-OOTD产品总体设计.md)：本人参考、Look/日期、文件、任务、三维、导出与删除 |
| 技术决定与启用边界 | [Design01](design/01-技术选型.md) |
| Woo效果、官方证据、候选与成本 | [Design29](design/29-Woo虚拟人偶与自用成本决策.md) |
| 先验证什么 | [PRD10功能拆解](prd/10-OOTD产品需求.md#功能到执行的拆解)：先一套满意可旋转的本人Look，再五套代表样本；设备与真实费用待验 |
| 原工程进度/完成依据 | [产品计划](plan/10-OOTD产品实施计划.md)、[Backlog](../BACKLOG.md)、[交接](../HANDOVER.md)：原结果不变，新执行须映射当前REQ |
| 历史研究与失败证据 | [Acceptance10](acceptance/10-OOTD产品系统验收.md#历史研究与工程证据)、[Acceptance14](acceptance/14-AI虚拟试穿验收.md#当前-poc-与-demo-验收) |
| 保留哪些工程设计 | [设计索引](design/README.md#保留的工程合同)：数据库、API、账号与Web等独特合同 |
| 原后端工程规范 | [PROJECT.md](../PROJECT.md)、[Design02](design/02-后端架构.md)、[后端AGENTS](../backend/AGENTS.md) |
| 合并了哪些设计 | [Design03历史追溯](design/03-OOTD产品总体设计.md#历史设计追溯) |
| 改动摘要 | [CHANGELOG](../CHANGELOG.md) |

## 功能追踪

产品实现设计集中在Design03；当前行为以PRD为准。原领域验收保留状态及证据，不因链接迁移或需求确认认领新版本通过。

| 功能 | 当前需求 | 设计入口 | 原验收 |
| --- | --- | --- | --- |
| 本人虚拟形象/照片 | [PRD11](prd/11-数字形象与照片采集需求.md) | [输入准备](design/03-OOTD产品总体设计.md#人物参考与输入准备) | [Acceptance11](acceptance/11-数字形象与照片采集验收.md) |
| 手工照片衣橱 | [PRD12](prd/12-数字衣橱与衣物录入需求.md) | [数据/版本](design/03-OOTD产品总体设计.md#数据版本与日期)、[文件保存](design/03-OOTD产品总体设计.md#媒体保存与恢复) | [Acceptance12](acceptance/12-数字衣橱与衣物录入验收.md) |
| 推荐延期 | [PRD13](prd/13-穿搭推荐需求.md) | [实施边界](design/03-OOTD产品总体设计.md#实施与证据边界) | [Acceptance13](acceptance/13-穿搭推荐验收.md) |
| 风格化图/按需模型 | [PRD14](prd/14-AI虚拟试穿需求.md) | [任务设计](design/03-OOTD产品总体设计.md#图片与模型任务) | [Acceptance14](acceptance/14-AI虚拟试穿验收.md) |
| 静态观察/动态延期 | [PRD15](prd/15-动态预览需求.md) | [三维宿主](design/03-OOTD产品总体设计.md#三维宿主与交互安全) | [Acceptance15](acceptance/15-动态预览验收.md) |
| 日期/计划/实际/反馈 | [PRD16](prd/16-穿搭记录与反馈需求.md) | [事实与日期](design/03-OOTD产品总体设计.md#数据版本与日期) | [Acceptance16](acceptance/16-穿搭记录与反馈验收.md) |
| 个人API/任务/费用 | [PRD17](prd/17-云端生成与任务管理需求.md) | [任务设计](design/03-OOTD产品总体设计.md#图片与模型任务) | [Acceptance17](acceptance/17-云端生成与任务管理验收.md) |
| 隐私/导出/删除 | [PRD18](prd/18-隐私与数据控制需求.md) | [数据控制](design/03-OOTD产品总体设计.md#隐私导出与删除) | [Acceptance18](acceptance/18-隐私与数据控制验收.md) |
| 私人日记/月历 | [PRD19](prd/19-每日记录与穿搭社区需求.md) | [页面](design/03-OOTD产品总体设计.md#信息架构与页面)、[事实](design/03-OOTD产品总体设计.md#数据版本与日期) | [Acceptance19](acceptance/19-每日记录与穿搭社区验收.md) |

## 维护规则

[Design](design/README.md) → [PRD](prd/README.md) → [Plan](plan/README.md) → [Acceptance](acceptance/README.md)。只在各自责任文档维护一次事实：01管技术、03管产品实现设计、29管官方研究/金额，PRD管行为，Plan管spec/checklist，Acceptance管实测。

旧设计编号保留追溯，不回收或重排；历史链接指向合并说明或承接合同，不能理解为原测试重新验证了新方案。新切片须先映射当前REQ，不照旧分期自动开启账号、公共服务或上线。

文档清理不修改代码、依赖或验收勾选，不覆盖并行改动；改动摘要写入根CHANGELOG。
