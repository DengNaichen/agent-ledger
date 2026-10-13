# 导航、写入与恢复

## 导航与任务上下文

`shared.tsx` 用 History API 和 popstate 导航；页面都在 `/<slug>/` 下，路径第一段选 workspace，详情路径是 `/<slug>/issue/<identifier>`，Go 静态处理器提供直达路由回退。`/` 读取 workspace 列表，用 `replaceState` 换成按 slug 排序的第一个 `/<slug>/`；没有 workspace 时显示用 `POST /api/workspaces` 创建的提示。不存在的 slug 显示接口返回的 workspace not found 错误。`App.tsx` 的 `WorkspaceView` 持有当前路径、列表/看板及排序偏好；页面内容每次 History 导航重新挂载，将筛选、批量选择、折叠与弹窗留在当前目的地；Sidebar、WorkspaceMenu、Session 和团队目录在同 workspace 内保持挂载。刷新恢复 URL 范围，但不恢复临时筛选或视图偏好。

左上角 `WorkspaceMenu.tsx` 显示当前 workspace 名称与箭头，菜单按 slug 列出当前用户有权访问的 workspace、标记当前项，提供 Create workspace… 与 Rename workspace…；云端改名只向 Admin 启用。切换及创建成功都进入对应 `/<slug>/`，组件重新挂载并重置原 workspace 的筛选、选择和视图；浏览器前进后退按 URL 恢复 workspace。创建填 slug 和名称，改名只填名称；保存期间禁用输入和关闭，浏览器导航离开不会取消已发送的写入，但旧响应不会再导航或更新弹窗。成功后显示服务端返回的名称，失败保留输入并显示 role=alert。菜单重新打开时重读列表，加载失败可关闭重开；菜单及弹窗支持 Tab/Enter 和 Escape，关闭后焦点回到左上角入口。窄屏保留该入口。

Sidebar 的 Workspace 有 All issues、All projects、Labels 与 Settings；每个 Team 有 Issues、Projects，Team emoji 只作展示。点击 Team 标题行（含右侧箭头）展开/收起自己的子导航，不改变 URL；各 Team 默认展开并独立折叠，状态在同 workspace 导航期间保留，切换 workspace 或刷新后重置。标题是带 aria-expanded、aria-controls 的原生按钮，支持 Tab、Enter、Space；收起后的子链接不进入 Tab 顺序。Settings 直接进入正式页面，不经过浮窗菜单。侧栏不再显示状态或局部结果数；状态筛选收进内容区的 Filter，当前结果数留在任务工具栏。窄屏用 Navigation 按钮展开同一组目的地，选择后收起。Teams 旁的 “+” 沿用 Create team 弹窗，成功后进入新 Team 的 Issues 范围，失败保留输入并显示 role=alert。

范围 URL：`/<slug>/` 是全部任务，`/<slug>/projects` 是全部项目，`/<slug>/team/<key>/issues` 与 `/<slug>/team/<key>/projects` 是 Team 范围，`/<slug>/project/<id>` 是项目全部任务；Project 使用稳定 id，Team 使用不可变 key（保留迁移产生的历史 key），已有 issue 详情路径继续可用。直达、刷新、前后退恢复范围。每次进入 sidebar 目的地或项目都清空 status、type、priority、标签、搜索、显式 Team 筛选和批量选择，包括再次点击同一路径；同 workspace 内保留 list/board 和排序。

`ProjectList.tsx` 展示项目列表，默认排除归档，Active projects / Archived projects 可切换活动与归档列表；Team 列表依据 Project 的显式关联过滤，空项目仍展示，历史待归属项目只在全部项目中展示。进入项目不继承来源 Team，内容区提供显式 Team 筛选和可清除 chip。项目名称与说明来自 by-id 查询；归档项目直达可查看已有票，待归属/归档项目禁用新建任务并说明原因。无 Team 时提供创建入口；空项目列表说明没有当前范围的活动项目；失效范围和请求失败显示错误、Retry 与返回全部任务入口。

type、priority、标签、搜索和显式 Team 筛选参与 API 查询，项目范围使用 project_id。搜索按 Enter 提交；状态在列表数据中选择可见列。全部状态且无搜索时默认折叠 Done/Canceled；具体状态或有搜索时展开。切状态、Team 或提交搜索清除折叠覆盖，范围导航重新初始化。折叠组不渲染行，看板不折叠。列表中超过 50 条的展开分组只挂载视口及上下各 8 条缓冲行，按真实行高测量，保留分组吸顶边界；当前获得焦点的行及前后邻行保持挂载，供菜单焦点恢复和 Tab 连续遍历。分组计数、筛选、排序和 Select visible issues 始终基于完整查询结果，不受挂载窗口限制。浏览器自带页内查找只能匹配已挂载行，查找整个范围使用工具栏 Search。新建沿用当前状态，Team 与 Project 的默认值按下方归属规则决定。创建成功重读当前范围，筛选决定是否显示新票。

