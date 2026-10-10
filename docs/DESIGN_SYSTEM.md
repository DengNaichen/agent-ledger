# 共享视觉约定

## 来源与复用

`web/src/app.css` 的 `:root` 管理共享颜色、圆角和字体：`--bg` 是外层背景，`--panel` 是主面板，`--raised` 是抬升表面，`--hover`/`--selected` 表达交互状态，`--text` 至 `--faint` 分配文字层级，`--accent`/`--accent-hover` 表达主要操作。当前固定 dark color-scheme，使用 Inter Variable；没有主题切换器。

共享状态控件与元信息在 `shared.tsx` 的 `StatusSelect`、`IssueMeta`；`.button`/`.primary`、`.chip`、`.error`、`.issue-dialog` 与 `.status-icon-*` 在 CSS 中统一消费。改全局角色先改已有 token 或共享规则，局部布局则定位消费选择器，不另造平行主题系统。

## 状态与可访问外观

状态图标供侧栏、列表、看板和详情复用。列表/卡片中的原生 select 透明覆盖图标；不要把透明误认为无需保留，键盘操作与 accessible name 来自这个 select。`.status-control:focus-within` 显示焦点，链接和按钮有 `:focus-visible` 描边。禁用按钮降低不透明度并改变 cursor；保存流程见 [交互](INTERACTION_DESIGN.md)。

增加状态需同时核对共享 `columns` 和图标 CSS，再在列表、卡片及详情检查，不凭单页截图证明全部场景。仓库没有自动视觉回归测试或对比度认证。

## 布局

桌面 `.app-shell` 是 232px 侧栏与主面板；1024px 以下详情隐藏右侧 properties、展示内联属性，760px 以下切换单列并隐藏部分侧栏与元信息。最小 body 宽度为 320px。改变断点或共享尺寸时检查两侧布局、长标题、Markdown 表格/代码横向滚动与焦点可见性，按 [验证](VALIDATION.md) 分别报告构建与人工结果。
