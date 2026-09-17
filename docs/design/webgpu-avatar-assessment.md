# WebGPU + Three.js 虚拟形象展示独立评估

研究日期：2026-09-17。状态：研究建议，未切换生产渲染器。

本报告按用户要求独立判断：现有文档用于识别产品需求和实现差距，“路线已冻结”等表述不作为技术优劣的论据。实际读取 then-app `e290b2a`、then-server `038d2e5` 及当前工作树；已有资产目录移动保持原样。使用 GitHub、Firecrawl、Context7 获取外部证据，并运行独立 WKWebView 能力探测。

## 1. 明确判断

**可以使用，而且 Three.js + GLB + WebGPU 优先 / WebGL2 回退，是这个虚拟形象产品值得采用的渲染方向。**

推荐的成立条件是：产品主要为一个可交互角色、受控服装目录、轻量动作与精致的展示舞台，并有 Web 展示或跨端共享渲染逻辑的价值。Three.js 可以承担这些职责；WebGPU 为新材质、后处理和后续 GPU 计算提供空间。它不是建模、自动服装适配或真实人体试衣算法。

区分三个决定：

1. **使用 Three.js 展示角色：推荐。** 已确认的旋转、缩放、GLB、骨骼、形变、换装和眼部交互都在其能力范围。
2. **新展示层按 WebGPURenderer 设计并保留 WebGL2 回退：推荐进入验证。** 不是要求所有设备必须运行 WebGPU。
3. **现在就把正式 iOS 版本强制改为 WebGPU-only：不推荐。** 原因是项目实际宿主、生产模型和最低真机尚未完成验证；不是因为旧文档禁止修改，也不是因为 WebGPU 没有能力。

当前证据不支持“WebGPU 一定更快”“切换后自然更可爱”“所有 iPhone 都可用”或“现有自定义协议一定不能用”。

## 2. 用户需要的效果，以及谁来实现

| 产品能力 | 主要实现层 | WebGPU 的必要性 |
| --- | --- | --- |
| 三套可爱但有成人感的形象，统一审美 | 模型比例、面部/头发造型、材质、美术审核 | 非必要；决定效果的是资产与美术 |
| 连续旋转、缩放、相机复位、透明舞台 | Three.js 相机、变换、输入、背景合成 | WebGL2 已可实现 |
| 呼吸、重心变化、拖动与换装反馈 | 变换或已有动画，生命周期控制 | WebGL2 已可实现 |
| 自然眨眼、轻微视线跟随 | 眼睑 morph、眼球节点、调度与角度限制 | WebGL2 已可实现；模型必须具有对应数据 |
| 两上装 × 两下装 × 两鞋 × 三人物 | 模块资产、骨架/形变匹配、遮挡、原子换装 | 不依赖 WebGPU，也不会被 WebGPU 自动解决 |
| 布料质感、皮肤柔和感、轮廓光、柔和阴影 | PBR/风格化材质、环境光、灯光、色彩管理 | WebGPU/TSL 有扩展空间，基础效果 WebGL2 足够 |
| AO、复杂后处理、未来更丰富的动态 | TSL、后处理、必要时 GPU compute | WebGPU 的价值较明显，但应以实测成本决定启用 |
| 保存、收藏、编辑取消、重启恢复、删除 | 原生/网页业务状态与持久化 | 与图形 API 无关 |
| 用户照片变成完整可换装人物、任意真实衣服上身 | 重建/生成、拓扑、绑定、服装建模与适配服务 | 这套渲染技术不直接提供 |
| 真实尺码、面料垂坠与合身预测 | 身体/服装数据和经验证的物理/拟合算法 | compute 能力不等于已经具备完整算法 |

Three.js 官方 r185 示例直接展示 GLTFLoader + AnimationMixer + WebGPURenderer 的骨骼播放；标准材质库会将 MeshStandardMaterial、MeshPhysicalMaterial、MeshToonMaterial 等映射到节点材质。Context7 还返回了面部 morph 和压缩 glTF 示例，但它的示例来自 dev 分支，因此只作能力定位，具体接入须核对锁定版本。[S2][S3]

三种预设可以共享骨架、槽位和参数语义以降低成本；这不等于把一个模型改名称当作三个人物。即便共用骨架，衣服仍需检查 bind pose、权重、体型形变、身体遮挡和动作极值。软件能播放 morph，不代表某个资产具备正确的 morph。

