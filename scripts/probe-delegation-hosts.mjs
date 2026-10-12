// Real Claude/Codex main + native subagent hooks against a disposable InfTask.
// The model APIs are local scripted fixtures; no real model credentials are used.
// Usage: node scripts/probe-delegation-hosts.mjs <loopback-url> <test-identifier> <output-dir>
import assert from "node:assert/strict";
import http from "node:http";
import { spawn, spawnSync } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";

const [url, identifier, output] = process.argv.slice(2);
if (!url || !identifier || !output) throw new Error("Expected loopback URL, disposable issue identifier, and output directory");
const endpoint = new URL(url);
if (endpoint.protocol !== "http:" || !["127.0.0.1", "localhost"].includes(endpoint.hostname) || endpoint.port === "8700" || endpoint.pathname !== "/") {
  throw new Error("Use your own disposable loopback InfTask server, never production or port 8700");
}
const root = path.resolve(import.meta.dirname, "..");
const artifacts = path.resolve(output);
fs.mkdirSync(artifacts, { recursive: true });
const temp = fs.mkdtempSync(path.join(os.tmpdir(), "inftask-host-probe-"));
const detailURL = new URL(`/api/w/default/issues/${encodeURIComponent(identifier)}`, endpoint);
const mcpURL = new URL("/w/default/mcp", endpoint).href;
let host;
let modelCalls = 0;

function sendJSON(response, body) {
  response.writeHead(200, { "content-type": "application/json" });
  response.end(JSON.stringify(body));
}
function sendSSE(response, events) {
  response.writeHead(200, { "content-type": "text/event-stream" });
  response.end(events.map(([event, data]) => `${event ? `event: ${event}\n` : ""}data: ${JSON.stringify(data)}\n\n`).join(""));
}
function modelReply(request) {
  const tools = (request.tools ?? []).flatMap((tool) => tool.type === "namespace"
    ? (tool.tools ?? []).map((entry) => ({ ...entry, namespace: tool.name })) : [tool]);
  const delegate = tools.find((tool) => tool.name?.endsWith("__delegate_issue") || (tool.name === "delegate_issue" && tool.namespace === "mcp__inftask"));
  const input = host === "claude" ? request.messages ?? [] : request.input ?? [];
  const child = input.filter((entry) => entry.role === "user").some((entry) => JSON.stringify(entry).includes("CHILD_FIXTURE"));
  const hasDelegate = JSON.stringify(input).includes("fixture_delegate_call");
  const hasSpawn = JSON.stringify(input).includes("fixture_spawn_call");
  const spawner = tools.find((tool) => tool.name === "spawn_agent" || tool.name === "Agent");
  if (delegate && !hasDelegate) return { id: "fixture_delegate_call", tool: delegate, args: { identifier } };
  if (!child && !hasSpawn && spawner) return {
    id: "fixture_spawn_call", tool: spawner,
    args: host === "claude"
      ? { description: "Child fixture", subagent_type: "general-purpose", prompt: `CHILD_FIXTURE: delegate ${identifier} once using the local InfTask tool, then finish.` }
      : { message: `CHILD_FIXTURE: delegate ${identifier} once using the local InfTask tool, then finish.`, fork_context: false },
  };
  if (host === "codex" && !child && hasSpawn && !JSON.stringify(input).includes("fixture_wait_call")) {
    const wait = tools.find((tool) => tool.name === "wait_agent");
    const result = input.find((entry) => entry.type === "function_call_output" && entry.call_id === "fixture_spawn_call");
    const agentID = result ? JSON.parse(result.output).agent_id : undefined;
    if (wait && agentID) return { id: "fixture_wait_call", tool: wait, args: { targets: [agentID], timeout_ms: 10_000 } };
  }
  return null;
}
const model = http.createServer(async (request, response) => {
  try {
    const chunks = [];
    for await (const chunk of request) chunks.push(chunk);
    const body = JSON.parse(Buffer.concat(chunks).toString("utf8") || "{}");
    if (request.url.includes("count_tokens")) return sendJSON(response, { input_tokens: 100 });
    if (++modelCalls > 20) throw new Error("Fixture exceeded its bounded model-call budget");
    const call = modelReply(body);
    if (host === "codex") {
      const item = call ? {
        type: "function_call", call_id: call.id, name: call.tool.name, arguments: JSON.stringify(call.args),
        ...(call.tool.namespace ? { namespace: call.tool.namespace } : {}),
      } : { type: "message", id: "fixture_message", role: "assistant", content: [{ type: "output_text", text: "Fixture done" }] };
      return sendSSE(response, [
        ["", { type: "response.created", response: { id: "fixture_response" } }],
        ["", { type: "response.output_item.done", item }],
        ["", { type: "response.completed", response: { id: "fixture_response", usage: { input_tokens: 1, output_tokens: 1, total_tokens: 2, input_tokens_details: null, output_tokens_details: null } } }],
      ]);
    }
    const block = call ? { type: "tool_use", id: call.id, name: call.tool.name, input: call.args } : { type: "text", text: "Fixture done" };
    const message = { id: "fixture_message", type: "message", role: "assistant", model: body.model, content: [block], stop_reason: call ? "tool_use" : "end_turn", stop_sequence: null, usage: { input_tokens: 1, output_tokens: 1 } };
    if (!body.stream) return sendJSON(response, message);
    sendSSE(response, [
      ["message_start", { type: "message_start", message: { ...message, content: [], stop_reason: null } }],
      ["content_block_start", { type: "content_block_start", index: 0, content_block: call ? { ...block, input: {} } : { type: "text", text: "" } }],
      ["content_block_delta", { type: "content_block_delta", index: 0, delta: call ? { type: "input_json_delta", partial_json: JSON.stringify(call.args) } : { type: "text_delta", text: block.text } }],
      ["content_block_stop", { type: "content_block_stop", index: 0 }],
      ["message_delta", { type: "message_delta", delta: { stop_reason: message.stop_reason, stop_sequence: null }, usage: { output_tokens: 1 } }],
      ["message_stop", { type: "message_stop" }],
    ]);
  } catch (error) {
    response.writeHead(500, { "content-type": "application/json" });
    response.end(JSON.stringify({ error: { type: "fixture_error", message: error.message } }));
  }
});
await new Promise((resolve) => model.listen(0, "127.0.0.1", resolve));
const modelURL = `http://127.0.0.1:${model.address().port}`;

