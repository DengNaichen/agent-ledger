# InfTask

An agent-first delivery board for local use or a shared cloud instance: one Go process, one SQLite file, one React Kanban.

## Run

```sh
npm --prefix web install
npm --prefix web run build
go run ./cmd/inftask
```

Open <http://localhost:8787>. Data is stored in `inftask.db` by default. Every issue belongs to a team, and each team numbers its own issues: team `AL` issues `AL-1`, `AL-2`, and so on, independently of team `TOK`. A new database has no teams; create one with the `create_team` MCP tool before creating issues.

New databases use the current schema directly. Existing databases use the append-only migration mechanism. Migration 1 introduces teams: each identifier prefix already in the database becomes a team, and its numbering continues after its highest number. Arbitrary legacy schemas are not automatically converted. Do not delete the database to apply a schema change. See [data reliability](docs/rules/RELIABILITY.md).

Use the sidebar to filter by status or Project, the search box (press Enter) to match titles and descriptions, and the type menu to filter by type; active filters appear as chips above the list, and their × clears them. Click list group headers to collapse or expand them. Click an issue to open its page at `/issue/<identifier>`: the description and comments render as Markdown, relations and sub-issues link to each other, linked pull requests show their state and open on GitHub, the properties panel shows and changes the status, and Edit changes the title and description. In list and board view, click an issue's status icon to change its status; in board view you can also drag a card by its grip. New issue opens a dialog for the title, description, and team (the first team is preselected); new issues use the selected status filter (or Triage under All issues) and the selected Project.

Use **Labels** in the sidebar to create, edit, archive, restore, and merge global issue labels. Ungrouped labels are multi-select; each group allows one value per issue. Open **Edit labels** on an issue, or select issues in the list/board and choose **Change labels** for an atomic batch operation. The Labels filter supports any, all, none of the selected labels/groups, and issues with no labels. Archived labels remain on existing issues and in filters. Labels are independent of type, status, and Project.

For frontend hot reload, keep the Go server running and start:

```sh
npm --prefix web run dev
```

The Vite server proxies `/api` and `/auth` to the Go server.

## Data and backups

The server keeps one copy of the database per day in a `backups` directory next to the database file (`daily-YYYY-MM-DD.db`, newest 14 kept) and checks hourly whether today's copy exists. Before upgrading the schema it also writes `before-migration-<n>.db`, which is never pruned.

To restore, stop the server, delete the database's `-wal` and `-shm` files, copy a backup over the database file, and start the server again. Do not copy the live database file with `cp` while the server runs; most recent writes may still be in the `-wal` file.

## GitHub pull requests

The server can link GitHub pull requests to issues. It polls each repository once a minute; cloud deployments can also trigger immediate refreshes through a signed webhook:

```sh
GITHUB_TOKEN=$(gh auth token) inftask -github-repo owner/name -github-repo owner/other
```

- `-github-repo owner/name` turns on sync for a repository and can be repeated; without it nothing syncs. `GITHUB_TOKEN` must allow reading the repository's pull requests. When `-github-repo` is given without `GITHUB_TOKEN`, the server exits at startup.
- A pull request links to every issue whose identifier (`<team key>-<n>` for any existing team, in any case) appears in its branch name or title, such as branch `al-2-github-sync` or title `AL-2 Sync pull requests`. The teams are read again on every sync, so a new team's identifiers are recognized from the next sync on; pull requests read before the team existed are not read again. The pull request description is ignored because it often mentions other issues. A pull request may name an issue of an existing team that does not exist yet; it shows on the issue once the issue is created.
- `get_issue` and `GET /api/issues/<identifier>` list linked pull requests under `pull_requests` with repo, number, title, url, state (`open`, `draft`, `merged`, or `closed`), and `updated_at`.
- Status only moves forward. An open, non-draft pull request moves an issue in `triage`, `todo`, `agent_ready`, or `in_progress` to `in_review`. A merged pull request moves an issue that is not `done` or `canceled` to `done`. Draft pull requests, pull requests closed without merging, and pull requests reopened after their issue is done change nothing. Each change adds a `status_changed` event whose detail ends with `via owner/name#<number>`.
- Each sync reads pull requests by most recent update and stops at the first one updated before the previous sync; the first sync reads them all. A repository that fails is logged and read again in the next round.