## 3. 画质判断：优先投资在哪里

如果目标是 Woo 风格的可爱、干净、精致展示，建议按这个次序投资：

1. **角色本体与服装轮廓**：头身比、脸型、眼睛、发型、手脚、衣服版型；正侧背都必须成立。
2. **材质与灯光**：统一色彩管理；皮肤、头发、针织、皮革等有不同粗糙度与高光；主光、补光、轮廓光与脚底接触阴影形成层次。
3. **动作节奏**：自然眨眼、克制注视、轻微待机，拖动优先；动作不能让身体、衣服或脸部穿插。
4. **少量后处理**：只有可见收益明确时加入 AO、轻量抗锯齿或其他处理。避免让服装颜色失真、皮肤油亮或背景过度发光。
5. **再考虑计算密集效果**：头发物理、复杂布料、SSS/SSGI 等按实际产品价值和设备预算逐项加入。

前三项足以形成有品质的展示，不依赖 WebGPU。WebGPU 提供更好的现代图形接口和新效果通路，但不会为低质量资产补出造型细节。单角色场景也不天然是 CPU draw call 瓶颈，WebGPU 优于 WebGL 的幅度只能测量；官方手册也明确提醒，部分场景仍可能是 WebGLRenderer 性能更好。[S1]

因此，**采用 WebGPU 的理由应是新展示层的能力与演进空间，而不是承诺一次 API 替换就带来画质或帧率跃升。**

## 4. 当前代码的独立事实审查

这些事实用于估算工作量，不限制方案选择：

| 位置 | 当前源码事实 | 对评估的影响 |
| --- | --- | --- |
| `then-app/ThenApp/Resources/AvatarStudio/Renderer/avatar.js` | WebGLRenderer；DPR 上限 2；正交相机；半球光、主方向光、轮廓光、平台；已有基础阴影 | Three.js 已经在用，新增的是后端与呈现升级 |
| 同上 | 一次加载 8 个工程 GLB，按槽位设置可见性；RAF 控制待机/反馈 | 可以复用场景与动作意图，但初始化、错误及质量策略需调整 |
| `ThreeAvatarView.swift` | 非持久 WKWebView；`avatar://local/avatar.html`；原生校验资源；桥接 revision/session | 离线宿主可保留为首个验证对象，不能未经测量断言需改成远程网页 |
| `AvatarAssetCatalog.swift` | 工程资产结构和肩宽/躯干 morph 合同校验严格 | 接入正式人物、眼部数据前要更新受控资产合同；与 WebGPU 无关 |
| `avatar.js` 资源销毁 | 释放 geometry/material，但没有显式遍历纹理释放 | 工程资产无贴图尚不能证明生产纹理长期运行的内存表现 |
| `frontend/package.json` | Next.js/React 基础已在，未声明 Three.js | Web 端可复用纯渲染模块，但现状不是双端角色展示已经完成 |
| 11-04 / 11-05 计划 | 眼部、生产三人物/24 组合、完整 Look 持久化及最低真机仍有缺口 | 后端切换不能抵扣这些产品能力 |

本轮使用 Python 标准库直接解析 8 个随包 GLB 的 JSON/accessor：

- 8 个文件总计 **333,976 bytes，约 326 KiB**。
- 所有候选部件合计 **6,000 个三角形**；这包括互斥服装，不是每帧全部可见几何数，也不包括代码创建的平台。
- 每个文件有 skin；**animations 数均为 0，images 数均为 0**。
- morph 名称均为 `shoulderWidth`、`torsoDepth`，没有眼睑 morph。

这说明当前测试内容是很轻的工程夹具。用它证明 WebGPU 快、正式角色质量好或眨眼完成，都会得到错误结论。生产测试必须加入真实贴图、头发透明材质、眼部和服装。

## 5. 平台支持与本轮实际探测

### 官方证据

WebKit 官方宣布 Safari 26.0 在 macOS、iOS、iPadOS、visionOS 支持 WebGPU，并说明它更直接映射 Metal。[S4] 这支持在现代 Apple 设备上评估该路线，但不能替代某个 WKWebView 宿主与设备的验证。

W3C WebGPU 定义将 `navigator.gpu` 放在 SecureContext 下，且 `requestAdapter()` 的结果允许为空。能读取 API、能拿到 adapter、能创建 device、能提交绘制是四个不同阶段。[S5]

### 实测方法与结果

