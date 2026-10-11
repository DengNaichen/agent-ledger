# 架构与改动落点

## 运行与依赖方向

`cmd/inftask/main.go` 解析启动参数，打开 `app.Service`，启动备份与可选 GitHub 轮询，并组装 HTTP 服务；第一个参数是 `import-linear` 时改为运行 `cmd/inftask/import.go` 的一次性导入，它调用 `internal/app/linear.go` 的 `ImportLinear`，不启动服务。配置 UC 时由 `internal/ucauth` 包裹页面、REST 和 MCP，承接网页会话与 MCP OAuth；单独挂载验签的 webhook 入口。`internal/httpapi` 挂载 `/api`、每个 workspace 的 `/w/<slug>/mcp` 和静态页面；HTTP 与 `internal/mcpserver` 均调用 `internal/app`，业务层不依赖传输层。

workspace 只做分组：team、Project、标签和 PR 记录各属于一个 workspace，没有成员和权限。`/api/workspaces` 列出、新建和改名 workspace；其余 REST 路由在 `/api/w/<slug>/` 下，`httpapi` 先用 `Service.InWorkspace` 按路径里的 slug 取得只读写该 workspace 的 Service，slug 不存在时返回 404。MCP 每个请求同样按路径绑定 workspace，工具参数不含 workspace。`Service` 的查询都带 `workspace_id`，parent、关系、Project 和标签只能指向同一 workspace 的记录，别的 workspace 的记录按不存在处理；标签与所属组的 workspace 一致由复合外键保证。

`internal/app/service.go` 只持有 `Service` 及其单连接 SQLite 的打开和关闭；模型、输入校验和业务操作按领域放在 `workspace.go`、`issue.go`、`project.go`、`team.go`，导出的错误集中在 `errors.go` 并各带类别（找不到、输入非法、冲突），HTTP 按类别选状态码。建票在同一事务里把 team 的 `next_number` 加一并插入 issue。Service 直接执行 SQL，没有 repository 或 mock 分层。`schema.sql` 描述当前表结构，`migrate.go` 处理已有数据库；SQLite 是持久状态来源，浏览器状态只是页面副本。

`web/src/App.tsx` 按路径第一段选 workspace（`/` 跳到按 slug 排序的第一个，没有 workspace 时显示创建提示），其中 `WorkspaceView` 持有路径与会话视图/排序偏好，每次导航按递增 visit 挂载 `ScopeView`，隔离当前范围的筛选、选择和弹窗。`shared.tsx` 的 `WorkspaceRoute` 区分全部任务/项目、Team 任务/项目、稳定 Project id、详情和标签路径；`ProjectList.tsx` 展示活动/归档项目列表，Team 项目按显式关联筛选；`ProjectManagement.tsx` 提供创建/编辑modal与稳定项目页归档/恢复操作。页面组装中：`useIssues.ts` 加载默认未归档项目目录与 Team，项目范围另以 by-id 读取（可含归档），任务按 team/project_id 和临时筛选查询；挂载、查询及读取版本门禁防止旧范围响应或写入回调更新新页面，创建后重读筛选结果，`Sidebar.tsx` 是侧边栏，`IssueFilters.tsx` 是筛选栏，`IssueViews.tsx` 是看板与列表，`Dialogs.tsx` 提供新建 issue/team 弹窗及共用 `IssueProjectSelect`，新建依据明确范围预选，提交 stable project_id；`IssueDetail.tsx` 管详情、评论和编辑，详情及编辑显示只读 Team，Project 编辑器按 Team 单独读含归档目录并只允许保留当前归档项目。`shared.tsx` 共享类型、状态列、API helper（`apiPath`、`pagePath` 给路径加上当前 workspace）、导航、状态和优先级控件。`web/vite.config.ts` 将构建输出写到 `internal/web/dist/app`，`internal/web/web.go` 嵌入 `internal/web/dist`、从 `app` 提供页面，并为前端路由回退到 index 页面。`dist/.gitkeep` 受 Git 跟踪，没构建前端时 embed 也有匹配。

## 从用户动作到写入

状态选择或拖动调用 `PATCH /api/w/{workspace}/issues/{identifier}/status`，HTTP 适配为 `app.UpdateIssue`，交给 `Service.Update` 校验并更新数据库。MCP 的 `update_issue` 走同一个 Service；因此业务校验要放在 Service，不能只保护网页路径。传输层负责解析请求、调用和结果表达。

任务创建/编辑成功由 `App.tsx` 显示结果与查看入口，`useIssues.ts` 重读当前范围与筛选，不强插新票。`IssueDetail.tsx` 的 `loadVersion` 对详情读取编号，编辑成功先失效旧读取并应用返回字段，再重读详情；编辑器项目目录用独立版本与挂载门禁，目录失败保留草稿并提供重试。新建/编辑保存期间禁止关闭，卸载后的保存回调不再影响新范围；这些局部门禁不代表全部交错写入已经串行化。

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

`web/src/Labels.tsx` 提供 `/<slug>/labels` 管理页、详情/批量选择器和筛选对话框；标签类型与徽标在 `shared.tsx`。REST 标签路由在 `internal/httpapi/labels.go`，MCP 适配在 `internal/mcpserver/labels.go`。

## 外部同步

`internal/app/github.go` 轮询配置的 GitHub repository，每个仓库由 `-github-repo <slug>=<owner>/<name>` 绑定一个 workspace，每轮按该 workspace 当时的 team key 从 PR 分支名和标题识别 issue identifier，记录 PR（带 workspace）并自动推进状态；PR 描述不参与匹配。启用 linkback 时还会向 GitHub 评论写入 ticket 内容，见 [安全边界](rules/SECURITY.md)。同步游标和 PR 记录持久化在 SQLite，不能视作纯浏览器功能。`POST /webhooks/github` 只验签并通知同一个串行 worker 读取最新状态；重复通知合并，轮询仍补漏。

云端仍为单实例 SQLite；workspace 只分组，所有登录用户都能看到并修改全部 workspace，没有 tenant 隔离。UC 凭据通过配置注入，网页会话由 `internal/ucauth/sessions.go` 在数据库旁的加密 `.sessions` 文件中原子持久化，access 到期时按需向 UC 续期；登录流程和 MCP 短期授权码仍在进程内存中，重启后失效；部署入口见 [开发](DEVELOPMENT.md#云端团队部署)。

选择检查见 [验证](VALIDATION.md)，本地启动与构建顺序见 [开发](DEVELOPMENT.md)。
