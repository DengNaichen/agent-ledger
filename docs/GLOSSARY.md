# 项目术语

| 术语 | 在本项目中的含义与边界 |
| --- | --- |
| workspace | 一组 team、Project 和标签，由不能修改的 slug 命名：页面在 `/<slug>/`，REST 在 `/api/w/<slug>/`，MCP 在 `/w/<slug>/mcp`。只做分组和切换，没有成员、角色和权限，不能删除。不是 Git 工作树或 HarnessKit 的 workspace membership。 |
| Issue / ticket | 同一工作记录，标识符由服务端生成，形如 `<team key>-<number>`，编号由所属 team 依次分配，只在所属 workspace 内唯一。不能推测下一个编号。 |
| team | 有自己票号前缀（key）和独立编号的 issue 归属，如本仓库的 `AL`。建票必须指定 team，没有默认值；key 在 workspace 内唯一，建好后不能改，team 不能改名、删除或换 workspace，票不能换 team。状态所有 workspace 共用，Project 由同一 workspace 的 team 共用。不是 GitHub team 或 UC 用户组。 |
| Project | 按交付结果组织 ticket 的具名记录，不是 Git 仓库或源码目录。名称在 workspace 内唯一且不可改名；归档后默认列表不展示，仍可显式查询。 |
| triage / backlog / todo | triage 是新进来、还没决定要不要的票；backlog 是决定保留、但还没承诺什么时候做的票；todo 是已承诺要做的工作，本身不表示可直接由 Agent 执行。 |
| agent_ready | 通过仓库 ticket skill 的执行门槛后进入 Agent 队列。门槛是贡献流程，不能宣称 Service 自动审核 ticket 描述。 |
| in_progress / in_review / done / canceled | 正在处理、PR 可审阅、合并后完成、取消或被替代。GitHub 同步仅按源码中的特定规则自动推进。 |
| type | 可选分类：`spec`、`feature`、`bug`、`improvement`、`chore`；与 status 相互独立。 |
| label / label group | workspace 内的 issue 标签；普通标签多选，单层标签组内互斥。标签 ID 稳定、名称可改、创建后所属组固定；归档保留关联和筛选，合并转移关联并删除源 ID。与 type、Project 和 status 独立。 |
| parent / children | 工作归属与子任务。不能自引用；当前服务不检查较长的 parent 环。 |
| blocks / blocked_by | 有方向的阻塞关系；`blocked_by` 以反向 `blocks` 存储。 |
| related | 无方向关联，数据库规范化两端顺序以避免重复。与 parent 的归属含义不同。 |
| link / pull_requests | 前者是人工附加的标题和 URL；后者是 GitHub 同步记录的 PR，有仓库、编号与状态。 |
| linkback | 默认关闭的 GitHub 回写：把 issue 标题、描述作为评论发到关联的 open/draft PR。不是本地 link。 |
| events / comments | events 记录创建与状态变化；comments 是用户或 Agent 追加的正文，不能编辑或删除；`author` 只有从 Linear 导入的评论才有，本地追加的为空。 |

定义以 `internal/app/service.go`、`schema.sql`、`github.go`、MCP 工具描述及 [ticket skill](../.agents/skills/ticket/SKILL.md) 为依据。
