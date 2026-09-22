# Kafka 与工程规范化执行计划

## 状态与授权

- 2026-09-22 用户明确要求：先提交并推送现有工作区，再使用 Kafka、简化入口目录并开展规范化。
- 原工作区已提交并推送：`755943e654029175055da68951396abe201ec00c`。
- 契约状态：approved；执行状态：completed（本地工程与合成数据开发验证）；后续实现尚未提交/推送，远端 CI 未运行。
- 本片对应 PRD 17 的云任务基础设施、PRD 18 的可靠删除和 PRD 19 的通知；不改变业务 API、DTO、表结构或用户发布范围。

## 固定合同

1. 唯一入口移动到 `backend/main.go`，移除空 `cmd/then-server` 目录；保留 `internal/bootstrap` 组装与生命周期，构建/运行统一为 `go build .` / `go run .`。
2. Kafka 替换 RabbitMQ；使用 Apache Kafka KRaft 与 Go franz-go 客户端。沿用合成数据本地开发边界，broker 只允许 loopback，不自动开放生产。
3. 使用三个独立 topic：`<prefix>.media-check`、`<prefix>.media-delete`、`<prefix>.community-notification`；默认 prefix 为 `then`，测试使用唯一 prefix，不清空共享消息。各 topic 有独立稳定 consumer group。
4. 消息只含 event ID/type/aggregate ID，key 使用 aggregate ID；不携带图片、会话或私人正文。Outbox 仅在 Kafka `acks=all` 确认后标记发布；启用客户端幂等生产，跨进程重复仍由 PostgreSQL Inbox/条件状态处理。
5. 消费关闭自动提交，每次最多处理一条；业务成功后同步提交 offset，失败不越过该记录。失败重试有退避与次数上限，耗尽后退出保留 offset；重启继续。非法消息停止消费者并保留 offset，不能悄悄丢弃。
6. 消费处理与提交期间阻止 rebalance，处理设超时，结束时允许 rebalance。退出时先取消并等待所有 worker 协程，再关闭依赖；取消不提交未完成工作。
7. 本地/测试创建单分区、单副本 topic；单机确认不等于生产高可用。生产启用前必须另外落实 TLS/SASL/ACL、多副本/min ISR、retention、监控与回放操作，不在本片宣称完成。
8. 已确认的内部素材保持忽略。文档、Compose、示例环境、CI、架构测试及真实依赖测试同步新入口与 Kafka。

## 迁移与回退

先停止 Then 自身的旧 worker，确认旧 Outbox 已发布但未完成的事件并按 ID 对账；未完成事件需在 PostgreSQL 中有审计地重新置为待发布，再启动 Kafka worker。不能仅凭 published_at 判断业务完成，不能 purge 旧共享队列或重置其他应用 consumer group。本次只使用隔离合成夹具，不迁移运行中的用户数据、不停止系统 RabbitMQ。

回退使用本次前基线及旧配置；停止新 worker 后按同一对账流程处理未完成事件，禁止双消费者并行写同一业务库。本片不引入双写兼容层。

## Checklist

- [x] 提交并推送原工作区；核对远端 SHA。
- [x] 固定 Kafka 配置、topic、offset、失败和退出合同。
- [x] 更新入口、架构测试、Docker 与 CI。
- [x] 替换适配器、配置与真实依赖测试。
- [x] 验证失败重试、重启重投、提交进度及 topic 隔离。
- [x] 通过 gofmt、module verify、vet、unit/race、services、integration 与 OCI container。
- [x] 同步 PROJECT/Design/PRD/README 与验收结果，区分当前测试和历史 RabbitMQ 证据。

## 官方依据

- [Kafka Docker](https://kafka.apache.org/42/getting-started/docker/)
- [franz-go 生产与消费](https://github.com/twmb/franz-go/blob/master/docs/producing-and-consuming.md)
- [franz-go kgo API](https://pkg.go.dev/github.com/twmb/franz-go@v1.22.0/pkg/kgo)

## 验证证据

详细证据见 [Acceptance 17：17-26](../acceptance/17-云端生成与任务管理验收.md#17-26-kafka-与工程规范化验收)，编号 `THEN-KAFKA-ENGINEERING-20260922-01`。

在 `backend/` 执行 gofmt、module verify、vet、unit、完整 race、services、integration 与最终 OCI 镜像检查，均通过；隔离 Compose Kafka 主机端协议测试通过并清理该次资源。完整 services 首次发现交付超时后，固定单次请求 10 秒、交付预算 30 秒，完整复测通过。

本机 Docker 的 Ryuk 端口映射异常使用命令级 `TESTCONTAINERS_RYUK_DISABLED=true` 复测，资源仍由既有测试夹具回收；CI 未采用该覆盖。原文档基线提交的远端 CI 通过，不代表尚未提交的 Kafka 实现已通过远端 CI。未进行生产切换或旧事件迁移。
