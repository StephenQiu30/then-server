# Web 穿搭计划数据核验设计

状态：`approved`（2026-09-23 用户指定 `then-server` 后端及 Web 页面先行，App 后续）。需求：[PRD 16](../prd/16-穿搭记录与反馈需求.md) `OUTFIT-REQ-010`～`012`、`OUTFIT-RULE-001/011`；接口事实源为运行中 Go API `/openapi.json`，实施见 [17-32](../plan/17-32-Web穿搭计划数据核验执行计划.md)。

## 页面合同

`/plans` 是登录后的本人计划核验页，避开后端 `GET /outfit-plans` 的同源代理路径。用户从已记录的真实衣物选择 1～20 件，确认本地日期、浏览器 IANA 时区与可空场景后主动保存；创建 UUID 在不确定重试时保持不变。不可穿衣物逐件明确确认，不能由页面替用户默认勾选。列表按服务端稳定游标分页，显示计划状态、原日期/时区和服务端衣物快照；`content=null` 显示已清除，不回填当前衣物资料。

active 计划可编辑、取消、标记未穿和永久删除；not_worn 可恢复；其余状态只读。含已清除衣物快照的计划保留历史，不允许直接编辑以免无意丢掉原有占位；用户可另建计划。编辑使用当前衣橱 ID/revision 重新选择，冲突保留草稿并由用户主动重载。删除有二次确认，取消和未穿都不得产生实际穿着。账号 401 跳登录，其他失败只显示安全文案。页面不实现推荐、实际穿着表单、反馈、日记、图片、离线或 App 同步。

复用现有 `PageShell`、shadcn Field/Card/Alert/AlertDialog 与生成客户端。路由入口保持 Server Component，交互位于 `components/outfit-plan`，请求位于 `hooks/outfit-plan`，纯表单映射位于 `lib/outfit-plan`；不新增 BFF、手写 DTO 或数据库 schema。

## 验收

隔离 PostgreSQL/Redis + Go API + Next.js 下验证 A 创建计划不增加 WearEvent，刷新、编辑、取消、未穿/恢复、删除及两账号隔离；衣物 revision 变化造成 409，旧草稿不丢；历史快照清除后保留占位。核对分页、401、断网恢复、320/390/1280px、键盘和 axe，运行 lint/test/typecheck/format/build/生产依赖审计。公开发布仍依赖账号生产门禁；本页不抵扣 App 离线计划与实际穿着验收。
