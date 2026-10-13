# WEARLOG 项目接手

本地父目录为 `/Users/stephenqiu/Desktop/StephenQiu/Wearlog`，包含独立 Git 仓库 wearlog-server 与 wearlog-app。正式名称是衣序 / WEARLOG。先检查各仓库 Git 状态，保留并行改动；不要使用历史分支、服务或 CI 快照推断当前状态。

当前 Figma 已重新设计，入口与 27 个画板见 [Figma MVP 设计验收](docs/validation/Figma-MVP设计验收.md)。Web/iOS 已完成名称和 W 标识迁移；无边框人物展台、直接选衣、手机选衣面板与对比主线已提交至 main；15 项生产浏览器回归通过，证据见 [页面验收](docs/validation/试衣间页面验证.md)。其余画板与真实生成不能由本次主线实现推断完成。命名迁移、备份和 worktree 保留证据见 [命名迁移](docs/validation/WEARLOG命名迁移.md)。

- 工程边界：[PROJECT.md](PROJECT.md)，运行方法：[后端](backend/README.md)、[Web](frontend/README.md)、[iOS](../wearlog-app/README.md)。
- 共享视觉：[DESIGN.md](DESIGN.md)，与 iOS 同名文件保持一致。
- Web 组件按 PROJECT 4.3 的设计职责命名：View/Form/Layout/Picker/Preview/Dialog；收藏与详情已拆分到 components/look，旧名称与路径不保留兼容层，架构测试约束命名和数据边界。
- 当前待办：[BACKLOG.md](BACKLOG.md)。最新版 Figma 的 Web 页面与品牌已获授权实现；真实试穿与环绕生成仍未接入。

2026-10-13 使用 pm-market-research 更新七份需求文档（入口及01～06）：明确人物展示→选衣试穿→对比→保存→实穿记录的目标，补齐衣物、系统与非功能合同。优先级/指标为建议，外部用户访谈与真实试穿质量尚未验证；待决策 D-01～05 见 [需求入口](docs/requirement/README.md)。下述二维与示例页面是既有实现记录，不是产品需求上限。

当前 [MVP 范围与实现差距](docs/requirement/07-MVP范围与实现差距.md) 已更新为 server `f1fe12f` / app `3a61235` 的已推送代码基线，Figma 以新版 27 个无边框画板为准。真实试穿、账户 Look、全库检索、单品明细与导出关系仍是缺口；[首版技术实现方案](docs/research/首版技术实现方案.md) 明确模块、输入转换、版本、真实 provider、视频合同和实施门槛。随后修复 PostgreSQL 首次写入与重放的时间精度差异；代码与 CI 证据见 [交付记录](docs/validation/main交付与文档复核.md)。

2026-10-13 品牌设计已更新：新 Logo 与导出文件在 `docs/assets/brand/`，规范见共享 DESIGN。蓝白只作品牌点缀，按钮、选中、导航与表单保持 Vercel/shadcn 黑白灰。Web 导航、认证页与浏览器图标已接入，iOS 运行时标识与资源已迁移，bundle/数据库身份保持。Web 同步完成单件选择、独立对比示例、本地收藏列表/详情和实穿列表优先布局；当前证据见 [页面验证](docs/validation/试衣间页面验证.md)，真实账户 Look 与生成缺口保留。

文档统一入口见 [docs](docs/README.md)。当前人物研究见 [可行性结论](docs/research/人物展示可行性.md)：Woo 作者确认视频帧机制，但连续素材生产、实际成本与用户衣物保真未通过验证。既有人物实验已汇总，不继续 img2threejs，不把四视角切换当连续视频。2026-10-13 已移除旧人物实验入口、脚本、专用依赖与本地演示，恢复位置见文档入口；没有新生成或采购。当前 [Figma 试衣间](https://www.figma.com/design/fYWIHuNucsMQBNYzXBPuEq/?node-id=5-4) 以人物展示、选衣试穿和前后对比为核心，核心流程已落到 Web 首页：人物展示、选衣、前后对比、本地照片预览和示例草稿保存；使用成对设计素材，不发起生成请求。真实衣橱/计划/记录仍由下面的既有功能承接，27 个最新画板的全部业务未交付。下列工程状态仍需接手核对。

2026-10-12 已完成五张现有 Codex 示例素材的图片衣橱、筛选、固定平铺单件替换、日期计划及实穿/纠正统计。当前本机 Web 为 `http://127.0.0.1:3100`，API 为 `127.0.0.1:8080`，预览数据在独立开发库 `then_wardrobe_2d`；不要与其他已有开发库混用。页面使用合成账户和验证记录。继续前检查进程与数据库，不凭此记录认定服务仍在运行。

真实 API/数据库与浏览器证据、素材和统计边界见 [二维最小交付](docs/validation/25-二维衣橱与搭配最小交付.md)。真实照片上传、补图和换图已完成本地实际闭环；运行需启用现有本地媒体 all 角色及 MinIO/Kafka，见 [照片上传与补图](docs/validation/26-真实衣物照片上传最小交付.md)。完整统计详情及实际设备验收仍待继续；历史媒体集成曾被 Quay MinIO 镜像 401 阻塞；当前 main 的重新检查见交付记录，不把旧失败继续视为当前结论。

照片整理已使用浏览器 U2netp/ORT CPU 实测抠图、四边裁剪及原文件分别保留，见 [本地抠图与裁剪](docs/validation/27-本地抠图与简单裁剪最小交付.md)。新衣橱 ready 源文件不再被通用清扫器提前清理；旧源已清理时仅可使用净化图并有提示。前端 predev/prebuild 从固定 npm 包复制本地 WASM 资产，不另启动 Python/GPU 服务。

旧 Figma 31/56 个画面的核对属于同日历史过程；其节点只保留为需求审查证据。后续设计与实现均使用新版文件及 27 个画板，不从旧日志恢复过期页面。
