<!-- BEGIN:nextjs-agent-rules -->

# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->

# Then Web 工程规范

修改 `frontend/` 前先读取上方要求的已安装 Next.js 对应版本文档，并读取 [`PROJECT.md`](../PROJECT.md) 的组件、目录与状态规则。以根目录 [`DESIGN.md`](../DESIGN.md) 作为唯一视觉与交互标准，遵循 Design → PRD → Plan → Implementation → Acceptance 流程。

组件体系固定为 shadcn/ui + Radix Primitives + Tailwind CSS v4，shadcn 基础类型为 `radix`。已初始化 radix-nova 配置并接入 Button/Badge，Radix Themes 已移除；只按实际用例增加组件。组件接入前用 npm runner 执行 shadcn `info` 和 `docs --base radix`，读取对应官方文档。

## 目录职责

```text
frontend/
├── src/
│   ├── app/                       # App Router 路由、layout/page 与路由自有样式
│   ├── api/                       # Umi OpenAPI 生成客户端
│   ├── components/
│   │   ├── ui/                    # 按需接入 shadcn 基础组件
│   │   ├── layout/                # 共享页面结构（PageShell）
│   │   └── <business>/            # 按需：业务组件
│   ├── providers/                 # 应用上下文，当前 query-provider.tsx
│   ├── hooks/<business>/          # 按需：业务请求与交互 hooks
│   └── lib/
│       ├── api/                   # Axios 统一请求入口
│       ├── utils.ts               # 统一 cn() 入口
│       └── <business>/            # 按需：校验、纯函数与视图映射
├── tests/
│   ├── architecture/              # 实际源码目录与依赖检查
│   ├── unit/                      # 单元测试，当前 request.test.ts
│   ├── e2e/                       # 按需：浏览器交互验收
│   └── tsconfig.json              # 独立测试类型检查
├── components.json                # 已初始化：radix-nova / Lucide / RSC
├── next.config.ts
├── openapi2ts.config.ts
└── package.json
```

- `src/providers/query-provider.tsx` 持有 QueryClient，根 layout 直接使用；不创建泛化 AppProviders 转发层。
- `src/app` 只负责路由组织，不放通用 HTTP 客户端、共享 Provider 或业务杂项。
- `src/lib/api/request.ts` 是唯一 Axios 实例入口；`src/api/` 直属 `src/`，只由 Umi OpenAPI 覆盖生成，不手工修改。
- 页面样式优先使用 Tailwind CSS utility；只有 Tailwind 不适合表达的路由私有样式才保留 CSS Module。
- 业务组件放到 `src/components/<business>/`，跨页面布局放 `components/layout`，基础组件放 `components/ui`；Server Component 保持默认，交互边界才使用 `use client`。
- 业务按 `components/<business>`、`hooks/<business>`、`lib/<business>` 归属，不再创建并行的 `features`/`modules` 目录。只有业务切片实际实现时才创建所需目录，不建空 `public`、`assets` 或占位路由。
- 视觉值集中映射到现有 `src/app/globals.css` 的语义 token；组件优先使用 variant/size，页面不覆盖组件颜色和字体。Radix 的触发器组合采用 `asChild`。
- 配置文件保留在 `frontend/` 根目录，测试按类型放在 `tests/`，不得混入生成目录。

## 当前实现与自动检查

- `src/app/globals.css` 映射根 DESIGN 的双端中性视觉标准；默认浅色，Web 使用 Geist 与中文系统字体回退。Button/Badge 的 Then 变体归 `components/ui`，不在页面覆盖颜色或字体。
- `components/layout/page-shell.tsx` 是首页、404 和路由错误页共用的内容结构；各页面必须保持唯一 `main-content`，供根 layout 的跳转链接使用。
- Next.js 16.3 错误边界使用 `retry()` 重新获取并渲染；不输出内部错误正文，不建立测试专用生产路由。业务加载/空态在实际数据页实现后接入。
- ESLint 禁止回引 Themes、其他 primitive 体系及页面直用 Axios；目录与依赖边界由 tests/architecture 检查，涵盖别名、相对路径、re-export 和字面量动态 import。UI 不反向依赖业务，生产代码不导入 tests/assets/根工具配置；具体允许方向见 PROJECT。
- `npm run typecheck` 先执行 `next typegen`，再分别检查应用和 tests/tsconfig.json；`npm test` 自动发现 tests 下的测试，保证新测试不会漏跑；CI 使用 Node/npm 固定版本与 `npm ci`，执行 lint/test/typecheck/format/build/生产依赖审计。
