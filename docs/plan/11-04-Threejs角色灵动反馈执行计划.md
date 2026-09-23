# Three.js 角色灵动反馈执行计划

更新：2026-09-22。已有工程微动与生命周期证据保留；整体最低真机验收仍 pending。按新设计，眼部与模块资产任务为 **deferred**，不是 11-05/14-01 首版依赖。本片不因路线替换扩大已测范围。

## 已实现范围

随包 Three.js 的整体微动、拖动反馈、换装反馈、Reduce Motion、原生可见性/scene 状态、活动时钟与 WebContent 恢复已有模拟器证据。新整套 Look 可以复用这些生命周期处理；默认不要求持续待机或眼部动画，直接操作必须保留。

## Checklist

| ID | 任务 | 状态 | 完成证据 |
| --- | --- | --- | --- |
| `11-04-DOC-01` | 对齐 Design、PRD、Plan 与 Acceptance 的 Three.js 单一路线 | `completed` | 已补齐此前漏改的 Design 01、18、目录入口；AVATAR-BASELINE-01、REQ-036/037、ACC-026 固定禁用 Blender 和变更规则 |
| `11-04-IOS-01` | 实现人物层、待机微动、拖动反馈和换装确认 | `completed` | `avatar.js` diff 与模拟器可见行为 |
| `11-04-IOS-02` | 贯通 Reduce Motion、后台暂停和销毁释放 | `completed` | Swift 将 scene、舞台和覆盖页状态桥接到 renderer；JS 使用仅活动时递增的时钟，暂停时清理 pointer/lean/pulse，context lost 停帧，退出释放加载中及已挂载资源；模拟器完成覆盖页、后台 30 秒和 WebContent 重建 |
| `11-04-TEST-01` | 增加 Reduce Motion 配置回归测试 | `completed` | `AvatarStudioModelTests` 新用例 |
| `11-04-TEST-02` | Debug 构建与相关测试通过 | `completed` | iPhone 17 / iOS 26.5：模型与舞台策略 5 passed；覆盖页关闭→后台 30 秒→恢复 1 passed；真实画布拖动→换装→renderer ready 1 passed；均 0 failed/0 skipped，独立 xcresult 已复核 |
| `11-04-ACC-01` | 模拟器录屏核对正常/Reduce Motion/拖动/换装 | `completed` | 34.46 秒 H.264 录屏覆盖待机、真实画布拖动、衣橱选择和换装完成；9.19 秒 Reduce Motion 录屏的两帧角色区域 YAVG=0/YMAX=0；WebContent PID 替换后角色重新显示；五秒拖动期间打开系统设置，返回后 renderer ready，专项 UI 验收 1 passed |
| `11-04-ACC-02` | 最低支持真机核对帧率、内存、发热与 WebContent 恢复 | `pending` | 真机 trace 与录屏 |



## 后续任务处置

| 原 ID | 当前状态 | 后续启用条件 |
| --- | --- | --- |
| 11-04-ASSET-01 | deferred | 仅当重新排期高级角色时取得合法可用的眼部/模块资产；不再采购为首版前置 |
| 11-04-EYES-01 | deferred | 眼睑闭合/睁开与注视的实际资产和动作合同 |
| 11-04-EYES-02 | deferred | 原生状态/Reduce Motion/切换/恢复与眼部联动 |
| 11-04-EYES-03 | deferred | 正式角色可见动作和最低真机证据，AVATAR-ACC-027 独立通过 |

MetaPerson 官方样例曾证明眼部数据技术可行，Avaturn 样例未发现眼部数据；均不是当前生产资产，不是首版等待商务代表包的理由。AVATAR-REQ-038 保留为 deferred，旧测试不抵扣。

## 复用要求

后台/不可见停止 loop，恢复不追赶时间；页面销毁释放 renderer 与输入监听；WebContent 重建只恢复当前未删除的有效 Look revision。新纹理/模型切换需在 11-05 重新验证内存释放和最低设备；原轻量工程 GLB 的测试不能证明生产资产性能。
