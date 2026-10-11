---
name: ticket
description: "Find, create, and update tickets on an InfTask board through the inftask MCP tools: take the team and workspace from the repository's AGENTS.md, choose the Project by outcome, set type and status, apply the agent_ready gate, write self-contained bodies, link related tickets, and let GitHub sync move tickets to in_review and done."
---

# Ticket

Tickets live on an InfTask board. Use the `inftask` MCP tools: `list_issues`, `get_issue`, `create_issue`, `update_issue`, `add_comment`, `add_link`, `add_relation`, `remove_relation`, `list_teams`, and `list_projects`. Labels have their own tools; reuse existing labels before creating one.

## Team, workspace, and language

- Take the team key and workspace from the current repository's `AGENTS.md`. When it does not name them, ask the user. Never guess a team.
- The workspace is part of the MCP server URL (`/w/<slug>/mcp`); no tool takes a workspace argument. When `list_teams` does not list the repository's team, the server points at another workspace or instance: stop and tell the user instead of writing elsewhere.
- `create_issue` requires `team` and has no default: always pass the repository's team key. Filter `list_issues` by it too, so results stay within the repository's team.
- Write ticket titles, bodies, and comments in the language `AGENTS.md` asks for; when it names none, use the language the user writes in. Paths, commands, code identifiers, and `status` and `type` values stay as written.
- Follow the rest of `AGENTS.md` where it is stricter than this skill.

## Finding tickets

- `list_issues` returns summaries without descriptions, 50 by default. Filter by `team`, `status`, `type`, `project`, `parent`, `query` (title and description text), and labels; pass `limit: -1` when the result must not be truncated.
- Read a ticket with `get_issue` before acting on it. Its description, comments, relations, children, and linked pull requests are only there.

## New work without a ticket

- Follow the ticket requirement in the repository's `AGENTS.md`. Reuse the current ticket for the same outcome.
- Before creating, search `list_issues` for a ticket with the same outcome and read candidates with `get_issue`. Create one only when none matches.
- Identifiers are `<KEY>-<n>`, and they come only from `create_issue` results. Never guess the next number.
- For work the user asked to do now, create the ticket as `in_progress`, read it back with `get_issue`, and use the returned identifier for the branch, commits, and pull request.
- When the user says the work belongs to a particular existing ticket and the session does not identify it, ask for the identifier.
- When ticket creation fails, report the actual error and pause the changes that need the ticket.

## Working a ticket

- The description's scope, non-goals, and acceptance are the contract. Ask before going beyond scope.
- Set `in_progress` when you start. Record lasting decisions, progress, and verification evidence with `add_comment`; comments cannot be edited or deleted.
- Attach references other than pull requests with `add_link`.

## Project

1. Use the Project the user named. Otherwise choose by the ticket's main delivered outcome, not by the directories or languages it touches. There is no default Project.
2. Read the candidates with `list_projects`. When none fits or several fit equally, ask the user. Do not create a Project unless the user asks.
3. Ideas nobody has committed to go to `triage` without a Project.

## Type, status, and priority

- `type`: `spec` for a top-level user-visible requirement; `feature`, `bug`, `improvement`, or `chore` for a ticket that delivers a change; empty for records.
- `triage`: newly arrived; whether to keep it is undecided. New tickets default to it.
- `backlog`: worth keeping, with no commitment to when.
- `todo`: committed to; says nothing about type or who executes it.
- `agent_ready`: admitted to the agent queue after passing the gate below. Work the user asked to do now goes straight to `in_progress`.
- `in_progress`: being worked on.
- `in_review`: its pull request can be reviewed. Set by GitHub sync.
- `done`: its pull request merged. Set by GitHub sync.
- `canceled`: superseded or narrowed away. Comment with the ticket that replaces it.
- `priority`: `0` none (default), `1` urgent, `2` high, `3` medium, `4` low. On `update_issue`, omitting it keeps it and `0` clears it.

## agent_ready gate

Check every item before setting `agent_ready`:

- It belongs to an unarchived Project and has no children.
- It can be delivered and reviewed in one pull request.
- Scope names file or directory paths, and a non-goals section blocks scope creep.
- Every scope item points to a file, function, or behavior that exists on the default branch. Check before writing.
- `Red`: the smallest rerunnable check that fails before the change and passes after it. When you cannot write one, narrow the ticket first. When `Red` is a new test, it does not exist yet before the change; say so, and have the executor add the test first and watch it fail against the unchanged code.
- Every acceptance item can be rerun by a reviewer and names the exact test, command, or search pattern. Never "confirm it works", and never "such as" or "for example".
- Every acceptance item gives different results before and after the change; a check that already passes cannot be an acceptance item on its own.
- When acceptance depends on a file or tool outside the repository, state that prerequisite. When a check needs a new worktree, name the commit or branch to create it from.
- Non-goals do not conflict with acceptance.
- Tickets that change the same files are ordered with `blocks` and never executed in parallel.

## Body

- Title: an imperative phrase naming the deliverable. Record tickets start with a "Record:" prefix in the ticket language.
- The body must be self-contained: the executor cannot see this conversation. Link documents instead of copying them.
- Decisions the user has not made stay open; do not write a recommendation as if it were agreed. List open product details with the default the executor follows, so nothing is left to guess.

Sections of an `agent_ready` ticket, with headings in the ticket language:

```md
Background:
Scope:
Non-goals:
Open items and defaults: (only when product details are open)
Red:
Acceptance:
- <command / test name / search pattern>
```

A record ticket has status `triage`, no Project, no type, and says it produces no repository change.

## Splitting and relations

- A `spec` parent exists only when several tickets share one user-visible outcome. Two outcomes mean two specs. Standalone bugs and chores stay top-level.
- Each child is a self-contained one-pull-request ticket.
- `parent` means ownership; `blocks` / `blocked_by` means required order; `related` covers anything else. Add no relation when order does not matter, and no intermediate parents for grouping by layer or component.

## Pull requests and GitHub sync

An instance configured to sync the repository links a pull request to every ticket whose identifier `<KEY>-<n>` appears, in any letter case, in the pull request's branch name or title. The pull request description is ignored.

- Put the identifier in the branch name and the pull request title.
- Opening a non-draft pull request moves the ticket to `in_review`; merging it moves the ticket to `done`. Drafts and pull requests closed without merging change nothing. Do not set `in_review` or `done` by hand, and do not record the pull request with `add_link`.
- About a minute after opening the pull request, run `get_issue` and check that `pull_requests` lists it and the status is `in_review`. If it is still missing a few minutes later, the instance may not sync this repository: tell the user instead of setting the status, unless `AGENTS.md` says otherwise.
