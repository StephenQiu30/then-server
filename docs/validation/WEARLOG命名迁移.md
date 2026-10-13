# 衣序 / WEARLOG 命名迁移

日期：2026-10-13。用户确认正式品牌为衣序 / WEARLOG，授权统一 Figma、页面品牌、本地目录和两个仓库，并保留所有未提交改动。本轮未提交或推送源码。

## 当前命名

| 对象 | 当前结果 |
| --- | --- |
| 正式品牌 | 衣序 / WEARLOG，不使用中文拼音 |
| 本地父目录 | `/Users/stephenqiu/Desktop/StephenQiu/Wearlog` |
| GitHub 仓库与本地子目录 | `wearlog-server`、`wearlog-app` |
| Go module | `github.com/StephenQiu30/wearlog-server/backend` |
| Web 包 / 镜像入口 | `wearlog-frontend` / `wearlog-backend` |
| iOS project、target、scheme、module | `WearlogApp` |
| 测试 target | `WearlogAppTests`、`WearlogAppUITests` |
| App 源码标识 | `WearlogPalette`、`WearlogAppModel`、`WearlogSchema` |
| 新环境变量前缀 | `WEARLOG_*`，CI、测试与示例已同步 |

GitHub 已实际改名并复核：[wearlog-server](https://github.com/StephenQiu30/wearlog-server)、[wearlog-app](https://github.com/StephenQiu30/wearlog-app)。两个 origin 已更新；远端代码仍是原提交，名称变更不代表本地源码已推送。

新 Figma 为 [衣序 WEARLOG](https://www.figma.com/design/fYWIHuNucsMQBNYzXBPuEq/?node-id=5-4)。旧文件网页显示不可用，编辑工具也拒绝访问，故在同一团队建立正式新稿。设计范围与原型见 [Figma MVP 设计验收](Figma-MVP设计验收.md)。

## 保留证据

- 迁移前备份：`/Users/stephenqiu/.codex/backups/wearlog-rename-20261013-113603`。server 512、app 130、独立 worktree 505 个非忽略源码文件，含 SHA256 清单、归档、未提交/暂存差异、HEAD、配置和 worktree 清单。
- 两个仓库的 HEAD 和暂存差异逐字节一致。worktree 移至 `Wearlog/.worktrees/backend-next-plan-20260930`，通过 `git worktree repair` 修复；其未提交状态与备份完全一致，未改写该分支源码，未清理原有 prunable 注册。
- 旧的无引用品牌资源被 W 标识替换，原件在备份中。未重置、清理或切换分支。
- 数据库 `then_wardrobe_2d`、iOS Bundle ID `com.stephenqiu.then`、已有 `OOTD` 数据目录与 migration ID 保留为数据身份，避免改名造成新空库或另一份安装。这些不是新的项目命名，没有重建用户数据。
- Web 草稿写入 `wearlog.try-on.sample-draft.v1`，可恢复旧品牌键；下一次成功保存后清理旧键。恢复和迁移使用浏览器测试验证。

## 检查结果

| 检查 | 结果与范围 |
| --- | --- |
| GitHub 新 URL、origin、HEAD、暂存区与 worktree | 已实际核对，源码未提交或推送 |
| Go module 校验、vet、完整 race 单测 | 通过；未重跑 services / integration / OCI 镜像测试 |
| Web lint、类型、格式、生产构建 | 通过；英文标识 200% 窄屏溢出已修复 |
| Web 单元 / 架构测试 | 21 项通过 |
| Web 浏览器测试 | 13/13 通过（11.7 秒），含旧键草稿恢复、成功保存迁移和 200% 窄屏 |
| iOS 模拟器测试 | xcresult 顶层 184 项通过、2 项跳过、0 失败；设备级含参数化运行 273 次通过 |
| iOS 工具链 | 本机 Xcode 27.0 / iOS Simulator 27.0，与 CI 固定 26.6 / 26.5 分开记录 |
| iOS 新品牌资产后的构建 | 通过，日志 `/tmp/wearlog-brand-xcodebuild.log` |
| iOS 测试结果 | `/tmp/wearlog-rename-20261013.xcresult`；UI、真机和 Vision 质量未验收 |

Web 和 API 已从新目录启动，生成保持关闭。Web/iOS 品牌与图标已统一，新 Figma 整体布局尚待实现；本轮没有图片/视频生成或新采购。

Codex 侧栏的项目登记仍指向旧 `Then` 目录。已确认没有可用的项目路径修改工具，应用也禁止自动操作自身；需要在 Codex 中重新选择 `Wearlog` 文件夹。没有通过旧目录软链接掩盖此差异。
