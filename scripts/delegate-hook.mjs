// Repository-local provenance injection; no credentials, transcripts, or network.
import { realpathSync } from "node:fs";
import { hostname } from "node:os";
import { pathToFileURL } from "node:url";

export const delegateTool = /^mcp__(?:inftask|inftask-cloud|inftask_cloud|plugin_inftask_inftask)__delegate_issue$/;

function field(value, name, limit, required = false) {
  if (value === undefined && !required) return "";
  if (typeof value !== "string" || !value || value !== value.trim() ||
      Buffer.byteLength(value) > limit || /\p{Cc}/u.test(value)) {
    throw new Error(`invalid or missing hook ${name}`);
  }
  return value;
}

export function injectDelegation(event, agent) {
  if (event?.hook_event_name !== "PreToolUse" || !delegateTool.test(event.tool_name ?? "")) return {};
  if (agent !== "codex" && agent !== "claude") throw new Error("unsupported hook host");
  if (!event.tool_input || typeof event.tool_input !== "object" || Array.isArray(event.tool_input)) {
    throw new Error("missing tool arguments");
  }
  const context = {
    agent,
    session_id: field(event.session_id, "session_id", 256, true),
    agent_id: field(event.agent_id, "agent_id", 256),
    agent_type: field(event.agent_type, "agent_type", 128),
    hostname: field(hostname(), "hostname", 253, true),
  };
  return {
    hookSpecificOutput: {
      hookEventName: "PreToolUse",
      permissionDecision: "allow",
      updatedInput: { ...event.tool_input, client_context: context },
    },
  };
}

async function main() {
  try {
    const chunks = [];
    let length = 0;
    for await (const chunk of process.stdin) {
      length += chunk.length;
      if (length > 1024 * 1024) throw new Error("hook input exceeds 1 MiB");
      chunks.push(chunk);
    }
    let event;
    try { event = JSON.parse(Buffer.concat(chunks).toString("utf8")); }
    catch { throw new Error("invalid hook JSON"); }
    const output = injectDelegation(event, process.argv[2]);
    process.stdout.write(JSON.stringify(output) + "\n");
  } catch (error) {
    process.stdout.write(JSON.stringify({
      hookSpecificOutput: {
        hookEventName: "PreToolUse",
        permissionDecision: "deny",
        permissionDecisionReason: `InfTask delegation context unavailable: ${error.message}. Fix the repository hook; do not invent session metadata.`,
      },
    }) + "\n");
  }
}

if (process.argv[1] && import.meta.url === pathToFileURL(realpathSync(process.argv[1])).href) await main();
