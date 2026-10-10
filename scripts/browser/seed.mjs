// Seed an empty development server with tickets for browser checks.
// Usage: node scripts/browser/seed.mjs --url http://127.0.0.1:<port>
import { parseArgs } from "node:util";

const usage = `Usage: node scripts/browser/seed.mjs --url http://127.0.0.1:<port>

Creates two Projects and eleven tickets through the server's /mcp endpoint:
every status and type, a spec with two children, blocks, blocked_by and
related relations, a link, and Markdown tables and code blocks in a
description and a comment. Refuses to write when the server already has
tickets. On a new database AL-2 is the spec that shows all of these.`;

const { values } = parseArgs({ options: { url: { type: "string" }, help: { type: "boolean" } } });
if (values.help) {
  console.log(usage);
  process.exit(0);
}
if (!values.url) {
  console.error(usage);
  process.exit(2);
}

const endpoint = new URL("/mcp", values.url);
let requestId = 0;

async function call(name, args) {
  const response = await fetch(endpoint, {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json, text/event-stream" },
    body: JSON.stringify({ jsonrpc: "2.0", id: ++requestId, method: "tools/call", params: { name, arguments: args } }),
  });
  if (!response.ok) throw new Error(`${name}: HTTP ${response.status} ${await response.text()}`);
  const message = await response.json();
  if (message.error) throw new Error(`${name}: ${message.error.message}`);
  if (message.result.isError) throw new Error(`${name}: ${message.result.content[0].text}`);
  return message.result.structuredContent;
}

const existing = await call("list_issues", { limit: 1 });
if (existing.issues.length > 0) {
  console.error(`${values.url} already has tickets; seed only a development server started on a new temporary database.`);
  process.exit(1);
}

async function issue(fields) {
  const created = await call("create_issue", fields);
  console.log(`${created.identifier}\t${created.status}\t${created.type || "-"}\t${created.title}`);
  return created.identifier;
}

const specBody = `## Goal

Agents should be able to pick up work without asking. This spec defines the **agent_ready** gate and what a ticket must contain before it passes.

## Requirements

- Every ticket has a *scope*, *non-goals*, and *acceptance* section.
- Acceptance lists commands that prove the change, such as \`make test\`.
- Tickets that fail the gate stay in \`todo\`.
  - Reviewers add a comment explaining the gap.

### Checklist

- [x] Draft the gate rules
- [ ] Wire the gate into \`update_issue\`

| Field | Required | Notes |
| --- | --- | --- |
| Scope | yes | One outcome per ticket |
| Non-goals | yes | What not to touch |
| Acceptance | yes | Commands with exit codes |

\`\`\`go
func ready(issue Issue) bool {
	return issue.Scope != "" && issue.Acceptance != ""
}
\`\`\`

> Keep the gate small: a check that blocks useful work is worse than no check.

See the [MCP section of the README](https://example.com/docs/mcp) for tool details.`;

const reviewComment = `Checked the gate against the open tickets:

| Ticket | Scope | Acceptance |
| --- | --- | --- |
| Child feature | yes | yes |
| Child chore | yes | no |

\`\`\`sh
go test ./internal/app -run TestIssueLifecycle
\`\`\``;

await call("create_project", { name: "Dogfooding v1", description: "Run Agent Ledger as the tracker for its own development." });
await call("create_project", { name: "Web polish", description: "Make the page pleasant to read and operate." });

const backup = await issue({ title: "Back up the database daily and keep fourteen copies", status: "done", type: "feature", project: "Dogfooding v1" });
const spec = await issue({ title: "Define the agent_ready gate for tickets", description: specBody, status: "in_progress", type: "spec", project: "Dogfooding v1" });
await issue({ title: "Reject agent_ready without an acceptance section", description: "Return an error from `update_issue` when the description has no Acceptance heading.", status: "agent_ready", type: "feature", project: "Dogfooding v1", parent: spec });
await issue({ title: "Document the gate in the ticket skill", status: "todo", type: "chore", project: "Dogfooding v1", parent: spec });
await issue({ title: "Status menu loses focus after a drag on Safari", description: "Repro: drag a card, then open its status menu.", status: "triage", type: "bug", project: "Web polish" });
await issue({ title: "Show relative dates in the issue list", status: "todo", type: "improvement", project: "Web polish" });
const shortcut = await issue({ title: "Keyboard shortcut to create an issue from anywhere on the page", status: "in_review", type: "feature", project: "Web polish" });
await issue({ title: "Explore a hosted multi-user mode", status: "canceled", type: "spec" });
await issue({ title: "Untyped triage note about migration ordering when two branches add migrations at the same time", status: "triage" });
const filters = await issue({ title: "Add list_issues filters for type and parent", status: "done", type: "improvement", project: "Dogfooding v1" });
await issue({ title: "Investigate slow first load on cold start", status: "in_progress", type: "bug" });

await call("add_relation", { identifier: spec, kind: "blocked_by", other: backup });
await call("add_relation", { identifier: spec, kind: "blocks", other: shortcut });
await call("add_relation", { identifier: spec, kind: "related", other: filters });
await call("add_link", { identifier: spec, title: "Ticket skill draft", url: "https://example.com/agent-ledger/skills/ticket/SKILL.md" });
await call("add_comment", { identifier: spec, body: "Started on this. The gate lives in the service layer so MCP and REST share it." });
await call("add_comment", { identifier: spec, body: reviewComment });
await call("add_comment", { identifier: spec, body: "Decision: keep the gate to acceptance only for now." });