在独立临时程序中，使用公开 WKWebView/WKURLSchemeHandler API、非持久数据存储，不配置私有安全协议选项。分别加载：

- `avatar://local/probe.html`
- 随包 `file://.../probe.html`

探测 `isSecureContext`、`navigator.gpu`、adapter/device、canvas context；可用时提交 32×32 canvas 的清屏 render pass，等待队列完成并检查 validation error。另建 canvas 检查 WebGL2。这个探测没有加载生产人物或运行整套 Three.js。

| 环境 | 页面来源 | 安全上下文 | WebGPU API | adapter | 结果 |
| --- | --- | --- | --- | --- | --- |
| macOS 27.0 / Build 26A428 原生 WKWebView | avatar 自定义协议 | true | 存在 | 成功 | device/context 成功，绘制提交完成，validation 为 null |
| 同上 | 本地 file | true | 存在 | 成功 | 同上 |
| iPhone 17 / iOS 26.5 Simulator | avatar 自定义协议 | true | 存在 | null | 未进入 device/绘制阶段 |
| 同上 | 本地 file | true | 存在 | null | 同上 |

构建工具为本机 Xcode 27.0 / 27A266a；这是实际环境，不能引用项目文档中的 Xcode 26.6 当成本轮环境。

上述四种环境/来源组合均成功取得 WebGL2 context。这仅证明底层 WebGL2 可用，尚未测试 WebGPURenderer 的完整 WebGL2 回退场景。

**必须据此修正的判断：自定义协议不必然被判为不安全，现有 avatar 协议不能被直接判定为 WebGPU 阻断。** 两种来源在模拟器都未取得 adapter，也不能把失败归因于自定义协议。原因未在本轮定位，不能据此推断真实 iPhone 的支持或帧率。

探测源代码和完整原始结果保存在本机忽略目录 `.firecrawl/webgpu-20260917/`，便于复查；它们不进入生产包。真机最低系统、能耗、长期运行、生产 GLB 及 Three.js 双后端未测试。

## 6. 推荐架构

```text
SwiftUI 页面／Next.js 页面
    │ 结构化角色配置、服装 recipe、交互、生命周期
    ▼
一个 Three.js 展示模块
    ├─ GLB / 资产清单 / 材质 / 眼部与动作 / 相机
    └─ WebGPURenderer
         ├─ WebGPUBackend：初始化成功时使用
         └─ WebGLBackend：初始化阶段回退
    ▼
都不可用：明确的原生／网页静态替代
```

这是一套场景逻辑、一个 WebGPURenderer 抽象下的两个 backend。WebGL 回退并不是把旧 WebGLRenderer 原封不动包在新 renderer 里；新的 WebGL backend 也需要独立验证。

Three.js r185 源码具有 `getFallback` 和 `forceWebGL`。[S2] 不要把这理解为所有运行中错误都自动恢复：device 丢失会进入 `renderer.onDeviceLost`，应用还需销毁/重建资源、恢复最后有效 recipe，必要时以低等级或 WebGL2 重建；反复失败要有界退出。[S6]

iOS 保留原生页面与业务状态，Three.js 只承担舞台，是合适的职责分配。Web 页面可用 Next.js 客户端组件挂载；同一帧的动画和眼球更新留在渲染模块，不通过 Swift bridge 或 React state 逐帧传输。跨端共享的是资产语义、材质与控制逻辑，不要求共享整套页面。

当前不需要为了 Three.js 引入 React Three Fiber、Unity 或另一套人物格式。如果未来核心转为原生 AR、复杂系统级 3D 交互或大型游戏场景，应重新比较原生/引擎方案；本报告不是所有产品场景下 Three.js 都最优的声明。

## 7. 实际迁移工作量与风险

