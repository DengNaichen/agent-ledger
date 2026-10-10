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

For frontend hot reload, keep the Go server running and start:

```sh
npm --prefix web run dev
```

The Vite server proxies `/api` to the Go server.

## MCP

The running server exposes these tools over streamable HTTP at `/mcp`:

- `list_issues`: filter by status, type, Project name, parent identifier, or title/description query (AND). Returns summaries ordered by `updated_at` descending, with a default limit of 50.
- `get_issue`: retrieve the full issue, comments, links, parent identifier, children summaries, and `blocks`, `blocked_by`, and `related` summaries.
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
