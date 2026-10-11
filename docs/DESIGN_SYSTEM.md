# 共享视觉约定

## 来源与复用

`web/src/app.css` 的 `:root` 定义基础尺度和 light 颜色；`@media (prefers-color-scheme: dark)` 只覆盖主题颜色与 `color-scheme`。无深色偏好时使用 light，页面打开期间跟随系统变化。使用 Inter Variable，没有手动主题开关或主题偏好存储。

基础尺度保留现有紧凑密度，消费规则用 CSS custom properties；原始视觉值集中在 token 定义。同一个数值承担不同角色时可分别命名，避免改一个控件时意外改变另一个。

| 类别 | Token 与用途 |
| --- | --- |
| 表面与交互 | `--bg` 画布、`--panel` 面板、`--raised` 抬升表面；`--hover`/`--selected`、`--button-bg`/`--button-hover`、`--chip-bg`、`--card-hover`、`--drop-bg` 分开表达状态 |
| 文字与边界 | `--text`、`--text-2`、`--text-3`、`--muted`、`--faint`；`--border`/`--border-strong` 与 `--link`/`--link-underline` |
| 操作与反馈 | `--accent`/`--accent-hover` 配 `--accent-text`；`--success`、`--error-text`/`--error-bg`/`--error-border`；`--pr-open`/`--pr-merged`/`--pr-closed` 各有 border token |
| 弹窗与代码 | `--dialog-bg`、`--backdrop`、`--shadow-dialog`、`--code-bg` |
| 间距与字体 | `--space-*` 是现有像素间距尺度；`--space-prose-*` 是随正文大小变化的 em 间距；`--font-size-*`、`--weight-*`、`--line-*`、`--tracking-*`、`--font-ui`、`--mono`、`--font-features`、`--font-optical-heading` 控制字体 |
| 形状与尺寸 | `--radius*`、`--border-width`、`--focus-width`、`--icon-size`/`--icon-stroke`、`--status-icon-size`；`--sidebar-width`、`--board-column-width`、`--property-panel-width`、`--content-max-width`、`--dialog-width` 等表达组件尺寸 |
| 其他共享值 | `--opacity-disabled`、`--duration-fast`、`--font-code-relative`、`--code-tab-size`；列表三列控件共用 `--list-control-size`，日期列用 `--list-date-width` |

