# Web 衣橱数据核验设计

状态：`approved`（2026-09-23 用户要求先完成 then-server 的 Web 页面验证）。需求来源为 [PRD 12](../prd/12-数字衣橱与衣物录入需求.md#web-数据核验入口2026-09-23)，接口事实源为运行中 Go API `/openapi.json`，实施见 [17-31](../plan/17-31-Web衣橱数据核验执行计划.md)。

## 范围与页面

`/wardrobe` 是登录后的无图衣橱核验页，首页与账户页提供入口。桌面为编辑区和本人衣物列表双栏，移动端依次排列；列表按服务端稳定游标继续加载。用户可创建或选中衣物编辑名称、类别、可用状态和四项可留“未知”的确认属性。创建始终显式写入 `source=wardrobe`，不从内置参考或其他账号复制拥有事实。

删除前先调用影响预览，显示受影响计划与实际记录数量及两种历史处理方式；确认时提交当前 revision、服务端摘要和用户选的策略。默认保留历史占位，不静默删除关联历史。取消不写入；编辑冲突保留表单并可重新加载，删除失败显示安全错误并由用户重新预览影响。账号 401 跳登录。

页面只证明现有账号衣橱 API 与真实 PostgreSQL 的行为。它不替代 App 离线衣橱、推荐、计划/实际页面、照片或完整 Look，也不增加 BFF、手写 DTO 或第二套 schema。私有列表只在当前会话的 TanStack Query 缓存中，退出/删号沿账户切片清理缓存。

## 组件与验收

沿用 `PageShell`、shadcn Field/Input/Card/Alert/AlertDialog，固定选项用 shadcn Native Select。路由入口保留 Server Component，账户操作与 Query 状态在 `components/wardrobe`、`hooks/wardrobe`，纯映射在 `lib/wardrobe`。语义标签、错误关联、44px 操作区和实际 320/390px 布局按 [PROJECT](../../PROJECT.md) 检查。

验收使用隔离 PostgreSQL/Redis、实际 Go API 与 Next.js：A 建衣物、刷新、编辑属性/状态、旧 revision 409、删除预览与两种策略、B 不可见、401、网络故障重试和移动布局。生成客户端从实际 OpenAPI 再生成并检查无差异；lint/test/typecheck/format/build、生产依赖审计和浏览器可访问性均需记录。真实衣物照片、推荐与 App 发布门禁不由本页抵扣。
