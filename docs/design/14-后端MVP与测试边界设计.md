# 后端MVP与测试边界设计

## 状态与关联需求

状态：approved。2026-09-14 用户明确要求规划后端功能、固定 SOP、按 MVP 开发，并将 test 服务独立。关联 [PRD 17](../prd/17-云端生成与任务管理需求.md#后端mvp与测试服务边界)、[Design 02](02-后端架构.md#开发sop) 与 [17-11 执行计划](../plan/17-11-后端MVP与测试边界执行计划.md)。

这里的“test 服务独立”定义为测试代码、配置、依赖图、数据命名空间和执行入口与生产进程分离。它不新增线上微服务、`APP_ROLE=test`、第二个 `go.mod`、`cmd/test` 或常驻测试 HTTP 服务。

## MVP功能分期

当前产品主路径是离线、无上传的内置三维换装，因此后端 MVP 只交付能支撑当前开发和后续云切片的最小运行底座。没有真实云端用户动作时，不创建账号、衣橱同步、对象上传或空 worker。

| 阶段 | 交付范围 | 当前状态 | 进入下一阶段的门槛 |
| --- | --- | --- | --- |
| B0 运行底座 | 单 Go module/binary、严格配置、PostgreSQL 连接、live/ready、内嵌 OpenAPI/Swagger、有界退出、分层测试 | 已实现；17-11 补齐测试边界 | 普通构建不包含测试 SDK；本机与隔离测试均可独立执行 |
| B1 云入口基础 | 内部用户身份、目的同意、撤回与删除接纳、对应 GORM schema | 待云功能进入近期计划 | 用户行为、数据字段、保留/删除和认证方式在独立切片获批 |
| B2 单次生成闭环 | 上传意图/finalize、任务创建/查询/取消、MinIO 私有资产、PG Outbox/Inbox、RabbitMQ worker | 分期，未授权开发 | B1 完成；供应商、地域、成本、质量、迟到结果及删除 POC 通过 |
| B3 同步与远程目录 | 衣橱/搭配/穿着增量同步、远程资产目录 | 后续 | 冲突、墓碑、版本、离线恢复与多设备产品契约获批 |

B0 不伪造业务 API。B1–B3 只保留路线图，不预建目录、表、DTO、消费者或配置项。

## Go代码与依赖边界

- `main.go` 只组装 API 生命周期；业务进入时按 `transport → service → repository/platform` 单向依赖。
- 包级单元测试与源码同目录，文件使用 `_test.go`；跨包黑盒、真实依赖和进程测试位于 `backend/tests`，使用外部 `tests` 包。
- `backend/tests` 仍属于唯一 Go module。测试专用 SDK 只由带 build tag 的 `_test.go` 导入，不进入生产二进制依赖图。
- 生产配置使用 `APP_*`、`DATABASE_URL` 等运行变量；本机测试连接只使用 `THEN_TEST_*`。测试不回退读取生产变量，不加载项目 `.env`，也不输出 SDK 原始错误。
- 本机 services 测试只允许 loopback；PostgreSQL 使用临时表，MinIO 使用随机桶，Redis 默认 DB 15 与随机 key，RabbitMQ 使用随机 quorum queue，所有资源由测试清理。

Go 官方建议将服务器内部包放入 `internal`，并通过 `_test.go` 与 `go test` 使用内建测试能力；本项目保持现有根 `main.go` 是因为只有一个命令，不为套用通用目录模板迁移到 `cmd/`。依据：[Organizing a Go module](https://go.dev/doc/modules/layout)、[Add a test](https://go.dev/doc/tutorial/add-a-test)。build tag 只隔离真实依赖测试，不用于生产功能分支。

## 独立测试矩阵

| 层级 | 位置与入口 | 外部依赖 | 证明范围 |
| --- | --- | --- | --- |
| Unit/contract | 各包 `*_test.go`；`go test ./...` | 无 | 领域/配置/Handler/资源生命周期的确定性规则 |
| Race | `go test -race ./...` | 无 | 当前测试覆盖内的数据竞争；不等于压力测试 |
| Local services | `backend/tests/*`；`-tags=services` | 本机 PG/MinIO/Redis/RabbitMQ | 实际协议、鉴权、TTL、confirm/requeue 与清理 |
| Integration | `backend/tests/*`；`-tags=integration` | Testcontainers PG | 实际进程、断连恢复、错误退出与契约分发 |
| Container | `backend/tests/*`；`-tags=container` | 显式构建的本地镜像与 Docker | 非 root、只读根、资源限制、健康和 SIGTERM |

各入口独立执行、独立失败。缺少依赖不得跳过并记为通过；services 测试不得连接非 loopback 地址。普通 `go test ./...` 保持快速，不启动 Docker 或本机中间件。

## 开发SOP落地

后端实现严格沿用 Design 02 的 SOP-00 至 SOP-08，并以以下检查点控制 MVP：

1. Design 固定用例、所有权、失败和最小架构；PRD 确认用户行为与非目标。
2. 只为近期切片创建一个 approved 的 `17-SS` 计划；同一时间最多一个任务 `in_progress`。
3. API 变更先改唯一 OpenAPI 并编译 Swift Client；数据变更先评审 migration、约束和恢复。
4. Red：先写能复现业务规则或隔离缺口的失败测试；Green：完成最小代码；Refactor：只消除已出现的重复或越界。
5. 依次执行格式、unit、vet、race，再运行受影响的 services/integration/container；各层证据不能互相替代。
6. acceptance 记录实际 SHA、命令、环境、结果和限制；本地通过不冒充 CI 或生产发布。

## 失败、安全与回滚

- 测试配置非法或指向远程主机时，在建立连接前失败，错误只包含配置名和安全原因。
- 测试创建的资源必须带 `then-test-` 随机前缀或使用会话级临时对象；清理失败使测试失败。
- 不使用生产数据、真实照片、生产凭据或共享业务表；测试不执行 schema DDL，除非未来 migration 切片使用独占测试数据库。
- 当前调整仅移动测试辅助职责并增加约束。回滚删除 test environment helper、恢复调用即可，不涉及 API、数据库 schema、用户数据或生产进程。

## 重新评估条件

只有测试包编译时间或依赖冲突形成可量化问题时，才评估独立测试 module；只有需要部署后持续执行的合成探针有明确运维用例时，才设计独立 probe binary。两者都不能由“目录看起来更完整”触发。
