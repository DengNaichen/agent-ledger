---
name: ticket
description: "Create and update InfTask tickets: choose the Project by outcome, set type and status, apply the agent_ready gate, write self-contained bodies, and link related tickets."
---

# Ticket

Tickets live in the InfTask instance. Manage them with the `inftask` MCP tools: `create_issue`, `update_issue`, `get_issue`, `list_issues`, `delegate_issue`, `add_work_log`, `list_work_logs`, `get_work_log`, `add_comment`, `add_link`, `add_relation`, `remove_relation`, and `list_projects`. `create_issue` requires `team`, and there is no default: use the team the repository's `AGENTS.md` names (`AL` in this repository). Identifiers come only from `create_issue` results; never guess the next number.

## New work without an existing ticket

- Follow the ticket requirement and no-ticket exceptions in the repository's [AGENTS.md](../../../AGENTS.md#工作策略). Reuse the current ticket for the same outcome. A documentation or skill change that alters shared agent behavior requires a ticket.
- When the user requests a new repository change that requires a ticket and none is established in the session, check `list_issues` for an existing ticket with the same outcome, then read any match with `get_issue`. If none matches, select the Project below and automatically create a self-contained one-PR ticket with `create_issue`; do not ask the user to supply or guess a new identifier.
- For work the user asked to execute now, create the ticket as `in_progress`, read it back with `get_issue`, and use the returned identifier for the branch, commits, and pull request under [git-workflow](../git-workflow/SKILL.md). Direct execution does not need to pass through `agent_ready`.
- Ask for an existing identifier only when the user says the work belongs to a particular existing ticket and the session and branch do not identify it. Project ambiguity still follows the Project rules below.
- If ticket creation is unavailable or fails, report the actual failure and pause the repository changes that require that ticket. Do not invent a number or continue through the no-ticket exception.

## Agent participation (repository dogfooding)

