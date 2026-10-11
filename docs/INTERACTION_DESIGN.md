# 导航、写入与恢复

## 导航与任务上下文

`shared.tsx` 用 History API 和 popstate 导航；页面都在 `/<slug>/` 下，路径第一段选 workspace，详情路径是 `/<slug>/issue/<identifier>`，Go 静态处理器提供直达路由回退。`/` 读取 workspace 列表，用 `replaceState` 换成按 slug 排序的第一个 `/<slug>/`；没有 workspace 时显示用 `POST /api/workspaces` 创建的提示。不存在的 slug 显示接口返回的 workspace not found 错误。`App.tsx` 的 `WorkspaceView` 持有当前路径、列表/看板及排序偏好；`ScopeView` 每次 History 导航重新挂载，将筛选、批量选择、折叠与弹窗留在当前目的地。刷新恢复 URL 范围，但不恢复临时筛选或视图偏好。

左上角 `WorkspaceMenu.tsx` 显示当前 workspace 名称与箭头，菜单按 slug 列出全部 workspace、标记当前项，提供 Create workspace… 与 Rename workspace…。切换及创建成功都进入对应 `/<slug>/`，组件重新挂载并重置原 workspace 的筛选、选择和视图；浏览器前进后退按 URL 恢复 workspace。创建填 slug 和名称，改名只填名称；保存期间禁用输入和关闭，浏览器导航离开不会取消已发送的写入，但旧响应不会再导航或更新弹窗。成功后显示服务端返回的名称，失败保留输入并显示 role=alert。菜单重新打开时重读列表，加载失败可关闭重开；菜单及弹窗支持 Tab/Enter 和 Escape，关闭后焦点回到左上角入口。窄屏保留该入口。

Sidebar 的 Workspace 有 All issues、All projects 与 Labels；每个 Team 有 Issues、Projects，Team 名称旁保留 emoji 与改名入口。侧栏不再显示状态或局部结果数；状态选择器和当前结果数放在任务内容区。窄屏用 Navigation 按钮展开同一组目的地，选择后收起。Teams 旁的 “+” 沿用 Create team 弹窗，成功后进入新 Team 的 Issues 范围，失败保留输入并显示 role=alert。

范围 URL：`/<slug>/` 是全部任务，`/<slug>/projects` 是全部项目，`/<slug>/team/<key>/issues` 与 `/<slug>/team/<key>/projects` 是 Team 范围，`/<slug>/project/<id>` 是项目全部任务；Project 使用稳定 id，Team 使用不可变 key（保留迁移产生的历史 key），已有 issue 详情路径继续可用。直达、刷新、前后退恢复范围。每次进入 sidebar 目的地或项目都清空 status、type、priority、标签、搜索、显式 Team 筛选和批量选择，包括再次点击同一路径；同 workspace 内保留 list/board 和排序。

`ProjectList.tsx` 展示项目列表，默认排除归档，Active projects / Archived projects 可切换活动与归档列表；Team 列表依据 Project 的显式关联过滤，空项目仍展示，历史待归属项目只在全部项目中展示。进入项目不继承来源 Team，内容区提供显式 Team 筛选和可清除 chip。项目名称与说明来自 by-id 查询；归档项目直达可查看已有票，待归属/归档项目禁用新建任务并说明原因。无 Team 时提供创建入口；空项目列表说明没有当前范围的活动项目；失效范围和请求失败显示错误、Retry 与返回全部任务入口。

type、priority、标签、搜索和显式 Team 筛选参与 API 查询，项目范围使用 project_id。搜索按 Enter 提交；状态在列表数据中选择可见列。全部状态且无搜索时默认折叠 Done/Canceled；具体状态或有搜索时展开。切状态、Team 或提交搜索清除折叠覆盖，范围导航重新初始化。折叠组不渲染行，看板不折叠。新建继续沿用当前 Team、Project 和状态，未指定 Team 时仍由既有表单预选按 key 排序的第一个；完整新建归属交互另票处理。创建成功重读当前范围，筛选决定是否显示新票。

## 项目管理

全部项目与 Team 项目列表的 New project 打开创建表单，Team 范围预选当前 Team，全局范围必须显式选择至少一个 Team。名称、说明和多 Team 使用共享 modal 与原生 checkbox；保存成功保留列表范围并重读，展示 View project 入口，空项目立即按关联可见。归档列表发起创建后回到活动列表。

稳定项目页展示说明、关联 Team 和任务，Edit project 修改名称、说明或关联，URL 始终使用同一 id。历史待归属项目标明状态并提供 Assign teams / Edit project；仅修改名称或说明时省略未变化的 teams，不强迫归属。移除仍有该 Team 任务的关联由服务端拒绝，错误和草稿留在表单。Archive project 后默认列表不再展示，Archived projects 中可直达并 Restore project；归档详情仍显示原任务，新建任务禁用。

保存期间禁止关闭与修改，失败保留输入可重试，取消不写入；目录读取失败不会卸载已打开的表单，范围导航卸载后旧保存回调不影响新页面。成功后重读当前范围，不由客户端猜测归属。名称空白与重名分别由Service返回必填与冲突错误。

## Team 显示名称

侧栏每个 Team 的铅笔按钮和当前 Team 范围标题旁按钮打开 Rename team 对话框；窄屏可展开 Navigation 或从当前标题进入。名称去首尾空白后必填，提示 key 与已有票号保持不变。保存成功后替换 Team 缓存，侧栏、标题和新建任务的 Team 下拉立即使用服务端名称；失败保留输入并显示 role=alert。保存中禁用输入、提交和关闭，浏览器切换 workspace 导致卸载后旧响应不更新界面。取消不发送写入；Escape 关闭，焦点回到原入口。Project 改名由项目编辑表单及 REST/MCP 提供。

