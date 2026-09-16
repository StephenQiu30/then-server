<!-- BEGIN:nextjs-agent-rules -->

# This is NOT the Next.js you know

This version has breaking changes — APIs, conventions, and file structure may all differ from your training data. Read the relevant guide in `node_modules/next/dist/docs/` (resolved from this file's directory; in monorepos the `next` package may not be visible from the repo root) before writing any code. Heed deprecation notices.

This block is written and re-added by `next dev` — verify at `node_modules/next/dist/server/lib/generate-agent-files.js`. Removing it from a diff only re-creates the uncommitted change; committing it with your work keeps the tree clean.

<!-- END:nextjs-agent-rules -->

# Then Web 工程规范

修改 `frontend/` 前先读取上方要求的已安装 Next.js 对应版本文档，以仓库根目录 [`DESIGN.md`](../DESIGN.md) 作为唯一视觉与交互标准，并遵循 Design → PRD → Plan → Implementation → Acceptance 流程。

## 目录职责

```text
frontend/
├── src/
│   ├── app/                       # App Router 路由、layout/page 与路由自有样式
│   ├── api/                       # Umi OpenAPI 生成客户端
│   ├── components/                # 跨路由共享的 UI 与 Client Provider
│   └── lib/
│       └── api/                   # Axios 统一请求入口
├── tests/
│   └── unit/                      # 不参与生产源码组织的单元测试
├── next.config.ts
├── openapi2ts.config.ts
└── package.json
```

- `src/app` 只负责路由组织，不放通用 HTTP 客户端、共享 Provider 或业务杂项。
- `src/lib/api/request.ts` 是唯一 Axios 实例入口；`src/api/` 直属 `src/`，只由 Umi OpenAPI 覆盖生成，不手工修改。
- 页面样式优先使用 Tailwind CSS utility；只有 Tailwind 不适合表达的路由私有样式才保留 CSS Module。
- 共享 Client Component 放到 `src/components/`，Server Component 保持默认。
- 只有业务切片获批并实际实现时才创建 `src/features/<feature>`；不创建空 `public`、`assets`、`features` 或占位路由。
- 配置文件保留在 `frontend/` 根目录，测试按类型放在 `tests/`，不得混入生成目录。
