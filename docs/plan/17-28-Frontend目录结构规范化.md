# Frontend 目录结构规范化

## 范围与状态

- 2026-09-22 用户明确本轮是 frontend 文件目录结构的实现与优化。
- 契约 approved；实施 completed（本地实现与验证）。交付版本以 Git 提交记录为准，远端验证以对应 Actions 结果为准。视觉与产品范围保持既有 17-27 基线。
- 依据：[PROJECT](../../PROJECT.md#5-规范化前端目录)、[Design 16](../design/16-Web账户管理设计.md)、[前端规范](../../frontend/AGENTS.md)。

## 实施合同

1. `src/app` 保留直接路由入口及全局样式，不为入口另建转发页面组件。基础组件与布局保留 `components/ui`、`components/layout`。
2. 应用上下文独立于展示组件：将 `components/providers/app-providers.tsx` 移至 `providers/query-provider.tsx`，按实际 QueryClient 职责命名；layout 直接使用它，不叠加 AppProviders 包装。
3. `src/api` 继续由 Umi 全量生成，`lib/api/request.ts` 为唯一 Axios 入口，`lib/utils.ts` 为 cn 入口；不重命名生成文件、不创建空业务目录。
4. 请求测试放 `tests/unit/request.test.ts`；架构检查归 `tests/architecture`。应用 tsconfig 显式纳入 src、Next 类型和必要根配置，tests/tsconfig.json 单独检查测试类型；所有测试由同一 npm test 发现执行。
5. 架构检查验证真实目录与依赖：源码不混入测试；UI 只引用 UI/cn；hooks 消费生成客户端；lib、Provider 不反向依赖路由；生产源码不导入 tests、内部素材或根工具配置。别名、相对路径、re-export 与动态 import 同样检查。
6. PROJECT、前端 AGENTS/README、Design 16、计划和验收同步真实树。已有内部素材保留私有本地位置，退出 TypeScript/ESLint 源码扫描。

## Checklist

- [x] 确认现有结构与用户要求，撤回误做的首页展示调整。
- [x] Provider 与测试迁移、引用和配置同步。
- [x] 目录/依赖约束执行，验证相对路径与字面量动态导入不能绕过。
- [x] lint、全部测试、应用/测试 typecheck、format、生产 build 与生产依赖审计。
- [x] 浏览器确认首页与 404 路径行为，回写 [Acceptance 17](../acceptance/17-云端生成与任务管理验收.md#17-28-frontend-目录结构规范化验收)。

## 交付边界

没有 API、业务页面、数据库或共享设计数值变化，不重生成 SDK，不改变 App。验证失败只回退本片文件迁移及引用，不撤销用户已有研究修改。提交、远端 CI 和部署状态单独记录。
