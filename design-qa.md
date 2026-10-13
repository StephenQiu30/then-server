# 方案 3 Web 设计 QA

日期：2026-09-26。检查范围：场景首页的视觉还原，以及与真实推荐入口相关的响应式和浏览器交互。

## 对照证据

- Source visual truth：`/Users/stephenqiu/.codex/generated_images/01a0db4b-02b7-7910-969c-d07fd28f72c1/exec-c6ab80e0-57f9-4142-9d2e-a53761b9bf9e.png`，1487×1058 px。
- Rendered implementation：`/Users/stephenqiu/.codex/visualizations/2026/09/26/01a0db4b-02b7-7910-969c-d07fd28f72c1/home-desktop-final.png`，1487×1058 px；本机 `http://127.0.0.1:3017/`，亮色、默认“日常出行”。
- 对照视口：1487×1058 CSS px；源稿和实现图均为同尺寸 1487×1058 px，按 1× 像素密度对齐，无缩放。完整并排图：`/Users/stephenqiu/.codex/visualizations/2026/09/26/01a0db4b-02b7-7910-969c-d07fd28f72c1/design-compare-final.png`。
- 重点区域并排图：同目录 `compare-hero-final.png`（标题、场景、CTA 与主照片）及 `compare-cards-final.png`（灵感卡）。二者来自上述实际截图的等位裁切，已与完整并排图一起目视检查。
- 其他状态：`home-mobile-final.png` 为 390×844 CSS 视口、浏览器可见截图 375×812 px；`home-320.png` 为 320×700 CSS 视口的早期截图，随后修正并重新目视检查了 320px 导航。移动截图受内置浏览器可见区域裁切，没有用它与桌面源稿做像素比较。768px 平板检查了布局与页面无横向溢出。

## Findings

最终无待修复的 P0/P1/P2 视觉问题。以下是与概念稿的有意差异：

- [已接受] 概念稿的灵感卡呈现商品式衣物小图和“换一组”操作；实现使用原创风格影像、文案和通往真实推荐的链接，并明确标注“示例灵感”。用户尚未上传衣物照片或拥有可用的换组排序事实，不能把示例图做成个人推荐结果。
- [已接受] 概念稿有搜索、头像和额外场景；当前产品没有对应的搜索和额外场景合同。实现保留可工作的账户入口及六个真实可选择的场景，避免空入口。
- [P3] 主照片与概念稿不是同一张，人物位置和墙面留白不同；同为柔和自然光、燕麦色穿搭与街景方向，可在真实品牌影像确定后替换。此差异不影响示例／真实数据界限或主要视觉层级。

## 五项视觉核对

| 表面 | 结论 |
| --- | --- |
| 字体／排版 | Geist Sans 与系统中文后备字体实际加载；宽屏 64px Hero、正常字重、标题换行与源稿接近，320px 缩至 32px；按钮和辅助文案可读。 |
| 间距／布局 | 顶部导航、左文右图、照片高度和第二屏卡片起点与源稿在同一 1487×1058 视口接近；卡片改为并排影像＋信息，移动端自然单列。 |
| 颜色／tokens | 复用共同黑白语义色、细边和深色主行动；场景选中状态有文字和填充共同表示，非选中使用浅灰。 |
| 图像质量／素材 | 主视觉和三张灵感图为独立原创影像，使用 Next Image 提供尺寸与裁切；所有示例图均标明非本人衣橱，未用 CSS、Emoji 或占位形状替换视觉素材。 |
| 文案／内容 | 首页讲清场景入口和真实衣橱，灵感卡明确是示例；真实候选页逐件显示本人衣物、理由、未知和无解，不把示例穿搭冒充结果。 |

## Comparison history