Filter 弹层容纳状态、Team、type、priority 和标签筛选，已应用条件在工具栏显示可清除 chip；搜索草稿保存在筛选栏内，输入不触发任务行重渲染；仍按 Enter 提交。Display 弹层容纳 List/Board、排序和 Select visible issues。两个弹层均可用 Tab、Enter 和 Escape 操作；标签筛选的原生 dialog 在 Filter 内打开，Escape 先关闭标签 dialog，焦点回到 Labels 入口，再次 Escape 关闭 Filter。没有选中当前范围条目时不显示批量工具栏；有选择时显示数量、Select visible、Change labels 和 Clear selection。条目复选框默认视觉隐藏但可键盘聚焦，hover/focus、已选中或已有选择时显示，触屏始终显示。

点击或用 Enter/Space 展开、收起状态分组时，以点击前标题的屏幕位置为锚点，在 React 布局提交后、浏览器绘制前补偿列表滚动；定位采用分组自然位置，避免吸顶标题掩盖跳过已展开行的问题。虚拟行随后测量高度时继续保持锚点，下一次滚轮、触摸、指针或键盘操作以及布局切换会释放它。收起后剩余内容不足时允许原生滚动边界限制，标题保持可见，不额外填充空白。

## 任务归属与编辑

New issue 明确展示 Team 和 Project。Team 任务页预选当前 Team；项目页预选 Project，只有一个关联 Team 时预选该 Team，共享项目要求用户选择。workspace 全部任务没有默认 Team；未选时 Project 与提交禁用。项目页的临时 Team 筛选不替代共享项目的新建选择。Project 可为 No project，候选是当前 Team 关联的未归档项目；切换 Team 保留原 Project、标题、正文与优先级，不兼容时标明原因并禁止提交，用户清空或重选后继续，不自动扩大项目关联。

详情的正文和属性组整体滚动，桌面属性在正文旁，窄屏排列到主内容流，保留所有属性和标签；详情不显示创建/更新时间。详情展示只读 Team 名称（title 含 key），Project 链接使用稳定 id。Edit 中 Team 只读，修改请求不含 Team；Project 可改为同一 Team 的活动项目或 No project。编辑器单独读取该 Team 的含归档项目目录，只允许保留当前归档 Project，不允许新加入其它归档项目。项目目录加载中或失败时禁用保存，Retry projects 保留其它草稿；Service 在提交时再次校验兼容性与归档边界。

创建和编辑以 project_id 提交，成功后显示 Created/Updated 与 View issue 链接，并重读当前范围；不匹配当前范围与筛选的票不强插列表。详情编辑与快捷字段、评论、标签共用同票写入队列，写入开始使先前详情读取失效；编辑成功应用返回字段并重读详情，不重读无关目录。新建和编辑的保存期间禁用可改字段、提交和关闭，失败保留草稿，取消不发送写入；离开范围后已发出的请求可能完成，但卸载后的回调不再更新新范围。

## 项目管理

全部项目与 Team 项目列表的 New project 打开创建表单，Team 范围预选当前 Team，全局范围必须显式选择至少一个 Team。名称、说明和多 Team 使用共享 modal 与原生 checkbox；保存成功保留列表范围并重读，展示 View project 入口，空项目立即按关联可见。归档列表发起创建后回到活动列表。

稳定项目页展示说明、关联 Team 和任务，Edit project 修改名称、说明或关联，URL 始终使用同一 id。历史待归属项目标明状态并提供 Assign teams / Edit project；仅修改名称或说明时省略未变化的 teams，不强迫归属。移除仍有该 Team 任务的关联由服务端拒绝，错误和草稿留在表单。Archive project 后默认列表不再展示，Archived projects 中可直达并 Restore project；归档详情仍显示原任务，新建任务禁用。

保存期间禁止关闭与修改，失败保留输入可重试，取消不写入；目录读取失败不会卸载已打开的表单，范围导航卸载后旧保存回调不影响新页面。项目表单成功后重读项目与受影响的任务范围，不由客户端猜测归属；仅改 emoji 时使用返回的项目详情，不重读任务。名称空白与重名分别由Service返回必填与冲突错误。

## Team 设置

`/<slug>/settings` 是正式的团队设置页，侧栏 Settings 直接进入；直达、刷新和浏览器前后退均按 URL 恢复。`SettingsPage.tsx` 展示各团队名称、emoji 与只读编号。名称和 emoji 先进入本地草稿，Save changes 一次 PATCH 保存；Reset 丢弃草稿，不发送写入。保存期间禁用该团队编辑，成功使用服务端返回值更新侧栏，失败保留草稿并显示 role=alert。重读目录不覆盖正在编辑的草稿；离开页面丢弃未保存修改，卸载后的写入回调不再影响新页面。团队页头也只展示图标和名称。Project 改名继续由项目编辑表单及 REST/MCP 提供。

