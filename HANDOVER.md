# WEARLOG 项目接手

本地父目录为 `/Users/stephenqiu/Desktop/StephenQiu/Wearlog`，包含独立 Git 仓库 wearlog-server 与 wearlog-app。正式名称是衣序 / WEARLOG。先检查各仓库 Git 状态，保留并行改动；不要使用历史分支、服务或 CI 快照推断当前状态。

当前 Figma 已重新设计，入口与 27 个画板见 [Figma MVP 设计验收](docs/validation/Figma-MVP设计验收.md)。Web/iOS 已完成名称和 W 标识迁移；无边框人物展台、直接选衣、手机选衣面板与对比主线已在当前 Web 工作区实现；15 项生产浏览器回归通过，证据见 [页面验收](docs/validation/试衣间页面验证.md)。其余画板与真实生成不能由本次主线实现推断完成。命名迁移、备份和 worktree 保留证据见 [命名迁移](docs/validation/WEARLOG命名迁移.md)。

- 工程边界：[PROJECT.md](PROJECT.md)，运行方法：[后端](backend/README.md)、[Web](frontend/README.md)、[iOS](../wearlog-app/README.md)。
- 共享视觉：[DESIGN.md](DESIGN.md)，与 iOS 同名文件保持一致。
- Web 组件按 PROJECT 4.3 的设计职责命名：View/Form/Layout/Picker/Preview/Dialog；收藏与详情已拆分到 components/look，旧名称与路径不保留兼容层，架构测试约束命名和数据边界。
- 当前待办：[BACKLOG.md](BACKLOG.md)。最新版 Figma 的 Web 页面与品牌已获授权实现；真实试穿与环绕生成仍未接入。

2026-10-13 使用 pm-market-research 更新七份需求文档（入口及01～06）：明确人物展示→选衣试穿→对比→保存→实穿记录的目标，补齐衣物、系统与非功能合同。优先级/指标为建议，外部用户访谈与真实试穿质量尚未验证；待决策 D-01～05 见 [需求入口](docs/requirement/README.md)。下述二维与示例页面是既有实现记录，不是产品需求上限。

随后完成 [MVP 范围与实现差距](docs/requirement/07-MVP范围与实现差距.md)：只读核对 Figma 31 个顶层画面与当前未提交工作区；核心缺口是真实试穿、账户内 Look 及二者与衣橱/实穿的贯通。复用既有任务、导出和记录服务，补全库检索、单品明细与衣物照片导出覆盖；日历/自动拆衣/高级统计后置，环绕首发条件仍待决定。本次仅维护文档，没有重跑运行测试或开始产品代码实现。

2026-10-13 品牌设计已更新：新 Logo 与导出文件在 `docs/assets/brand/`，规范见共享 DESIGN。蓝白只作品牌点缀，按钮、选中、导航与表单保持 Vercel/shadcn 黑白灰。Web 导航、认证页与浏览器图标已接入，iOS 运行时资源仍待迁移。Web 同步完成单件选择、独立对比示例、本地收藏列表/详情和实穿列表优先布局；当前证据见 [页面验证](docs/validation/试衣间页面验证.md)，真实账户 Look 与生成缺口保留。

文档统一入口见 [docs](docs/README.md)。当前人物研究见 [可行性结论](docs/research/人物展示可行性.md)：Woo 作者确认视频帧机制，但连续素材生产、实际成本与用户衣物保真未通过验证。既有人物实验已汇总，不继续 img2threejs，不把四视角切换当连续视频。2026-10-13 已移除旧人物实验入口、脚本、专用依赖与本地演示，恢复位置见文档入口；没有新生成或采购。当前 [Figma 试衣间](https://www.figma.com/design/DgasIy7ulxxb1MWmew76Zs/?node-id=65-141) 以人物展示、选衣试穿和前后对比为核心，核心流程已落到 Web 首页：人物展示、选衣、前后对比、本地照片预览和示例草稿保存；使用成对设计素材，不发起生成请求。真实衣橱/计划/记录仍由下面的既有功能承接，31 个设计画面未全部重建。下列工程状态仍需接手核对。

2026-10-12 已完成五张现有 Codex 示例素材的图片衣橱、筛选、固定平铺单件替换、日期计划及实穿/纠正统计。当前本机 Web 为 `http://127.0.0.1:3100`，API 为 `127.0.0.1:8080`，预览数据在独立开发库 `then_wardrobe_2d`；不要与其他已有开发库混用。页面使用合成账户和验证记录。继续前检查进程与数据库，不凭此记录认定服务仍在运行。

真实 API/数据库与浏览器证据、素材和统计边界见 [二维最小交付](docs/validation/25-二维衣橱与搭配最小交付.md)。真实照片上传、补图和换图已完成本地实际闭环；运行需启用现有本地媒体 all 角色及 MinIO/Kafka，见 [照片上传与补图](docs/validation/26-真实衣物照片上传最小交付.md)。完整统计详情及实际设备验收仍待继续；全量旧媒体集成被 Quay MinIO 镜像 401 阻塞。未提交或推送本轮改动。

照片整理已使用浏览器 U2netp/ORT CPU 实测抠图、四边裁剪及原文件分别保留，见 [本地抠图与裁剪](docs/validation/27-本地抠图与简单裁剪最小交付.md)。新衣橱 ready 源文件不再被通用清扫器提前清理；旧源已清理时仅可使用净化图并有提示。前端 predev/prebuild 从固定 npm 包复制本地 WASM 资产，不另启动 Python/GPU 服务。

2026-10-13 随后按用户授权完成 [Figma MVP 设计验收](docs/validation/Figma-MVP设计验收.md)：当前同页 56 个顶层画面（含状态规范），已在 Chrome 点通桌面主线；文档按 pm-execution 的 PRD/场景方法和 pm-toolkit 的文字逻辑检查整理。结果图仍为历史素材占位，真实单件效果与环绕没有新验收；产品代码未改。后续实施应以本次节点和 07 最新设计覆盖表为准，保留原实现差距。