1. 首次实现截图 `home-desktop.png`：P1 首页上方留白使灵感卡掉出首屏，卡片还是竖向照片而非概念稿横向结构。修正首页专用 PageShell 顶部节奏，并用 shadcn Card 改为左图右文。
2. 修正后截图 `home-desktop-v2.png`：P2 宽屏内容被全局最大宽度收窄，标题和主 CTA 比概念稿小。首页改为宽屏左右贴近稿面，Hero 放大至 64px，主行动改为 56px 高的 shadcn size variant。
3. 最终截图 `home-desktop-final.png`、完整并排图与两张重点区域并排图：重新核对上面两项已解决；剩余差异列于 Findings，均为明确产品约束或 P3 影像替换。
4. 320px 首查导航四项分成两行；将窄屏导航间距从 24px 降至 16px，复查四项在同一行，主 CTA 不横向溢出。390px、768px 页面 `scrollWidth` 小于或等于视口宽度。

## 浏览器交互

在本机 Next + Go API + 隔离 PostgreSQL 环境中检查：场景入口、未登录提示、注册合成账号、创建三件真实衣物、生成候选、主动保存计划、实际列表仍为空，以及正式度未知时无解。候选可见截图为同目录 `recommend-result.png`；浏览器控制台 error 日志为空。该记录属于当时版本，当前验证结果以当次交付为准。

## 品牌方案 3“衣序”复核

- Source visual truth：`frontend/public/brand/` 中的当前品牌资源。独立透明图形、小尺寸版本与深色主屏幕图标已保存于 `frontend/public/brand/`。
- Rendered implementation：本机 `http://127.0.0.1:3017/` 的内置浏览器截图，522×733 px、亮色、首页默认场景；截图随浏览器核对输出内联展示，未另存为文件。概念板与同一时刻的页面截图在同一次图像输出中打开核对。当前 Header 使用 32×32 px 导航标识与字标，不把整张 1254×1254 概念板当作 522×733 页面稿做像素比较。
- 字体／文案：导航使用可读“衣序”；首页、推荐、实际穿着和站点标题使用新名称。概念板的大字标在窄屏导航缩为组件字体，属于空间约束下的有意变化。
- 布局／颜色：标识与文字在窄屏同列，右侧为菜单按钮；账户入口在菜单内。珊瑚色只出现在标识落点，按钮与状态仍使用既有语义 token。
- 图像质量：真实 Image Gen 图形资产保留开口线框、越过门槛的弧线和单点，透明底小尺寸版本在首页截图中完整可辨；深色方形应用图标与概念板反白方向一致，`apple-touch-icon` 路径在实际 HTML 中指向该资产且资源返回 PNG。未以 CSS 或手绘 SVG 代替标识。
- 交互：从首页进入推荐页，标题显示“真实衣橱推荐 · 衣序”，再点击品牌链接返回首页；页面可达且导航语义清楚。品牌资产没有改变推荐数据或示例标注。
- Findings：无待修复 P0/P1/P2。P3：概念板字标有定制字形，当前 Web 用系统中文字体组合，正式跨端字标资产可在 iOS 品牌实施时进一步统一。商标、iOS 应用图标和公开发布不在本次 Web QA 范围。

## 全站 BasicLayout 复核

- 原始检查时根布局一次渲染共享 Header、唯一 `main-content` 与 Footer；登录、注册的共享结构现已由下方「独立认证页复核」更新。当前业务路由组复用 BasicLayout，认证路由组独立渲染 AuthLayout。
- 320、390、1440px 视口下无横向溢出；320px 时 Header、内容与 Footer 左边界均为 16px，1440px 时均为 128px。320px 的 404 页显示页脚且主内容将其推至视口底部；首页保留独立的纵向节奏。
- 根据用户提供的 shadcn 文档页参考，Header/Footer 改用无边框表面和文字层级；桌面导航由官方 NavigationMenu 组合，窄屏由官方 Sheet 组合。当前页以字重和下划线标识。1440px 首页与页脚、390px 首页与打开的菜单均已截图目视核对；Sheet 有标题和关闭按钮，点击“我的衣橱”后菜单关闭并按未登录状态进入登录页。320px 首页 `scrollWidth` 与视口同为 320px。
- 浏览器页面错误为空；前端 lint、15 项测试、typecheck、格式检查和生产构建通过。远端 CI 与发布验收未在本次本地布局改动中运行。

