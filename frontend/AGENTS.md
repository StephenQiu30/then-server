<!-- BEGIN:nextjs-agent-rules -->

# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->

# Then Web 工程规范

修改 `frontend/` 前先读取上方要求的已安装 Next.js 对应版本文档，并读取 [`PROJECT.md`](../PROJECT.md) 的组件、目录与状态规则。以根目录 [`DESIGN.md`](../DESIGN.md) 作为唯一视觉与交互标准，遵循 Design → PRD → Plan → Implementation → Acceptance 流程。

目标组件体系固定为 shadcn/ui + Radix Primitives + Tailwind CSS v4，shadcn 基础类型为 `radix`。当前工程仍使用 Radix Themes，尚未初始化 shadcn；迁移按 PROJECT 的差距清单实施，不把文档确认写成组件已接入。组件接入前用 npm runner 执行 shadcn `info` 和 `docs --base radix`，读取对应官方文档。

## 目录职责

```text
frontend/
├── src/
│   ├── app/                       # App Router 路由、layout/page 与路由自有样式
│   ├── api/                       # Umi OpenAPI 生成客户端
│   ├── components/
│   │   ├── ui/                    # 按需接入 shadcn 基础组件
│   │   ├── providers/             # Client Provider
│   │   ├── layout/                # 按需：共享页面结构
│   │   └── <business>/            # 按需：业务组件
│   ├── hooks/<business>/          # 按需：业务请求与交互 hooks
│   └── lib/
│       ├── api/                   # Axios 统一请求入口
│       ├── utils.ts               # shadcn 接入时添加 cn()
│       └── <business>/            # 按需：校验、纯函数与视图映射
├── tests/
│   ├── unit/                      # 不参与生产源码组织的单元测试
│   └── e2e/                       # 按需：浏览器交互验收
├── components.json                # shadcn 接入时生成
├── next.config.ts
├── openapi2ts.config.ts
└── package.json
```

- `src/app` 只负责路由组织，不放通用 HTTP 客户端、共享 Provider 或业务杂项。
- `src/lib/api/request.ts` 是唯一 Axios 实例入口；`src/api/` 直属 `src/`，只由 Umi OpenAPI 覆盖生成，不手工修改。
- 页面样式优先使用 Tailwind CSS utility；只有 Tailwind 不适合表达的路由私有样式才保留 CSS Module。
- 业务组件放到 `src/components/<business>/`，跨页面布局放 `components/layout`，基础组件放 `components/ui`；Server Component 保持默认，交互边界才使用 `use client`。
- 业务按 `components/<business>`、`hooks/<business>`、`lib/<business>` 归属，不再创建并行的 `features`/`modules` 目录。只有业务切片实际实现时才创建所需目录，不建空 `public`、`assets` 或占位路由。
- 视觉值集中映射到现有 `src/app/globals.css` 的语义 token；组件优先使用 variant/size，页面不覆盖组件颜色和字体。Radix 的触发器组合采用 `asChild`。
- 配置文件保留在 `frontend/` 根目录，测试按类型放在 `tests/`，不得混入生成目录。
