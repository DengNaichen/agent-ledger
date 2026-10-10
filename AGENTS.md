# Agent Ledger contributor guide

Agent Ledger tracks its own development. Tickets live in the maintainer's running instance at `http://127.0.0.1:8700`, which agents reach through the `agent-ledger` MCP server configured in `.mcp.json` (Claude Code) and `.codex/config.toml` (Codex).

## Working a ticket

Read the ticket with `get_issue`. Its description is the contract: scope, non-goals, and acceptance. Ask before going beyond it. Set it to `in_progress` when you start.

- [ticket](.agents/skills/ticket/SKILL.md): creating and updating tickets, statuses, and the `agent_ready` gate.
- [git-workflow](.agents/skills/git-workflow/SKILL.md): branches, worktrees, commits, pull requests, merging, and cleanup.

Record decisions and evidence that should outlive the conversation as ticket comments with `add_comment`.

Development builds must not touch the instance. Run them with a temporary database (`-data "$(mktemp -d)/dev.db"`) on the default port 8787.

## Code

Match the existing code. Check only inputs that can actually occur:

- Validate external input (enum values, required fields, referenced records) once, in the service layer.
- Let SQLite constraints (`CHECK`, `FOREIGN KEY`, `UNIQUE`, `NOT NULL`) hold data invariants.

Do not add:

- nil or empty checks for values that cannot be nil or empty
- the same check in the MCP, service, and SQL layers
- silent defaults or corrections for invalid input; return an error instead
- retries, extra timeouts, `recover`, or panic guards
- compatibility code for removed tools, states, or schemas
- interfaces with a single implementation, repository layers, or mock layers
- configuration flags the ticket does not ask for
- error wrapping or logging at every layer
- transactions around a single statement
- tests for cases that cannot happen

Before finishing, go through every new `if`, `try`/`catch`, `?.`, and `??` in the diff and remove the ones that guard against no input that can actually occur. Do not refactor code outside the ticket's scope.

## Schema

The instance holds real data, so no change may require deleting the database.

- `internal/app/schema.sql` always describes the current shape. New databases are created from it and skip all migrations.
- Add a new table there with `CREATE TABLE IF NOT EXISTS`.
- To change an existing table, update `schema.sql` and append a migration to `migrations` in `internal/app/migrate.go` that upgrades an existing database. Never edit a migration that has shipped.
- Changing a `CHECK` constraint, such as adding a status, means rebuilding the table: create the new table, copy the rows, drop the old one, rename. Foreign keys are off while migrations run, and a migration that leaves violations is rolled back.
- The server copies the database to `backups/before-migration-<n>.db` before running migration `n`.

## Updating the instance

After a merge that changes the server:

```sh
git switch main && git pull
make build
install -m 755 agent-ledger ~/.local/bin/agent-ledger
launchctl kickstart -k gui/$(id -u)/io.github.dengnaichen.agent-ledger
```
