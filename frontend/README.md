# WEARLOG Frontend

衣序 / WEARLOG 的 Web 前端。项目使用 Next.js App Router、React、TypeScript、shadcn/ui + Radix Primitives、Tailwind CSS、TanStack Query、Axios、ESLint 与 Prettier。不使用 Vite 或 React Router。

前后端工程总规范见 [PROJECT.md](../PROJECT.md)。采用 radix-nova / Lucide / RSC 和统一语义 token；现有账户、衣橱、推荐、计划与实际记录页面消费运行时 OpenAPI 生成客户端。路由入口直接，QueryProvider 独立于展示组件；生成 API 与唯一 Axios 入口保留。测试自动发现架构和单元用例，类型检查覆盖应用与测试。

## 本地开发

首页 `/` 已按 2026-10-13 无边框紧凑版 Figma 实现人物展台、直接选衣网格、搜索与手机选衣 Sheet，品牌为衣序 / WEARLOG；设计入口见 [Figma-MVP设计验收](../docs/validation/Figma-MVP设计验收.md)。当前支持示例选衣、桌面并排/滑动对比、手机前后切换、全身照片本地预览和浏览器示例草稿。人物素材位于 `public/look-samples/`，衣物复用 `public/wardrobe-samples/`，来源与哈希见 `public/look-samples/assets.json`。当前只有示例人物“海军蓝衬衫 + 牛仔裤”的既有成对图片；其他选衣与用户照片不返回该结果，不接入付费生成或假 360°。示例草稿只存当前浏览器，不写入账户或穿着次数。

二维衣橱在 `/wardrobe`，图片搭配与日期计划在 `/plans`，实际记录和统计在 `/wear`。支持 JPG/PNG/WebP 照片上传、补图和换图，保存后沿用后端私有媒体校验；类别由用户确认，补换图不改变实穿次数。`public/wardrobe-samples/` 的五张 Codex 示例图仍可主动添加，没有图片也可记录。筛选覆盖已加载单品，统计明确显示今年，保存计划不计实穿。交付证据与限制见 [本地抠图与裁剪](../docs/validation/27-本地抠图与简单裁剪最小交付.md)。

固定工具链为 Node.js 24.19.0 与 npm 12.0.2：

```sh
npm ci
npm run dev
```

Next.js 开发服务默认使用 `http://127.0.0.1:3000`，`next.config.ts` 只把已注册的 API 语义根路径 rewrite 到 `WEARLOG_BACKEND_ORIGIN`（默认 `http://127.0.0.1:8080`）。浏览器会话由 HttpOnly Cookie 管理；前端不得读取或持久化会话令牌。

`WEARLOG_BACKEND_ORIGIN` 在 **构建时** 写入 Next.js rewrite；若后端不是默认地址，执行 `WEARLOG_BACKEND_ORIGIN=http://127.0.0.1:18080 npm run build` 后再 `npm run start`。只在启动 `next start` 时修改该变量不会改变已构建的代理目标。

## OpenAPI 客户端

先按后端 README 启动本机 API 并开启 `API_DOCS_ENABLED=true`，再运行：

```sh
npm run openapi
```

`@umijs/openapi` 默认读取 `http://127.0.0.1:8080/openapi.json`，生成到 `src/api/`，并通过 `src/lib/api/request.ts` 的 Axios 实例发送请求。需要读取其他开发地址时可临时设置 `WEARLOG_OPENAPI_SCHEMA`；仓库不保存第二份 OpenAPI JSON/YAML。

生成目录由工具全量覆盖，不手工编辑。生成器固定依赖当前仍有无上游修复的开发期安全公告；它不进入生产 bundle，`npm audit --omit=dev` 必须保持为 0。

## 目录

