# 架构与改动落点

## 运行与依赖方向

`cmd/agent-ledger/main.go` 解析启动参数，打开 `app.Service`，启动备份与可选 GitHub 轮询，并组装 HTTP 服务。`internal/httpapi` 挂载 `/api`、`/mcp` 和静态页面；HTTP 与 `internal/mcpserver` 均调用 `internal/app`，业务层不依赖传输层。

`internal/app/service.go` 持有单连接 SQLite 访问、模型、输入校验和业务操作。它直接执行 SQL，没有 repository 或 mock 分层。`schema.sql` 描述当前表结构，`migrate.go` 处理已有数据库；SQLite 是持久状态来源，浏览器状态只是页面副本。

`web/src/App.tsx` 管列表、看板、筛选、新建和路径切换；`IssueDetail.tsx` 管详情、评论和编辑。`shared.tsx` 共享类型、状态列、API helper、导航和状态控件。`web/vite.config.ts` 将构建输出写到 `internal/web/dist/app`，`internal/web/web.go` 嵌入 `internal/web/dist`、从 `app` 提供页面，并为前端路由回退到 index 页面。`dist/.gitkeep` 受 Git 跟踪，没构建前端时 embed 也有匹配。

## 从用户动作到写入

状态选择或拖动调用 `PATCH /api/issues/{identifier}/status`，HTTP 适配为 `app.UpdateIssue`，交给 `Service.Update` 校验并更新数据库。MCP 的 `update_issue` 走同一个 Service；因此业务校验要放在 Service，不能只保护网页路径。传输层负责解析请求、调用和结果表达。

增加 issue 字段时，先核对 `CreateIssue`、`UpdateIssue`、查询列和扫描，再判断是否需要 schema/迁移、MCP schema、HTTP 测试和前端类型/编辑器一起变更。REST 并未暴露 MCP 的全部能力，不因工具存在就假设有对应 HTTP endpoint。

增加状态时，核对后端 `Statuses`、schema 的 CHECK、MCP 描述、前端 `columns` 和 `.status-icon-*`，以及 GitHub 自动推进逻辑；迁移与旧数据保留要求见 [可靠性](rules/RELIABILITY.md)。

## 外部同步

`internal/app/github.go` 轮询配置的 GitHub repository，从 PR 分支名和标题识别 issue identifier，记录 PR 并自动推进状态；PR 描述不参与匹配。启用 linkback 时还会向 GitHub 评论写入 ticket 内容，见 [安全边界](rules/SECURITY.md)。同步游标和 PR 记录持久化在 SQLite，不能视作纯浏览器功能。

选择检查见 [验证](VALIDATION.md)，本地启动与构建顺序见 [开发](DEVELOPMENT.md)。