| 工作 | 必须完成的内容 | 当前复杂度判断 |
| --- | --- | --- |
| 构建入口 | WebGPU build、TSL 和 addons 统一版本；离线打包、完整性与资源白名单 | 中 |
| 启动 | `await renderer.init()`；区分 backend 就绪、资产就绪、首帧、应用 revision；RAF 不得早于初始化 | 中 |
| 材质与光照 | 普通材质可映射；统一色彩/曝光；头发、睫毛、透明、双面和阴影逐项比对 | 中，依赖正式资产 |
| 自定义 shader | ShaderMaterial / RawShaderMaterial / onBeforeCompile 迁移到节点材质/TSL | 现有舞台暂无这类自定义代码，当前负担低 |
| 后处理 | 原 EffectComposer 不可直接搬到 WebGPURenderer；使用其节点管线 | 当前无旧后处理链，新增时逐项预算 |
| 生命周期 | 页面遮挡/后台停帧、device lost、WebContent 退出、恢复有效配置 | 中，需实测 |
| 内存 | 纹理、render target、几何、材质、mixer、异步加载失败和共享资源引用释放 | 中；生产资产比工程夹具更关键 |
| 资产、换装、Look | 三形象、眼部数据、衣物适配、原子替换、保存/取消/删除 | 独立主工作量，不能并入“换一行 renderer” |

WebGPURenderer 官方手册目前仍提示 experimental，以及上述 shader、后处理和性能差异。[S1] 这表示需要应用级验证，不等于不能用于有明确边界的生产产品。

版本也必须区分：本项目随包声明 `0.185.1`；本次 GitHub latest 返回 **r186，发布日期 2026-09-08**。r186 release 包括 PCFSoftShadowMap 移除相关说明，而当前舞台仍设置它。[S7] 建议先在同一 r185 代际做后端对照以隔离变量，再单独评估升级；若从零开工可选择验证后的新版本，不能把升级和后端切换的差异混在一起。

GitHub #34558 提供了 0.185.1 TRAA 首帧导致相机 aspect 异常的报告。[S8] 当前舞台没有该效果，因此不是当前已知缺陷；其价值是提示将来引入后处理时需要首帧和移动端回归。没有把社区报告当成本项目复现，也没有用旧版本的大场景性能问题推导本项目速度。

## 8. 推荐验证顺序与可验收输出

### 第一步：宿主与后端

在真实最低支持 iPhone 和一台主流 iPhone 上运行本轮同类探测，记录 OS/设备/来源/安全上下文/adapter/device/首帧。测现有 avatar 协议，不因猜测先重写离线资源服务。

之后用相同模型、灯光、画布尺寸与 DPR 比较三组：

- A：现有 WebGLRenderer。
- B：WebGPURenderer 的 WebGPU backend。
- C：WebGPURenderer 的 `forceWebGL: true` backend。

只有 B 才是 WebGPU 成功；“页面显示了”不足以证明没有回退。C 要独立记录效果、错误和性能，不能拿 A 的结果代替。

### 第二步：一个正式代表 Look

一个符合目标画风的人物、两件几何不同的上装、一下装、一双鞋；必须有可用眼睑 morph 和眼球节点、真实贴图与可核实使用权。

完成连续旋转/缩放、真实服装替换、眨眼、注视、Reduce Motion、前后台恢复。先确认画质和值得保留的效果，再扩三个角色。免费或付费来源都可以按同一交付标准评估；采用何种制作工具本身不决定 renderer 优劣。

### 第三步：画质与性能一起比较

以下是**建议的 POC 起始指标，不是行业标准、已批准需求或已达成成绩**：

| 项目 | 建议测法/起始目标 |
| --- | --- |
| 持续交互 | 主流机争取 60 fps；最低机稳定 30 fps，记录 p50/p95/p99 帧时间，不能只看平均 FPS |
| 首次可见 | 20 次随包冷启动采样，先以首个正确完整帧 p95 ≤ 2 秒作为讨论目标；初始化/解码/上传/编译分别计时 |
| 预热换装 | 资源在内存且材质已编译后，正确完整换装可见延迟争取 ≤ 300 ms；冷加载单列 |
| 长时间 | 正常亮度交互/待机至少 5 分钟，记录热状态、功耗与帧时间变化；不以模拟器替代 |
| 内存 | 记录 App、WebContent、GPU 相关占用与峰值；100 次换装/进出后检查是否收敛，不只看 JS heap |
| 失败与恢复 | adapter null、device lost、WebContent 退出、坏资产、快速切换、旧回调逐项注入/复现 |
| 后台/无障碍 | 不持续提交装饰动画；回到前台不追赶累计时间；Reduce Motion 下直接交互仍有效 |

画质质量档先控制 DPR、阴影尺寸、贴图与后处理；高像素密度手机不宜无条件使用最高 devicePixelRatio。可以从 DPR 1～1.5、单个主要投影灯、1K～2K 贴图与克制材质开始测，但最终值由可见质量和目标真机共同决定。高端设备再逐项加效果，不先写复杂 GPU 布料系统。