## 精简 Footer 与组件规范复核

- 按用户提供的 FrameFetch 页脚截图，仅参考单行排版和轻量文字层级：衣序 Footer 保留品牌、简短说明与四个实际路由；没有引入不属于产品的 MIT、GitHub、搜索或文档入口。桌面 1440px 实际截图 `/Users/stephenqiu/.codex/visualizations/2026/09/26/01a0db4b-02b7-7910-969c-d07fd28f72c1/footer-desktop-compact.png` 显示 Footer 为单行；390px 实际截图同目录 `footer-mobile-compact.png` 显示自然换行，320px 页面 `scrollWidth=320px`。
- 对照 Vercel Next.js 与 shadcn 规则核对源码：Header 静态品牌为 Server Component，导航交互及账户链接单独为 Client Component；表单的衣物多选和关联删除策略使用官方 Checkbox/RadioGroup，三个列表和两处编辑器空态使用官方 Empty。源码业务组件扫描不再有原生 `button/input/select/textarea/img/hr/fieldset/legend` 标签（基础组件源码与根跳转链接除外）。
- 本地 lint、15 项测试、typecheck、format:check、production build 通过；浏览器桌面/窄屏导航可见、Sheet 开关正常且错误日志为空。原 Playwright 注册用例引用旧首页文案和入口，已更新为当前首页 → 账户 → 登录 → 注册流程。另用模拟会话及衣物在浏览器验证计划中选择待洗衣物、确认后取消，以及删除影响弹窗的两项历史处理策略；4 项 Playwright 用例全部通过。本轮没有重新跑真实后端的衣物选择与删除旅程。

## 精简 Header 复核

- 桌面 1440px 首页截图 `/Users/stephenqiu/.codex/visualizations/2026/09/26/01a0db4b-02b7-7910-969c-d07fd28f72c1/header-desktop-compact.png`：Header 为无边框单行，左侧品牌、右侧五个文字导航；当前页以字重和下划线强调。390px 首页截图同目录 `header-mobile-compact.png`：仅显示品牌和菜单按钮；`header-mobile-menu.png`：Sheet 内含五个路由，账户可见。
- 390px 菜单点击“账户”后关闭，并按未登录状态进入登录页；320px 首页 `scrollWidth=viewport=320px`，浏览器页面错误日志为空。移动菜单保留可访问的标题、描述和关闭按钮，描述不占视觉空间。

## 独立认证页复核

- 参考 shadcn 官方 `login-03` / `signup-03`：柔和的全屏背景上，衣序标识居中置于白色表单卡上方，卡内用官方 Card、Field、Input、Button 与 Alert；返回首页在卡外。登录、注册使用不同 URL，并提供互跳链接。真实产品只支持邮箱密码，故不展示未接入的社交登录、找回密码或协议入口。
- 1280×800 登录截图：`/Users/stephenqiu/.codex/visualizations/2026/09/26/01a0db4b-02b7-7910-969c-d07fd28f72c1/then-auth-login-desktop.png`。390×844 登录和注册截图：同目录 `then-auth-login-mobile.png`、`then-auth-register-mobile.png`。已目视检查品牌与表单层级、注册字段在窄屏可见、无横向溢出。内置浏览器的生产构建还验证了登录→注册页面互跳和字段可访问名称。
- Playwright 使用模拟未登录会话核对认证页不包含业务 Header/Footer、路由互跳和 404 唯一主区域；已有注册流程和确认密码错误用例继续覆盖表单行为。浏览器自动化并不代表真实邮件投递、生产认证或辅助技术验收。

## Implementation checklist

- [x] 选定稿与同尺寸桌面截图并排比较。
- [x] Hero、卡片两处重点区域比较，修正首次 P1/P2 差异后重新截图。
- [x] 检查 320/390/768 响应式、交互、示例标注和控制台。
- [x] 确认示例视觉不代替真实衣橱推荐事实。
- [x] 将选定的品牌概念板与浏览器实际导航同次并列核对，检查图形、字标、小尺寸可读性和返回首页行为。

final result: passed
