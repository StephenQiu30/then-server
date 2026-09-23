# Web 账户管理设计

## 状态与实施边界

- 状态：基础工程与账户页面 `approved / implemented`（2026-09-23 用户明确要求先完成 then-server 的 Web 页面）；生产公开注册仍受账号与运维门禁约束。
- 用户目标：仓库新增语义化顶层目录 `frontend/`，提供注册、登录和本人账户 CRUD 页面。
- 当前决定：Web 使用 Next.js App Router、shadcn/ui + Radix、Tailwind CSS、TypeScript、Umi OpenAPI、Axios 与 TanStack Query。2026-09-23 用户明确要求先在 then-server 完成后端与 Web 页面，不修改 App。
- 上游合同：[Design 15](15-账号认证与账户数据设计.md)、[PRD 17](../prd/17-云端生成与任务管理需求.md#账号认证与web账户管理)、Go API 运行时 `/openapi.json`。
- 执行计划：[17-13 Web 账户管理](../plan/17-13-Web账户管理执行计划.md)。

## 页面范围

`/` 是产品说明与账户预览入口，404 返回首页。首个 Web 业务 MVP 提供三类路由：

1. `/register`：显示名称、邮箱、密码和前端确认密码；成功后建立会话并进入账户页。
2. `/login`：邮箱、密码；成功后进入账户页，错误只显示安全、可恢复的信息。
3. `/account`：显示当前邮箱、名称和创建时间；允许修改、退出，以及二次确认后删除本人账户。

登录用户访问登录/注册页时转到账户页，未登录访问账户页时转到登录页。通用 404 继续返回 `/`；不增加后台用户表格、运营仪表盘、组织/角色、忘记密码或社交登录占位页面。

页面沿用“于是”的排版、色彩与交互规范：桌面为介绍与表单双栏，移动为单列；注册、登录和账户页均有真实加载、失败与恢复状态。删除使用带标题、说明、取消和确认按钮的 AlertDialog。浏览器截图与键盘/无障碍核查记录在验收 17。

## 技术边界

- 使用 React、TypeScript、Next.js App Router、shadcn/ui（Radix Primitives 基础类型）与 Tailwind CSS；17-27 已移除 Radix UI Themes 并接入 Button/Badge，实施规则见 [PROJECT.md](../../PROJECT.md)。Next.js 文件路由管理页面，TanStack Query 管理需要的客户端服务状态，Axios 作为 Umi 生成请求的唯一适配器。精确版本只认 [Design 01](01-技术选型.md#web-frontend)。
- 目录采用职责分层：`src/app` 只保存路由入口和路由自有样式；基础 UI 归 `src/components/ui`，账户组件归 `src/components/account`，Client Provider 归 `src/providers`，业务 hooks 与纯函数分别归 `src/hooks/account`、`src/lib/account`，单元测试归 `tests/unit`。不创建平行 `features` 目录、空 `public`/`assets` 或占位页面。
- `@umijs/openapi` 只读取本机 Go API 的 `/openapi.json`，生成到 `src` 直属的 `src/api/`。`openapi2ts.config.ts` 固定 `schemaPath`、`serversPath`、`projectName` 和项目请求适配器，并通过不设置 `mockFolder` 禁止 mock 产物；页面不得手写 URL、DTO 或第二份 schema。
- Next.js rewrites 只代理 OpenAPI 已注册的语义根路径；页面与 API 从同一站点入口提供，不新增业务 BFF、跨源凭据 CORS 或前端直连 PostgreSQL/Redis/Kafka/MinIO。
- Cookie 由浏览器管理，JavaScript 不读取会话令牌，不把认证信息写入 Local Storage、Session Storage 或日志。
- 账户删除沿用服务端 `202` 删除回执合同：先关闭访问，界面显示真实 `pending|complete` 状态和回执 ID；前端二次确认不替代服务端授权。
- Axios 只在 `src/lib/api/request.ts` 创建一个实例并统一合并请求配置、发送请求和返回响应正文；生成代码和页面不得创建第二个 Axios 实例。请求 URL 来自运行时 OpenAPI，不在 Axios 或 Next.js rewrites 中拼接路径版本前缀。

## 生成器预检结论

2026-09-14 用户进一步确认 Web 使用 Umi OpenAPI 生成 API 文件，替代此前 Hey API 候选。仅做了可撤销的本地生成预检，未保留 `frontend/` 代码：

- `@umijs/openapi` 1.14.1 + TypeScript 6.0.3 已验证可消费运行时 OpenAPI 3.1.2；2026-09-23 实际 API 为 0.17.0，共 98 个 operation。每次合同变化必须从实际 `/openapi.json` 重新生成并验证函数数量，不复用旧产物。
- 该 CLI 对 HTTP `schemaPath` 使用 JSON 解析，不能直接消费 `/openapi.yaml`；Go API 因此直接暴露由 Huma operation 与标注类型生成并校验的 `/openapi.json`。
- 会话 Cookie 只通过 OpenAPI security scheme 表达，不生成函数参数；Umi 产物中没有 `then_session` 参数，浏览器随同源请求自动发送 HttpOnly Cookie。
- 生成器包没有声明其 CLI 实际需要的 `tslib`，前端正式安装时将 `tslib` 2.8.1 作为显式开发依赖；这不是生成后修补。当前完整开发依赖审计因生成器固定依赖的 `mockjs` 原型污染公告报告 2 个 high 且无上游修复，`npm audit --omit=dev` 为 0。前端开工时必须复核；生成器不进入生产 bundle，`mock` 固定关闭。

2026-09-16 基础工程当时已从实际本机 API 重新生成 API 0.16.0 的 98 个函数，生成目录为 `src/api/`，请求适配器为 `src/lib/api/request.ts`；新增社区发现、互动、关系、评论、安全、通知和申诉 client，HttpOnly Cookie 未成为参数。Tailwind CSS 4 通过 PostCSS 接入并由根布局加载，生成时不设置 `mockFolder`，因此不产出 mock；完整开发依赖审计仍以依赖审计切片证据为准。当前版本与生成差异以本文件上方 2026-09-23 结果为准。

## 页面状态与可访问性清单

以下矩阵对应已实现的账户页面；首页与通用路由状态见 17-27。

| 状态 | 必须可见的反馈 |
| --- | --- |
| 初始 | 标签、自动填充语义、12–72 UTF-8 字节提示、主操作 |
| 提交中 | 主按钮不可重复提交且名称说明正在处理 |
| 字段错误 | 错误与字段建立语义关系，焦点可到达 |
| 401 | 清除缓存的当前用户并回到登录页 |
| 409 | 明确提示邮箱已使用，不泄漏其他账户资料 |
| 网络失败 | 保留非密码输入并提供重试，不显示技术栈信息 |
| 更新成功 | 账户页显示服务端返回的新资料和状态消息 |
| 删除确认 | 模态标题、说明、取消与危险操作均可由键盘操作 |
| 删除受理 | 清空 Query 缓存，显示 `202` 回执与真实状态；刷新后转登录页且仍为未登录 |

页面使用语义 HTML、可见焦点、44pt 以上点击目标、动态文字空间、减少动态效果和增强对比度。桌面双栏仅是视觉编排，移动端必须保持单列阅读与键盘顺序。

## 合同核对

1. 三页的桌面与移动布局、路由、401/409/网络故障、退出和删除状态已按本合同实现并由真实浏览器验证。
2. 实际 Go API 0.17.0 的 `/openapi.json` 再生成无差异，生产依赖审计为 0；开发生成器公告仍按既有边界记录。
3. 公开注册发布前仍需邮件验证、TLS、隐私与运维等独立门禁；本地合成账号验证不等于生产开放。

## 2026-09-22 前端基础规范同步

执行见 [17-27](../plan/17-27-Web设计体系与工程规范同步执行计划.md)。组件体系采用 shadcn/ui（Radix 基础类型），唯一 token 映射在 globals.css，视觉依据为根 DESIGN 的 Then Web foundation。components.json 固定 RSC、Tailwind v4、Lucide 与 @/* 别名。账户旅程按 17-13 单独验收。

## 当前前端目录实施

2026-09-22 按用户要求执行 [17-28 目录结构规范化](../plan/17-28-Frontend目录结构规范化.md)：路由入口保留 src/app，QueryProvider 从展示组件中移到 src/providers；测试按 architecture/unit 分组并独立类型检查。生产目录和导入方向通过 npm test 检查，生成代码保持 src/api，统一请求保持 lib/api/request.ts。实际目录及允许依赖以 PROJECT 第 5 节为准，本轮不改变页面视觉或新增账户业务。
