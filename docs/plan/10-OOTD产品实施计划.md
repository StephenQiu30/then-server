# OOTD 产品实施计划

## 状态

`in_progress`，2026-08-30。当前只执行技术基线收口与安全清理；产品功能代码尚未进入实现。

## 关联文档

- [`../prd/10-OOTD产品需求.md`](../prd/10-OOTD产品需求.md)
- [`../design/01-技术选型.md`](../design/01-技术选型.md)
- [`../design/02-后端架构.md`](../design/02-后端架构.md)
- [`../design/10-OOTD产品总体设计.md`](../design/10-OOTD产品总体设计.md)
- [`../design/19-OOTD技术债清理与迁移设计.md`](../design/19-OOTD技术债清理与迁移设计.md)
- [`../acceptance/10-OOTD产品系统验收.md`](../acceptance/10-OOTD产品系统验收.md)

## 范围

本计划把已批准的 OOTD 产品与技术设计转化为可发布的 SwiftUI App 和 Go 后端。先保证本地衣橱、推荐与穿搭记录独立成立，再增加静态 AI 试穿；动态预览是条件增强。

计划不包含社区、电商导购、实时 AR、真实 3D 人体、精确尺码、面料物理、未批准多设备同步或用真实用户图片训练模型。

## 里程碑

| 里程碑 | 交付结果 | 退出条件 | 状态 |
| --- | --- | --- | --- |
| M0 基线与债务收口 | 单一 PRD/技术基线、SwiftUI 守卫、旧实现冻结与迁移决策入口 | 当前文档一致，工程可构建，安全清理有证据 | in_progress |
| M1 OOTD SwiftUI shell | 今日、衣橱、穿搭簿三个 Tab 与可恢复启动流程 | 空状态、离线、本地化、无障碍 UI 测试通过 | pending |
| M2 本地衣橱 | 手工添加、图片导入、批量确认、媒体目录与 GRDB | 核心流程离线可用，历史数据库策略已验证 | pending |
| M3 本地推荐与记录 | 三套方案、硬约束、锁定/替换、计划、实际穿着与反馈 | 推荐确定性、解释、冲突和保存竞态测试通过 | pending |
| M4 云端静态试穿 | 账号、上传、异步任务、供应商、结果和删除链 | POC、隐私、成本、质量、延迟和恢复门槛全部通过 | pending |
| M5 条件动态预览 | 2.5D 帧资产、拖拽播放器与静态降级 | 独立 feature flag、Reduce Motion、性能和成本验收通过 | pending |
| M6 发布收尾 | 历史代码成组清理、迁移/导出、App Icon、真机与运维演练 | 系统验收无阻断项，回滚与删除演练完成 | pending |

## 任务

### M0：基线与债务收口

| 任务 | 状态 | 依赖 | 验证方式 |
| --- | --- | --- | --- |
| 将 PRD 10 与 01/02、10–19 号设计设为当前事实源 | completed | 用户确认产品转向 | 文档索引与活动状态检查 |
| 固定 SwiftUI + Observation，限制 UIKit 为系统桥接 | completed | Xcode 工程 | `scripts/validate-ios-architecture.sh`、通用模拟器构建 |
| 固定 Gin/GORM/Atlas/RabbitMQ/Redis/对象存储边界 | completed | 技术评审 | 01、02、17 号设计一致性检查 |
| 清除 PBX SDK 绝对路径、悬空 plist 与空 `schema.sql` | completed | 工程审计 | `plutil`、Xcode 构建、路径搜索 |
| 确认旧版本是否发布及真实本地数据处理策略 | pending | 产品与发布记录 | 书面选择保留、导出或删除；历史 DB 样本盘点 |
| 为 OOTD 隐私、本地化、Reduce Motion 与范围建立新门禁 | pending | 首个 OOTD 页面和数据流 | 脚本 fixture 正反例与 CI |

### M1：OOTD SwiftUI shell

| 任务 | 状态 | 依赖 | 验证方式 |
| --- | --- | --- | --- |
| 建立 Today、Wardrobe、OutfitBook 三 Tab RootView | pending | M0 | Swift Testing + XCUITest |
| 将同步初始化改为显式 AppBootstrap 状态 | pending | M0 数据策略 | 冷启动、数据库失败、Keychain 失败测试 |
| 建立每 Feature 精确依赖注入 | pending | Root shell | 静态审查，禁止新页面接收巨型环境容器 |
| 建立 OOTD String Catalog、App Icon 与发布 Info.plist | pending | 视觉与隐私文案 | 编译资源、Dynamic Type、VoiceOver 与 Archive 检查 |

### M2：本地衣橱

| 任务 | 状态 | 依赖 | 验证方式 |
| --- | --- | --- | --- |
| 批准 OOTD GRDB schema 与 migration 路径 | pending | 旧数据策略 | 空库与历史库升级测试 |
| 实现私有媒体目录、资产版本和 Data Protection | pending | 资产模型 | 文件保护、manifest、崩溃恢复测试 |
| 实现手工添加、PhotosPicker、相机和商品图导入 | pending | SwiftUI shell | 权限允许/拒绝/撤回真机测试 |
| 实现 Vision 质量门、分割与多衣物草稿 | pending | 授权测试集 | 固定样本、低质量、遮挡与降级测试 |
| 实现批量确认、编辑、状态与归档 | pending | 数据层 | Repository 与核心旅程测试 |

