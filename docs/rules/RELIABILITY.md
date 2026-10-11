# 数据可靠性与恢复

## Schema 的两条路径

真实实例有数据，任何变更都不能要求删除数据库。`internal/app/schema.sql` 始终描述当前形状；新数据库直接使用它，`user_version` 设为迁移数量，不执行历史迁移。增加表使用 `CREATE TABLE IF NOT EXISTS`。

修改已有表时同时更新 schema，并向 `internal/app/migrate.go` 的 `migrations` 追加升级 SQL；已交付迁移不得编辑。当前有 4 项：

- 迁移 1 新建 `teams`，重建 `issues` 并按 identifier 回填 `team` 和 `number`，每个前缀成为一个 team，`next_number` 是该前缀最大编号加一。迁移 1 之后，旧版二进制仍能读库，但建票会因缺 `team` 和 `number` 失败；回滚到旧版要恢复 `before-migration-1.db`，此后的写入会丢失。
- 迁移 2 把表之间的引用从 identifier、Project 名称和 team key 改为整数 id：`teams`、`projects` 改用 `id` 主键（`key`、`name` 仍全实例唯一），`issues` 改存 `team_id`、`project_id`、`parent_id`，`relations` 改存 `issue_id`、`other_id`，`links`、`comments`、`issue_labels` 改存 `issue_id`。issue、链接和评论保留原 id；related 关系按 id 重排为 `issue_id < other_id`。`pull_requests.issue_identifier` 不变。迁移 2 之后，旧版二进制仍能启动，但读写 issue 会因旧列名不存在而失败；回滚到旧版要停服务并恢复 `before-migration-2.db`，此后的写入会丢失。
- 迁移 3 新建 `workspaces(id, slug, name)` 和 workspace `default`（名称 `Default`），重建 `teams`、`projects`、`issues`、`label_groups`、`labels`、`pull_requests`、`github_sync`，各加 `workspace_id` 并全部填为 `default`，所有行保留原 id。唯一约束改为 workspace 内唯一：team key、Project 名称、标签组名、标签名和 issue identifier；`labels` 通过 `(workspace_id, group_id)` 复合外键引用同一 workspace 的组；`pull_requests` 主键和 `github_sync` 游标都带 `workspace_id`。重建 `labels` 前先删掉引用它的 `issue_labels_exclusive` 触发器，结束时重建。迁移 3 之后，旧版二进制仍能启动、读取和修改已有记录，但新建 issue、team、Project 和标签会因 `workspace_id` 缺失或冲突子句不匹配而失败；回滚到旧版要停服务并恢复 `before-migration-3.db`，此后的写入会丢失。

- 迁移 4 重建 `issues`，在 status CHECK 中加入 `backlog`；保留全部字段、内部 id、workspace 和自增序列上限，重建 team/number、status/updated 和 parent 索引。引用 `issues(id)` 的表不重建，迁移事务内检查外键。已有版本 3 数据库升级前生成 `before-migration-4.db`；新库直接使用含 backlog 的 schema。旧版服务不接受 backlog 输入，页面也没有该状态，回滚时要停服务并恢复升级前备份，此后的写入会丢失。

登记的迁移只覆盖这些历史形状，不代表能把任意旧结构自动转换。

更改 CHECK（例如增加状态）要新建目标表、复制数据、删除旧表、重命名。迁移期间关闭外键，每项迁移在事务内执行并做 `foreign_key_check`，违规回滚该项；此前已经提交的迁移不会随后一项失败一起回滚。新旧路径都要测试，参考 `migrate_test.go`。

既有贡献约定要求迁移 n 前保留 `backups/before-migration-<n>.db`。当前实现是在待执行批次开始前，用下一项版本号生成一次备份，并非批次内每项都备份；修改迁移执行器时需明确这个差异，不能把约定当作已实现保证。

## Schema 票的排队与合并

所有修改 `internal/app/schema.sql` 或 `internal/app/migrate.go` 的票遵守以下规则：

1. 票、分支和计划不预写迁移编号，只写“追加一个迁移”。编号由合并时该项在 `internal/app/migrate.go` 的 `migrations` 中的位置决定（从 1 开始），不能提前占号。
2. 同一时间只执行一张改 schema 的票。设为 `agent_ready` 前，分别用 `list_issues` 查询 `agent_ready`、`in_progress`、`in_review` 的票（`limit: -1`，避免默认上限漏票），再用 `get_issue` 阅读候选票的范围，确认是否也修改上述任一文件；有冲突就用 `blocks` 明确先后顺序，前票合并后再执行后票。门禁流程见 [ticket skill](../../.agents/skills/ticket/SKILL.md#agent_ready-gate)。
3. 合并前 rebase 到最新 `main`。若 `main` 新增了迁移，将自己的未交付迁移挪到 `migrations` 末尾，并按合并后的当前 `schema.sql` 重写其中的建表和重建语句；复制数据的语句也必须保留别人刚加入的列及其数据。已交付迁移仍不得改写。合并流程见 [git-workflow skill](../../.agents/skills/git-workflow/SKILL.md#commits-and-pull-requests)。

完成 rebase 和迁移调整后，重新验证新库与已有库的升级路径，检查要求见 [验证指南](../VALIDATION.md)。

## 写入与同步

创建 issue、更新状态及对应事件需要一起成功，沿用 Service 的事务；单语句不另包事务。数据库使用 WAL、外键和单个打开连接，不能以单连接推断外部 GitHub 操作也具备事务性。

GitHub 同步每轮顺序处理仓库，某仓库失败（包括绑定的 workspace 还不存在）会记录日志，下轮再读；游标按 workspace 和仓库分别记录，仅在完整读取成功后推进，仓库改绑到另一个 workspace 后第一轮从头读取。PR 关联可重复 upsert，自动状态只按既有条件向前推进，不应增加额外重试。

linkback 先发送 GitHub 评论，再记录本地时间戳。远端成功但响应丢失或本地记录失败时，下轮可能再次评论；当前不保证恰好一次。改变此流程要明确部分成功状态，不能以本地事务声称外部副作用可回滚。

## 备份与恢复

启动后检查每日备份，此后每小时检查；使用 `VACUUM INTO` 生成 UTC 日期命名的 `daily-YYYY-MM-DD.db`，保留最新 14 份，每日已有副本则跳过。迁移前副本不参加日常清理。备份失败记录日志，不代表业务写入自动停止。

恢复时先停止使用该数据库的服务，保留需调查的数据，再按 README 清理对应 WAL/SHM 并恢复选定备份。运行中不能用普通 `cp` 复制主数据库冒充一致备份。开发验证只能操作自身临时目录；维护者数据恢复不属于常规测试。

验证入口见 [验证指南](../VALIDATION.md)。
