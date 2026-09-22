# Then Frontend

“于是”Web 前端基础工程。项目使用 Next.js App Router、React、TypeScript、shadcn/ui + Radix Primitives、Tailwind CSS、TanStack Query、Axios、ESLint 与 Prettier，配置以当前官方 `create-next-app` 脚手架为基准。不使用 Vite 或 React Router。

前后端工程总规范见 [PROJECT.md](../PROJECT.md)。2026-09-22 已按 [17-27](../docs/plan/17-27-Web设计体系与工程规范同步执行计划.md) 初始化 radix-nova / Lucide / RSC，接入 Button、Badge 和统一语义 token，移除旧 Themes 依赖。首页、404 与错误重试共用基础页面结构；账户业务页面仍未交付。

目录实施见 [17-28](../docs/plan/17-28-Frontend目录结构规范化.md)。路由入口保持直接，QueryProvider 独立于展示组件；生成 API 与唯一 Axios 入口保留。`npm test` 自动发现架构和单元测试；`npm run typecheck` 分别检查应用与测试。内部 assets 不参与源码扫描。

## 本地开发

固定工具链为 Node.js 24.19.0 与 npm 12.0.2：

```sh
npm ci
npm run dev
```

Next.js 开发服务默认使用 `http://127.0.0.1:3000`，`next.config.ts` 只把已注册的 API 语义根路径 rewrite 到 `THEN_BACKEND_ORIGIN`（默认 `http://127.0.0.1:8080`）。浏览器会话由 HttpOnly Cookie 管理；前端不得读取或持久化会话令牌。

## OpenAPI 客户端

先按后端 README 启动本机 API 并开启 `API_DOCS_ENABLED=true`，再运行：

```sh
npm run openapi
```

`@umijs/openapi` 默认读取 `http://127.0.0.1:8080/openapi.json`，生成到 `src/api/`，并通过 `src/lib/api/request.ts` 的 Axios 实例发送请求。需要读取其他开发地址时可临时设置 `THEN_OPENAPI_SCHEMA`；仓库不保存第二份 OpenAPI JSON/YAML。

生成目录由工具全量覆盖，不手工编辑。生成器固定依赖当前仍有无上游修复的开发期安全公告；它不进入生产 bundle，`npm audit --omit=dev` 必须保持为 0。

## 目录

```text
frontend/
├── src/
│   ├── app/                 直接路由入口、layout 和 globals.css
│   ├── api/                 Umi 生成客户端
│   ├── components/
│   │   ├── layout/          PageShell
│   │   └── ui/              Button、Badge
│   ├── providers/
│   │   └── query-provider.tsx
│   └── lib/
│       ├── api/request.ts   唯一 Axios 请求适配器
│       └── utils.ts         cn()
├── tests/
│   ├── architecture/dependencies.test.ts
│   ├── unit/request.test.ts
│   └── tsconfig.json        独立测试类型检查
├── assets/                  内部素材说明与本地忽略的输入
├── components.json          shadcn 配置
├── next.config.ts           路由转发
├── openapi2ts.config.ts     生成配置
├── tsconfig.json            应用类型检查
└── package.json             统一质量命令；其他工具配置同层保存
```

上述为已实现结构。业务进入实施后才建立 `components/<business>`、`hooks/<business>`、`lib/<business>`；不建立平行的 `features`/`modules`，不创建空目录或占位页面。完整目标结构与导入规则见 [PROJECT.md](../PROJECT.md)。

## 质量门禁

```sh
npm run lint
npm run test
npm run typecheck
npm run format:check
npm run build
npm audit --omit=dev
```

当前目录只交付可运行基础工程和生成客户端；注册、登录与账户页面仍按 `docs/plan/17-13-Web账户管理执行计划.md` 的后续任务实施。

## 维护基础组件

执行 `npx shadcn@latest info --json` 核对 radix 配置；先读 `npx shadcn@latest docs <component> --base radix` 返回的文档，再从明确的 `@shadcn` registry 按需添加。现有 Button/Badge 有 Then 的圆角、触控和语义样式，升级先 `--dry-run`/`--diff`，不得直接覆盖。图标统一 Lucide；业务、请求和 DTO 不进入 ui 目录。

本机 npm 12 的 `npm exec` 会给子进程带入 `npm_config_allow_scripts`，若 shadcn 子安装报告 `EALLOWSCRIPTS`，使用 npm runner 清除该调用的两种变量名即可，不改用户全局配置：

```sh
npm exec --yes --package shadcn@4.21.0 -- env -u npm_config_allow_scripts -u NPM_CONFIG_ALLOW_SCRIPTS shadcn add @shadcn/<component>
```

组件 CLI 与动画 CSS 为构建期依赖；运行时使用本地组件源码，发布 bundle 不包含 CLI。生产审计与开发生成器审计分开记录，不用开发期工具公告冒充线上组件漏洞。