## 项目与团队 emoji

Team 的 Settings 表单与当前 Project 页标题提供共用 emoji 选择器。项目列表名称进入稳定项目页，侧栏 Issues/Projects 进入对应范围；不再用名称叠加隐藏筛选。窄屏可展开 Navigation 进入 Settings；Project 使用当前标题入口。输入立即按英文名称、常用别名或 emoji 过滤；无结果时显示空态，清除搜索回到常用及分类列表。搜索框 ArrowDown 进入网格，Enter 选择首个结果；网格支持方向键、Home/End 和 Enter/Space。Escape 或点击外部关闭，Escape 后焦点回到图标按钮；在 Create team 内只关闭 emoji 弹层，保留团队草稿。

已有项目等待 PATCH 成功才更新图标和关闭；写入期间禁用选择及关闭，失败保持弹层、搜索词和原图标，显示错误，可以再次选择。Restore default 写入空值，刷新后仍保留服务端值。Settings 与 Create team 的 emoji 属于草稿，分别随 Save changes 与创建请求保存；失败保留草稿。最近成功选择保存在浏览器 localStorage；存储不可用时仍能选择和保存图标。

## 优先级和排序

priority 独立于 status/type/标签。列表、看板及详情的选择器逐次保存；写入期间禁用该票的优先级控件，服务端成功后才更新显示，失败保留此前值并显示错误。创建与编辑弹窗的优先级属于草稿，提交成功后持久化，失败保留草稿。菜单内容仅在打开时挂载；关闭触发器上的字母查找由选项数据处理。原生 dialog 内也使用同一 Radix 选择器，支持 Tab、Enter/Space、方向键和 Escape，用 Escape 关闭菜单后焦点回到触发器。

优先级筛选参与 API 查询，包含显式 No priority。切换筛选清除批量选择；修改后的票不再匹配当前优先级筛选时重读列表。默认排序仅在 Agent ready 分组按 Urgent→High→Medium→Low→No priority，其他状态按最近更新；可全局切换 Last updated 或 Priority，同级按最近更新。筛选和排序只保存在组件内存中，刷新后重置。MCP/REST 的 `status=agent_ready` 默认优先级排序，显式 `sort=updated` 覆盖；其他查询默认更新时间倒序。服务端在 limit 之前排序，同时间按内部 id 倒序。未实现 Linear 的同档手动顺序、全局 P 快捷键和 Urgent 通知。

## 自动明暗主题

页面默认跟随系统的 `prefers-color-scheme`，没有深色偏好时使用 light。首次加载、刷新和详情直达使用同一套 CSS 媒体查询；打开页面后系统偏好改变无需刷新。没有手动覆盖或 localStorage 主题状态。主题变化只改变样式，不卸载组件，不关闭 dialog 或清空编辑草稿。原生表单控件通过 `color-scheme` 跟随主题，浏览器 UI 的 `theme-color` 通过两条带 media 的 meta 跟随。token 分类与字面量边界见 [设计系统](DESIGN_SYSTEM.md)。

## 写入完成点

列表/看板移动状态先乐观更新，失败恢复该票此前值并展示错误；详情状态等待响应后更新。两处都不能把发出请求当作持久化完成。同一票的状态、优先级和标签写入按发起顺序执行；状态失败仅恢复最近确认的状态，保留其它字段。范围或查询已改变时不再应用旧查询的写入回调。批量标签保存与所选票的已有写入协调，保持最终查询结果一致。

任务新建与编辑使用原生 dialog 和紧凑 composer 布局，标题、Markdown 说明及可换行的属性控件保留字段标签与键盘入口；打开后聚焦标题。保存期间禁用可改字段、提交与关闭，成功关闭，失败保留内容并显示 role=alert。关闭/取消没有未保存草稿保护，不能承诺关闭后可恢复。评论成功后追加响应，仅在提交后没有继续编辑时清空输入；提交期间新写的草稿保留，失败保留文本，提交期间禁用按钮。

页面在窗口重新获得焦点时重读各自资源，未使用 WebSocket 推送；任务搜索和筛选只重读任务，不连带加载团队或项目。独立目录失败显示自己的错误和 Retry，不卸载详情、设置或已经打开的表单。任务写入期间的 focus/筛选读取延后，合并为最后一次，在排队写入结束后执行；写入前的旧读取不能覆盖保存结果。详情首次加载失败显示错误和 Retry，不停留在 Loading。离开页面后忽略已发出的旧响应，并取消尚未发出的排队写入；已发送的写入仍可能由服务端完成。该顺序保证只涵盖当前页面，不协调其它客户端的写入。

## 标签操作

