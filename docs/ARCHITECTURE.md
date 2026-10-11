# 架构与改动落点

## 运行与依赖方向

`cmd/inftask/main.go` 解析启动参数，打开 `app.Service`，启动备份与可选 GitHub 轮询，并组装 HTTP 服务；第一个参数是 `import-linear` 时改为运行 `cmd/inftask/import.go` 的一次性导入，它调用 `internal/app/linear.go` 的 `ImportLinear`，不启动服务。配置 UC 时由 `internal/ucauth` 包裹页面、REST 和 MCP，承接网页会话与 MCP OAuth；单独挂载验签的 webhook 入口。`internal/httpapi` 挂载 `/api`、每个 workspace 的 `/w/<slug>/mcp` 和静态页面；HTTP 与 `internal/mcpserver` 均调用 `internal/app`，业务层不依赖传输层。

workspace 只做分组：team、Project、标签和 PR 记录各属于一个 workspace，没有成员和权限。`/api/workspaces` 列出、新建和改名 workspace；其余 REST 路由在 `/api/w/<slug>/` 下，`httpapi` 先用 `Service.InWorkspace` 按路径里的 slug 取得只读写该 workspace 的 Service，slug 不存在时返回 404。MCP 每个请求同样按路径绑定 workspace，工具参数不含 workspace。`Service` 的查询都带 `workspace_id`，parent、关系、Project 和标签只能指向同一 workspace 的记录，别的 workspace 的记录按不存在处理；标签与所属组的 workspace 一致由复合外键保证。

`internal/app/service.go` 持有单连接 SQLite 访问、模型、输入校验和业务操作；建票在同一事务里把 team 的 `next_number` 加一并插入 issue。它直接执行 SQL，没有 repository 或 mock 分层。`schema.sql` 描述当前表结构，`migrate.go` 处理已有数据库；SQLite 是持久状态来源，浏览器状态只是页面副本。

`web/src/App.tsx` 按路径第一段选 workspace（`/` 跳到按 slug 排序的第一个，没有 workspace 时显示创建提示），管列表、看板、筛选、新建和路径切换；`IssueDetail.tsx` 管详情、评论和编辑。`shared.tsx` 共享类型、状态列、API helper（`apiPath`、`pagePath` 给路径加上当前 workspace）、导航、状态和优先级控件。`web/vite.config.ts` 将构建输出写到 `internal/web/dist/app`，`internal/web/web.go` 嵌入 `internal/web/dist`、从 `app` 提供页面，并为前端路由回退到 index 页面。`dist/.gitkeep` 受 Git 跟踪，没构建前端时 embed 也有匹配。

## 从用户动作到写入

状态选择或拖动调用 `PATCH /api/w/{workspace}/issues/{identifier}/status`，HTTP 适配为 `app.UpdateIssue`，交给 `Service.Update` 校验并更新数据库。MCP 的 `update_issue` 走同一个 Service；因此业务校验要放在 Service，不能只保护网页路径。传输层负责解析请求、调用和结果表达。

增加 issue 字段时，先核对 `CreateIssue`、`UpdateIssue`、查询列和扫描，再判断是否需要 schema/迁移、MCP schema、HTTP 测试和前端类型/编辑器一起变更。REST 并未暴露 MCP 的全部能力，不因工具存在就假设有对应 HTTP endpoint。

增加状态时，核对后端 `Statuses`、schema 的 CHECK、MCP 描述、前端 `columns` 和 `StatusIcon`，以及 GitHub 自动推进逻辑；迁移与旧数据保留要求见 [可靠性](rules/RELIABILITY.md)。

## 标签

`internal/app/labels.go` 管理 workspace 内的标签、单层互斥组、事务内增量批量关联、合并及筛选。`issue_labels` 连接 issue 和标签；SQLite trigger 阻止同组双选，Service 添加前删除旧组值。标签新增表由幂等 schema 在现有库启动时安装，不改写已有表。

`web/src/Labels.tsx` 提供 `/<slug>/labels` 管理页、详情/批量选择器和筛选对话框；标签类型与徽标在 `shared.tsx`。REST 标签路由在 `internal/httpapi/labels.go`，MCP 适配在 `internal/mcpserver/labels.go`。

## 外部同步

`internal/app/github.go` 轮询配置的 GitHub repository，每个仓库由 `-github-repo <slug>=<owner>/<name>` 绑定一个 workspace，每轮按该 workspace 当时的 team key 从 PR 分支名和标题识别 issue identifier，记录 PR（带 workspace）并自动推进状态；PR 描述不参与匹配。启用 linkback 时还会向 GitHub 评论写入 ticket 内容，见 [安全边界](rules/SECURITY.md)。同步游标和 PR 记录持久化在 SQLite，不能视作纯浏览器功能。`POST /webhooks/github` 只验签并通知同一个串行 worker 读取最新状态；重复通知合并，轮询仍补漏。

云端仍为单实例 SQLite；workspace 只分组，所有登录用户都能看到并修改全部 workspace，没有 tenant 隔离。UC 凭据通过配置注入，网页会话由 `internal/ucauth/sessions.go` 在数据库旁的加密 `.sessions` 文件中原子持久化，access 到期时按需向 UC 续期；登录流程和 MCP 短期授权码仍在进程内存中，重启后失效；部署入口见 [开发](DEVELOPMENT.md#云端团队部署)。

选择检查见 [验证](VALIDATION.md)，本地启动与构建顺序见 [开发](DEVELOPMENT.md)。
