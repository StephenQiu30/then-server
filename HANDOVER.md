# Then 项目交接

更新：2026-09-30。当前执行顺序已收敛为 POC 与最小 Demo；下方交付表保留截至 2026-09-23 的历史快照。实时状态以对应 [Plan](docs/plan/README.md)、[Acceptance](docs/acceptance/README.md) 和根目录 [Backlog](BACKLOG.md) 为准，接手时核对最新代码与工作区，不用旧快照代替当次测试。

当前接手优先读 [14-02 本地生成 POC 与最小 Demo](docs/plan/14-02-本地生成POC与最小Demo执行计划.md)：先证明一个实际图片→GLB 样本，再复用已有后端/Web 跑通 Demo。六套内容、完整 App、社区、同步等新扩展先延期；已有代码保留，真实验收标准不降低。

## 项目与仓库

Then（于是）是围绕穿搭决策、真实衣橱和每日记录的产品。`then-server` 保存 Go API、Next.js Web 和中央产品文档；同级独立仓库 [`then-app`](../then-app/README.md) 保存 SwiftUI iOS App。两个仓库各有自己的 `main`、CI 和工作区；父目录 `Then/` 不是 Git 仓库。共享视觉标准是两仓库根目录字节一致的 `DESIGN.md`。

| 要找的事实 | 入口 |
| --- | --- |
| 产品边界、分期和技术路线 | [PROJECT.md](PROJECT.md)、[产品总体设计](docs/design/03-OOTD产品总体设计.md)、[技术选型](docs/design/01-技术选型.md) |
| 功能与非功能需求 | [PRD 索引](docs/prd/README.md)，整体见 [PRD 10](docs/prd/10-OOTD产品需求.md) |
| 全局顺序、依赖、工作量与待办 | [BACKLOG.md](BACKLOG.md)；单项细节见 [Plan 索引](docs/plan/README.md) |
| 已验证结果与发布门禁 | [Acceptance 索引](docs/acceptance/README.md)，整体见 [系统验收](docs/acceptance/10-OOTD产品系统验收.md) |
| 开发和运行规则 | [服务端规范](AGENTS.md)、[后端说明](backend/README.md)、[前端说明](frontend/README.md)、[iOS 说明](../then-app/README.md) |

## 当前产品决定

默认体验是**无需账号、照片或网络的内置完整 Look**：三个人物，每人两套完整造型，图片优先，真实静态 GLB 用 Three.js 观察和切换。用户使用本人素材时，先生成可独立保存的完整穿搭图；只有主动要求三维时才生成整套静态 GLB，并按内容版本缓存。三维失败不得阻断图片、衣橱或记录。首版不以逐件三维换装、骨架/眼部动作或视频为前置；不使用 Blender。具体合同见 [11-05](docs/plan/11-05-三维人物与造型闭环执行计划.md) 与 [14-01](docs/plan/14-01-完整穿搭图与按需三维生成执行计划.md)。

Look、拥有的衣物、穿搭计划、实际穿着、穿后反馈和私人日记是不同事实；保存图片或 Look 不自动产生“拥有”或“已穿”记录。云功能需要真实账号、逐项用途同意、owner 隔离、费用/任务恢复和删除链路。社区内容须经审核后公开，私人内容默认不公开。

## 截至 2026-09-23 的交付状态