```text
frontend/
├── src/
│   ├── app/                 直接路由入口、layout 和 globals.css
│   ├── api/                 Umi 生成客户端
│   ├── layout/              SiteLayout、AuthLayout、Header、Footer、PageContainer 和共享容器
│   ├── components/
│   │   ├── account/         账户表单和资料页
│   │   ├── wardrobe/        本人衣橱表单和列表
│   │   ├── outfit-plan/     本人计划表单和列表
│   │   ├── wear-event/      本人实际记录表单和列表
│   │   └── ui/              按需接入的 shadcn 组件
│   ├── providers/
│   │   └── query-provider.tsx
│   ├── hooks/account/       账户查询与操作
│   ├── hooks/wardrobe/      衣橱查询与操作
│   ├── hooks/outfit-plan/   计划查询与操作
│   ├── hooks/wear-event/    实际记录查询与操作
│   └── lib/
│       ├── api/request.ts   唯一 Axios 请求适配器
│       ├── account/         账户校验和错误文案
│       ├── wardrobe/        衣橱表单映射和错误文案
│       ├── outfit-plan/     计划表单映射和错误文案
│       ├── wear-event/      实际记录校验和错误文案
│       └── utils.ts         cn()
├── tests/
│   ├── architecture/dependencies.test.ts
│   ├── unit/                请求、账户、衣橱、计划与实际记录纯逻辑测试
│   └── tsconfig.json        独立测试类型检查
├── assets/                  内部素材说明与本地忽略的输入
├── components.json          shadcn 配置
├── next.config.ts           路由转发
├── openapi2ts.config.ts     生成配置
├── tsconfig.json            应用类型检查
└── package.json             统一质量命令；其他工具配置同层保存
```

上述为已实现结构。不建立平行的 `features`/`modules` 或占位页面。完整目录与导入规则见 [PROJECT.md](../PROJECT.md)。

## 质量门禁

```sh
npm run lint
npm run test
npm run typecheck
npm run format:check
npm run build
npm audit --omit=dev
```

当前目录交付账户、无图衣橱、穿搭计划和实际穿着页面的本地开发验证；公开注册仍需账号、TLS、隐私与运维发布门禁。计划只表示意图；离线 Look、推荐和穿后反馈页面尚未交付。

## 维护基础组件

执行 `npx shadcn@latest info --json` 核对 radix 配置；先读 `npx shadcn@latest docs <component> --base radix` 返回的文档，再从明确的 `@shadcn` registry 按需添加。现有 Button/Badge 有 WEARLOG 的圆角、触控和语义样式，升级先 `--dry-run`/`--diff`，不得直接覆盖。图标统一 Lucide；业务、请求和 DTO 不进入 ui 目录。

本机 npm 12 的 `npm exec` 会给子进程带入 `npm_config_allow_scripts`，若 shadcn 子安装报告 `EALLOWSCRIPTS`，使用 npm runner 清除该调用的两种变量名即可，不改用户全局配置：

```sh
npm exec --yes --package shadcn@4.21.0 -- env -u npm_config_allow_scripts -u NPM_CONFIG_ALLOW_SCRIPTS shadcn add @shadcn/<component>
```

组件 CLI 与动画 CSS 为构建期依赖；运行时使用本地组件源码，发布 bundle 不包含 CLI。生产审计与开发生成器审计分开记录，不用开发期工具公告冒充线上组件漏洞。

照片功能须启用现有本地媒体角色与依赖：API 使用 `APP_ROLE=all`、`MEDIA_DEVELOPMENT_ENABLED=true` 并配置本机 MinIO/Kafka；凭据按后端运行配置管理，不进入前端。仅运行默认 API 角色不会消费图片校验任务，页面会保留上传并提示稍后重试。本轮演示沿用独立库 `then_wardrobe_2d`，不要改用其他项目数据。

照片编辑支持“自动抠图”、四边百分比裁剪、原图对照与恢复。U2netp 模型由本地静态资源提供，ONNX Runtime Web CPU WASM 在点击时加载；`npm run dev` / `npm run build` 会自动复制锁定包的 WASM/MJS 到 public，不需要 Python、GPU 或付费推理服务。请保留 `public/wardrobe-processing/` 的模型与许可来源文件。新上传原文件逐字节保留，展示 PNG 可透明；旧源已清理的照片会提示使用已有净化图。

## 试衣间职责

首页路由保持 Server Component；`FittingRoomView` 编排照片、选择与预览状态，`GarmentPicker` / `GarmentOptions` 接收衣物数据并提供选择，`LookComparison` 按需加载。业务名称由项目定义，Vercel / Next.js 不要求名为 try-on 的目录。历史浏览器草稿键保持不变，以保留已有数据；这不代表独立试穿服务已经接入。
