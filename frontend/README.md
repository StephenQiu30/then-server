# Then Frontend

“于是”Web 前端基础工程。项目使用 Next.js App Router、React、TypeScript、Radix UI Themes、Tailwind CSS、TanStack Query、Axios、ESLint 与 Prettier，配置以当前官方 `create-next-app` 脚手架为基准。不使用 Vite 或 React Router。

前后端工程总规范见 [PROJECT.md](../PROJECT.md)。2026-09-22 确定目标组件体系为 shadcn/ui + Radix Primitives + Tailwind CSS，保留既有 Next.js 和请求层；当前尚未初始化 shadcn，Radix Themes 与视觉 token 的替换属于后续实施切片。

## 本地开发

固定工具链为 Node.js 24.19.0 与 npm 12.0.2：

```sh
npm install
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
src/app/                 App Router 路由入口与路由样式
src/components/ui/       接入时创建：shadcn 基础组件
src/components/providers/ Client Provider
src/components/<business>/ 按需：业务组件
src/hooks/<business>/    按需：请求与交互 hooks
src/api/                 Umi OpenAPI 生成客户端
src/lib/api/             Axios 统一请求入口
src/lib/<business>/      按需：纯函数、校验与视图映射
tests/unit/              单元测试
```

业务切片进入实施后按上述职责创建目录，不再建立平行的 `features`/`modules` 组织方式，不保留空目录或占位页面。完整目标结构与导入规则见 [PROJECT.md](../PROJECT.md)。

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
