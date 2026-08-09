# iOS

“于是”的 iOS 客户端使用 Xcode 26.6、Swift 6.3.3 和 SwiftUI 原生开发，最低支持 iOS 18。App 显示名为“于是”，内部 Xcode target 和 Swift module 使用 `ThenApp`。

实现原则：

- SwiftUI + Observation 为主要 UI 和状态观察，必要时桥接 UIKit。
- Swift 6 language mode、Complete Strict Concurrency、Approachable Concurrency，UI 默认 Main Actor 隔离。
- P0 只保留一个 `ThenApp` Swift module 加测试 target，不把 Feature 拆成内部 framework 或 Swift Package。
- 采用 feature-first + MVVM + Repository。
- EventKit、UserNotifications、Core Location、MapKit 和 Vision 等系统能力通过服务协议封装。
- 本地数据固定使用 GRDB 7.11.1 + 系统 SQLite 的 DatabasePool/WAL；记账、缓存日程和结束行程必须支持离线。
- P1 API 客户端从 `../backend/openapi.yaml` 生成；P0 不联系业务后端。

## Swagger/OpenAPI Client 生成

- 唯一契约是 `../backend/openapi.yaml`。
- `ThenApp/openapi.yaml` 是指向该契约的符号链接，用于让 Xcode 目标直接使用后端契约，不得替换成手工复制文件。
- `ThenApp/openapi-generator-config.yaml` 生成 Swift types 和 client。
- 生成源码由 Xcode Build Tool Plugin 放入 DerivedData，不提交、不手工修改。
- 当前配置已使用 Swift OpenAPI Generator 1.13.0 完成人工实际生成验证；tag commit、命令和结果记录在 [`../docs/acceptance/01-技术基线验收.md`](../docs/acceptance/01-技术基线验收.md)，持续 CI 尚待 Xcode 工程创建。

创建 Xcode 工程时，在 ThenApp target 中：

1. 使用 Exact requirement 添加 GRDB 7.11.1、`apple/swift-openapi-generator` 1.13.0、`apple/swift-openapi-runtime` 1.12.0 和 `apple/swift-openapi-urlsession` 1.3.1，并提交 `Package.resolved`。
2. 将 `OpenAPIGenerator` 加入 target 的 **Run Build Tool Plug-ins**。
3. 将 `ThenApp/openapi.yaml` 和 `ThenApp/openapi-generator-config.yaml` 都加入 ThenApp target 的 **Compile Sources**。不要再加入同一契约的 external file reference，否则插件会识别到多份文档。
4. 使用 `OpenAPIRuntime` 和 `OpenAPIURLSession` 创建底层 Client，再由手写 Service/Repository 封装鉴权、重试、错误映射和领域模型转换。

每次 `backend/openapi.yaml` 变更后必须重新构建 iOS target。CI 中的 iOS 编译是客户端生成是否成功的强制检查。

创建 Xcode 工程后，还应在这里补充 Scheme、构建命令、测试命令和签名配置说明。完整选型与排除项见 [`../docs/design/01-技术选型.md`](../docs/design/01-技术选型.md)。