- After reading a ticket, a Claude/Codex main agent or subagent starting work calls `delegate_issue` with only `identifier`. The repository `PreToolUse` hook supplies the current agent/session/subagent/hostname context; never guess or manually generate it.
- A human `assignee` is required before every delegation, including repeated calls. If missing, first assign an authorized human using `list_assignees` and `update_issue`; do not substitute the Agent session or silently invent a responsible person. Clearing the assignee preserves existing delegation history but prevents further calls until someone is assigned.
- This is independent of status. Register even when the ticket is already `in_progress`; then follow the normal status workflow. Comments, edits, and status changes do not register participation.
- Each subagent calls for the ticket it actually takes. Shared parent session IDs do not replace the host-provided subagent ID. Repeated calls from the same execution identity are idempotent.
- These hooks are repository-local, not part of the distributed `inftask` plugin. They require Node 22+ and a trusted hook configuration; see [development](../../../docs/DEVELOPMENT.md#仓库内-agent-接票).
- If the connected server does not expose `delegate_issue`, report that participation recording is not deployed and continue the existing ticket workflow without inventing a record. Other hosts without a provenance adapter do not fabricate metadata. A configured hook failure must be fixed before claiming registration succeeded.

## Project

1. Use the Project the user named. Otherwise choose by the ticket's main delivered outcome, not by the directories or languages it touches. There is no default Project.
2. Read the candidates with `list_projects`. When none fits or several fit equally, ask the user. Do not create a Project unless the user asks.
3. Ideas nobody has committed to go to `triage` without a Project.

## Type and status

- `type`: `spec` for a top-level user-visible requirement; `feature`, `bug`, `improvement`, or `chore` for a ticket that delivers a change; empty for records.
- `triage` means newly arrived work whose retention has not been decided.
- `backlog` means work worth keeping, without a commitment to when it will be done.
- `todo` means the work is committed to. It says nothing about the type or who executes it.
- `agent_ready` admits a ticket to the agent queue once it passes the gate below. Work the user asked to do right now goes straight to `in_progress`.
- `in_review` once the pull request can be reviewed; `done` after it is merged. GitHub sync sets both; see [git-workflow](../git-workflow/SKILL.md).
- A ticket that is superseded or narrowed away becomes `canceled`, with a work log naming the ticket that replaces it.

## agent_ready gate

Check every item before setting `agent_ready`:

- It belongs to an unarchived Project and has no children.
- It can be delivered and reviewed in one pull request.
- Scope names file or directory paths, and a non-goals section blocks scope creep.
- For tickets changing `internal/app/schema.sql` or `internal/app/migrate.go`, query `list_issues` separately for `agent_ready`, `in_progress`, and `in_review`, with `limit: -1` to avoid truncation, then read candidate scopes with `get_issue`. If another ticket changes either file, order them with `blocks` before setting `agent_ready`; only one schema ticket may execute at a time, and the predecessor must merge before the successor starts. See [schema queue and merge rules](../../../docs/rules/RELIABILITY.md#schema-票的排队与合并).
- Every scope item points to a file, function, or behavior that exists. Check it on `main` before writing.
- `Red`: the smallest rerunnable check that fails on the current `main` and passes after the change. When you cannot write one, narrow the ticket first.
- When `Red` is a new test, it cannot fail on `main`, where it does not exist yet: `go test -run <name>` exits 0 with `[no tests to run]`. Say so in the ticket, and have the executor add the test first and run it against the unchanged code, checking the output for `--- FAIL`, before making the change.
- Every acceptance item can be rerun by a reviewer. `Red` and acceptance name the exact test name, command, or grep pattern. Never "confirm it works", and never "such as" or "for example" ("如……", "例如……").
- Every acceptance item gives different results before and after the change. A check that already passes on `main` cannot be an acceptance item on its own.
- When acceptance depends on a file or tool outside the repository, state the prerequisite: the path or the tool.
- When a check needs a new worktree, name the commit or branch to create it from.
- Non-goals do not conflict with acceptance. When acceptance requires a narrow-screen screenshot, a non-goal cannot say "no layout changes".

## Body

- Ticket titles, bodies, work logs, and comments are in Chinese. Paths, commands, code identifiers, and `status` and `type` values stay as written. See the Language section of [git-workflow](../git-workflow/SKILL.md#language).
- Title: an imperative phrase naming the deliverable. Records start with `记录：`.
- The body must be self-contained: the executor cannot see this conversation. Write the background, scope, non-goals, and acceptance; link documents instead of copying them.
- For schema work, write “追加一个迁移” in the ticket and plan, without reserving a migration number. Its number comes from its position in `internal/app/migrate.go`'s `migrations` at merge time.
- Decisions the user has not made stay open in the body; do not write a recommendation as if it were agreed.
- When product details are still open, list them under `待定项和默认做法`, each with the default the executor follows. Leave nothing for the executor to guess.
- Agent progress, findings, verification, and handoff go in `add_work_log` with a short `summary` and a Markdown `body`; use `kind=progress` (default). Use `kind=decision` only for a settled choice, explaining the context, choice, and reasons. Both kinds are append-only. Do not record every command or tentative thought.
- `get_issue` includes decision summaries and the work-log count. Read relevant bodies with `get_work_log`; use `list_work_logs` and its `next_before` cursor when earlier work is needed. Do not load every log by default.
- Comments are for human discussion. Use `add_comment` only when the user explicitly asks you to post a comment. Historical comments remain unclassified. If the connected server lacks work-log tools, report the version gap; do not silently route agent logs back into comments.
- Other references go in with `add_link`; GitHub sync records pull requests.

agent_ready ticket:

```md
背景：
范围：
非目标：
待定项和默认做法：（仅在有未决产品细节时写）
Red：
验收：
- <command / test name / grep pattern>
```

Record ticket, status `triage`, no Project, no type:

```md
记录：
不产生仓库改动。
```

## Splitting and relations

- A `spec` parent exists only when several tickets share one user-visible outcome. Two outcomes mean two specs. Standalone bugs and chores stay top-level.
- Each child is a self-contained one-pull-request leaf.
- `parent` means ownership; `blocks` / `blocked_by` means required order; `related` covers anything else. Add no relation when order does not matter, and no intermediate parents for grouping by layer or component.
- `agent_ready` tickets that change the same files are ordered with `blocks` and never executed in parallel.
- Work whose purpose is to change rules or workflow in `AGENTS.md`, `docs/`, or `.agents/skills/` runs between batches of tickets. Do not merge it while another ticket is being executed. Updating the document that owns a fact together with the code change, such as `docs/DEVELOPMENT.md` or `docs/DESIGN_SYSTEM.md`, is not restricted.
