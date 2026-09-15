# Three.js 角色灵动反馈执行计划

## 当前约束

本轮研究及用户回复已固定标准 GLB，并通过 REQ-038 新增首版眨眼与轻微注视目标。它们不在下方已完成工程微动的测试范围内，具体资产未到位，验收 ACC-027 为 pending；下一增量的输入与检查步骤见 [研究中的执行规格](../design/threejs-avatar-research.md#下一阶段的可验收输入与步骤)。不把既有 checkbox 或 4＋1 测试结果改写成眼部动态通过。

遵循 [AVATAR-BASELINE-01](../design/01-技术选型.md#人物技术冻结与变更规则) 与 `AVATAR-REQ-036`～`037`：本切片不使用 Blender；现有 GLB 仅验证交互。正式资产优先验证授权成品交付，来源未定不能记为已选定。模型画风或穿模失败不得自动触发渲染器更换。

## 状态

- 契约状态：`approved`
- 执行状态：`in_progress`
- 需求：`AVATAR-REQ-032`～`038`（眼部独立增量未实现）
- 设计：[Design 20 真 3D 技术设计](../design/20-OOTD完整产品能力与阶段架构设计.md#真-3d-技术设计)
- 验收：`AVATAR-ACC-025`～`027`

## SMART 目标

在现有 SwiftUI + 隔离 WKWebView + Three.js + 随包 GLB 纵切上，于本切片完成可观察的轻微待机起伏、重心摆动、拖动倾斜反馈和一次换装确认反馈；开启 Reduce Motion、页面隐藏或退出时停止装饰性运动。实现不得新增远程运行代码、第二个 renderer、生成脚本、人物事实或未经资产支持的眨眼/表情/挥手。

## 实现范围

1. Three.js 将平台与人物分层，只移动人物实体，不让底座跟随待机动作。
2. 使用 `requestAnimationFrame` 驱动小幅位置、旋转和缩放；帧间隔设上限，恢复前台时不补算后台时间。
3. 拖动仅增加短暂倾斜并平滑归零；换装 ID 确实变化时播放一次 420 ms 确认反馈。
4. Swift 将系统 Reduce Motion 作为布尔合同发送给 renderer；renderer 关闭自动和装饰性动作，保留直接 yaw 旋转。
5. 页面隐藏时取消动画帧；销毁时取消帧并释放模型、底座和 renderer。

## 当前工程增量非目标

- 不生成或修改 GLB，不新增骨骼、眼睑 morph、动画 clips、布料模拟、行走或物理系统。
- 不接入 img2threejs/TRELLIS，不上传人物或衣物。
- 不以当前工程 GLB 的微动效果抵扣正式人物画风、穿模、来源、真机性能或 24 套 Look 验收。

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

## 退出条件

相关构建和自动测试已经通过；只有模拟器和最低支持真机均证明正常动效、Reduce Motion、前后台恢复及无明显性能回退时，`AVATAR-ACC-025` 才能标为通过。当前工程 GLB 仍为回归夹具，正式人物资产继续独立验收。

## 首版眼部增量 Checklist

以下任务承接已确认 REQ-038，保留 Three.js/GLB，不依赖更换渲染器。资产达到输入合同前不伪造可见能力。

| ID | 状态 | 完成判定 |
| --- | --- | --- |
| 11-04-ASSET-01 | pending | 代表 GLB 的眼睑 morph/眼球节点/默认方向可验证；来源/授权与衣物适配满足 ACC-026，具体采购预算确认 |
| 11-04-EYES-01 | pending | 受控眨眼完整闭合/睁开；轻微视线平滑回正，拖动/换装不跳变；无运行时网络或额外人物框架 |
| 11-04-EYES-02 | pending | 行为测试及模拟器录屏覆盖正常、Reduce Motion、后台/恢复、快速人物切换与迟到加载；不复用旧人物动画 |
| 11-04-EYES-03 | pending | 正式代表包视觉与最低真机性能/恢复通过，ACC-027 有实际证据 |

相关资产字段与下一阶段步骤以 [人物研究规格](../design/threejs-avatar-research.md#下一阶段的可验收输入与步骤) 为准；这里仅维护任务完成状态。

## 生命周期修复的可执行要求

源码审查 5aabe58 发现仅有 document.visibilitychange/pagehide、RAF 绝对相位与 context lost 后未硬停止。本轮已由原生 scene/舞台/覆盖页状态驱动停止和恢复，改用仅活动时递增的时间，离开时清理 pointer/lean/pulse，context lost 后停止循环，异步加载失败或提前销毁也会释放模型资源。

模拟器已覆盖后台 30 秒恢复、展示覆盖页再返回、Reduce Motion 前后切换、真实画布拖动、拖动中断、换装和 WebContent 终止重建；自动化、双帧和动态录屏均未出现 fallback。拖动中断的精确调度记录与验收结果分别位于 `/tmp/ThenAvatarInterruptedDrag-20260915-6.interrupt.log` 和 `/tmp/ThenAvatarInterruptedDrag-20260915-6.xcresult`。最低支持真机的帧率、内存、发热和恢复仍为 ACC-02，因此本计划继续保持 `in_progress`。
