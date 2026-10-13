# WEARLOG 项目规范

更新：2026-10-13。适用范围：`wearlog-server/frontend`、`wearlog-server/backend` 及两者共享的产品与接口约束。

本文维护前后端工程边界，视觉与交互遵循根目录 [DESIGN.md](DESIGN.md)。技术版本以锁文件和工具链配置为准；HTTP 以 `backend/internal/adapter/httpapi` 的 Huma operation 和运行时 OpenAPI 为准；数据库以 `backend/internal/adapter/postgres` 的 GORM record 为准。执行细则见 [后端 AGENTS](backend/AGENTS.md) 与 [前端 AGENTS](frontend/AGENTS.md)。

## 当前产品与实现边界

正式品牌为衣序 / WEARLOG。本地父目录为 `Wearlog`，两个仓库为 `wearlog-server` 和 `wearlog-app`；Web 包为 `wearlog-frontend`，Go module 为 `github.com/StephenQiu30/wearlog-server/backend`，iOS 工程为 `WearlogApp`。不使用中文拼音命名。当前设计以 [WEARLOG Figma](https://www.figma.com/design/fYWIHuNucsMQBNYzXBPuEq/?node-id=5-4) 为准；工程名称与数据身份的边界见 [命名迁移](docs/validation/WEARLOG命名迁移.md)。

当前工作先明确需求与验证方案；产品目标为人物展示、选衣试穿、前后对比、真实衣橱与实穿记录。现有二维能力与固定示例是实现基础，不代表真实试穿已交付。建议 P0、独立环绕门槛及待决策项见需求文档；Web 页面按最新版 Figma 与已授权任务维护；真实生成仍按需求门槛验证。需求见 [入口](docs/requirement/README.md)、[功能与交互](docs/requirement/03-功能需求与交互.md)、[验收条件](docs/requirement/06-交付阶段与验收.md)；技术结论见 [人物展示可行性](docs/research/人物展示可行性.md)。

文档职责固定：`docs/requirement` 保存产品目标、范围、业务规则和验收；`docs/research` 保存来源、竞品源码和技术候选；`docs/validation` 保存实现、截图、耗时、费用与测试证据。需求改变才更新需求正文，不为每轮操作新增需求分析文件。工程待办与接手状态仍归 BACKLOG/HANDOVER。

已有二维衣橱、照片上传/补换图、平铺搭配、日期计划和实穿统计的本地实现状态见 [二维验证](docs/validation/25-二维衣橱与搭配最小交付.md)、[照片上传](docs/validation/26-真实衣物照片上传最小交付.md)、[抠图与裁剪](docs/validation/27-本地抠图与简单裁剪最小交付.md)。这些是已做工作的记录，不是本轮新验收通过或扩展全部模块的指令；完整统计、设备与环境限制按原记录保留。

2026-10-13 按当前用户授权，Web 首页已实现新版 Figma 的人物展示、选衣与前后对比核心交互，含本地照片预览和示例草稿。它复用成对设计素材；未接入供应商推理，也不代表完整 31 个画面、真实照片试穿或连续环绕已经交付。原二维业务功能继续保留。

现有 Go/Web 工程继续保留，账号、社区和同步不是个人首版前置；本次不修改 iOS 或重建既有服务。计划、实穿与可选照片分别表达，未知属性不补成事实，私人媒体不公开。历史人物与生成实验已移出需求目录，仅保留研究/验证证据。

不新增商城、广告、订阅计费、私信、直播、任意模型上传或通用工作流平台。技术选型依据后续受控样本的实际结果决定；img2threejs 已因实际质感失败停止；新人物方案只做研究，实施需另行确定。

## 3. 固定技术基线

| 层           | 决定                                                 | 约束                                                            |
| ------------ | ---------------------------------------------------- | --------------------------------------------------------------- |
| Web 框架     | Next.js App Router + React + TypeScript strict       | 保留现有工程；不另建 Vite、Pages Router 或第二个 Web 应用       |
| Web 设计实现 | shadcn/ui + Radix UI Primitives + Tailwind CSS v4    | shadcn 基础类型固定 `radix`，不混入 Base UI/React Aria 基础类型 |
| 图标         | 统一使用 Lucide，components.json 已固定              | 已接入；不在同一 Web 产品混用多套图标                           |
| 服务端状态   | TanStack Query                                       | 沿用现有 Provider，不再增加 SWR 或并行的请求缓存体系            |
| HTTP 客户端  | `@umijs/openapi` + Axios                             | 从运行中 Go API 的 `/openapi.json` 生成 `src/api/`              |
| Web 质量     | ESLint + Prettier + TypeScript + Node.js Test Runner | 沿用 npm 与 `package-lock.json`；UI 需浏览器交互验收            |
| 后端         | Go + Gin/Huma + GORM + PostgreSQL                    | 单 module、模块化单体、一个二进制与 OCI 镜像                    |
| 基础设施     | Redis、Kafka、MinIO                                  | 各自只承担已实现用途，按切片启用；PostgreSQL 保存业务事实       |

Vercel 插件的 Next.js/React 指南用于框架边界与性能规范。采用这些指南不改变已有 Go 服务部署方式，也不要求现在迁移到 Vercel 托管。

### 3.1 shadcn 与 Radix 的关系

shadcn/ui 提供项目内可维护的组件源码，Radix Primitives 提供交互与无障碍基础，Tailwind/语义 token 承接 WEARLOG 视觉。业务页面统一从 `@/components/ui/*` 导入基础组件；缺少组件时先查官方 registry，再做必要组合，不直接另造 Button、Dialog 或 Select。[官方 Next.js 接入说明](https://ui.shadcn.com/docs/installation/next)

已通过官方 CLI 初始化 `radix-nova`，配置固定 Radix、Lucide、RSC 与 @/* 别名，实际接入 Button/Badge。旧 `@radix-ui/themes`、Theme Provider 和样式已移除；QueryClient Provider 保留。新增组件继续用 npm runner 显式核对 radix，再按实际使用从官方 registry 添加，不提前安装整套组件库。

## 4. 前端设计规范

### 4.1 视觉与 token

1. 遵循 `DESIGN.md` 的黑白画布、真实内容优先、Geist 排版、细边界和轻量层级。页面结构按 WEARLOG 的功能设计组织，不能照搬参考站点的品牌、营销导航或开发者术语。
2. 颜色、字体、间距、圆角和焦点样式在 `src/app/globals.css` 集中映射；Tailwind v4 使用 `@theme inline`。组件消费 `bg-background`、`text-foreground`、`text-muted-foreground` 等语义类，不在页面写十六进制、原始色阶或逐组件 `dark:` 色彩覆盖。[shadcn 主题规范](https://ui.shadcn.com/docs/theming)
3. 默认浅色。暗色目前缺少完整设计验收，不因 shadcn 默认模板而自动增加切换入口；启用前在同一 token 体系补齐状态并验收。
4. 先使用组件已有 `variant`/`size`；新增视觉变体归组件定义，调用方 `className` 用于布局。业务表面以间距、排版、细边界和轻阴影区分；大面积氛围渐变只用于首页。
5. Web 加载 OFL 授权 Geist Sans/Mono，中文使用系统字体回退；不只在 CSS 声明未加载的字体。iOS 保留动态字号与平台语义。
6. 使用 `flex/grid` 与 `gap-*`，不使用 `space-x-*`/`space-y-*`；等宽高用 `size-*`，省略文本用 `truncate`，条件类使用统一 `cn()`。

基础语义映射固定如下，具体值从 `DESIGN.md` 取得：

| shadcn token                        | WEARLOG 语义                                                 |
| ----------------------------------- | --------------------------------------------------------- |
| `background` / `foreground`         | 页面画布 / 主正文                                         |
| `card`、`popover` 与对应 foreground | 内容与浮层表面 / 其上正文                                 |
| `primary` / `primary-foreground`    | 主要行动色 / 行动色上的文字                               |
| `secondary`、`muted`、`accent`      | 次级表面、弱化区域、选中或强调表面                        |
| `border`、`input`、`ring`           | 分隔线、输入边界、键盘焦点                                |
| `destructive`                       | 删除及不可逆操作，已在 DESIGN 的 WEARLOG Web foundation 补齐 |

`accent` 不另起第二个品牌色；destructive/错误 token 已补齐；新增成功等状态色须先补齐 `DESIGN.md` 后统一映射，不在页面各自选色。弱化正文仍需可读，不能直接把 disabled 颜色用于重要说明。

### 4.2 组件与表单

| 场景         | 统一用法                                                                                |
| ------------ | --------------------------------------------------------------------------------------- |
| 主次操作     | `Button` 的已有变体；加载用 `Spinner`、`data-icon`、`disabled`，不虚构 `isLoading` 属性 |
| 表单         | `FieldGroup` + `Field` + 关联标签；说明/错误留在字段附近                                |
| 复合输入     | `InputGroupInput`/`InputGroupTextarea` 配合 `InputGroupAddon`                           |
| 少量平级选项 | `ToggleGroup`；复选/单选字段组使用 `FieldSet` + `FieldLegend`                           |
| 模态与侧栏   | `Dialog`/`Sheet`/`Drawer`，必须提供对应 Title                                           |
| 危险确认     | `AlertDialog`，说明对象、后果与提交状态                                                 |
| 列表与空态   | 按内容选择列表/Table；空态 `Empty`，加载 `Skeleton`                                     |
| 状态与分隔   | `Badge`、`Separator`、`Progress`；不用临时 div 模拟                                     |
| 简短完成提示 | Radix 路线统一 Sonner                                                                   |

`data-invalid` 放在 Field，`aria-invalid` 放在控件，错误与控件关联；disabled 同时表达容器状态和控件禁用。必填项有可见标签，placeholder 不代替标签。[Field 官方说明](https://ui.shadcn.com/docs/components/radix/field)

Select/Menu/Command 的 Item 放入对应 Group，TabsTrigger 放入 TabsList；Avatar 配备 Fallback；需要 Card 时按 Header/Title/Description/Content/Footer 的内容职责组合。按钮图标使用 `data-icon`，大小交给组件，不在每个调用点覆盖。

Radix 自定义触发器使用 `asChild`，承接的叶子组件必须传递 props 和 ref，避免嵌套 button；不混用 Base UI 的 `render` API。焦点陷阱、关闭恢复、键盘行为优先由 Radix 管理，不自行改写，也不为浮层随意堆叠 z-index。[Radix 组合说明](https://www.radix-ui.com/primitives/docs/guides/composition)

### 4.3 组件命名与抽象边界

以设计中的职责命名，不以实现先后、颜色、页面位置或笼统的 Panel/Shell/Experience 命名。文件用 kebab-case，主组件导出对应的 PascalCase；Next 路由文件保留框架命名，shadcn 复合组件保留官方命名。

| 层级 / 设计对象 | 当前组件与归属 | 职责 |
| --- | --- | --- |
| 基础控件 | `components/ui/Button`、`ToggleGroup`、`Badge`、`Dialog` | 通用交互和视觉变体，不知道衣物、搭配、账户或示例数据 |
| 全站布局 | `layout/SiteLayout`、`AuthLayout`、`AccountLayout` | 布局结构；SiteLayout/AuthLayout 持有各自唯一 main |
| 页面内容与标题 | `layout/PageContainer`、`PageHeader` | 共享宽度、标题、说明、操作插槽，不读取业务数据 |
| 品牌入口 | `layout/BrandLink` | Logo、字标和首页链接；不能把带导航行为的组件只命名为 Logo |
| 路由对应业务界面 | `FittingRoomView`、`WardrobeView`、`LookCollectionView`、`LookDetailView`、`OutfitPlanView`、`WearEventView`、`AccountView`、`RecommendationView` | 编排数据、页面状态与下层组件；收藏与详情是独立 View，不用 detail 布尔值切换两个页面 |
| 表单 | `WardrobeItemForm`、`OutfitPlanForm`、`WearEventForm`、`AuthForm` | 表单值、校验展示、提交/取消；数据契约沿用 hooks/lib/生成 API |
| 编辑与选择 | `OutfitFlatlayEditor`、`WardrobePhotoEditor`、`GarmentPicker` | Editor 用于平铺/图片编辑；Picker 接收可选衣物与当前选择，通过行动插槽组合页面操作；试穿服务状态不归 Picker，不从示例数组导出组件类型 |
| 内容展示 | `PersonPreview`、`WardrobeItemImage`、`LookCard`、`LookComparison`、`WearStatistics` | 名称表达展示对象；可复用的搭配卡片和对比接收视图模型，不内置固定人物图片、存储或请求 |
| 模态操作 | `PersonPhotoDialog`、`SaveLookDialog` | 动作/对象 + Dialog；避免无法判断保存什么的 SaveDialog |

通用视觉变体描述外观，不承载业务名：Toggle 使用 `solid/subtle/card/tile`，Badge/Alert 使用 `muted`，Button 使用 `outline-subtle/surface`。`garment/look/comparison/stage/scene` 不作为基础控件变体；无调用的旧变体直接删除，不留兼容别名。相同色值继续引用中性 token，品牌蓝仅出现在品牌资源中。

业务 View 与本地适配器可以读取示例数据；展示组件不能把样例写死为唯一输入。Look 的展示模型在 `lib/look/model.ts`，本地示例映射在 `lib/look/samples.ts`，草稿持久化在 `hooks/look/use-look-draft.ts`。纯函数/常量（如日期范围、场景选项、照片校验）属于 `lib`，不从组件模块反向导入。只提取已存在的共享结构，不创建万能组件、空目录、大型 barrel 或只转发 props 的包装层。

试衣间首页使用 `components/fitting-room/FittingRoomView`；通用选衣使用 `components/wardrobe/GarmentPicker` 与 `GarmentOptions`，选项类型在 `lib/wardrobe/selection.ts`，样例适配在 `lib/fitting-room/samples.ts`。页面保留 Server Component 路由入口，交互状态进入 Client Component；非首屏对比使用模块级 `next/dynamic`。这是本项目按职责制定的命名，不是 Vercel 要求特定业务目录名称。

官方依据为 [Next.js 项目结构](https://nextjs.org/docs/app/getting-started/project-structure)、[服务端与客户端组件](https://nextjs.org/docs/app/getting-started/server-and-client-components) 和 [按需加载](https://nextjs.org/docs/app/guides/lazy-loading)。本地实现使用已安装版本文档核对 API；保留项目现有 TanStack Query、shadcn/Radix 和 Go 边界，不为套用指南另建数据层。

自动检查负责主导出与文件名一致、目录依赖和展示组件的数据边界；路由、文档引用、桌面/窄屏交互仍要验证。规则是长期约束，不把代码重命名当作新增产品能力。

### 4.4 状态、响应式与无障碍

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
│   ├── layout/                    # SiteLayout、Header、Footer、页面内容容器
│   ├── components/
│   │   ├── ui/                    # shadcn 基础组件源码；已接入 Button/Badge
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
- `tests/architecture` 解析实际源码导入；别名、相对路径、re-export 和字面量动态 import 统一检查。UI 仅依赖 UI/cn；layout 只依赖 layout/ui/lib，持有唯一全站 Header、main、Footer 与共享宽度；业务 components 可依赖自身、layout、ui、hooks、lib，hooks 可依赖 hooks/lib/api，lib 仅依赖 lib，providers 可依赖 providers/lib；以上均可使用职责内的外部包。app 负责组装。生产代码不得依赖 tests/内部 assets/根工具配置；生成 API 不手改，调用方使用具体生成模块，不导入其聚合 index。
- `tsconfig.json` 只纳入 src、Next 生成类型和 next/openapi 配置；`tests/tsconfig.json` 单独纳入测试并显式声明 Node 类型。`npm run typecheck` 检查两者；`npm test` 自动发现 tests 下的测试，不逐个列文件路径。内部 assets 不参与 TypeScript、ESLint 或 Prettier 源码检查。
- 表单模型与视图模型放在所属业务旁，只表达 UI 特有字段；接口 DTO 必须引用生成类型，不另建手写 `types/api.ts`。
- 配置留在 frontend 根目录；测试不进 app、ui 或生成目录；不创建空目录、空路由、重复 assets 副本或额外工作区层级。

## 6. 规范化后端目录与服务规则

沿用已实现架构，完整细则见 [backend/AGENTS.md](backend/AGENTS.md) 和 Go 工程设计。

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
│   │   ├── outfitfeedback/         # 穿后反馈与穿着统计
│   │   ├── diary/                  # 私人日记/日历
│   │   ├── community/              # 发布、互动与治理
│   │   ├── media/                  # 媒体生命周期
│   │   └── eventworker/            # Outbox、媒体处理/删除编排
│   ├── adapter/
│   │   ├── httpapi/                # Gin/Huma、contract/routes/handlers、Swagger
│   │   ├── postgres/               # GORM records、事务、查询、AutoMigrate
│   │   ├── objectstore/            # MinIO 适配
│   │   └── messagequeue/           # Kafka 适配
│   └── platform/                  # config/database/httpserver/mail/ratelimit 生命周期
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

顺序固定为：读取 PROJECT/DESIGN 与当前需求 → 最小端到端实现 → 验证 → 记录结果。视觉、API、数据库或状态变化要更新对应事实源，不建立第二套规范副本。

### 前端实现门禁

在 `frontend/` 运行现有 `npm run lint`、`npm run test`、`npm run typecheck`、`npm run format:check`、`npm run build`；生产依赖运行 `npm audit --omit=dev`。生成契约变化增加实际 OpenAPI 再生成检查。CI 使用锁文件安装，不能混用 npm/pnpm/yarn 锁文件。

真实浏览器检查成功、失败、空态、慢请求、防重提、刷新/返回、会话过期、键盘与焦点、小屏/桌面和文字放大。静态截图不等于交互通过；现有 `npm run test` 覆盖目录/依赖边界和请求层，不能据此声称全部组件或业务端到端已验收。新增可重复 UI 测试按实际切片落入 `tests/`，不为本文单独创建测试框架。

新增/更新 shadcn 组件前执行 `info`、查 registry、`docs <component> --base radix` 并读取对应文档；添加后检查源码和组合关系。更新先用 `--dry-run`/`--diff`，保留本地变体；不要通过全量覆盖丢失定制。

### 后端实现门禁

在 `backend/` 运行 `gofmt -l .`、`go mod verify`、`go vet ./...`、`go test ./... -count=1`、`go test -race ./... -count=1`。涉及真实依赖时追加 `-tags=services` 与 `-tags=integration` 的 `./tests/...`；镜像变化再运行 `-tags=container`。包内 `_test.go` 跟随被测包，跨包验收按上述 tests 分层。

API 需覆盖契约、鉴权/越权、错误响应、幂等/冲突与取消；数据库需真实约束和事务证据；worker 需重投、重试、清理与关闭证据。详细命令和环境以对应 [后端细则](backend/AGENTS.md) 为准。

只改文档时检查交叉链接、规则一致性与 diff，不把未运行的构建、浏览器、远程 CI、生产或设备门禁写成通过。提交/推送按用户明确指令执行。