### M3：本地推荐与记录

| 任务 | 状态 | 依赖 | 验证方式 |
| --- | --- | --- | --- |
| 实现场景输入、硬约束和可行候选生成 | pending | M2 | 属性组合与无解属性测试 |
| 实现三套多样化排序、解释和不确定性 | pending | 候选生成 | 固定 seed、排序稳定性和解释一致性测试 |
| 实现锁定、单件替换、快捷调整与撤销 | pending | 推荐 UI | 状态机与 XCUITest |
| 实现计划、实际穿着、收藏和反馈 | pending | Outfit schema | 离线、跨日、归档衣物和竞态测试 |

### M4：云端静态试穿

| 任务 | 状态 | 依赖 | 验证方式 |
| --- | --- | --- | --- |
| 完成 Provider、地域、合同、训练禁用和删除 POC | pending | 法务与供应商 | 授权样本报告、删除证明、故障演练 |
| 建立 Go 配置、健康检查、Gin 中间件与认证 | pending | M0、云资源 | 单元、契约和安全测试 |
| 建立 Atlas baseline、GORM Repository 与幂等模型 | pending | 数据设计 | 真实 PostgreSQL migration/事务测试 |
| 建立私有对象存储上传/finalize/资产血缘 | pending | 对象存储 | 越权、过期 URL、内容校验、删除测试 |
| 建立 RabbitMQ Outbox/Inbox、worker、重试和 DLQ | pending | PostgreSQL、RabbitMQ | 重复、乱序、断线、崩溃窗口测试 |
| 实现静态试穿状态、取消、结果与质量反馈 | pending | Provider adapter | 客户端/服务端端到端和降级测试 |

### M5：条件动态预览

| 任务 | 状态 | 依赖 | 验证方式 |
| --- | --- | --- | --- |
| 关闭静态试穿质量、成本和延迟门槛 | pending | M4 生产观察 | 指标评审记录 |
| 实现帧生成、FFmpeg 白名单和 manifest | pending | 动态 Provider POC | 恶意输入、资源上限与校验测试 |
| 实现 SwiftUI 拖拽播放与静态降级 | pending | 帧资产 | 真机性能、VoiceOver、Reduce Motion 测试 |
| 建立独立 feature flag 与停止开关 | pending | 运维配置 | 回滚演练 |

### M6：发布收尾

| 任务 | 状态 | 依赖 | 验证方式 |
| --- | --- | --- | --- |
| 按批准策略迁移、导出或删除旧生活管理数据 | pending | M0 数据决策 | 历史样本、manifest 和用户路径证据 |
| 成组移除旧 Feature、服务、权限、资源、测试和脚本 | pending | 替代功能完成 | 引用搜索、干净构建、完整测试 |
| 执行安全、隐私、无障碍、性能与删除演练 | pending | M1–M5 | 验收文档中的可复现证据 |
| 完成 Archive、签名、App Store 隐私资料与回滚 | pending | 发布账号 | Organizer、TestFlight 与回滚演练 |

## 依赖

- 产品：旧数据处置、动态预览首发范围、用户可见名称与 Bundle ID。
- 设计：OOTD 视觉系统、照片引导、失败文案和隐私同意原型。
- 供应商：主要地域、AI Provider、对象存储、PostgreSQL、RabbitMQ、Redis、KMS。
- 合规：隐私政策、服务协议、账号注销、供应商分包和跨境结论。
- 工程：最低支持设备、测试用无真人合成资产、Apple Developer Team 与 CI 环境。

## 风险与缓解措施

| 风险 | 缓解措施 |
| --- | --- |
| 旧代码未跟踪或包含未保存成果 | 迁移前建立可恢复提交/分支；只成组清理，保留 Git 证据 |
| 旧真实数据被新产品切换破坏 | 未确认按已发布处理；先导出与升级测试，不自动清库 |
| 衣橱录入负担导致流失 | OOTD-first、批量确认、渐进补全，不设置机械件数门槛 |
| AI 生成改变人物或衣物细节 | 授权测试集、内容安全、质量门、诚实免责声明和一键反馈 |
| 队列重复造成重复计费或迟到结果复活 | 幂等键、Outbox/Inbox、租约、fencing token、删除 tombstone |
| 技术栈一次引入过多运维复杂度 | 按阶段启用；本地核心不运行后端，云生成才启用完整运行栈 |
| 图片与穿着规律泄露 | 端侧净化、私有存储、短签名、最小日志、目的分离与删除审计 |

## 发布与回滚

- M1–M3 可以在不启用云端生成的情况下发布；云入口必须由服务端能力与 feature flag 双重门控。
- M4 先内部和小流量灰度，限制并发、日成本、单用户配额和供应商地域；删除演练未通过不得扩大流量。
- M5 使用独立开关，关闭后静态试穿、推荐与保存仍可用。
- 数据 migration 使用 expand/contract；发布前建立数据库与对象恢复点，不在同一版本执行不可逆旧数据删除和大规模 UI 切换。
- 客户端回滚不能依赖删除新数据；旧版本必须能忽略新表、新字段和未知任务状态。
