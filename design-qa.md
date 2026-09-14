# 三维穿搭工作室设计验收

- source visual truth: <https://x.com/LerSentAI/status/2090783821452943404?s=20>
- implementation: `then-app/ThenApp/Features/Avatar/Presentation/AvatarStudioView.swift`
- viewport: iPhone 17，iOS 26.5，393 × 852 pt（截图 1206 × 2622 px）
- evidence:
  - `docs/acceptance/evidence/11-02-avatar-studio-main.png`
  - `docs/acceptance/evidence/11-02-built-in-wardrobe.png`
  - `docs/acceptance/evidence/11-02-avatar-studio-dressed.png`

## 对照结果

| 优先级 | 区域 | 观察结果 | 后续判定 |
| --- | --- | --- | --- |
| P0 | 人物视觉 | 已交付可拖动、可转向、可更换上装/下装/鞋履几何的 Three.js 工程角色；当前为不可复现的预编译低模，与参考中的高完成度人物视觉仍有明显差距 | 按 2026-09-15 架构用 Blender 可编辑母版重建人物、发型、材质、灯光与服装资产后重新逐帧对照 |
| P0 | 服装适配 | 两上装 × 3×3 体型 × 正/侧/背的 54 场景均能加载；人工复核发现蓝色衬衫肩/肘/袖口身体穿出，manifest 的 coverage 还未在 renderer 生效 | 从 `.blend` 修服装留量/权重/shape keys 和 body regions；不以自动测试绿色或运行时推顶点通过 |
| P0 | 资产事实源 | 八个 GLB 已通过 Khronos validator 和 Swift 恶意资源检查，但 `reproducibleFromRepository=false`、`sourceGeneratorRetained=false` | 正式资产同时交付 `.blend`、源贴图、source revision、许可证、GLB/hash 与验证报告 |
| P1 | 完整页面 | 首页、内置衣橱分类、选择托盘和换装结果已运行；照片输入、AI Refining、结果确认、360° 生成/分享尚未形成真实业务闭环 | Provider、隐私、质量与成本门通过后按独立切片实现，当前不得以占位页算通过 |
| P2 | 状态与历史 | 当前换装状态只保存在内存；重启恢复、已保存 Look 浏览、删除与迟到结果恢复尚未接入本地事实源 | 接入 GRDB 配置和完整生命周期测试后验收 |
| P2 | 视觉对照证据 | 已逐段检查参考视频并保存实现截图；本轮没有取得可合法归档的参考逐帧本地文件，因此无法形成同尺寸叠图差分 | 后续取得可归档参考帧后补同视口并排和像素级差异图 |

## 已符合的可见结构

- 白色沉浸式主画布、左上品牌、右上日历/菜单和底部三入口胶囊导航。
- 中央人物、左右转向、底部日期/颜色/单品卡片，以及收藏、分享、删除动作。
- TOPS、OUTERWEAR、BOTTOMS、SHOES 分类，黑色已选托盘和蓝色 `Dress up` 动作。
- 内置服装可以直接使用；首次进入不要求照片、相册权限、账号或网络。
- 页面使用项目品牌“于是”和项目自有资源；参考软件名称没有进入产品源码与文件名。

## 比较历史

1. 首轮渲染因 `file://` ES module 跨源限制停留在加载态；改用受限 `avatar://` 本地资源协议后，Three.js 和核心模块均可离线加载。
2. 第二轮人物头发遮挡面部；调整发帽、眼睛和嘴部位置后，正面五官可见。
3. 第三轮从内置衣橱选择“雾蓝宽松衬衫”并执行 `Dress up`，主页面上装从短袖针织几何变为有衣领、长袖的衬衫几何，截图已保存。

## 验收边界

当前结果是可运行纵切，已经验证页面骨架和无上传三维换装主路径。人物资产完成度、照片 AI 链、360° 输出、状态持久化、全部异常/无障碍组合和真机资源门仍未达到参考软件完整功能与页面的一比一交付标准。

final result: blocked
