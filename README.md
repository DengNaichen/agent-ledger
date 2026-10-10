# Agent Ledger

A local, agent-first delivery board: one Go process, one SQLite file, one React Kanban.

## Run

```sh
npm --prefix web install
npm --prefix web run build
go run ./cmd/agent-ledger
```

Open <http://localhost:8787>. Data is stored in `agent-ledger.db` by default.

For frontend hot reload, keep the Go server running and start:

```sh
npm --prefix web run dev
```

The Vite server proxies `/api` to the Go server.

## MCP

The running server exposes `list_issues`, `create_issue`, and `move_issue` over streamable HTTP at `/mcp`. It is the only process that opens the database, so the page and every agent see the same data. Keep the server running while agents work:

```sh
codex mcp add agent_ledger --url http://127.0.0.1:8787/mcp
claude mcp add --transport http agent-ledger http://127.0.0.1:8787/mcp
```

Browser requests from other sites and requests with a non-local `Host` header are rejected.

## Validate

```sh
make test
```

