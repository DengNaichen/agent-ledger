# Agent Ledger contributor guide

Agent Ledger tracks its own development. Tickets live in the maintainer's running instance at `http://127.0.0.1:8700`, which agents reach through the `agent-ledger` MCP server configured in `.mcp.json` (Claude Code) and `.codex/config.toml` (Codex).

## Working a ticket

1. Read the ticket with `get_issue`. Its description is the contract: scope, non-goals, and acceptance. Ask before going beyond it.
2. Set it to `in_progress` with `update_issue`.
3. Branch from the latest `origin/main` as `al-<number>-<short-name>`, for example `al-2-github-sync`.
4. Run `go vet ./...` and `make test` before pushing. Write code, comments, and commit messages in English.
5. Push and open a pull request whose title starts with the identifier, for example `AL-2 Sync pull requests from GitHub`. Record it with `add_link`, then set the ticket to `in_review`.
6. Once the pull request is merged, set the ticket to `done`.

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

The instance holds real data, so no change may require deleting the database. Add tables with `CREATE TABLE IF NOT EXISTS`. Changing an existing table needs an `ALTER TABLE` applied at startup.

## Updating the instance

After a merge that changes the server:

```sh
git switch main && git pull
make build
install -m 755 agent-ledger ~/.local/bin/agent-ledger
launchctl kickstart -k gui/$(id -u)/io.github.dengnaichen.agent-ledger
```
