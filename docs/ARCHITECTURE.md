# 架构与改动落点

## 运行与依赖方向

`cmd/inftask/main.go` 解析启动参数，打开 `app.Service`，启动备份与可选 GitHub 轮询，并组装 HTTP 服务；第一个参数是 `import-linear` 时改为运行 `cmd/inftask/import.go` 的一次性导入，它调用 `internal/app/linear.go` 的 `ImportLinear`，不启动服务。配置 UC 时由 `internal/ucauth` 包裹页面、REST 和 MCP，承接网页会话与 MCP OAuth；单独挂载验签的 webhook 入口。`internal/httpapi` 挂载 `/api`、每个 workspace 的 `/w/<slug>/mcp` 和静态页面；HTTP 与 `internal/mcpserver` 均调用 `internal/app`，业务层不依赖传输层。

workspace 是云端成员访问边界：team、Project、标签和 PR 记录各属于一个 workspace。UC 将验证过的 sub/name/email 写入请求上下文；`app/access.go` 维护用户、Admin/Member 成员及 active/suspended 状态。`/api/workspaces` 只列当前用户的活动成员工作区；新建者成为 Admin，改名要求 Admin。其余 REST 路由在 `/api/w/<slug>/` 下，`Service.InWorkspace` 绑定范围并检查成员，slug 不存在返回 404，无访问权返回 403。MCP 每个请求按同一规则绑定，业务方法也重新检查，不能靠缓存的 Service 绕过停用。公开团队任务向所有活动成员开放；私有团队任务仅向显式团队成员开放。