light 使用浅色画布、白色面板、深色文字和靛蓝操作色；状态色按浅色背景调整明度。dark 保留既有配色。两套颜色都在 `app.css`，是 InfTask 的设计选择；[Refero 的 Linear 风格参考](https://styles.refero.design/style/90ce5883-bb24-4466-93f7-801cd617b0d1) 仅作为 token 组织参考，没有提供 light 配色，也不是 Linear 官方应用规范。

共享状态控件与元信息在 `shared.tsx` 的 `StatusIcon`、`StatusSelect`、`IssueMeta`；`.button`/`.primary`、`.chip`、`.error`、`.issue-dialog` 与 `.icon` 在 CSS 中统一消费。改全局角色先改已有 token 或共享规则，局部布局则定位消费选择器，不另造平行主题系统。type 筛选箭头用两片 CSS 三角形消费 `--muted`，不在 SVG data URL 中编码颜色。

允许保留的字面量边界：

- `0`、百分比、viewport 单位、`auto`/`none`/`transparent`/`currentColor`、Grid 列位置与 flex 比例表达布局结构；旋转角度表达箭头几何。
- 状态和优先级 SVG 的 viewBox、路径、圆心、半径、扇形、虚线和属性尺寸分别是 14×14 和 16×16 几何；实际显示尺寸由 CSS token 控制。lucide 路径由库管理。
- CSS media 条件保留 `1024px` 和 `760px`：原生 custom property 不能用于 media 条件。不能为 token 整理改变断点或引入另一套预处理系统。
- `LabelBadge` 的 `label.color` 与标签编辑器的初始颜色是业务数据，不随主题改写。新增组件颜色不属于此例外。
- `web/index.html` 的两条 `theme-color` meta 分别用带 media 的 `#f5f6f8` 和 `#08090a`，必须与两个 `--bg` 同步；meta content 不能直接引用 CSS var。这是 HTML 平台边界，其他消费点不可照此复制色值。

## 图标

标签使用 `shared.tsx` 的 `LabelBadge`：颜色仅用于小圆点，文字沿用主题 token；长名称截断，title 显示完整名称和描述，归档标记单独展示。管理页和选择器样式集中在 `app.css` 的标签区块。

界面图标来自 `lucide-react`（ISC 许可）。组件写 `className="icon"`，尺寸和描边由 `.icon` 统一设为 16px、1.5，不在组件上传 `size` 或 `strokeWidth`。现有对应：`List` 是全部 issue 和列表视图，`Columns2` 是看板视图，`Box` 是 Project，`ChevronDown` 是分组折叠，`Search` 是搜索，`Tag` 是 type 筛选，`X` 是清除筛选和关闭弹窗。lucide 在没有 accessible name 时自动加 `aria-hidden`；只有图标的按钮在按钮上写 `aria-label`。

状态图标由 `shared.tsx` 的 `StatusIcon` 在 14×14 网格上自绘，按 14px 显示，不用 lucide，也不从其他项目复制代码。形状写在组件里，`statusColors: Record<Status, string>` 指向 `--status-*` 颜色 token。dark 颜色参照 Linear 的状态配色：todo、in_progress、done 按 Linear 截图取色；triage、backlog 采用 AL-32 指定的颜色；in_review、canceled 的取值没有找到公开来源核实；agent_ready 在 Linear 没有对应状态，用自定的紫色。

| 状态 | 形状 | Token | dark | light |
| --- | --- | --- | --- | --- |
| triage | 实心圆加 ⇄ 形状的双箭头 | `--status-triage` | `#fc7840` | `#fc7840` |
| backlog | 虚线圈 | `--status-backlog` | `#bec2c8` | `#737b86` |
| todo | 空心圈 | `--status-todo` | `#e2e2e2` | `#626974` |
| agent_ready | 空心圈加中心小点 | `--status-agent-ready` | `#b49bdd` | `#8665b0` |
| in_progress | 圈内填一半扇形 | `--status-in-progress` | `#f2c94c` | `#987000` |
| in_review | 圈内填四分之三扇形 | `--status-in-review` | `#4cb782` | `#258253` |
| done | 实心圆加对勾 | `--status-done` | `#5e6ad2` | `#5e6ad2` |
| canceled | 实心圆加叉 | `--status-canceled` | `#95a2b3` | `#697789` |

双箭头、对勾和叉用 `--panel` 色画在实心圆上。侧栏里状态图标和 16px 图标同列，`.nav-item .status-icon` 左右各留 1px 让文字对齐；列表分组头和条目共用三列 24px 控件空间，展开箭头与复选框居中于首列，优先级居中于第二列，状态图标居中于第三列。

## Workspace 菜单

左上角 `WorkspaceMenu.tsx` 使用已有 Radix Popover 与原生 ModalDialog：触发器显示名称和 ChevronDown，长名称截断并用 title 提供完整名称；列表显示名称、slug 和当前项 Check。弹层消费已有 sidebar 宽度、主题、边界、圆角及菜单阴影 token，受视口宽高限制，名称可换行且列表内部滚动。新建和改名复用 `.issue-dialog` 表单；窄屏隐藏 sidebar 分组时保留 Workspace 触发器。

## 项目与团队 emoji

`EmojiPicker.tsx` 使用 `@radix-ui/react-popover`（MIT）。项目/团队的 emoji 入口与导航按钮并列；空值沿用 Project 的 Box 和 team 的缩写。布局参考本地 Linear 应用的 Emoji 页：顶部搜索、Frequently used 和带标题的连续分类网格；只提供 emoji，没有 Icons 页和颜色选择。桌面弹层宽/高约 450px、14 列，窄屏按可用宽度减少列数（390px 视口为 11 列），宽度不超过视口减 16px，高度按可用空间缩短，内容内部滚动。`--emoji-picker-width`、`--emoji-picker-height`、`--emoji-cell-height` 和 `--font-emoji` 控制尺寸与系统 emoji 字体，其余消费现有主题 token。原生 dialog 内的 portal 挂在 dialog 上。

目录来自 Unicode 16.0 的 fully-qualified emoji 数据，存于 `internal/app/emojidata/emoji.json`，Go 和前端共用；来源与 Unicode 许可见同目录 LICENSE 和 `emoji.go`。默认分类浏览省略肤色变体，按完整英文名称或粘贴 emoji 可搜索这些序列。表情用本机字体绘制，不同系统的字形可能不同。Frequently used 由最近成功选择及常用默认项组成，浏览器 localStorage 仅保存最多 28 个 emoji，无外部图片/CDN 请求。

## 优先级

Priority 的档位与 [Linear](https://linear.app/docs/issue-property#priority) 对齐：No priority、Urgent、High、Medium、Low，对应 0–4。`shared.tsx` 的 `PriorityIcon` 自绘 16px 图形，参照官方菜单截图：未设置为三段横线，紧急为圆角方块加感叹号，其他档位为三根递增的实心柱、以亮柱数量表示级别。紧急使用 `--priority-urgent`（dark `#f2994a`、light `#a75b14`，项目自定强调色），其余消费现有文字 token；菜单阴影使用 `--shadow-priority-menu`、宽度使用 `--priority-menu-width`、未亮柱透明度使用 `--priority-muted-opacity`；不声称所有像素或取色与 Linear 完全一致。

`PrioritySelect` 使用开源 `@radix-ui/react-select` 基础控件（MIT），项目 CSS 消费现有 light/dark token 控制菜单、悬停、当前项勾选和焦点；没有引入 shadcn/Tailwind。列表和看板为 24px 图标触发器，详情及创建/编辑弹窗展示图标加名称。列表共享列依次为选择、优先级、状态、编号、标题、元信息及日期，分组标题保留优先级列空位。原生 dialog 内的菜单 portal 挂到 dialog 本身，保持在 modal 的 top layer 中，否则会被遮挡或无法操作。

## 状态与可访问外观

状态图标供侧栏、列表、看板和详情复用。列表/卡片中的原生 select 透明覆盖图标；不要把透明误认为无需保留，键盘操作与 accessible name 来自这个 select。`.status-control:focus-within` 显示焦点，链接和按钮有 `:focus-visible` 描边。禁用按钮降低不透明度并改变 cursor；保存流程见 [交互](INTERACTION_DESIGN.md)。

增加状态需同时核对共享 `columns` 和 `StatusIcon` 的颜色与图形（两张表按 `Status` 类型定义，缺项时前端构建报错），再在列表、卡片及详情检查，不凭单页截图证明全部场景。仓库没有自动视觉回归测试或对比度认证。

## 布局

桌面 `.app-shell` 是 232px 侧栏与主面板；1024px 以下详情隐藏右侧 properties、展示内联属性，列表元信息换到第二行并隐藏日期，760px 以下切换单列并隐藏部分侧栏与 Project 元信息。筛选控件允许换行。列表使用 CSS Grid 和 subgrid 共享列宽，编号列按当前可见条目的内容宽度分配，标题起点一致，列间距为 8px；分组条有圆角，列表左右留白为 16px，760px 以下为 8px。空组文字及第二行元信息与编号列起点对齐。最小 body 宽度为 320px。改变断点或共享尺寸时检查两侧布局、长标题、Markdown 表格/代码横向滚动与焦点可见性，按 [验证](VALIDATION.md) 分别报告构建与人工结果。
