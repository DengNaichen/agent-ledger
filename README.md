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

Agent Ledger exposes `list_issues`, `create_issue`, and `move_issue` over stdio:

```sh
go run ./cmd/agent-ledger -mode=mcp -data=/absolute/path/to/agent-ledger.db
```

## Validate

```sh
make test
```

