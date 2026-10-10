# 共享视觉约定

## 来源与复用

`web/src/app.css` 的 `:root` 管理共享颜色、圆角和字体：`--bg` 是外层背景，`--panel` 是主面板，`--raised` 是抬升表面，`--hover`/`--selected` 表达交互状态，`--text` 至 `--faint` 分配文字层级，`--accent`/`--accent-hover` 表达主要操作。当前固定 dark color-scheme，使用 Inter Variable；没有主题切换器。

共享状态控件与元信息在 `shared.tsx` 的 `StatusIcon`、`StatusSelect`、`IssueMeta`；`.button`/`.primary`、`.chip`、`.error`、`.issue-dialog` 与 `.icon` 在 CSS 中统一消费。改全局角色先改已有 token 或共享规则，局部布局则定位消费选择器，不另造平行主题系统。

## 图标

界面图标来自 `lucide-react`（ISC 许可）。组件写 `className="icon"`，尺寸和描边由 `.icon` 统一设为 16px、1.5，不在组件上传 `size` 或 `strokeWidth`。现有对应：`List` 是全部 issue 和列表视图，`Columns2` 是看板视图，`Box` 是 Project，`ChevronDown` 是分组折叠，`Search` 是搜索，`Tag` 是 type 筛选，`X` 是清除筛选和关闭弹窗。lucide 在没有 accessible name 时自动加 `aria-hidden`；只有图标的按钮在按钮上写 `aria-label`。

状态图标由 `shared.tsx` 的 `StatusIcon` 在 14×14 网格上自绘，按 14px 显示，不用 lucide，也不从其他项目复制代码。形状和颜色都写在组件里。颜色参照 Linear 的状态配色：todo、in_progress、done 按 Linear 截图取色；triage、in_review、canceled 的取值没有找到公开来源核实；agent_ready 在 Linear 没有对应状态，用自定的紫色。

- triage：虚线圈，`#bec2c8`
- todo：空心圈，`#e2e2e2`
- agent_ready：空心圈加中心小点，`#b49bdd`
- in_progress：圈内填一半扇形，`#f2c94c`
- in_review：圈内填四分之三扇形，`#4cb782`
- done：实心圆加对勾，`#5e6ad2`
- canceled：实心圆加叉，`#95a2b3`

对勾和叉用 `--panel` 色画在实心圆上。侧栏里状态图标和 16px 图标同列，`.nav-item .status-icon` 左右各留 1px 让文字对齐；列表分组头的 `.chevron` 左移 2px，让分组头的状态图标对齐行内的状态图标。

## 状态与可访问外观

状态图标供侧栏、列表、看板和详情复用。列表/卡片中的原生 select 透明覆盖图标；不要把透明误认为无需保留，键盘操作与 accessible name 来自这个 select。`.status-control:focus-within` 显示焦点，链接和按钮有 `:focus-visible` 描边。禁用按钮降低不透明度并改变 cursor；保存流程见 [交互](INTERACTION_DESIGN.md)。

增加状态需同时核对共享 `columns` 和 `StatusIcon` 的颜色与图形（两张表按 `Status` 类型定义，缺项时前端构建报错），再在列表、卡片及详情检查，不凭单页截图证明全部场景。仓库没有自动视觉回归测试或对比度认证。

## 布局

桌面 `.app-shell` 是 232px 侧栏与主面板；1024px 以下详情隐藏右侧 properties、展示内联属性，760px 以下切换单列并隐藏部分侧栏与元信息。最小 body 宽度为 320px。改变断点或共享尺寸时检查两侧布局、长标题、Markdown 表格/代码横向滚动与焦点可见性，按 [验证](VALIDATION.md) 分别报告构建与人工结果。