`main.go` 向 HTTP/MCP 传 `Service.ForRequests(auth != nil)`；云端缺身份拒绝访问，原始 Service 留给后台同步与本地 CLI。无 UC 的 loopback 开发保留业务操作，但不开放云端成员管理。`users.subject` 来自 UC，不接受请求正文指定操作者。邀请以随机 token 的摘要存储、可重置复用，只加入 Member；成员变更的最后一个活动 Admin 检查与更新在同一事务。已有工作区通过 `bootstrap-access` 显式初始化，见 [开发](DEVELOPMENT.md#初始化已有工作区的管理员)。业务查询继续带 `workspace_id`，跨 workspace 的引用按不存在处理。

`internal/app/service.go` 只持有 `Service` 及其单连接 SQLite 的打开和关闭；模型、输入校验和业务操作按领域放在 `workspace.go`、`issue.go`、`project.go`、`team.go`，导出的错误集中在 `errors.go` 并各带类别（找不到、输入非法、冲突），HTTP 按类别选状态码。建票在同一事务里把 team 的 `next_number` 加一并插入 issue。Service 直接执行 SQL，没有 repository 或 mock 分层。`schema.sql` 描述当前表结构，`migrate.go` 处理已有数据库；SQLite 是持久状态来源，浏览器状态只是页面副本。

`web/src/App.tsx` 按路径第一段选 workspace；`/` 进入第一个 workspace，没有时显示创建提示。`WorkspaceView` 保留 Sidebar、WorkspaceMenu、Session、团队目录与会话视图/排序偏好；仅页面目的地按递增 visit 重新挂载，隔离筛选、选择和弹窗。同 workspace 导航不会重新请求团队、workspace 或会话。`shared.tsx` 定义类型、API helper、History 路由和共享控件，`WorkspaceRoute` 区分全部/Team 任务与项目、稳定 Project id、详情、标签与设置。

页面按路由懒加载：`IssuesPage.tsx` 管任务范围的筛选、批选、新建与项目概览，`ProjectsPage.tsx` 管活动/归档目录与创建，`IssuePage.tsx` 组装详情、编辑与结果反馈，`LabelsPage.tsx` 管标签，`SettingsPage.tsx` 管 Team 草稿。详情的 Markdown 依赖不进入初始入口；加载失败由页面错误边界显示重新加载入口，Sidebar 仍可导航。页面只加载自己的资源：设置不依赖项目/标签目录；搜索与筛选只重读任务；项目 emoji 成功后只更新项目详情。

`useResource.ts` 提供单资源的加载、错误、focus 刷新和版本门禁；团队由 workspace 外壳持有，项目目录和 by-id 详情由所属页面分别持有，`useLabels.ts` 拥有标签目录与筛选类型。`useIssues.ts` 只负责任务查询和写入，不加载团队或项目。`useRequestScope.ts` 统一挂载代际、最新读取与写入队列：同票操作按发起顺序执行，批量标签操作等待所选票的已有写入；写入使旧读取失效，期间触发的读取合并为最后一次，待写入结束再执行。离开页面后忽略已发出请求的结果，尚未发出的排队写入取消。查询门禁另外隔离切换筛选后的旧回调。

`IssueViews.tsx` 的行内容按任务对象和单行选中/保存状态缓存，稳定事件入口转交最新已提交回调；超过 50 行的分组使用 TanStack Virtual 测量动态行高，计数和批选基于完整数据。`IssueFilters.tsx` 持有搜索草稿；`Dialogs.tsx` 提供新建 issue/team 与共用 Project 控件；`ProjectManagement.tsx` 提供项目表单及归档操作；`IssueDetail.tsx` 管详情展示、评论和编辑器。`web/vite.config.ts` 输出到 `internal/web/dist/app`，Go 嵌入 `internal/web/dist` 并提供 SPA 回退；构建输出不手改。

## 从用户动作到写入

状态选择或拖动调用 `PATCH /api/w/{workspace}/issues/{identifier}/status`，HTTP 适配为 `app.UpdateIssue`，交给 `Service.Update` 校验并更新数据库。MCP 的 `update_issue` 走同一个 Service；因此业务校验要放在 Service，不能只保护网页路径。传输层负责解析请求、调用和结果表达。

任务创建/编辑成功由所属页面显示结果与查看入口；创建后 `useIssues.ts` 重读筛选结果，不强插新票。详情的快捷状态、优先级、负责人、评论、标签和编辑器保存共享同票队列；编辑成功应用响应后重读详情。评论仅在提交后未再编辑时清空输入，后续草稿保留。列表乐观状态失败恢复最近确认的状态，不回滚整票，也不覆盖已保存的其它字段。编辑器项目目录独立读取同 Team 的含归档目录，失败保留草稿并提供重试。这些队列只协调当前页面发起的操作，不提供跨浏览器/跨客户端事务或冲突检测。

Agent 接票由独立 MCP `delegate_issue` 调用 `Service.Delegate`，不通过 `Update` 的状态变化推断。`delegation.go` 校验客户端来源、沿用 issue 的 workspace/team 权限，在同一事务内检查人类 assignee 非空并插入 `issue_delegations`；详情 `delegations` 按 id 返回所有参与者。每次调用都要求负责人，包括重复调用；清空负责人保留历史，重新分配后可继续登记。唯一键为 issue/agent/session_id/agent_id/hostname，重复调用保留原 actor、类型和时间，不改变 issue 的 updated_at、status 或 assignee。`IssueDetail.tsx` 展示参与 Agent 和可选择复制的来源历史，不表示实时运行状态。仓库本地 hooks 注入工具参数，分发插件不含这些 hooks；配置见 [开发](DEVELOPMENT.md#仓库内-agent-接票)。

增加 issue 字段时，先核对 `CreateIssue`、`UpdateIssue`、查询列和扫描，再判断是否需要 schema/迁移、MCP schema、HTTP 测试和前端类型/编辑器一起变更。REST 并未暴露 MCP 的全部能力，不因工具存在就假设有对应 HTTP endpoint。

增加状态时，核对后端 `Statuses`、schema 的 CHECK、前端 `columns` 和 `StatusIcon`，以及 GitHub 自动推进逻辑；迁移与旧数据保留要求见 [可靠性](rules/RELIABILITY.md)。

## MCP 契约演进

`internal/mcpserver` 的所有工具（含标签工具）通过 `addTool` 集中注册，保留 `outputSchema` 和 `structuredContent`。输出字段只做加法：各层输出 object 允许额外字段，包含 `properties`、`items`、`$defs` 中的对象，让缓存旧 schema 的客户端接受新增字段。删字段、改名、改类型、改语义都是破坏性变更，必须先保留旧字段过渡；既有 `required` 列表保持不变。输入 schema 保持严格，拒绝未知参数。

输出 schema 在交给 go-sdk `mcp.AddTool` 前放宽，SDK 仍按该 schema 校验 handler 返回的 JSON，保留必填与类型检查；不能只修改 `tools/list` 公布的副本。首次部署此修复后，已连接客户端仍缓存旧的严格 schema，需要重连一次；取得新 schema 后，后续新增输出字段不会再因额外字段被拒绝。

## Project 与 Team

`project_teams(project_id, team_id)` 保存显式多对多关联；外键保护引用，`project_teams_workspace` 插入触发器拒绝跨 workspace 关联。Project 输出既有 `projects.id` 与排序后的 Team key 数组。`project.go` 在事务中替换关联，移除仍有任务（含已关闭任务）的 Team 会整体回滚。`issue.go` 在建票或更新事务内校验 Team 兼容与归档状态，失败建票不消耗编号；省略 Project 不改归属，显式提交当前归档 Project 也允许编辑其它字段。

REST 提供项目 POST、GET、PATCH，当前名称入口为 `/projects/{name}`，稳定 id 入口为 `/projects/by-id/{id}`；MCP `get_project`/`update_project` 接受 `id`/`name`，双传必须一致。任务 create/update/list 同时支持 `project_id` 与当前名称 `project`。迁移根据现有任务回填关联，历史空项目保持待归属；`linear.go` 在原导入事务内维护任务形成的关联，历史归档项目仍可导入。所有查询在复用单连接前关闭 rows，不在持有事务时调用使用 `s.db` 的查询。

## 标签

`internal/app/labels.go` 管理 workspace 内的标签、单层互斥组、事务内增量批量关联、合并及筛选。`issue_labels` 连接 issue 和标签；SQLite trigger 阻止同组双选，Service 添加前删除旧组值。标签新增表由幂等 schema 在现有库启动时安装，不改写已有表。

`web/src/LabelsPage.tsx` 与 `LabelManager.tsx` 提供 `/<slug>/labels` 管理页，`Labels.tsx` 提供详情/批量选择器和筛选对话框；标签类型与徽标在 `shared.tsx`。REST 标签路由在 `internal/httpapi/labels.go`，MCP 适配在 `internal/mcpserver/labels.go`。

## 外部同步

`internal/app/github.go` 轮询配置的 GitHub repository，每个仓库由 `-github-repo <slug>=<owner>/<name>` 绑定一个 workspace，每轮按该 workspace 当时的 team key 从 PR 分支名和标题识别 issue identifier，记录 PR（带 workspace）并自动推进状态；PR 描述不参与匹配。启用 linkback 时还会向 GitHub 评论写入 ticket 内容，见 [安全边界](rules/SECURITY.md)。同步游标和 PR 记录持久化在 SQLite，不能视作纯浏览器功能。`POST /webhooks/github` 只验签并通知同一个串行 worker 读取最新状态；重复通知合并，轮询仍补漏。

云端仍为单实例 SQLite，工作区成员权限在应用层实施，不提供独立数据库隔离。UC 凭据通过配置注入，网页会话由 `internal/ucauth/sessions.go` 在数据库旁的加密 `.sessions` 文件中原子持久化，access 到期时按需向 UC 续期；登录流程和 MCP 短期授权码仍在进程内存中，重启后失效；部署入口见 [开发](DEVELOPMENT.md#云端团队部署)。

选择检查见 [验证](VALIDATION.md)，本地启动与构建顺序见 [开发](DEVELOPMENT.md)。

## 团队成员与私有内容

`team_access.go` 统一团队内容可见性与管理权。`teams.private` 默认 false，`team_members` 保存 member/owner；创建者加入为 owner，历史团队不自动加入用户。成员必须属于团队的 workspace，由 Service 和插入触发器共同约束。

`ListTeams` 只返回可读内容的团队；`TeamDirectory` 为 Admin 额外返回私有团队设置元数据，未加入时不返回真实 next_number。管理员身份不进入内容可见性谓词。`viewerSQL` 对源码中已知的位置参数编号并绑定可信 subject，`visibleTeamIDs` 用于列表、直达、父子/关系、创建引用与批量标签；过滤发生在排序和 limit 之前。不可读任务/团队/Project 按不存在返回。

Project 无关联的历史记录保留可见；有至少一个可见关联团队时可见，返回的 teams 仅含可见关联。更新 teams 只替换调用者可见的部分，保留隐藏关联；不能传入不可访问团队以改变关联。私有任务的 parent 和详情关系也经同一过滤。合并标签若涉及不可见任务则拒绝整个事务，避免通过全局操作修改隐藏关联。

Settings 使用独立 team-directory 提供加入/退出、成员增删、owner 和可见性管理；侧栏显示已加入团队及当前正在浏览的公开团队。本地无 UC 模式仍可访问所有业务数据，不提供云端成员操作。

## 人的任务负责人

`issue.go` 的 create/update/list/get 与 `assignee.go` 共用负责人契约：输入 `assignee` 是 UC subject，空字符串清空，update 省略保持；输出是 `{subject,name,email}` 或 null。候选经 `GET /api/w/{workspace}/teams/{key}/assignees` 或 MCP `list_assignees(team)` 读取；只有可访问团队的调用者能读取其候选。公开团队允许工作区全部活动成员，私有团队仅允许其活动成员。筛选 `assignee` 指定 subject，空值匹配未分配；`assigned_to_me=true` 从可信上下文取人，不能与 `assignee` 同用。

`issues.assignee_subject` 保存单一用户引用。事务内验证候选资格，并将实际变更写入 `issue_assignments`，包含 issue、可信 actor、from/to 用户引用及时间；相同值不重复记录历史。详情 `assignments` 按审计 id 排序，返回各人的当前显示资料及稳定 subject。本地内部操作的 actor 为 null，页面显示 System；请求正文不能提供 actor。停用或团队权限变化不抹除现有归属和历史，但后续显式分配必须重新通过候选资格校验。负责人不改变任务编辑权，不承载 Agent 会话。

`Assignee.tsx` 复用 `useResource` 加载候选和筛选目录，创建时换 Team 清空负责人；详情改派走同票写入队列，成功后重读历史，失败保留原值。`shared.tsx` 的 IssueMeta 在列表/看板显示人名；IssuesPage 将自己、未分配和成员选择转换为同一列表查询。