async function readIssue() {
  const response = await fetch(detailURL, { signal: AbortSignal.timeout(5_000) });
  assert.equal(response.status, 200, "Disposable issue must already exist");
  return response.json();
}
async function run(command, args, env, cwd) {
  const child = spawn(command, args, { env, cwd, stdio: ["ignore", "pipe", "pipe"] });
  let stdout = "", stderr = "";
  child.stdout.on("data", (chunk) => { stdout += chunk; });
  child.stderr.on("data", (chunk) => { stderr += chunk; });
  const timeout = setTimeout(() => child.kill("SIGKILL"), 60_000);
  let code;
  try { code = await new Promise((resolve, reject) => { child.on("error", reject); child.on("exit", resolve); }); }
  finally { clearTimeout(timeout); }
  fs.writeFileSync(path.join(artifacts, `${host}-stdout.jsonl`), stdout);
  fs.writeFileSync(path.join(artifacts, `${host}-stderr.txt`), stderr);
  assert.equal(code, 0, `${host} failed or timed out; inspect saved logs`);
}
try {
  for (host of ["claude", "codex"]) {
    modelCalls = 0;
    const before = await readIssue();
    const repo = path.join(temp, `repo-${host}`);
    const home = path.join(temp, `home-${host}`);
    for (const directory of [home, path.join(repo, "scripts"), path.join(repo, ".claude"), path.join(repo, ".codex")]) fs.mkdirSync(directory, { recursive: true });
    for (const file of ["scripts/delegate-hook.mjs", "scripts/run-delegate-hook", ".claude/settings.json", ".codex/hooks.json"]) fs.copyFileSync(path.join(root, file), path.join(repo, file));
    fs.writeFileSync(path.join(repo, ".codex/config.toml"), "");
    assert.equal(spawnSync("git", ["init", "-q", repo], { timeout: 5_000 }).status, 0);
    const env = {
      PATH: process.env.PATH, HOME: home, TMPDIR: temp, CODEX_HOME: home, CLAUDE_CONFIG_DIR: home,
      ANTHROPIC_API_KEY: "fixture-not-a-secret", ANTHROPIC_BASE_URL: modelURL, CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC: "1",
    };
    if (host === "claude") {
      const config = path.join(repo, "mcp.json");
      fs.writeFileSync(config, JSON.stringify({ mcpServers: { inftask: { type: "http", url: mcpURL } } }));
      await run("claude", ["-p", `Delegate ${identifier} using the local InfTask tool.`, "--model", "claude-sonnet-4-5", "--strict-mcp-config", "--mcp-config", config,
        "--setting-sources", "", "--settings", path.join(repo, ".claude/settings.json"), "--tools", "Agent", "--allowedTools", "mcp__inftask__delegate_issue,Agent",
        "--output-format", "stream-json", "--verbose", "--no-session-persistence", "--system-prompt", "Only use the local fixture InfTask tool."], env, repo);
    } else {
      fs.writeFileSync(path.join(home, "config.toml"), `model_provider = "fixture"\nmodel = "gpt-5.4"\n[model_providers.fixture]\nname = "Fixture"\nbase_url = "${modelURL}/v1"\nwire_api = "responses"\nrequires_openai_auth = false\n[mcp_servers.inftask]\nurl = "${mcpURL}"\n[projects."${fs.realpathSync(repo)}"]\ntrust_level = "trusted"\n`);
      // Only this isolated fake-model process bypasses approvals/trust; no user configuration is changed.
      await run("codex", ["--no-daemon", "exec", "--skip-git-repo-check", "--ephemeral", "--dangerously-bypass-hook-trust", "--dangerously-bypass-approvals-and-sandbox", "--json",
        `Delegate ${identifier} using the local InfTask tool.`], env, repo);
    }
    const after = await readIssue();
    const added = after.delegations.filter((entry) => !before.delegations.some((old) => old.id === entry.id));
    const main = added.find((entry) => entry.agent === host && !entry.agent_id);
    const subagent = added.find((entry) => entry.agent === host && entry.agent_id);
    assert(main && subagent, `${host}: expected real main + native subagent records`);
    assert.equal(main.session_id, subagent.session_id, "Probe versions expose the shared parent hook session ID");
    assert.notEqual(main.id, subagent.id);
    assert.equal(after.status, before.status);
    assert.equal(after.updated_at, before.updated_at);
    assert.deepEqual(after.assignee, before.assignee);
    const version = spawnSync(host, ["--version"], { encoding: "utf8", timeout: 5_000 }).stdout.trim();
    fs.writeFileSync(path.join(artifacts, `${host}-records.json`), JSON.stringify({ version, records: added }, null, 2));
    console.log(`${host} ${version}: main + native subagent hook → MCP → SQLite passed`);
  }
} finally {
  model.closeAllConnections();
  await new Promise((resolve) => model.close(resolve));
  fs.rmSync(temp, { recursive: true, force: true });
}
