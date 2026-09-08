# 后端目录实施约束

继承根目录 [AGENTS.md](../AGENTS.md)。本文件只约束后端操作入口；详细规则引用现有事实源，不维护第二份版本表或任务清单。

## 开始编码前

1. 核对 [技术选型](../docs/design/01-技术选型.md)、所属 PRD/design 和已批准的 FF-SS 执行计划，运行根目录 `scripts/verify-toolchain.sh`。
2. 查阅 [现有文件实现契约](../docs/design/02-后端架构.md#现有文件的实现契约)和[后续功能与文件落点](../docs/design/02-后端架构.md#后续目录的功能与文件落点)，确定修改哪个已有文件或为何需要新增文件。
3. 在原执行计划中按[编码前检查卡](../docs/design/02-后端架构.md#每个新文件的编码前检查卡)明确功能、输入输出、依赖、副作用、失败与测试；不另建规格/任务文件。
4. 没有真实用例不创建目录、接口、表或占位实现。根 `main.go` 是唯一入口，不增加项目壳层；按[开发 SOP](../docs/design/02-后端架构.md#后端开发与交付-sop)推进。

## 文件放置与边界

- `main.go` 只组装和管理进程；`platform` 拥有连接/协议/资源；`transport` 拥有 HTTP 输入输出和映射。
- 业务规则及状态机属于 `model/service`，业务 SQL、GORM record 和事务实现属于 `repository`，消息调度属于 `worker`。
- Service 不接收 Gin/GORM/AMQP/供应商 SDK 类型；接口由消费方定义；禁止万能 BaseRepository、全局服务容器与纯转发包装层。
- 当前健康检查可直接依赖窄 Probe 接口，不建立没有业务价值的 Service/Repository。
- OpenAPI 仍只在 `openapi.yaml`；schema 只在获批 migration SQL 和 atlas.sum；测试与实际职责一起交付。
- 修改职责回写 Design 02，修改选型回写 Design 01，修改任务状态只回写原 FF-SS 计划。文档规划不能代替实现或验收通过。
