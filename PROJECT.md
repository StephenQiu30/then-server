# Then 项目规范

更新：2026-09-22。适用范围：`then-server/frontend`、`then-server/backend` 及两者共享的产品与接口约束。

本文固定产品边界、Web 设计实现规则和前后端目录职责，作为后续开发的总入口。规则自本次文档确认生效；目标目录、组件和验收要求不代表代码已经实现。规范确认与功能实施分别记录；当前 Kafka 和入口简化的实施范围见 17-26，Web 基础组件和必要文件同步见 17-27，账户业务页面另按 17-13 推进。

## 1. 文档职责与事实来源

| 文件或位置 | 唯一职责 |
| --- | --- |
| `PROJECT.md` | 跨前后端工程基线、组件体系、目录与交付约束 |
| [DESIGN.md](DESIGN.md) | App/Web 共享视觉语言、颜色、字体、间距、圆角和响应式；不在本文复制第二套数值 |
| [技术选型](docs/design/01-技术选型.md) | 技术路线、启用条件；安装版本以 package/lockfile、`go.mod/go.sum` 核实 |
| [需求索引](docs/prd/README.md) / [设计索引](docs/design/README.md) | 各业务需求、状态、数据与接口设计 |
| [计划索引](docs/plan/README.md) / [验收索引](docs/acceptance/README.md) | 单切片任务、实际完成状态与验证证据 |
| [Backlog](BACKLOG.md) / [交接](HANDOVER.md) | 全局执行顺序与待办视图 / 标注日期的接手快照；不替代单切片事实 |
| [AGENTS.md](AGENTS.md)、[前端细则](frontend/AGENTS.md)、[后端细则](backend/AGENTS.md) | 对应目录的执行规则，与本文保持一致 |
| `backend/internal/adapter/httpapi` | Huma operation、HTTP DTO 与运行时 OpenAPI 的声明源 |
| `backend/internal/adapter/postgres` | GORM record、开发 schema 与事务实现 |

用户当前要求优先。工程规则读本文，具体视觉数值读 `DESIGN.md`，业务范围读对应 PRD/Design，完成情况读 Plan/Acceptance。发现冲突应在同一变更中修正所属事实源和入口，不能默默选一份继续实现。变更共享视觉标准时，按既有规则同步 `then-app/DESIGN.md`；Web 专属工程规则不复制到 iOS。

## 2. 固定产品需求与实施边界

Then（于是）是穿搭决策与记录产品。以下是既有需求的约束摘要，详细字段、状态和启用条件沿用 [PRD 10](docs/prd/10-OOTD产品需求.md)、[PRD 19](docs/prd/19-每日记录与穿搭社区需求.md) 和 [完整产品架构](docs/design/20-OOTD完整产品能力与阶段架构设计.md)。

| 能力 | 必须保留的需求 | 交付边界 |
| --- | --- | --- |
| 默认穿搭 | 三套人物预设的完整 Look 图片/静态 GLB、整套切换、保存与日期；默认 App 路径零账号、零上传、离线可用 | Web 不因采用同一设计体系而自动承诺离线/PWA 或完整 App 功能 |
| 真实衣橱 | 用户自愿建档、编辑、筛选、归档和删除；内置模板与实际拥有衣物分离 | 未知属性不得补成事实 |
| 推荐与记录 | 基于真实可用衣物的可解释推荐；计划、实际穿着、反馈、Look 分别记录 | 保存搭配或日记不自动产生实际穿着事实 |
| 私人日记 | 图文记录、原日期/时区、日历回顾，可独立于衣橱与穿着创建 | 私人内容默认不公开 |
| 图文社区 | 明确发布、审核、发现/关注、赞藏、评论、通知、举报/屏蔽与申诉 | 发布前具备治理闭环；公开 API 不等于社区页面已交付 |
| 账户与隐私 | 注册/登录、本人资料、会话控制、导出与删除 | 后端授权是最终判定，页面隐藏按钮不能替代鉴权 |
| AI 与三维 | 完整穿搭图优先，用户按需生成整套静态 GLB，Three.js 查看与版本缓存；视频/模块换装/眼部后移 | 11-05 离线 Look、14-01 图片/模型任务分片验收；图可独立保存，模型失败不阻塞；继续禁止 Blender |

