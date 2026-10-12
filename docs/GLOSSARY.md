# 项目术语

| 术语 | 在本项目中的含义与边界 |
| --- | --- |
| workspace | 一组 team、Project 和标签，也是云端成员访问边界。由不能修改的 slug 命名：页面在 `/<slug>/`，REST 在 `/api/w/<slug>/`，MCP 在 `/w/<slug>/mcp`。不能删除。不是 Git 工作树或 HarnessKit 的 workspace membership。 |
| Admin / Member | 工作区内的人类角色，采用 Linear 的基础成员模型。Admin 管理邀请、成员角色、停用/恢复和工作区改名；Member 可协作处理当前公开团队的业务数据。新建者为 Admin；已有工作区显式初始化。Guest 与 Agent 独立授权尚未实现。 |
| Team owner | 团队管理角色，创建团队或将其转为私有的人默认成为 owner；Workspace Admin 有团队管理能力，但读私有任务仍须显式加入。团队可以没有 owner，不应用工作区的最后 Admin 保护。 |
| private team | 只有显式团队成员能读写其中任务；普通成员不可发现未加入的私有团队。Admin 可在设置目录发现并管理，确认加入后才有内容访问权。公开团队允许未加入的工作区成员协作。 |
| suspended | 某人在某个工作区被停用，保留历史记录；后续 REST/MCP 请求失去访问权，邀请不能自行恢复。由活动 Admin 恢复，不能移除最后一个活动 Admin。 |
| assignee | 单一人类负责人，可为空；代表责任归属，不是编辑锁。任何有任务写权限的活动成员可改派或清空。候选必须是活动工作区成员且可访问任务团队；停用保留历史显示。Agent delegate/claim/session 不使用此字段。 |
| delegation / delegate_issue | 当前主或子 Agent 显式登记参与一张票，记录宿主 agent/session/subagent/hostname 与可信操作者。与 status 和人的 assignee 分离，不是编辑锁、任务调度或进程存活证明。相同执行身份去重，不同子 Agent 即使共享父 session 也各有记录。 |
| Issue / ticket | 同一工作记录，标识符由服务端生成，形如 `<team key>-<number>`，编号由所属 team 依次分配，只在所属 workspace 内唯一。不能推测下一个编号。 |
| team | 有自己票号前缀（key）和独立编号的 issue 归属，如本仓库的 `AL`。建票必须指定 team，没有默认值；key 在 workspace 内唯一，建好后不能改，显示名称可改，team 不能删除或换 workspace，票不能换 team。状态所有 workspace 共用，Project 显式关联同一 workspace 的一个或多个 team。不是 GitHub team 或 UC 用户组。 |
| Project | 按交付结果组织 ticket 的具名记录，不是 Git 仓库或源码目录。名称在 workspace 内唯一，可通过稳定 id 改名；当前名称也可寻址，旧名称不是别名。内部 id、Team 关联和任务引用不随改名变化。显式关联同一 workspace 的一个或多个 Team，空项目也按关联出现在 Team 列表。历史无任务项目保留待归属，仅 workspace 列表可见；新建必须指定 Team。任务只能属于兼容 Team 的项目；归档后默认列表不展示且不接受新任务归属，已有任务可保留。 |
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

定义以 `internal/app` 的 `workspace.go`、`issue.go`、`project.go`、`team.go`、`schema.sql`、`github.go`、MCP 工具描述及 [ticket skill](../.agents/skills/ticket/SKILL.md) 为依据。
