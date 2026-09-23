# Three.js、Tripo 与图片转三维研究

核查：2026-09-22。当前采用方向为 **完整穿搭图 → Tripo 静态 GLB → Three.js**；生产供应商与样本验收仍待完成。主设计归 [Design 20](20-OOTD完整产品能力与阶段架构设计.md)，本文件只保存可核实证据，不另立人物基线。

## 三种名称必须区分

| 项目 | 官方能力/源码证据 | 对本项目的意义 |
| --- | --- | --- |
| [Tripo 云 API](https://developers.tripo3d.ai/en/docs/introduction) | 图片/多视图转模型、纹理、自动绑骨、动作、分割/补全等独立接口 | 使用图片转带纹理 GLB 即可；其他调用不是展示前置 |
| [TripoSR](https://github.com/VAST-AI-Research/TripoSR) | 开源单图三维重建；代码/权重许可见仓库 | 自部署几何实验，不等于 Tripo 云服务或完整人物/服装系统 |
| [img2threejs](https://github.com/img2threejs/img2threejs/tree/6e60b5e22419464b4853e01ddb6c0e6f6659a733) | 将视觉目标变成程序化 Three.js 场景，另有 GLB/character 插件 | 不需要加入 Then 生产链，不能把名称理解成图片直接得到可换装角色 |

GitHub 核对 img2threejs main `6e60b5e22419464b4853e01ddb6c0e6f6659a733`：主线输出 TypeScript/THREE.Group；人物示例仍有占位内容，img2glb 插件调用托管 TRELLIS，character 插件消费既有 rigged GLB，未提供可靠的人物/衣物资产生产链。仓库可运行不等于已经解决穿搭保真、同骨架衣物或眼部资产。

## Three.js 加载与观察

Context7 对 `/mrdoob/three.js` 的文档核对表明：GLTFLoader 返回 `gltf.scene`，将其加入场景即可观察静态 mesh；改变对象或相机旋转无需 skeleton 或 AnimationMixer。只有消费模型动作 clips 才需动画播放管理；骨骼形变要求模型本身提供 skin、关节和权重。[GLTFLoader 官方文档](https://threejs.org/docs/#GLTFLoader)

Then 保留 Three.js `0.185.1`、WebGL2、隔离 WKWebView 和离线依赖。不能因生成器变化引入 Unity、RealityKit、VRM 或 WebGPU 必需门槛。风格、脸、衣物轮廓来自资产与灯光取景，换渲染器不自动修复这些问题。

## Tripo 补充评估（2026-09-22）

| 能力 | 已核查边界 | 首片决定 |
| --- | --- | --- |
| [单图转模型](https://developers.tripo3d.ai/en/docs/generation-image-to-model) | H3.1 `v3.1-20260211`，可指定纹理、面数等 | standard texture、triangle；约 2 万面起测 |
| [多视图](https://developers.tripo3d.ai/en/docs/generation-multiview-to-model) | front 必须存在，至少两个有效视图；各图需同人物/姿态/穿搭 | 有真实一致参考时再用，不额外合成三图作为固定前置 |
| [图像编辑](https://developers.tripo3d.ai/en/docs/generation-image-to-image) | Seedream v5 支持最多四个参考输入 | 可测试一人物加三衣物；多参考能力不保证细节保真 |
| [绑骨](https://developers.tripo3d.ai/en/docs/animations-rig)、[动作](https://developers.tripo3d.ai/en/docs/animations-retarget) | 云端有独立能力，不能以 TripoSR 缺少动画否定它 | 当前不调用；也不能据此保证眼睑 morph/眼球节点 |
| [分割](https://developers.tripo3d.ai/en/docs/mesh-segment)、[补全](https://developers.tripo3d.ai/en/docs/mesh-complete) | 部件/语义分割与补全有独立参数和成本 | 不能直接推导得到兼容的换装衣物；当前不调用 |
| [转换](https://developers.tripo3d.ai/en/docs/models-convert) | 输出格式/方向/压缩有明确约束 | 首片保留普通 GLB，避免额外转换成本和解码器 |

参数注意：quad 会改变输出格式为 FBX，不能送入现有 GLTFLoader 路线；parts 与 texture/PBR 等有组合限制，smart low-poly 下也不应假设一定产出部件。首片全部关闭这些非必要选项。使用压缩或 export orientation 前依官方任务阶段约束验证，避免成功状态掩盖格式/方向错误。

## 当前代码需要改变什么

2026-09-22 核对：App 的 AvatarAssetCatalog 只接受八个工程资产、固定单 mesh/skin/material 与 rig，拒绝 images/textures/animations；JS 通过 visibility 切模块，未消费导入动画 clips。当前仅能证明工程纵切。新路线需允许经过验证的内嵌纹理和多 mesh/material、整套模型加载/释放、缩放/复位、原子切换及版本化缓存；不能直接把 Tripo 文件换进去或关闭校验。

固定姿势 GLB 无需新增动画系统。已完成的整体微动/生命周期可复用，Reduce Motion 下直接操作仍保留。旧 MetaPerson 样例证明过眼部输入的技术可能性，但不再是当前首版资产采购或开工依赖；Avaturn/VRM/模块衣物保留为高级角色能力的后续研究。

## 服务使用边界与结论

[Tripo API 条款](https://developers.tripo3d.ai/en/terms)（页面条款日期 2025-07-11，2026-09-22 核查）区分免费与付费产物权利；向终端用户提供生成服务的条款要求事先书面许可。正式应用前核实适用合同、分发权、地域、人物处理与删除；本次研究不是法律许可或供应商准入结果。

费用、调用选择、失败放大系数统一见 [Design 20](20-OOTD完整产品能力与阶段架构设计.md#供应商选择与成本)。没有调用付费接口，没有真实 Tripo 产物在最低真机完成验收。现阶段可以确认**架构适合优先验证**，不能确认特定输入必然产出合格人物、真实背面、兼容衣物或自然眼部动画。