**当前 Web 首个业务切片固定为 `/register`、`/login`、`/account`**，依据 [Web 账户管理设计](docs/design/16-Web账户管理设计.md)。其他能力进入各自页面设计和实施切片后再建设，不提前建立占位路由。本文不把现有 draft 计划升级为已验收功能。

不新增商城、广告、订阅计费、私信、直播、任意模型上传或通用工作流平台。新业务先更新所属需求，不以组件库示例或脚手架模板扩大范围。

## 3. 固定技术基线

| 层 | 决定 | 约束 |
| --- | --- | --- |
| Web 框架 | Next.js App Router + React + TypeScript strict | 保留现有工程；不另建 Vite、Pages Router 或第二个 Web 应用 |
| Web 设计实现 | shadcn/ui + Radix UI Primitives + Tailwind CSS v4 | shadcn 基础类型固定 `radix`，不混入 Base UI/React Aria 基础类型 |
| 图标 | 统一使用 Lucide，components.json 已固定 | 已接入；不在同一 Web 产品混用多套图标 |
| 服务端状态 | TanStack Query | 沿用现有 Provider，不再增加 SWR 或并行的请求缓存体系 |
| HTTP 客户端 | `@umijs/openapi` + Axios | 从运行中 Go API 的 `/openapi.json` 生成 `src/api/` |
| Web 质量 | ESLint + Prettier + TypeScript + Node.js Test Runner | 沿用 npm 与 `package-lock.json`；UI 需浏览器交互验收 |
| 后端 | Go + Gin/Huma + GORM + PostgreSQL | 单 module、模块化单体、一个二进制与 OCI 镜像 |
| 基础设施 | Redis、Kafka、MinIO | 各自只承担已实现用途，按切片启用；PostgreSQL 保存业务事实 |

Vercel 插件的 Next.js/React 指南用于框架边界与性能规范。采用这些指南不改变已有 Go 服务部署方式，也不要求现在迁移到 Vercel 托管。

### 3.1 shadcn 与 Radix 的关系

