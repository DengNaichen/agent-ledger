# 项目术语

| 术语 | 在本项目中的含义与边界 |
| --- | --- |
| Issue / ticket | 同一工作记录，标识符由服务端生成，默认 `AL-<id>`；前缀由 `-prefix` 配置。不能推测下一个编号。 |
| Project | 按交付结果组织 ticket 的具名记录，不是 Git 仓库或源码目录。名称唯一且不可改名；归档后默认列表不展示，仍可显式查询。 |
| triage / todo | 前者是尚未承诺的想法，后者是已承诺工作；todo 本身不表示可直接由 Agent 执行。 |
| agent_ready | 通过仓库 ticket skill 的执行门槛后进入 Agent 队列。门槛是贡献流程，不能宣称 Service 自动审核 ticket 描述。 |
| in_progress / in_review / done / canceled | 正在处理、PR 可审阅、合并后完成、取消或被替代。GitHub 同步仅按源码中的特定规则自动推进。 |
| type | 可选分类：`spec`、`feature`、`bug`、`improvement`、`chore`；与 status 相互独立。 |
| parent / children | 工作归属与子任务。不能自引用；当前服务不检查较长的 parent 环。 |
| blocks / blocked_by | 有方向的阻塞关系；`blocked_by` 以反向 `blocks` 存储。 |
| related | 无方向关联，数据库规范化两端顺序以避免重复。与 parent 的归属含义不同。 |
| link / pull_requests | 前者是人工附加的标题和 URL；后者是 GitHub 同步记录的 PR，有仓库、编号与状态。 |
| linkback | 默认关闭的 GitHub 回写：把 issue 标题、描述作为评论发到关联的 open/draft PR。不是本地 link。 |
| events / comments | events 记录创建与状态变化；comments 是用户或 Agent 追加的正文，不能编辑或删除。 |

定义以 `internal/app/service.go`、`schema.sql`、`github.go`、MCP 工具描述及 [ticket skill](../.agents/skills/ticket/SKILL.md) 为依据。