## 项目与团队 emoji

点击侧栏图标打开共用 emoji 弹层，名称按钮仍用于筛选；当前项目或团队的标题也有入口，侧栏分组隐藏时可继续编辑。输入立即按英文名称、常用别名或 emoji 过滤；无结果时显示空态，清除搜索回到常用及分类列表。搜索框 ArrowDown 进入网格，Enter 选择首个结果；网格支持方向键、Home/End 和 Enter/Space。Escape 或点击外部关闭，Escape 后焦点回到图标按钮；在 Create team 内只关闭 emoji 弹层，保留团队草稿。

已有项目/团队等待 PATCH 成功才更新图标和关闭；写入期间禁用选择及关闭，失败保持弹层、搜索词和原图标，显示错误，可以再次选择。Restore default 写入空值，刷新后仍保留服务端值。Create team 的 emoji 属于草稿，只随创建请求保存，失败保留草稿，取消不会新建团队。最近成功选择保存在浏览器 localStorage；存储不可用时仍能选择和保存图标。

## 优先级和排序

priority 独立于 status/type/标签。列表、看板及详情的选择器逐次保存；写入期间禁用该票的优先级控件，服务端成功后才更新显示，失败保留此前值并显示错误。创建与编辑弹窗的优先级属于草稿，提交成功后持久化，失败保留草稿。原生 dialog 内也使用同一 Radix 选择器，支持 Tab、Enter/Space、方向键和 Escape，用 Escape 关闭菜单后焦点回到触发器。

优先级筛选参与 API 查询，包含显式 No priority。切换筛选清除批量选择；修改后的票不再匹配当前优先级筛选时重读列表。默认排序仅在 Agent ready 分组按 Urgent→High→Medium→Low→No priority，其他状态按最近更新；可全局切换 Last updated 或 Priority，同级按最近更新。筛选和排序只保存在组件内存中，刷新后重置。MCP/REST 的 `status=agent_ready` 默认优先级排序，显式 `sort=updated` 覆盖；其他查询默认更新时间倒序。服务端在 limit 之前排序，同时间按内部 id 倒序。未实现 Linear 的同档手动顺序、全局 P 快捷键和 Urgent 通知。

## 自动明暗主题

页面默认跟随系统的 `prefers-color-scheme`，没有深色偏好时使用 light。首次加载、刷新和详情直达使用同一套 CSS 媒体查询；打开页面后系统偏好改变无需刷新。没有手动覆盖或 localStorage 主题状态。主题变化只改变样式，不卸载组件，不关闭 dialog 或清空编辑草稿。原生表单控件通过 `color-scheme` 跟随主题，浏览器 UI 的 `theme-color` 通过两条带 media 的 meta 跟随。token 分类与字面量边界见 [设计系统](DESIGN_SYSTEM.md)。

## 写入完成点

列表/看板移动状态先乐观更新，失败恢复该票此前值并展示错误；详情状态等待响应后更新。两处都不能把发出请求当作持久化完成。失败仅恢复发起时的该票，范围或查询已改变时不再应用旧写入回调；不保证同一票多次交错写入的顺序。

新建与编辑使用原生 dialog，打开后聚焦标题；保存期间禁用提交按钮，成功关闭，失败保留内容并显示 role=alert。关闭/取消没有未保存草稿保护，不能承诺关闭后可恢复。评论成功后追加响应并清空输入，失败保留文本，提交期间禁用按钮。

页面在窗口重新获得焦点时重新加载数据，未使用 WebSocket 推送；列表还会随路径、Project、type、标签条件和已提交的搜索词变化加载。列表只应用当前挂载范围、当前查询的最新加载响应；离开范围后旧查询和旧写入回调不更新新范围。详情或标签页的非关键项目/团队目录失败只显示提示，不卸载编辑器和草稿；目录 Retry 不重建这些组件。详情首次加载失败可能仍显示 Loading 与全局错误，当前没有独立重试按钮。

## 标签操作

标签管理页为 `/<slug>/labels`。名称、颜色、描述和归档状态在原生 dialog 中编辑；失败保留草稿，创建时提示相似名称。标签所属组创建后固定，组支持创建与改名。合并前展示关联转移及源 ID 删除的影响。

详情标签选择器支持搜索与原生 checkbox 键盘操作，逐次增量保存，保存期间禁用选择，等待服务端成功和详情重读后显示结果。批量添加/移除由原生 dialog 提交一次事务；组内添加自动替换，移除可以同时选择多个组内值。列表/看板展示前两个标签和 `+N`。标签筛选对话框使用显式 Apply/Clear；组选项匹配任意子标签，any/all/none 组合所选标签和组，unlabeled 不允许再选标签。标签筛选参与 API 查询，筛选与批量选择不跨刷新持久化。

## 修改时验证

涉及保存与刷新时，用浏览器检查成功、服务不可达、慢响应和连续操作后的页面与实际记录；重点查看失败是否丢失文本、旧响应是否覆盖新选择。不要把这些需要检查的情形写成已经自动通过的保证。

状态可通过原生 select 修改，拖动不是唯一入口。检查 Tab、Enter、Escape、对话框关闭后的焦点和浏览器前进后退；窄屏下检查内联属性仍可操作。视觉来源见 [设计系统](DESIGN_SYSTEM.md)，执行环境见 [开发](DEVELOPMENT.md)。


## 云端登录会话

启用 UC 时，未登录的页面访问跳到 UC，成功后回到原始相对路径；API 过期返回 401 而不是登录 HTML。侧栏显示当前用户和退出按钮；退出失败保留页面并显示错误，成功进入已退出页。API 报会话过期时不卸载编辑器，在新标签页重新登录，返回时按既有 focus 刷新规则加载；不保证跨刷新保留草稿。