标签管理页为 `/<slug>/labels`。名称、颜色、描述和归档状态在原生 dialog 中编辑；失败保留草稿，创建时提示相似名称。标签所属组创建后固定，组支持创建与改名。合并前展示关联转移及源 ID 删除的影响。

详情标签选择器支持搜索与原生 checkbox 键盘操作，逐次增量保存，保存期间禁用选择，等待服务端成功和详情重读后显示结果。批量添加/移除由原生 dialog 提交一次事务；组内添加自动替换，移除可以同时选择多个组内值。列表/看板展示前两个标签和 `+N`。标签筛选对话框使用显式 Apply/Clear；组选项匹配任意子标签，any/all/none 组合所选标签和组，unlabeled 不允许再选标签。标签筛选参与 API 查询，筛选与批量选择不跨刷新持久化。

## Agent 接票展示

详情属性中，已登记的 Codex/Claude 显示为 Assignee 下方的子行，带对应 agent 标志及执行记录数量，按钮滚动到 `Agent executions`。历史显示宿主、session ID、可选子 Agent ID/profile、可信记录操作者和开始时间；长 ID 换行且可选择复制，窄屏不横向撑开页面。不同参与者均保留，不把最新子 Agent 覆盖成人类负责人；没有记录时不显示子行。不提供网页接票按钮、实时 running 标记或排他锁。窗口重新获得焦点沿用详情刷新，读取失败不伪造记录。

## 修改时验证

涉及保存与刷新时，用浏览器检查成功、服务不可达、慢响应和连续操作后的页面与实际记录；重点查看失败是否丢失文本、旧响应是否覆盖新选择。不要把这些需要检查的情形写成已经自动通过的保证。

列表、看板及详情的状态可通过共享 Radix Select 菜单修改，菜单保留全部八种状态，拖动不是唯一入口。检查 Tab、Enter、方向键、Escape、对话框关闭后的焦点和浏览器前进后退；窄屏下检查属性仍可操作，列表吸顶分组在组内滚动和组交界处遮挡完整。视觉来源见 [设计系统](DESIGN_SYSTEM.md)，执行环境见 [开发](DEVELOPMENT.md)。


## 云端登录会话

启用 UC 时，未登录的页面访问跳到 UC，成功后回到原始相对路径；API 过期返回 401 而不是登录 HTML。侧栏底部显示账户入口，弹层内展示当前用户及 Sign out；退出失败保留页面并显示错误，成功进入已退出页。API 报会话过期时不卸载编辑器，在新标签页重新登录，返回时按既有 focus 刷新规则加载；不保证跨刷新保留草稿。

## 工作区成员与邀请

Settings → Members 显示成员目录、角色和停用状态；Admin 可调整角色、Suspend/Restore 并生成邀请，Member 只读目录。操作等待服务端成功后更新，失败显示 role=alert；不能移除最后一个活动 Admin。修改自己的成员权限成功后刷新页面以重新读取访问权。无 UC 的本地开发隐藏成员管理。

邀请链接在生成成功后显示，只读输入框可全选复制；再次生成使旧链接失效，页面不会在刷新后重显旧 token。`/auth/join?token=...` 先通过 UC 登录，再主动 Accept invitation；成功进入工作区，失败保留页面并显示原因。根目录没有可访问工作区时提供创建入口，并提示向管理员索取邀请，不显示内部 API 操作说明。

## 团队访问

创建团队可选 Make team private，创建者成为 owner。Settings 展示可发现的团队，公开团队可直接 Join/Leave；加入决定侧栏常驻，未加入仍可通过 View issues 浏览公开任务。普通成员只看到已加入的私有团队，Admin 在目录可看到全部私有团队但不能直接 View issues，须点击 Join team 并确认访问提示。

Owner/Admin 可改可见性、添加工作区活动成员、升降 owner 和移除成员。私有团队普通成员可 Leave，退出后团队消失，不能自行加入。Workspace Admin 的有效 owner 来自工作区角色，团队页不能将其降级。写入等待成功后刷新权限与目录，失败保留选择并显示错误；窄屏控件换行。详情直达读取失败保留自身错误与 Retry，不被其它目录加载清空；失去资源访问时移除旧详情内容。

## 人的负责人

新建任务在 Team 后选择 Assignee，默认 Unassigned；换 Team 清空原负责人并重新加载候选。详情属性可改派或清空，保存等待服务端成功，错误保留原值；候选读取失败提供 Retry people。当前负责人失去资格时保留人名并标记 unavailable，不作为新候选。Assignment history 显示可信操作者、前后人名与时间。

列表和看板显示负责人；Filter → Assignee 支持全部、自己、未分配、指定工作区成员（包含用于历史筛选的 suspended 成员）。筛选参与服务端查询，与其它条件相交；变更筛选清空批选。负责人不锁定任务，也不会替代 Agent 认领或会话字段。
