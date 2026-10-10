---
name: ticket
description: "Create and update Agent Ledger tickets: choose the Project by outcome, set type and status, apply the agent_ready gate, write self-contained bodies, and link related tickets."
---

# Ticket

Tickets live in the Agent Ledger instance. Manage them with the `agent-ledger` MCP tools: `create_issue`, `update_issue`, `add_comment`, `add_link`, `add_relation`, `remove_relation`, and `list_projects`. Identifiers come only from `create_issue` results; never guess the next number.

## Project

1. Use the Project the user named. Otherwise choose by the ticket's main delivered outcome, not by the directories or languages it touches. There is no default Project.
2. Read the candidates with `list_projects`. When none fits or several fit equally, ask the user. Do not create a Project unless the user asks.
3. Ideas nobody has committed to go to `triage` without a Project.

## Type and status

- `type`: `spec` for a top-level user-visible requirement; `feature`, `bug`, `improvement`, or `chore` for a ticket that delivers a change; empty for records.
- `todo` means the work is committed to. It says nothing about the type or who executes it.
- `agent_ready` admits a ticket to the agent queue once it passes the gate below. Work the user asked to do right now goes straight to `in_progress`.
- `in_review` once the pull request can be reviewed; `done` after it is merged. GitHub sync sets both; see [git-workflow](../git-workflow/SKILL.md).
- A ticket that is superseded or narrowed away becomes `canceled`, with a comment naming the ticket that replaces it.

## agent_ready gate

Check every item before setting `agent_ready`:

- It belongs to an unarchived Project and has no children.
- It can be delivered and reviewed in one pull request.
- Scope names file or directory paths, and a non-goals section blocks scope creep.
- Every scope item points to a file, function, or behavior that exists. Check it on `main` before writing.
- `Red`: the smallest rerunnable check that fails on the current `main` and passes after the change. When you cannot write one, narrow the ticket first.
- Every acceptance item can be rerun by a reviewer. `Red` and acceptance name the exact test name, command, or grep pattern. Never "confirm it works", and never "such as" or "for example" ("如……", "例如……").
- Non-goals do not conflict with acceptance. When acceptance requires a narrow-screen screenshot, a non-goal cannot say "no layout changes".

## Body

- Title: an imperative phrase naming the deliverable. Records start with `Record:`.
- The body must be self-contained: the executor cannot see this conversation. Write the background, scope, non-goals, and acceptance; link documents instead of copying them.
- Decisions the user has not made stay open in the body; do not write a recommendation as if it were agreed.
- When product details are still open, list them under `Open items and defaults`, each with the default the executor follows. Leave nothing for the executor to guess.
- Progress, evidence, and decisions go in comments with `add_comment`. Other references go in with `add_link`; GitHub sync records pull requests.

agent_ready ticket:

```md
Background:
Scope:
Non-goals:
Open items and defaults: (only when product details are open)
Red:
Acceptance:
- <command / test name / grep pattern>
```

Record ticket, status `triage`, no Project, no type:

```md
Record:
No repository change.
```

## Splitting and relations

- A `spec` parent exists only when several tickets share one user-visible outcome. Two outcomes mean two specs. Standalone bugs and chores stay top-level.
- Each child is a self-contained one-pull-request leaf.
- `parent` means ownership; `blocks` / `blocked_by` means required order; `related` covers anything else. Add no relation when order does not matter, and no intermediate parents for grouping by layer or component.
- `agent_ready` tickets that change the same files are ordered with `blocks` and never executed in parallel.
- Work that changes `AGENTS.md`, `docs/`, or `.agents/skills/` runs between batches of tickets. Do not merge it while another ticket is being executed.