shadcn/ui 提供项目内可维护的组件源码，Radix Primitives 提供交互与无障碍基础，Tailwind/语义 token 承接 Then 视觉。业务页面统一从 `@/components/ui/*` 导入基础组件；缺少组件时先查官方 registry，再做必要组合，不直接另造 Button、Dialog 或 Select。[官方 Next.js 接入说明](https://ui.shadcn.com/docs/installation/next)

17-27 已通过官方 CLI 初始化 `radix-nova`，配置固定 Radix、Lucide、RSC 与 @/* 别名，实际接入 Button/Badge。旧 `@radix-ui/themes`、Theme Provider 和样式已移除；QueryClient Provider 保留。新增组件继续用 npm runner 显式核对 radix，再按实际使用从官方 registry 添加，不提前安装整套组件库。

## 4. 前端设计规范

### 4.1 视觉与 token

1. 遵循 `DESIGN.md` 的内容/影像优先、留白、单一品牌行动色与轻量层级。页面结构按 Then 的功能设计组织，不能照搬参考站点的品牌、营销导航或电商文案。
2. 颜色、字体、间距、圆角和焦点样式在 `src/app/globals.css` 集中映射；Tailwind v4 使用 `@theme inline`。组件消费 `bg-background`、`text-foreground`、`text-muted-foreground` 等语义类，不在页面写十六进制、原始色阶或逐组件 `dark:` 色彩覆盖。[shadcn 主题规范](https://ui.shadcn.com/docs/theming)
3. 默认浅色。暗色目前缺少完整设计验收，不因 shadcn 默认模板而自动增加切换入口；启用前在同一 token 体系补齐状态并验收。
4. 先使用组件已有 `variant`/`size`；新增视觉变体归组件定义，调用方 `className` 用于布局。表面以间距、排版和背景区分，不把每个区块都套 Card，不加装饰性渐变和阴影。
5. 字体沿用 `DESIGN.md` 的系统字体回退；不为了套用 Vercel 示例改成另一套品牌字体。加载自有字体时使用 `next/font` 并控制字体体积。
6. 使用 `flex/grid` 与 `gap-*`，不使用 `space-x-*`/`space-y-*`；等宽高用 `size-*`，省略文本用 `truncate`，条件类使用统一 `cn()`。

基础语义映射固定如下，具体值从 `DESIGN.md` 取得：

| shadcn token | Then 语义 |
| --- | --- |
| `background` / `foreground` | 页面画布 / 主正文 |
| `card`、`popover` 与对应 foreground | 内容与浮层表面 / 其上正文 |
| `primary` / `primary-foreground` | 主要行动色 / 行动色上的文字 |
| `secondary`、`muted`、`accent` | 次级表面、弱化区域、选中或强调表面 |
| `border`、`input`、`ring` | 分隔线、输入边界、键盘焦点 |
| `destructive` | 删除及不可逆操作，已在 DESIGN 的 Then Web foundation 补齐 |

`accent` 不另起第二个品牌色；destructive/错误 token 已补齐；新增成功等状态色须先补齐 `DESIGN.md` 后统一映射，不在页面各自选色。弱化正文仍需可读，不能直接把 disabled 颜色用于重要说明。

### 4.2 组件与表单

| 场景 | 统一用法 |
| --- | --- |
| 主次操作 | `Button` 的已有变体；加载用 `Spinner`、`data-icon`、`disabled`，不虚构 `isLoading` 属性 |
| 表单 | `FieldGroup` + `Field` + 关联标签；说明/错误留在字段附近 |
| 复合输入 | `InputGroupInput`/`InputGroupTextarea` 配合 `InputGroupAddon` |
| 少量平级选项 | `ToggleGroup`；复选/单选字段组使用 `FieldSet` + `FieldLegend` |
| 模态与侧栏 | `Dialog`/`Sheet`/`Drawer`，必须提供对应 Title |
| 危险确认 | `AlertDialog`，说明对象、后果与提交状态 |
| 列表与空态 | 按内容选择列表/Table；空态 `Empty`，加载 `Skeleton` |
| 状态与分隔 | `Badge`、`Separator`、`Progress`；不用临时 div 模拟 |
| 简短完成提示 | Radix 路线统一 Sonner |

`data-invalid` 放在 Field，`aria-invalid` 放在控件，错误与控件关联；disabled 同时表达容器状态和控件禁用。必填项有可见标签，placeholder 不代替标签。[Field 官方说明](https://ui.shadcn.com/docs/components/radix/field)

Select/Menu/Command 的 Item 放入对应 Group，TabsTrigger 放入 TabsList；Avatar 配备 Fallback；需要 Card 时按 Header/Title/Description/Content/Footer 的内容职责组合。按钮图标使用 `data-icon`，大小交给组件，不在每个调用点覆盖。

Radix 自定义触发器使用 `asChild`，承接的叶子组件必须传递 props 和 ref，避免嵌套 button；不混用 Base UI 的 `render` API。焦点陷阱、关闭恢复、键盘行为优先由 Radix 管理，不自行改写，也不为浮层随意堆叠 z-index。[Radix 组合说明](https://www.radix-ui.com/primitives/docs/guides/composition)

### 4.3 状态、响应式与无障碍

- 每页明确正常、首次加载、刷新、空数据、可恢复失败、未认证、无权限和提交中状态；失败保留用户输入，重试回到原上下文。
- 需持续处理的问题用 `Alert` 与明确动作；字段问题就地显示；路由渲染失败由 `error.tsx` 承接；短暂成功提示用 Sonner。Toast 不作为错误和删除进度的唯一载体。
- 主要操作有防重复提交，删除必须确认；收到异步删除 `202` 只能显示处理中，不能显示全部数据已清除。
- 按 `DESIGN.md` 的响应式规则实现，在 320/390、768/834、1024/1440px 及关键断点两侧检查。小屏单列，内容不横向溢出；不使用 `overflow-x: hidden` 掩盖布局问题。表格确需横向滚动时限定在表格容器内。
- 移动操作命中区至少 44 × 44px；键盘可完成表单、菜单和确认，关闭浮层回到触发点。按钮有可访问名称，图片按用途提供 alt，状态不只依赖颜色，支持减少动态效果与文字放大。[Radix 无障碍说明](https://www.radix-ui.com/primitives/docs/overview/accessibility)

### 4.4 Next.js 与状态管理

- Page/Layout 默认 Server Component；事件、状态、浏览器 API 放到最小必要 Client Component。共享 Client Provider 位于 `src/providers`，不能将整个根 layout 标记为客户端来规避边界。
- 独立请求并行发起；合理使用路由 loading/Suspense，避免组件层层串行等待。三维等重组件在实际进入时动态加载，不进入账户页首屏包。
- Client Component 通过业务 hooks 调用生成客户端，TanStack Query 管理请求状态；字段临时输入归表单本地状态，筛选/分页按需归 URL，不复制多份远端实体到全局 store。
- 私人账户、日记、衣橱与签名媒体不得进入公共 CDN/ISR 或跨用户全局缓存；服务端请求必须带当前请求的会话上下文，不能修改全局 Axios 默认 Cookie。若需要 SSR 请求，先扩展同一请求适配器并验证并发用户隔离。
- 登出或切换身份时清理对应客户端缓存；写入成功后精确失效相关 query。乐观更新仅用于可回滚动作，删除与审核不能虚报完成。
- 普通内容图片使用 `next/image` 并给出尺寸/sizes；认证媒体须验证其授权传递与缓存边界，不直接套用公共图片优化链路。
- 不引入业务 BFF、Next.js 数据库访问或镜像 Go 业务规则的 Server Actions。Next.js 负责页面与已声明 API 的同源转发，Go 负责授权和业务。

## 5. 规范化前端目录

下列为目标结构，已有目录沿用；带“按需”的目录只有首个真实实现进入时才创建。

```text
frontend/
├── src/
│   ├── app/                       # 路由、layout/page、loading/error/not-found
│   │   └── globals.css            # 唯一全局样式与语义 token 映射
│   ├── components/
│   │   ├── ui/                    # shadcn 基础组件源码；已接入 Button/Badge
│   │   ├── layout/                # 按需：导航、页面骨架等共享结构
│   │   └── <business>/            # 按需：account、wardrobe、diary 等业务组件
│   ├── providers/                 # QueryClient 等应用上下文；按实际能力命名
│   ├── hooks/
│   │   └── <business>/            # 按需：请求/交互 hooks 与 query keys
│   ├── lib/
│   │   ├── api/request.ts         # 唯一 Axios 创建和请求适配入口
│   │   ├── utils.ts               # 统一 cn()；不收容业务杂项
│   │   └── <business>/            # 按需：纯函数、表单校验和视图映射
│   └── api/                       # Umi OpenAPI 全量生成，不手改
├── tests/
│   ├── architecture/              # 生产目录与依赖边界检查
│   ├── unit/                      # 单元/组件行为测试
│   ├── e2e/                       # 按需：可重复浏览器验收
│   └── tsconfig.json              # 测试类型检查，继承应用的 strict 规则
├── public/                        # 按需：有公开发布权的静态资源
├── assets/                        # 已有内部素材，不映射公开静态路径
├── components.json                # 已初始化，固定 radix 和 aliases
├── next.config.ts
├── openapi2ts.config.ts
├── package.json / package-lock.json
└── tsconfig.json / eslint.config.mjs / .prettierrc.json
```

- `@/*` 对应 `src/*`；shadcn aliases 指向 `components/ui`、`components`、`hooks`、`lib` 与 `lib/utils`，不另起 `shared/ui` 或组件包。
- `app` 只编排路由；复杂表单和领域展示移入 `components/<business>`。业务以 account、wardrobe 等统一词汇命名，不并行创建 `features`、`modules`、`services` 三套组织法。
- 普通前端文件/目录使用 kebab-case；组件导出 PascalCase，hook 使用 `useXxx`。Next.js 特殊文件名和生成器输出保持原约定。
- `components/ui` 不依赖业务 hooks、生成 API 或 app；业务组件可以依赖 ui/hooks/lib，hooks 可以依赖生成 API，生成 API 只通过统一适配器发请求。禁止循环依赖和通过大 barrel 文件打包全部业务。
- `providers` 只装配全局上下文，当前 `query-provider.tsx` 直接由根 layout 使用；不叠加仅转发 children 的 AppProviders。普通组件不反向依赖 Provider 实现。
- `tests/architecture` 解析实际源码导入；别名、相对路径、re-export 和字面量动态 import 统一检查。UI 仅依赖 UI/cn，components 可依赖 components/ui/hooks/lib，hooks 可依赖 hooks/lib/api，lib 仅依赖 lib，providers 可依赖 providers/lib；以上均可使用职责内的外部包。app 负责组装。生产代码不得依赖 tests/内部 assets/根工具配置；生成 API 不手改，调用方使用具体生成模块，不导入其聚合 index。
- `tsconfig.json` 只纳入 src、Next 生成类型和 next/openapi 配置；`tests/tsconfig.json` 单独纳入测试并显式声明 Node 类型。`npm run typecheck` 检查两者；`npm test` 自动发现 tests 下的测试，不逐个列文件路径。内部 assets 不参与 TypeScript、ESLint 或 Prettier 源码检查。
- 表单模型与视图模型放在所属业务旁，只表达 UI 特有字段；接口 DTO 必须引用生成类型，不另建手写 `types/api.ts`。
- 配置留在 frontend 根目录；测试不进 app、ui 或生成目录；不创建空目录、空路由、重复 assets 副本或额外工作区层级。

## 6. 规范化后端目录与服务规则

沿用已实现架构，完整细则见 [backend/AGENTS.md](backend/AGENTS.md) 和 [Go 工程设计](docs/design/24-Go后端工程结构研究与规范.md)。

```text
backend/
├── main.go                         # 极薄进程入口
├── go.mod / go.sum                # 唯一 Go module
├── internal/
│   ├── bootstrap/                 # 配置、依赖组装、启动/迁移/关闭
│   ├── application/               # 按业务能力聚合模型、规则、用例、消费端口
│   │   ├── account/               # 账户、身份与会话
│   │   ├── privacy/               # 隐私相关用例
│   │   ├── wardrobe/              # 真实衣橱
│   │   ├── outfitplan/             # 穿搭计划
│   │   ├── wearevent/              # 实际穿着
│   │   ├── diary/                  # 私人日记/日历
│   │   ├── community/              # 发布、互动与治理
│   │   ├── media/                  # 媒体生命周期
│   │   └── eventworker/            # Outbox、媒体处理/删除编排
│   ├── adapter/
│   │   ├── httpapi/                # Gin/Huma、contract/routes/handlers、Swagger
│   │   ├── postgres/               # GORM records、事务、查询、AutoMigrate
│   │   ├── objectstore/            # MinIO 适配
│   │   └── messagequeue/           # Kafka 适配
│   └── platform/                  # config/database/httpserver/ratelimit 生命周期
├── tests/
│   ├── services/                  # 本机真实依赖
│   ├── integration/               # 实际进程 + 隔离依赖
│   ├── container/                 # 已构建镜像
│   └── internal/                  # 上述测试共享夹具
├── architecture_test.go           # 目录、入口、导入方向与 HTTP 文件职责
├── Dockerfile / .dockerignore
└── .env.example
```

依赖固定为 `main.go → bootstrap → application/adapter/platform`，`adapter → application`；application 不依赖 Gin、Huma、GORM、SDK 或具体 adapter。业务间调用只采用明确需要且架构测试允许的方向。接口定义在消费方，不为每个 struct 建接口。

同一能力的模型、错误、规则和用例共属一个 application package，不重新建立全局 domain/model/service 大包。HTTP 按资源统一使用 `*_contract.go`、`*_routes.go`、`*_handlers.go`，消费端口放在 handlers；router/health/errors/openapi 分担组装、健康、错误与合同生成；PostgreSQL 按真实事务和能力拆文件，仅在形成独立依赖边界时建子 package，不套用全局 controller/service/dao 目录。

- HTTP 层负责认证接入、解析、合同校验和状态映射，不写 SQL；application 编排用例与事务要求；postgres 承担实际事务、约束与记录映射。
- I/O 传递 `context.Context` 并有超时/取消；资源由创建者有界关闭。错误使用 `errors.Is/As`，日志记录 request_id 和安全业务标识，不输出凭据、Cookie、原图和连接密码。
- 唯一 `backend/main.go`，用 `APP_ROLE=api|worker|all` 选择已实现角色，不增加空命令、微服务、通用 BaseRepository 或依赖注入框架。
- PostgreSQL 保存持久业务事实，Redis 维持已批准短期用途。跨表变更有明确事务，唯一/外键/检查约束由真实 PostgreSQL 验证。
- 开发 schema 由 postgres record 和集中 AutoMigrate 维护，监听前失败即退出；不能因为当前开发阶段允许重建库就自动删除用户数据。进入保留数据的部署阶段前另行固定版本化迁移方案。
- 异步工作遵循已实现 Outbox、持久消息、发送确认/offset 提交、幂等、重试与清理机制。删除先撤销访问，保留对象清理追踪直到完成，再最终删除关联记录；不要提前级联删除追踪证据。

## 7. 前后端共同接口合同

1. Huma operation 和 HTTP DTO/tag 是唯一可编辑接口声明。Swagger 与 Umi 读取运行时 `/openapi.json`；不维护手写 YAML/JSON、第二份 DTO 或额外 Swagger 服务。
2. API 使用已有无版本前缀的语义根路径，不擅自添加 `/api/v1`。每个 operation 定义稳定 operationId、成功状态、鉴权、请求限制和预期错误。
3. 成功响应保留各 operation 的强类型正文及 HTTP 语义，不新增 `{ code: 0, data: ... }` 全局包装。无正文使用已声明的 204，异步接纳使用 202。
4. 失败使用已有 `ErrorResponse`：`code`、`message`、`request_id`、`retryable`，必要扩展必须进入同一声明源。400/401/403/404/405/409/413/429/500/503 等保持语义；当前输入校验统一 400，不另保留一套 Huma 默认 422 错误形状。
5. Request ID 在响应头、错误正文和日志中一致；业务错误、404/405、输入校验和 panic 恢复都要覆盖，不能只验证 handler 正常路径。
6. 前端统一适配器返回真实响应正文；按 code/status 映射用户文案，保留 request_id 供定位，不能用 HTTP 200 表示失败。自动重试需同时考虑 retryable、Retry-After 与操作幂等性，避免重复创建/删除。
7. 会话由 HttpOnly Cookie 管理，浏览器不读 token、不写本地存储。鉴权、owner、角色、撤销与 CSRF/Origin 防护在服务端实施，不能只依赖 SameSite 或隐藏页面。
8. API 改动后从实际运行服务重新生成 `src/api/`，检查 tracked/untracked 变化、类型检查和请求测试；生成代码不手改、不导入生成器到生产 bundle。
9. Next.js rewrites 只同步本次需要且 OpenAPI 已声明的 API 根路径，确认不与页面 URL 冲突；新增日记/社区入口时必须同时检查转发规则，不使用全路径通配代理。MinIO 仅允许后端授权的签名上传流程，不把存储凭据交给前端。

## 8. 实施与验收

顺序固定为：读取 PROJECT/DESIGN 与相关需求 → 更新功能设计和单切片 Plan → 最小端到端实现 → 验证 → 回写 Acceptance。视觉、API、数据库或状态变化要更新对应事实源，不建立第二套规范副本。

### 前端实现门禁

在 `frontend/` 运行现有 `npm run lint`、`npm run test`、`npm run typecheck`、`npm run format:check`、`npm run build`；生产依赖运行 `npm audit --omit=dev`。生成契约变化增加实际 OpenAPI 再生成检查。CI 使用锁文件安装，不能混用 npm/pnpm/yarn 锁文件。

真实浏览器检查成功、失败、空态、慢请求、防重提、刷新/返回、会话过期、键盘与焦点、小屏/桌面和文字放大。静态截图不等于交互通过；现有 `npm run test` 覆盖目录/依赖边界和请求层，不能据此声称全部组件或业务端到端已验收。新增可重复 UI 测试按实际切片落入 `tests/`，不为本文单独创建测试框架。

新增/更新 shadcn 组件前执行 `info`、查 registry、`docs <component> --base radix` 并读取对应文档；添加后检查源码和组合关系。更新先用 `--dry-run`/`--diff`，保留本地变体；不要通过全量覆盖丢失定制。

### 后端实现门禁

在 `backend/` 运行 `gofmt -l .`、`go mod verify`、`go vet ./...`、`go test ./... -count=1`、`go test -race ./... -count=1`。涉及真实依赖时追加 `-tags=services` 与 `-tags=integration` 的 `./tests/...`；镜像变化再运行 `-tags=container`。包内 `_test.go` 跟随被测包，跨包验收按上述 tests 分层。

API 需覆盖契约、鉴权/越权、错误响应、幂等/冲突与取消；数据库需真实约束和事务证据；worker 需重投、重试、清理与关闭证据。详细命令和环境以对应 [后端细则](backend/AGENTS.md) 为准。

只改文档时检查交叉链接、规则一致性与 diff，不把未运行的构建、浏览器、远程 CI、生产或设备门禁写成通过。提交/推送按用户明确指令执行。

## 9. 当前差距与下一实施切片

以下为 2026-09-22 本地文件检查，进度后续仍归对应 Plan/Acceptance：

| 当前事实 | 目标与下一步 |
| --- | --- |
| Next.js + Tailwind v4 + TanStack Query + Axios + Umi 已存在 | 保留基础工程，在已有 Web 账户切片推进 |
| 已初始化 radix-nova / Lucide / RSC，Button/Badge 已接入 | 后续只按真实页面需要增加组件，更新先查看差异 |
| Themes 已移除，globals 统一 DESIGN token，首页/404/error 使用共享 PageShell | 保持唯一主题、系统字体和最小客户端边界 |
| DESIGN 已补齐 Web 错误色、对比度映射、焦点和触控基线，并同步 App 根文件 | 具体表单状态按业务页面验收；暗色仍不开放 |
| 现有 Axios 只做请求与正文返回 | 接入业务时补齐错误解释、会话失效、取消及其验证 |
| Web 业务页面尚未交付，现有 rewrites 未覆盖所有新增业务域 | 先完成账户页面及状态矩阵；后续按切片同步 API 转发 |
| 后端已有 application/adapter 分层和架构测试 | 新增能力遵守现有边界，不进行无目标的目录重构 |

## 10. 官方依据

核对日期：2026-09-22。目录放置、业务边界和迁移策略是 Then 的项目决定；框架 API 以安装版本及以下官方资料为准。

- Vercel 插件 `vercel:nextjs` / `vercel:react-best-practices`；[Vercel Next.js 文档](https://vercel.com/docs/frameworks/full-stack/nextjs)。
- 已安装 Next.js 的 `frontend/node_modules/next/dist/docs/`：Project Structure、Server and Client Components；[Next.js 官方文档](https://nextjs.org/docs/app)。
- 用户指定的 shadcn 技能；[shadcn Next.js](https://ui.shadcn.com/docs/installation/next)、[主题](https://ui.shadcn.com/docs/theming)、[Radix Button](https://ui.shadcn.com/docs/components/radix/button)、[Radix Dialog](https://ui.shadcn.com/docs/components/radix/dialog)、[Radix Field](https://ui.shadcn.com/docs/components/radix/field)。
- [Radix Composition](https://www.radix-ui.com/primitives/docs/guides/composition) 与 [Accessibility](https://www.radix-ui.com/primitives/docs/overview/accessibility)。

2026-09-22 后续用户变更：Kafka 替换 RabbitMQ，入口直接放在 `backend/main.go`，对应执行与验证见 [17-26](docs/plan/17-26-Kafka与工程规范化执行计划.md)。Kafka 在本地开发中使用三个独立 topic、每 topic 一个稳定消费组、acks=all 和手动 offset 提交；业务提交后才推进消费位置，失败有界重试后停止并保留位置，重启可继续。生产集群与真实数据迁移另按该片门禁处理。

2026-09-22 前端基础规范实施归 [17-27](docs/plan/17-27-Web设计体系与工程规范同步执行计划.md)，验证证据归 Acceptance 17。

2026-09-22 后端目录与 HTTP 文件职责实施见 [17-29](docs/plan/17-29-Backend目录与HTTP文件职责规范化.md)。单 module、唯一根入口、跨 adapter 隔离和 HTTP 文件职责由既有架构测试执行；跨层协作继续通过 bootstrap 与消费端口，不改变事务、API 或 Kafka 合同。