### 第四步：产品闭环与扩量

代表包和双后端通过后，完成三人物 × 八服装组合；正侧背至少 72 个静态检查，加上连续旋转与动画极值。验证托盘、草稿、已应用场景、已保存 Look 一致；切换失败保留上一有效 Look；重启恢复和删除通过真实存储确认。

这一步是产品能力验收，不由渲染器初始化成功替代。PNG 分享、视频导出、云端生成分别建立实际输出验证，不能由一次 canvas 展示推断。

## 9. 成本与最终采用标准

工程投入主要来自资产质量、服装适配、原生宿主稳定性和产品状态，而不是 `new WebGPURenderer()`。生产资源尚未确定，不能给可靠的完整开发人日；最有效的止损点是先做宿主真机探测和一个代表 Look。

满足以下条件时，将 WebGPURenderer 作为正式主线是合理的：

- 目标真机能够初始化；不可用设备的 WebGL2 后端保留旋转/换装/眼部等核心行为。
- 同一内容下，质量、首帧、内存和持续表现达到预算。
- 生产材质没有无法接受的双后端差异。
- 初始化、device lost 和 WebContent 恢复不丢失用户造型。
- 相对现有 WebGLRenderer，获得可见质量、可测性能或实际需要的新效果，或以可接受成本统一下一阶段材质开发。

如果测得新后端显著增加冷启动、能耗或稳定性成本，就让该设备继续使用更稳定的路径；这不否定 Three.js，也不证明要改变人物产品方向。选择应由实测收益决定，不能由“WebGPU 更新”或“旧项目已经固定”决定。

**最终建议：采用这条方向做新展示层；把 WebGPU 作为优先能力而非硬性设备门槛。最先交付一个真正好看的、能眨眼换装的角色，并以真机双后端测试决定发布配置。**

## 10. 一手来源与证据范围

- S1：[Three.js WebGPURenderer 官方手册](https://threejs.org/manual/pages/webgpurenderer.html)。Firecrawl 实时读取，包含 fallback、TSL、异步初始化、迁移限制与 experimental 提示。
- S2：[r185 WebGPURenderer](https://github.com/mrdoob/three.js/blob/r185/src/renderers/webgpu/WebGPURenderer.js)；[r185 StandardNodeLibrary](https://github.com/mrdoob/three.js/blob/r185/src/renderers/webgpu/nodes/StandardNodeLibrary.js)。GitHub 插件读取，核实 fallback、forceWebGL、材质映射。
- S3：[r185 skinning 示例](https://github.com/mrdoob/three.js/blob/r185/examples/webgpu_skinning.html)。GitHub 直接读取；Context7 使用 `/mrdoob/three.js` 检索 skinning/morph/材质示例，索引 dev 代码不冒充本地锁版实现。
- S4：[WebKit Features in Safari 26.0](https://webkit.org/blog/17333/webkit-features-in-safari-26-0/#webgpu)。Firecrawl 实时读取，平台公布时间为 2025-09-15。
- S5：[W3C WebGPU：NavigatorGPU / GPU](https://gpuweb.github.io/gpuweb/#navigatorgpu)。Firecrawl 实时读取；SecureContext 与 nullable adapter 来自规范。定向 query 提取失败后改为正文提取成功。
- S6：[r185 WebGPUBackend](https://github.com/mrdoob/three.js/blob/r185/src/renderers/webgpu/WebGPUBackend.js)。GitHub 读取 device lost 与 error 回调，不能推导自动运行中切换成功。
- S7：[Three.js r186 release](https://github.com/mrdoob/three.js/releases/tag/r186)。GitHub latest API 本轮结果；项目当前版本与最新版本分开记录。
- S8：[Three.js #34558](https://github.com/mrdoob/three.js/issues/34558)。GitHub 返回的用户报告，只是测试设计线索，不作为本项目缺陷或性能结论。

Apple Developer Forums 指定页面抓取返回安全验证页，未将其算作 WKWebView 支持证据。Firecrawl 开发索引返回的第三方设备/性能说法没有被当作一手结论。没有访问或上传真实人物照片，没有购买或生成资产。

本轮只增加研究报告与索引，未修改 App/Web 运行代码、依赖、现行选型或验收通过状态；未提交/推送。独立探测通过与失败均按环境记录，正式人物视觉、真实 iPhone、性能和完整产品交付仍未验收。