| 范围 | 已有成果 | 尚未完成，不能宣称已交付 |
| --- | --- | --- |
| 产品文档 | 当前路线已拆为 Design、PRD、切片 Plan、Acceptance 和全局 Backlog；需求、非功能要求、顺序及验收门禁有追踪入口 | 文档通过不代表新 Look、云生成或完整产品已通过验收 |
| iOS 本地日常能力 | 真实本机衣橱、单件图片、确认属性、基础推荐、计划、实际穿着、结构化反馈及对应开发/模拟器证据；已有隔离 Three.js 工程舞台 | 多个切片仍缺最低真机、VoiceOver、数据保护与恢复发布验收；推荐锁件/换件/保存闭环 `13-02` 未实现 |
| 离线完整 Look | 图片/GLB/保存和渲染的目标合同已获批准；现有工程 GLB 可用于验证渲染底座 | 代表性正式带纹理资产、三人物六套内容、Look 持久化、整套切换、真实 GLB 观察和设备验收均为 `11-05` 待办；工程模型不能充当正式内容 |
| Go API 与数据 | 注册登录、会话、成年声明、账号衣橱/计划/实际、私人日记、社区审核/发现/互动/通知、可靠删号等后端开发切片已有实现和对应证据；Huma 运行时 OpenAPI 是唯一接口源 | 云图片/三维生成任务、Provider 对账和费用/删除闭环未实现；既有 API 不等于 App/Web 已接入或生产运营已就绪 |
| Web 与云端客户端 | Next.js 基础工程、设计 token、shadcn/Radix 基础组件与运行时 OpenAPI 生成链路已建立 | 账户业务页面仍待 `17-13` 合同批准和实现；iOS 云账号 Client `17-30`、私人日记 `19-07`、社区页面与治理 `19-08` 均待实现 |

本次交接依据的双仓库设计基线是服务端 [`9679fbe`](https://github.com/StephenQiu30/then-server/commit/9679fbef421a9990e4f7416302e36f7e769d0661) 与 App [`0f7498c`](https://github.com/StephenQiu30/then-app/commit/0f7498c62fc482ad2dae424db42dbd55aa57036b)：前者同步产品文档与 Backlog，后者同步路线说明；对应 [服务端 CI](https://github.com/StephenQiu30/then-server/actions/runs/35810701692) 和 [iOS CI](https://github.com/StephenQiu30/then-app/actions/runs/35810712181) 均通过。此证据仅覆盖那次提交与 CI，不能替代后续设备、供应商或发布验收。

## 接手后的执行顺序

1. 完成 `14-02-BASELINE-01`，核对主仓库与并行生成 worktree，保留未提交工作；落实授权代表图、真实工具/运行合同、网络/费用边界及隔离依赖。不得重做或覆盖已交付后端。
2. 复用 `14-01-LOCAL-POC-01` 先验证实际图片→img2threejs→带纹理静态 GLB，记录全链路 hash、合规、预算及视觉结果。缺图片生成服务时明示参考图子范围；模型不可行则先给出 POC 结论，不扩页面/六套或自动改云。
3. 模型通过后执行 `14-02-DEMO-01/VERIFY-01`：复用现有 Web/认证/API/PG/私有 MinIO，验实际结果观察、重复复用、重启、隔离和取消/删除。fixture 只作回归，不代替真实工具能力。
4. `14-02-DECISION-01` 按 [POC/Demo 验收](docs/acceptance/14-AI虚拟试穿验收.md#当前-poc-与-demo-验收) 回写通过范围与 go/no-go，再选择一个后续产品切片。11-05、完整14-01、17-30及19客户端等原需求与 [系统验收](docs/acceptance/10-OOTD产品系统验收.md) 标准保留，未验的正式内容/真机/生产门禁不自动解除。

真实供应商的地域、许可、费用、受理幂等和删除能力；正式资产及权利；最低真机/人工无障碍；公开注册、社区运营与生产保留策略，都是对应切片或发布的门禁。尚未验证时保留 `pending`/`blocked`，不要用模拟器、mock、工程资产或绿色 CI 替代。

## 接收检查

- [ ] 分别检查 `then-server` 与 `then-app` 的分支、未提交改动、远端提交和最新 CI；保留已有工作，不跨仓库混合提交。
- [ ] 阅读当前工作包的 Design → PRD → Plan → Acceptance，核对 Backlog 中的 ID、依赖和勾选状态。
- [ ] 确认代表资产/设备/测试环境或 Provider 是否具备，记录缺口及实际责任人；凭据、私人照片和本地测试产物不进仓库。
- [ ] 实现后先回写原 Plan 与 Acceptance 的证据，再同步 Backlog；开发通过、设备通过和发布通过分别记录。

接收检查是接手动作，不是产品功能的第二套完成账；完成状态只在原切片和验收事实中认领。
