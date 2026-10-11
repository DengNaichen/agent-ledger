# 数据可靠性与恢复

## Schema 的两条路径

真实实例有数据，任何变更都不能要求删除数据库。`internal/app/schema.sql` 始终描述当前形状；新数据库直接使用它，`user_version` 设为迁移数量，不执行历史迁移。增加表使用 `CREATE TABLE IF NOT EXISTS`。

修改已有表时同时更新 schema，并向 `internal/app/migrate.go` 的 `migrations` 追加升级 SQL；已交付迁移不得编辑。当前有 3 项：

- 迁移 1 新建 `teams`，重建 `issues` 并按 identifier 回填 `team` 和 `number`，每个前缀成为一个 team，`next_number` 是该前缀最大编号加一。迁移 1 之后，旧版二进制仍能读库，但建票会因缺 `team` 和 `number` 失败；回滚到旧版要恢复 `before-migration-1.db`，此后的写入会丢失。
- 迁移 2 把表之间的引用从 identifier、Project 名称和 team key 改为整数 id：`teams`、`projects` 改用 `id` 主键（`key`、`name` 仍全实例唯一），`issues` 改存 `team_id`、`project_id`、`parent_id`，`relations` 改存 `issue_id`、`other_id`，`links`、`comments`、`issue_labels` 改存 `issue_id`。issue、链接和评论保留原 id；related 关系按 id 重排为 `issue_id < other_id`。`pull_requests.issue_identifier` 不变。迁移 2 之后，旧版二进制仍能启动，但读写 issue 会因旧列名不存在而失败；回滚到旧版要停服务并恢复 `before-migration-2.db`，此后的写入会丢失。

- 迁移 3 用 `ALTER TABLE issues ADD COLUMN` 增加 `priority INTEGER NOT NULL DEFAULT 0 CHECK (priority IN (0, 1, 2, 3, 4))`，已有票回填为 No priority，原引用、正文及时间保留。迁移前备份为 `before-migration-3.db`（从版本 2 升级时）；旧版程序可以忽略此新增列，但不会读取或设置优先级。

登记的迁移只覆盖这些历史形状，不代表能把任意旧结构自动转换。

更改 CHECK（例如增加状态）要新建目标表、复制数据、删除旧表、重命名。迁移期间关闭外键，每项迁移在事务内执行并做 `foreign_key_check`，违规回滚该项；此前已经提交的迁移不会随后一项失败一起回滚。新旧路径都要测试，参考 `migrate_test.go`。

既有贡献约定要求迁移 n 前保留 `backups/before-migration-<n>.db`。当前实现是在待执行批次开始前，用下一项版本号生成一次备份，并非批次内每项都备份；修改迁移执行器时需明确这个差异，不能把约定当作已实现保证。

## 写入与同步

创建 issue、更新状态及对应事件需要一起成功，沿用 Service 的事务；单语句不另包事务。数据库使用 WAL、外键和单个打开连接，不能以单连接推断外部 GitHub 操作也具备事务性。

GitHub 同步每轮顺序处理仓库，某仓库失败会记录日志，下轮再读；游标仅在完整读取成功后推进。PR 关联可重复 upsert，自动状态只按既有条件向前推进，不应增加额外重试。

linkback 先发送 GitHub 评论，再记录本地时间戳。远端成功但响应丢失或本地记录失败时，下轮可能再次评论；当前不保证恰好一次。改变此流程要明确部分成功状态，不能以本地事务声称外部副作用可回滚。

## 备份与恢复

启动后检查每日备份，此后每小时检查；使用 `VACUUM INTO` 生成 UTC 日期命名的 `daily-YYYY-MM-DD.db`，保留最新 14 份，每日已有副本则跳过。迁移前副本不参加日常清理。备份失败记录日志，不代表业务写入自动停止。

恢复时先停止使用该数据库的服务，保留需调查的数据，再按 README 清理对应 WAL/SHM 并恢复选定备份。运行中不能用普通 `cp` 复制主数据库冒充一致备份。开发验证只能操作自身临时目录；维护者数据恢复不属于常规测试。

验证入口见 [验证指南](../VALIDATION.md)。
