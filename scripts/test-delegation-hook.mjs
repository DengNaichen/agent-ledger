import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { readFileSync, mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { hostname, tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { delegateTool, injectDelegation } from "./delegate-hook.mjs";

const root = resolve(import.meta.dirname, "..");
const event = {
  hook_event_name: "PreToolUse", tool_name: "mcp__inftask-cloud__delegate_issue",
  session_id: "parent-session", tool_input: { identifier: "AL-66", client_context: { agent: "forged", session_id: "forged", hostname: "forged" } },
};
for (const agent of ["codex", "claude"]) {
  const configured = JSON.parse(readFileSync(join(root, agent === "codex" ? ".codex/hooks.json" : ".claude/settings.json"), "utf8"));
  const group = configured.hooks.PreToolUse[0];
  assert.match(event.tool_name, new RegExp(group.matcher));
  assert.equal(group.hooks[0].timeout, 5);
  for (const name of ["inftask", "inftask-cloud", "inftask_cloud", "plugin_inftask_inftask"]) {
    const tool_name = `mcp__${name}__delegate_issue`;
    assert(delegateTool.test(tool_name));
    const result = injectDelegation({ ...event, tool_name }, agent).hookSpecificOutput;
    assert.equal(result.permissionDecision, "allow");
    assert.deepEqual(result.updatedInput, { identifier: "AL-66", client_context: {
      agent, session_id: "parent-session", agent_id: "", agent_type: "", hostname: hostname(),
    } });
  }
  const child = injectDelegation({ ...event, agent_id: "child-a", agent_type: "reviewer" }, agent);
  assert.equal(child.hookSpecificOutput.updatedInput.client_context.agent_id, "child-a");
  assert.equal(child.hookSpecificOutput.updatedInput.client_context.agent_type, "reviewer");
  assert.equal(child.hookSpecificOutput.updatedInput.client_context.session_id, event.session_id);
  for (const other of ["mcp__third_party__delegate_issue", "mcp__inftask__get_issue", "mcp__inftask__update_issue", "Bash"]) {
    assert.deepEqual(injectDelegation({ ...event, tool_name: other }, agent), {});
    assert(!new RegExp(group.matcher).test(other));
  }
  assert.deepEqual(injectDelegation({ ...event, hook_event_name: "PostToolUse" }, agent), {});
  for (const invalid of [
    { ...event, session_id: undefined }, { ...event, session_id: " " },
    { ...event, session_id: "line\nbreak" }, { ...event, session_id: "x".repeat(257) },
    { ...event, agent_id: "x".repeat(257) }, { ...event, agent_type: 7 },
    { ...event, tool_input: [] },
  ]) assert.throws(() => injectDelegation(invalid, agent));
  // Run the actual configured command, including desktop PATH resolution.
  for (const supplied of [event, { ...event, agent_id: "child-b", agent_type: "implementer" }]) {
    const result = spawnSync("/bin/sh", ["-c", group.hooks[0].command], {
      cwd: root, input: JSON.stringify(supplied), encoding: "utf8", timeout: 10_000,
      env: { ...process.env, PATH: "/usr/bin:/bin", CLAUDE_PROJECT_DIR: root },
    });
    assert.equal(result.status, 0, result.stderr);
    const actual = JSON.parse(result.stdout).hookSpecificOutput.updatedInput.client_context;
    assert.equal(actual.agent, agent);
    assert.equal(actual.session_id, supplied.session_id);
    assert.equal(actual.agent_id, supplied.agent_id ?? "");
  }
}
for (const input of ["not json", JSON.stringify({ ...event, session_id: undefined }), "x".repeat(1024 * 1024 + 1)]) {
  const result = spawnSync(process.execPath, [join(root, "scripts/delegate-hook.mjs"), "codex"], {
    input, encoding: "utf8", timeout: 10_000,
  });
  assert.equal(result.status, 0, result.stderr);
  assert.equal(JSON.parse(result.stdout).hookSpecificOutput.permissionDecision, "deny");
}
// Paths with spaces remain data, not shell syntax.
const temp = mkdtempSync(join(tmpdir(), "inftask delegate hook "));
try {
  const dir = join(temp, "scripts");
  mkdirSync(dir);
  for (const file of ["run-delegate-hook", "delegate-hook.mjs"]) {
    writeFileSync(join(dir, file), readFileSync(join(root, "scripts", file)));
  }
  const command = JSON.parse(readFileSync(join(root, ".claude/settings.json"), "utf8")).hooks.PreToolUse[0].hooks[0].command;
  const result = spawnSync("/bin/sh", ["-c", command], {
    cwd: temp, input: JSON.stringify(event), encoding: "utf8", timeout: 10_000,
    env: { ...process.env, CLAUDE_PROJECT_DIR: temp },
  });
  assert.equal(result.status, 0, result.stderr);
  assert.equal(JSON.parse(result.stdout).hookSpecificOutput.updatedInput.identifier, "AL-66");
} finally { rmSync(temp, { recursive: true, force: true }); }
console.log("Delegation hook contract passed (Claude/Codex main and subagent payloads; no network).");
