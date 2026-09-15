# 项目资产

本目录保存不公开、需要版本/hash 审查的设计与内容生产输入。这里的文件不会由 Go 服务直接公开，也不能因为进入 Git 就视为已发布产品资源。

- `avatar-poc/`：数字形象和服装内容生产 POC 输入；只用于内部验证。
- `acceptance/`：不公开的视觉验收静态证据；同目录 README 记录环境、边界和 hash。
- App 随包默认资源在 `../then-app/ThenApp/Resources/` 或 Asset Catalog 中维护。
- 需要独立更新的生产媒体和三维文件存入私有 MinIO，由数据库保存业务元数据并通过受控接口返回短期访问地址。
- 真正公开且无需授权的 Web 固定资源才可以进入对应 Web 服务的 `public/`；当前没有建立通用公开资源目录。

资产文件使用语义化名称，并由同目录 manifest 保存来源、用途、尺寸、字节数和 SHA-256。不得在此保存真实用户照片、凭据、签名 URL 或未经授权的第三方素材。

当前 GitHub 仓库为公开仓库。`avatar-poc/` 和 `acceptance/` 已忽略，仅在本机持久化，不随 clone 交付；如需协作分发，使用私有 MinIO 并登记 object revision/hash。只有明确可公开的素材才另行纳入 Git。
