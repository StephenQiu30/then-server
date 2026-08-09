# iOS

“于是”的 iOS 客户端使用 Xcode、Swift 和 SwiftUI 原生开发。App 显示名为“于是”，内部 Xcode target 和 Swift module 使用 `ThenApp`。

实现原则：

- SwiftUI 为主要 UI，必要时桥接 UIKit。
- 采用 feature-first + MVVM + Repository。
- EventKit、UserNotifications、Core Location、MapKit 和 Vision 等系统能力通过服务协议封装。
- 本地数据优先，记账、缓存日程和结束行程必须支持离线。
- API 客户端从 `../backend/openapi.yaml` 生成。

## Swagger/OpenAPI Client 生成

- 唯一契约是 `../backend/openapi.yaml`。
- `ThenApp/openapi.yaml` 是指向该契约的符号链接，用于让 Xcode 目标直接使用后端契约，不得替换成手工复制文件。
- `ThenApp/openapi-generator-config.yaml` 生成 Swift types 和 client。
- 生成源码由 Xcode Build Tool Plugin 放入 DerivedData，不提交、不手工修改。
- 当前配置已使用 Swift OpenAPI Generator 1.13.0 完成实际生成验证。

创建 Xcode 工程时，在 ThenApp target 中：

1. 添加 `apple/swift-openapi-generator`、`apple/swift-openapi-runtime` 和 `apple/swift-openapi-urlsession` Swift Package 依赖，并锁定已验证版本。
2. 将 `OpenAPIGenerator` 加入 target 的 **Run Build Tool Plug-ins**。
3. 将 `ThenApp/openapi.yaml` 和 `ThenApp/openapi-generator-config.yaml` 都加入 ThenApp target 的 **Compile Sources**。不要再加入同一契约的 external file reference，否则插件会识别到多份文档。
4. 使用 `OpenAPIRuntime` 和 `OpenAPIURLSession` 创建底层 Client，再由手写 Service/Repository 封装鉴权、重试、错误映射和领域模型转换。

每次 `backend/openapi.yaml` 变更后必须重新构建 iOS target。CI 中的 iOS 编译是客户端生成是否成功的强制检查。

创建 Xcode 工程后，还应在这里补充最低 iOS 版本、Scheme、构建命令、测试命令和签名配置说明。
