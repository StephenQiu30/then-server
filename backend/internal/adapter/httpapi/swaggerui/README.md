# Swagger UI静态资源

固定版本见 [Design 01](../../../../docs/design/01-技术选型.md)：Swagger UI v5.32.15。

- swagger-ui-bundle.js、swagger-ui.css：来自官方 GitHub tag 的 `dist/`。
- LICENSE：来自同一官方 tag 根目录，随二进制嵌入并可通过 `/docs/LICENSE` 查看。
- swagger-ui-bundle.js.LICENSE.txt：bundle 顶部要求保留的第三方许可声明，来自同一 tag 的 dist；同路径由 `/docs/` 提供。
- 官方来源：https://github.com/swagger-api/swagger-ui/tree/v5.32.15
- SHA256SUMS：记录上述未修改官方文件的 SHA-256；Go 测试核对实际文件与清单。
- index.html、swagger-initializer.js：本项目最小入口，所有资源同源，禁用请求调试、外部 validator、查询覆盖和授权持久化。
- logo-mark.png、favicon.ico：项目原创品牌资产，本地嵌入 Go 二进制；不属于 Swagger UI 官方文件，也不写入官方文件的 SHA256SUMS。

更新时从已评审的官方 tag 获取文件，核对许可证、哈希、OpenAPI 兼容性和真实页面，再同步 Design 01；不要从运行中的容器、CDN 或任意最新版本自动替换资源。README 和哈希清单不作为 HTTP 路由暴露。
