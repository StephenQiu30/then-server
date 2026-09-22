# 17-17 账号认证 Redis 限流执行计划

## 状态与授权

- 契约状态：`approved`
- 执行状态：`completed`
- 授权依据：用户已要求继续实施 Go 后端，并已明确选用本机 Redis；本切片收口账号公开入口仍缺失的认证限流，不恢复已暂停的 `frontend`。
- 关联：[Design 02](../design/02-后端架构.md)、[Design 15](../design/15-账号认证与账户数据设计.md)、[PRD 17](../prd/17-云端生成与任务管理需求.md#账号认证与web账户管理)、[Acceptance 17](../acceptance/17-云端生成与任务管理验收.md#账号认证与web账户管理验收)。

## MVP spec

只保护两个无需会话即可调用、会消耗认证资源的入口：

| operationId | 限流主体 | 固定窗口 | 上限 |
| --- | --- | --- | --- |
| `registerAccount` | 服务端 TCP 连接观察到的规范化源 IP | 1 小时 | 5 次 |
| `createSession` | 服务端 TCP 连接观察到的规范化源 IP | 15 分钟 | 10 次 |

- Gin 不信任任何代理头，本切片也不读取 `X-Forwarded-For`；进入受控反向代理部署前另行配置可信代理边界。
- 请求通过 Huma 的 JSON 合同校验后原子占用一次额度，成功与业务失败均计数，避免用账户是否存在或密码结果旁路限制。
- Redis key 只保存作用域和源 IP 的 SHA-256，不保存邮箱、密码、Cookie 或原始 IP；窗口到期自动删除。
- 超限返回 `429`、`RATE_LIMITED`、`retryable=true` 与向上取整秒数的 `Retry-After`，不调用 Account Service，不清除已有 Cookie。
- Redis 错误、无法提取合法源 IP 或计数结果不完整时失败关闭：返回 `503 NOT_READY` 与 `Retry-After: 1`，不继续注册或校验密码。
- Redis 仅保存限流临时状态；账号、凭据和会话事实继续只在 PostgreSQL。服务启动必须连通 Redis，readiness 同时探测 PostgreSQL 与 Redis。

本切片不做验证码、设备指纹、全局分布式配额、动态管理后台、邮件验证、密码找回、代理自动发现、RabbitMQ 或 MinIO 业务接入。

## 影响文件与职责

| 文件 | 职责 |
| --- | --- |
| `backend/internal/platform/config/config.go` | 校验单一 `REDIS_URL`；本机明文只允许 loopback，远端必须 `rediss` |
| `backend/internal/platform/ratelimit/redis.go` | Redis 连接、原子固定窗口、散列 key、探测与关闭 |
| `backend/internal/adapter/httpapi/account_handlers.go` | 从已验证请求上下文取得源 IP、选择固定策略、映射 429/503 |
| `backend/internal/bootstrap/application.go` | 组装 Redis limiter，并把 PG/Redis 合成 readiness 依赖 |
| `backend/tests/` | 真实 Redis 原子窗口、实际进程和 OCI 依赖验证 |

## Checklist

同一时间最多一个任务为 `in_progress`。

| 任务 ID | 状态 | 依赖 | 完成证据 |
| --- | --- | --- | --- |
| `17-17-DOC-01` | `completed` | 用户授权 | Design、PRD、spec/checklist 与 Acceptance 固定相同阈值、失败语义和边界 |
| `17-17-TEST-01` | `completed` | DOC-01 | HTTP Red 覆盖第 6 次注册、第 11 次登录、Retry-After、Service 未调用、不同 IP 隔离与 Redis 故障 503 |
| `17-17-GO-01` | `completed` | TEST-01 | platform Redis adapter、transport 策略、main 组装和 PG/Redis readiness 完成 |
| `17-17-OPENAPI-01` | `completed` | GO-01 | API 版本 0.5.0；两个 operation 的 429/503 及 Retry-After 由运行时 OpenAPI 生成并校验 |
| `17-17-INTEGRATION-01` | `completed` | GO-01 | 真实 Redis 32 并发请求只放行 10 次，TTL/散列 key；实际 binary 的限流、启动失败及断连恢复通过 |
| `17-17-VERIFY-01` | `completed` | OPENAPI-01、INTEGRATION-01 | gofmt、mod verify、vet、unit/race、services、integration 与受限 OCI 检查通过；证据回写 Acceptance |

## Red → Green → Refactor 与退出条件

1. Red：先添加 transport/config/真实 Redis 测试，确认当前构造函数、错误合同和运行时均不满足。
2. Green：实现最小 Redis 固定窗口和显式组装，不增加通用缓存接口或第二种算法。
3. Refactor：收敛 key、错误和 readiness 重复，保持 `transport/service/repository/platform` 依赖方向。

开发退出要求 checklist 全部完成，真实 Redis 与进程路径通过。该结果仍不代表公网账号系统已准入；邮件验证、密码找回、可信代理、生产 TLS/域名、审计与备份删除窗口继续单独验收。
