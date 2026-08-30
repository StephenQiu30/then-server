# 仓库脚本

## 当前门禁

| 脚本 | 用途 |
| --- | --- |
| `verify-toolchain.sh` | 校验 Xcode、Swift、Go 与仓库固定工具链 |
| `validate-commit-message.sh` | 校验 `type(scope): subject` 提交标题 |
| `validate-ios-architecture.sh` | 校验 SwiftUI/Observation 唯一 UI 基线、iOS 18、Swift 6、GRDB/OpenAPI 锁版和 UIKit 桥接边界 |

## 历史生活管理门禁

以下脚本仍用于理解和验证尚未迁移的旧代码，但不属于 OOTD 发布门槛：

- `validate-p0-scope.sh`
- `validate-ios-motion.sh`
- `validate-ios-privacy.sh`
- `validate-ios-localization.sh`

它们包含旧 19 张表、五 Tab、无自定义动效、财务/日历/位置权限和旧字符串目录等假设。删除旧实现前保留这些脚本，避免丢失历史迁移与隐私验证依据；OOTD 相应数据流落地后，先建立带正反例 fixture 的新门禁，再与旧 Feature、权限、资源和测试一起成组删除旧脚本。