`-github-linkback` (off by default) comments on an open or draft pull request the first time it is linked to an existing issue: the identifier and title in bold, then the issue description in a collapsed `<details>` block. **This copies the issue's title and description into GitHub, where everyone who can see the repository can read them; on a public repository that is everyone.** Merged and closed pull requests get no comment, so the first sync does not comment on old pull requests. Linkback needs a token that can comment on pull requests.

Set `GITHUB_WEBHOOK_SECRET` to enable `POST /webhooks/github`; at least one `-github-repo` is required. Configure GitHub to send `pull_request` events as JSON to that path over HTTPS, with the same webhook secret. Valid deliveries for configured repositories schedule a serial refresh through the existing sync worker. Pending notifications coalesce; the payload itself never writes PR state. Polling recovers downtime and failed reads. A `202` means scheduled, not that the ticket has already changed.

## Cloud team instance

The cloud instance uses InfOne UC for sign-in. All authenticated UC users share the same board and can use all existing operations. There are no workspace roles or tenant isolation. The UC client may temporarily be the existing HarnessKit client; configure it on the backend rather than copying credentials into this repository.

See [cloud deployment and recovery](docs/DEVELOPMENT.md#云端团队部署) for configuration, Compose, and the remote MCP OAuth contract. Deployment artifacts are [Dockerfile](Dockerfile), [Compose](deploy/compose.yaml), and [Caddyfile](deploy/Caddyfile). The single instance keeps SQLite on a persistent volume; it is not a multi-replica deployment.

## MCP

The running server exposes these tools over streamable HTTP at `/mcp`:

- `list_issues`: filter by status, type, team key, Project name, parent identifier, title/description query, and labels (AND). `label_ids` and `label_group_ids` select predicates combined using `label_mode`: `any` (default), `all`, or `none`; `unlabeled` requires empty selections. Returns summaries including labels, ordered by `updated_at` descending, with a default limit of 50.
- `get_issue`: retrieve the full issue, comments, links, linked GitHub pull requests, parent identifier, children summaries, and `blocks`, `blocked_by`, and `related` summaries.
- `create_issue`: create an issue with a required team key and title; optionally set description, status, type, Project, and parent. There is no default team. The issue gets the team's next number in one transaction with the insert.
- `list_teams`: list teams with their key, name, and `next_number`.
- `create_team`: create a team with a key of 2 to 6 uppercase letters or digits starting with a letter, and a required name. Numbering starts at 1. A key cannot be changed, and teams cannot be renamed or deleted.
- `update_issue`: update supplied fields only; an empty string clears type, Project, or parent.
- `add_comment`: append a comment; comments cannot be edited or deleted.
- `add_relation`, `remove_relation`: manage directed `blocks` (or its inverse `blocked_by`) and undirected `related` relations. Duplicate additions succeed without creating duplicates; self-relations are forbidden.
- `add_link`: attach a title and URL to an issue.
- `list_labels`, `list_label_groups`: discover existing labels and single-select groups. Labels exclude archived entries unless `include_archived` is true.
- `create_label`, `update_label`: create labels or edit supplied name/color/description/archived properties by stable ID. Names are trimmed and case-insensitively unique within a group; colors use `#RRGGBB`. `group_id: 0` means ungrouped; membership is fixed after creation.
- `save_label_group`: create a group, or rename an existing group by `id`; group names are unique and groups cannot nest.
- `change_issue_labels`: supply `identifiers` and incremental `add`/`remove` label ID arrays. The whole batch succeeds or rolls back. Adding a grouped label replaces the old selection; duplicate additions/removals are idempotent. Multiple additions in one group and overlapping add/remove arrays are rejected. Archived labels can be removed or retained, but cannot be newly applied.
- `merge_labels`: transfer `source_id` associations to an active `target_id` in the same group, then delete the source. External filters referencing the source ID must be updated.
- `list_projects`: list unarchived Projects by default; set `include_archived` to include all.
- `create_project`, `update_project`: create named Projects with outcome descriptions, or update description and archived state by name. Names are unique and cannot be changed.

Statuses are `triage` (the default), `todo`, `agent_ready`, `in_progress`, `in_review`, `done`, and `canceled`. Optional types are `spec`, `feature`, `bug`, `improvement`, and `chore`. Team, Project, and parent references must already exist. An issue cannot be its own parent; parent cycles are not checked.

Missing issues, teams, or Projects return not-found errors. SQLite constraints enforce unique Project names, valid stored values, foreign keys, and the prohibition on self-parenting and self-relations. Only issue creation and status changes add events.

It is the only process that opens the database, so the page and every agent see the same data. Keep the server running while agents work:

```sh
codex mcp add inftask --url http://127.0.0.1:8787/mcp
claude mcp add --transport http inftask http://127.0.0.1:8787/mcp
```

The server binds to loopback by default and wraps requests in Go cross-origin protection. Local mode has no separate `Host` allowlist or user authentication. Non-loopback listeners require UC authentication; see [security boundaries](docs/rules/SECURITY.md).

## Run as a macOS LaunchAgent

launchd can keep a long-lived instance running: it starts the server at login and restarts it when it exits. Give that instance a non-default port, such as 8700, and keep the default 8787 for development builds, so a development server never opens the instance's database. Register MCP clients with the instance's port, such as `http://127.0.0.1:8700/mcp`.

Save the following as `~/Library/LaunchAgents/com.example.inftask.plist`, replacing `/Users/you`, `owner/name`, and the label with your own. launchd does not expand `~` or `$HOME` and starts jobs with a minimal `PATH`, so every path is absolute, including `gh` (`which gh` prints it) and the `inftask` binary.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.example.inftask</string>
  <key>ProgramArguments</key>
  <array>
    <string>/bin/sh</string>
    <string>-c</string>
    <string>GITHUB_TOKEN="$(/opt/homebrew/bin/gh auth token)" exec /Users/you/.local/bin/inftask -addr 127.0.0.1:8700 -data "/Users/you/Library/Application Support/inftask/inftask.db" -github-repo owner/name</string>
  </array>
  <key>RunAtLoad</key>
  <true/>
  <key>KeepAlive</key>
  <true/>
  <key>StandardOutPath</key>
  <string>/Users/you/Library/Logs/inftask.log</string>
  <key>StandardErrorPath</key>
  <string>/Users/you/Library/Logs/inftask.log</string>
</dict>
</plist>
```

The token is not written into the plist: `/bin/sh -c` asks `gh auth token` for it each time the job starts, and `exec` then replaces the shell with the server. Without GitHub sync, drop `GITHUB_TOKEN="$(…)"` and `-github-repo`. The server does not create the database's directory, so create it once, then load the job:

```sh
mkdir -p ~/Library/"Application Support"/inftask
launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.example.inftask.plist
```

After rebuilding, install the binary at the path the plist names and restart the job:

```sh
make build
install -m 755 inftask /Users/you/.local/bin/inftask
launchctl kickstart -k gui/$(id -u)/com.example.inftask
```

To stop the job and unload it:

```sh
launchctl bootout gui/$(id -u)/com.example.inftask
```

`kickstart` does not reread the plist. After editing the plist, run `bootout` and then `bootstrap` again. A `bootstrap` right after `bootout` can fail with `Bootstrap failed: 5: Input/output error` while the old job is still shutting down; wait a few seconds and retry. Startup errors, such as a missing `GITHUB_TOKEN`, go to `~/Library/Logs/inftask.log`.

## Validate

```sh
make test
```


## License

[Apache-2.0](LICENSE)
