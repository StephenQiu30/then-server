# 后端MVP与测试边界设计

## 状态与关联需求

状态：approved。2026-09-14 用户明确要求规划后端功能、固定 SOP、按 MVP 开发，并将 test 服务独立。关联 [PRD 17](../prd/17-云端生成与任务管理需求.md#后端mvp与测试服务边界)、[Design 02](02-后端架构.md#开发sop) 与 [17-11 执行计划](../plan/17-11-后端MVP与测试边界执行计划.md)。

这里的“test 服务独立”定义为测试代码、配置、依赖图、数据命名空间和执行入口与生产进程分离。它不新增线上微服务、`APP_ROLE=test`、第二个 `go.mod`、`cmd/test` 或常驻测试 HTTP 服务。

## MVP功能分期

当前产品主路径仍包含离线、无上传的内置三维换装。17-19 已批准合成照片的开发闭环，因此后端只增加该闭环实际需要的对象上传和 worker；真实用户照片及第三方生成仍关闭。

| 阶段 | 交付范围 | 当前状态 | 进入下一阶段的门槛 |
| --- | --- | --- | --- |
| B0 运行底座 | 单 Go module/binary、严格配置、PostgreSQL 连接、live/ready、内嵌 OpenAPI/Swagger、有界退出、分层测试 | 已实现；17-11 补齐测试边界 | 普通构建不包含测试 SDK；本机与隔离测试均可独立执行 |
| B1 云入口基础 | 内部用户身份、目的同意、撤回与删除接纳、对应 GORM schema | 账号与 17-19 照片目的已完成合成数据验收 | 真实地域、公开承诺和发布负责人另行批准 |
| B2 单次生成闭环 | 上传意图/finalize、任务创建/查询/取消、MinIO 私有资产、PG Outbox/Inbox、RabbitMQ worker | 照片准备与删除已完成；第三方生成任务未实现 | Provider、地域、成本、质量、迟到结果及删除 POC 通过 |
| B3 同步与远程目录 | 衣橱/搭配/穿着增量同步、远程资产目录 | 17-20/17-21 已实现账号下衣橱 CRUD 与四项确认属性；App 同步未接入 | 冲突、墓碑、版本、离线恢复与多设备产品契约获批 |

B0 不伪造业务 API。B1/B2 只实现 17-19 已批准的本人照片准备子集；B3 已具备 17-20/17-21 的账号级 CRUD 与确认属性基础，增量同步、冲突合并、墓碑和多设备恢复仍只保留路线图。

## Go代码与依赖边界

- `cmd/then-server/main.go` 只委托 bootstrap；业务按 `httpapi → application/<feature>` 进入，PostgreSQL/MinIO/RabbitMQ 作为 adapter 由 bootstrap 注入。
- 包级单元测试与源码同目录，文件使用 `_test.go`；这是 Go 工具链的原生边界，生产构建不会编译这些文件。跨包真实依赖和进程验收位于 `backend/tests`。
- `backend/tests/services`、`integration`、`container` 分别对应本机依赖、实际进程和 OCI 镜像，使用同名 build tag；根目录不直接放 Go 文件。`tests/internal` 只共享测试资源所有权代码。
- `backend/tests` 仍属于唯一 Go module，不增加 test 服务或第二个 module。测试专用 SDK 不进入生产二进制依赖图。
- 生产配置使用 `APP_*`、`DATABASE_URL` 等运行变量；本机测试连接只使用 `THEN_TEST_*`。测试不回退读取生产变量，不加载项目 `.env`，也不输出 SDK 原始错误。
- 本机 services 测试只允许 loopback；PostgreSQL 使用临时表，MinIO 使用随机桶，Redis 默认 DB 15 与随机 key，RabbitMQ 使用随机 quorum queue，所有资源由测试清理。

Go 官方建议将服务器实现放入 `internal`，服务命令放入命名的 `cmd/<binary>`，并通过 `_test.go` 与 `go test` 使用内建测试能力；本项目固定 `cmd/then-server/main.go` 与 `internal/bootstrap`，包内单元测试继续跟随被测 package。依据：[Organizing a Go module](https://go.dev/doc/modules/layout)、[Add a test](https://go.dev/doc/tutorial/add-a-test)。build tag 只隔离真实依赖测试，不用于生产功能分支。

## 独立测试矩阵

| 层级 | 位置与入口 | 外部依赖 | 证明范围 |
| --- | --- | --- | --- |
| Unit/contract | 各包 `*_test.go`；`go test ./...` | 无 | 领域/配置/Handler/资源生命周期的确定性规则 |
| Race | `go test -race ./...` | 无 | 当前测试覆盖内的数据竞争；不等于压力测试 |
| Local services | `backend/tests/services`；`-tags=services` | 本机 PG/MinIO/Redis/RabbitMQ | 实际协议、鉴权、TTL、confirm/requeue 与清理 |
| Integration | `backend/tests/integration`；`-tags=integration` | Testcontainers PG/Redis/MinIO/RabbitMQ | 实际 `all` 进程、合成照片全链、断连恢复、错误退出与契约分发 |
| Container | `backend/tests/container`；`-tags=container` | 显式构建的本地镜像与 Docker | `api|worker|all`、非 root、只读根、资源限制、健康和 SIGTERM |

各入口独立执行、独立失败。缺少依赖不得跳过并记为通过；services 测试不得连接非 loopback 地址。普通 `go test ./...` 保持快速，不启动 Docker 或本机中间件。

## 开发SOP落地

后端实现严格沿用 Design 02 的 SOP-00 至 SOP-08，并以以下检查点控制 MVP：

1. Design 固定用例、所有权、失败和最小架构；PRD 确认用户行为与非目标。
2. 只为近期切片创建一个 approved 的 `17-SS` 计划；同一时间最多一个任务 `in_progress`。
3. API 变更在 Huma operation/Go tag 与 Handler 同一切片完成，并验证运行时 OpenAPI；只有实际消费者存在时才执行 Umi/App Client 生成与编译。数据变更先评审 GORM record、约束和恢复。
4. Red：先写能复现业务规则或隔离缺口的失败测试；Green：完成最小代码；Refactor：只消除已出现的重复或越界。
5. 依次执行格式、unit、vet、race，再运行受影响的 services/integration/container；各层证据不能互相替代。
6. acceptance 记录实际 SHA、命令、环境、结果和限制；本地通过不冒充 CI 或生产发布。

## 失败、安全与回滚

- 测试配置非法或指向远程主机时，在建立连接前失败，错误只包含配置名和安全原因。
- 测试创建的资源必须带 `then-test-` 随机前缀或使用会话级临时对象；清理失败使测试失败。
- 不使用生产数据、真实照片、生产凭据或共享业务表；需要验证 GORM AutoMigrate 时只对测试拥有的独占/临时数据库执行 schema DDL。
- 测试辅助代码只管理测试拥有的资源与稳定错误；回滚必须清理自有临时资源，不涉及共享数据库、用户数据或生产进程。

## 重新评估条件

只有测试包编译时间或依赖冲突形成可量化问题时，才评估独立测试 module；只有需要部署后持续执行的合成探针有明确运维用例时，才设计独立 probe binary。两者都不能由“目录看起来更完整”触发。
