# Web 账户管理设计

## 状态与实施边界

- 状态：基础工程 `approved / implemented`；账户页面与完整旅程仍为 `draft`。
- 用户目标：仓库新增语义化顶层目录 `frontend/`，提供注册、登录和本人账户 CRUD 页面。
- 当前决定：2026-09-16 用户明确要求 Web 使用 Next.js 及其 App Router，不使用 Vite 或 React Router；Radix UI、Tailwind CSS、TypeScript、ESLint、Prettier、Umi OpenAPI、Axios 与 TanStack Query 继续保留。本切片不越过页面稿与状态矩阵门禁实现账户业务页面。
- 上游合同：[Design 15](15-账号认证与账户数据设计.md)、[PRD 17](../prd/17-云端生成与任务管理需求.md#账号认证与web账户管理)、Go API 运行时 `/openapi.json`。
- 待批准计划：[17-13 Web 账户管理](../plan/17-13-Web账户管理执行计划.md)。

## 页面范围

首个 Web MVP 只包含三类路由：

1. `/register`：显示名称、邮箱、密码和前端确认密码；成功后建立会话并进入账户页。
2. `/login`：邮箱、密码；成功后进入账户页，错误只显示安全、可恢复的信息。
3. `/account`：显示当前邮箱、名称和创建时间；允许修改、退出，以及二次确认后删除本人账户。

登录用户访问登录/注册页时转到账户页；未登录访问账户页时转到登录页。404 回到账户入口，不增加营销首页、后台用户表格、运营仪表盘、组织/角色、忘记密码或社交登录占位页面。

Woo 参考没有展示认证页面，因此这些页面只能沿用“于是”的排版、色彩、材质与动效原则，不能声称为 Woo 逐帧复刻。编码前必须先完成桌面与移动端页面稿、正常/加载/错误/空状态矩阵及可访问名称审核。

## 技术边界

- 使用 React、TypeScript、Next.js App Router、Radix UI Themes 与 Tailwind CSS；Next.js 文件路由管理页面，TanStack Query 管理需要的客户端服务状态，Axios 作为 Umi 生成请求的唯一适配器。精确版本只认 [Design 01](01-技术选型.md#web-frontend)。
- 目录采用职责分层：`src/app` 只保存 App Router 路由入口和路由自有样式，跨路由 UI/Client Provider 放在 `src/components`，通用技术基础设施放在 `src/lib`，单元测试放在 `tests/unit`。业务切片获批前不预建空的 `features`、`public` 或 `assets`。
- `@umijs/openapi` 只读取本机 Go API 的 `/openapi.json`，生成到 `src` 直属的 `src/api/`。`openapi2ts.config.ts` 固定 `schemaPath`、`serversPath`、`projectName` 和项目请求适配器，并通过不设置 `mockFolder` 禁止 mock 产物；页面不得手写 URL、DTO 或第二份 schema。
- Next.js rewrites 只代理 OpenAPI 已注册的语义根路径；页面与 API 从同一站点入口提供，不新增业务 BFF、跨源凭据 CORS 或前端直连 PostgreSQL/Redis/RabbitMQ/MinIO。
- Cookie 由浏览器管理，JavaScript 不读取会话令牌，不把认证信息写入 Local Storage、Session Storage 或日志。
- 账户删除沿用服务端硬删除合同；前端二次确认不替代服务端授权。
- Axios 只在 `src/lib/api/request.ts` 创建一个实例并统一合并请求配置、发送请求和返回响应正文；生成代码和页面不得创建第二个 Axios 实例。请求 URL 来自运行时 OpenAPI，不在 Axios 或 Next.js rewrites 中拼接路径版本前缀。

## 生成器预检结论

2026-09-14 用户进一步确认 Web 使用 Umi OpenAPI 生成 API 文件，替代此前 Hey API 候选。仅做了可撤销的本地生成预检，未保留 `frontend/` 代码：

- `@umijs/openapi` 1.14.1 + TypeScript 6.0.3 已验证可消费运行时 OpenAPI 3.1.2；当前 API 0.14.0 共 48 个 operation，并包含账号状态/revision、公开资料、衣橱确认属性、计划/实际穿着以及私人日记/日历请求响应。每次合同变化必须从实际 `/openapi.json` 重新生成并验证函数数量，不复用旧产物。
- 该 CLI 对 HTTP `schemaPath` 使用 JSON 解析，不能直接消费 `/openapi.yaml`；Go API 因此直接暴露由 Huma operation 与标注类型生成并校验的 `/openapi.json`。
- 会话 Cookie 只通过 OpenAPI security scheme 表达，不生成函数参数；Umi 产物中没有 `then_session` 参数，浏览器随同源请求自动发送 HttpOnly Cookie。
- 生成器包没有声明其 CLI 实际需要的 `tslib`，前端正式安装时将 `tslib` 2.8.1 作为显式开发依赖；这不是生成后修补。当前完整开发依赖审计因生成器固定依赖的 `mockjs` 原型污染公告报告 2 个 high 且无上游修复，`npm audit --omit=dev` 为 0。前端开工时必须复核；生成器不进入生产 bundle，`mock` 固定关闭。

2026-09-16 基础工程已从实际本机 API 重新生成 API 0.14.0 的 48 个函数，生成目录为 `src/api/`，请求适配器为 `src/lib/api/request.ts`；新增 diary client，HttpOnly Cookie 未成为参数。Tailwind CSS 4 通过 PostCSS 接入并由根布局加载，生成时不设置 `mockFolder`，因此不产出 mock；完整开发依赖审计仍以依赖审计切片证据为准。

## 页面状态与可访问性清单

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
| 删除成功 | 清空 Query 缓存并进入注册页，刷新后仍为未登录 |

页面使用语义 HTML、可见焦点、44pt 以上点击目标、动态文字空间、减少动态效果和增强对比度。桌面双栏仅是视觉编排，移动端必须保持单列阅读与键盘顺序。

## 开工前关闭项

1. 审核三张页面稿以及手机宽度版本，确认颜色和品牌资产。
2. 审核路由跳转、401、409、网络错误、退出和删除后的状态矩阵。
3. 确认正式锁版仍能从 `/openapi.json` 生成当前 OpenAPI，生产依赖审计为 0，并复核开发生成器公告。
4. 基础工程已按本轮明确指令批准并创建；账户页面合同仍须在页面稿与状态矩阵审核后从 `draft` 改为 `approved`。
