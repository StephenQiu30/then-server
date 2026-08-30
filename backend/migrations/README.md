# PostgreSQL migrations

此目录是 OOTD 服务端 PostgreSQL schema 的唯一事实源，使用 Atlas `v1.3.0` versioned migration 工作流。

当前尚未创建业务 migration；第一个已批准的数据模型应生成不可变的 baseline SQL，并同时提交 `atlas.sum`。不要恢复根目录 `schema.sql`，也不要让 GORM `AutoMigrate` 修改共享、测试、预发或生产数据库。

每次 schema 变更必须：

1. 生成带时间戳的 SQL migration。
2. 人工评审锁级别、重写、索引、回填、兼容窗口与回滚。
3. 更新 `atlas.sum`。
4. 从空库逐条应用并执行真实 PostgreSQL 集成测试。
5. 对已有数据记录 expand/contract、验证和恢复方案。
