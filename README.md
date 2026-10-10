# Agent Ledger

A local, agent-first delivery board: one Go process, one SQLite file, one React Kanban.

## Run

```sh
npm --prefix web install
npm --prefix web run build
go run ./cmd/agent-ledger
```

Open <http://localhost:8787>. Data is stored in `agent-ledger.db` by default. Identifiers use `AL-<id>`; pass `-prefix TOK` to use `TOK-<id>`.

The issue model requires a fresh database. Delete an old development database and its `-wal`/`-shm` files while the server is stopped; no migrations are provided.

Use the sidebar to filter by status and click list group headers to collapse or expand them. Click an issue to open its page at `/issue/<identifier>`: the description and comments render as Markdown, relations and sub-issues link to each other, and you can change the status, add a comment, or use Edit for the title and description. In board view, drag a card by its grip to change status, or use its status menu. The `+` button focuses the new-issue title; new issues use the selected status filter (or Triage under All issues).

For frontend hot reload, keep the Go server running and start:

```sh
npm --prefix web run dev
```

The Vite server proxies `/api` to the Go server.

## Data and backups

The server keeps one copy of the database per day in a `backups` directory next to the database file (`daily-YYYY-MM-DD.db`, newest 14 kept) and checks hourly whether today's copy exists. Before upgrading the schema it also writes `before-migration-<n>.db`, which is never pruned.

To restore, stop the server, delete the database's `-wal` and `-shm` files, copy a backup over the database file, and start the server again. Do not copy the live database file with `cp` while the server runs; most recent writes may still be in the `-wal` file.

## GitHub pull requests

The server can link GitHub pull requests to issues. It listens only on localhost, where GitHub webhooks cannot reach it, so it polls each repository once a minute:

```sh
GITHUB_TOKEN=$(gh auth token) agent-ledger -github-repo owner/name -github-repo owner/other
```

- `-github-repo owner/name` turns on sync for a repository and can be repeated; without it nothing syncs. `GITHUB_TOKEN` must allow reading the repository's pull requests. When `-github-repo` is given without `GITHUB_TOKEN`, the server exits at startup.
- A pull request links to every issue whose identifier (`AL-<n>` with the configured prefix, in any case) appears in its branch name or title, such as branch `al-2-github-sync` or title `AL-2 Sync pull requests`. The pull request description is ignored because it often mentions other issues. A pull request may name an issue that does not exist yet; it shows on the issue once the issue is created.
- `get_issue` and `GET /api/issues/<identifier>` list linked pull requests under `pull_requests` with repo, number, title, url, state (`open`, `draft`, `merged`, or `closed`), and `updated_at`.
- Status only moves forward. An open, non-draft pull request moves an issue in `triage`, `todo`, `agent_ready`, or `in_progress` to `in_review`. A merged pull request moves an issue that is not `done` or `canceled` to `done`. Draft pull requests, pull requests closed without merging, and pull requests reopened after their issue is done change nothing. Each change adds a `status_changed` event whose detail ends with `via owner/name#<number>`.
- Each sync reads pull requests by most recent update and stops at the first one updated before the previous sync; the first sync reads them all. A repository that fails is logged and read again in the next round.

`-github-linkback` (off by default) comments on an open or draft pull request the first time it is linked to an existing issue: the identifier and title in bold, then the issue description in a collapsed `<details>` block. **This copies the issue's title and description into GitHub, where everyone who can see the repository can read them; on a public repository that is everyone.** Merged and closed pull requests get no comment, so the first sync does not comment on old pull requests. Linkback needs a token that can comment on pull requests.

## MCP

The running server exposes these tools over streamable HTTP at `/mcp`:

- `list_issues`: filter by status, type, Project name, parent identifier, or title/description query (AND). Returns summaries ordered by `updated_at` descending, with a default limit of 50.
- `get_issue`: retrieve the full issue, comments, links, linked GitHub pull requests, parent identifier, children summaries, and `blocks`, `blocked_by`, and `related` summaries.
- `create_issue`: create an issue with a required title; optionally set description, status, type, Project, and parent.
- `update_issue`: update supplied fields only; an empty string clears type, Project, or parent.
- `add_comment`: append a comment; comments cannot be edited or deleted.
- `add_relation`, `remove_relation`: manage directed `blocks` (or its inverse `blocked_by`) and undirected `related` relations. Duplicate additions succeed without creating duplicates; self-relations are forbidden.
- `add_link`: attach a title and URL to an issue.
- `list_projects`: list unarchived Projects by default; set `include_archived` to include all.
- `create_project`, `update_project`: create named Projects with outcome descriptions, or update description and archived state by name. Names are unique and cannot be changed.

Statuses are `triage` (the default), `todo`, `agent_ready`, `in_progress`, `in_review`, `done`, and `canceled`. Optional types are `spec`, `feature`, `bug`, `improvement`, and `chore`. Project and parent references must already exist. An issue cannot be its own parent; parent cycles are not checked.

Missing issues or Projects return not-found errors. SQLite constraints enforce unique Project names, valid stored values, foreign keys, and the prohibition on self-parenting and self-relations. Only issue creation and status changes add events.

It is the only process that opens the database, so the page and every agent see the same data. Keep the server running while agents work:

```sh
codex mcp add agent_ledger --url http://127.0.0.1:8787/mcp
claude mcp add --transport http agent-ledger http://127.0.0.1:8787/mcp
```

Browser requests from other sites and requests with a non-local `Host` header are rejected.

## Validate

```sh
make test
```


## License

[Apache-2.0](LICENSE)
