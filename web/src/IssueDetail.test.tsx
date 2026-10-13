import { act, fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { IssueDetail } from "./IssueDetail";
import { fixtureIssue, deferred, json } from "./test/fixtures";

vi.mock("./Assignee", () => ({ AssigneeSelect: ({ value, onChange, disabled, label }: { value: string; onChange: (value: string) => void; disabled: boolean; label: string }) => <select aria-label={label} value={value} disabled={disabled} onChange={(event) => onChange(event.target.value)}><option value="">Unassigned</option><option value="member">Member</option></select> }));

const catalog = { labels: [], groups: [], error: "", reload: async () => {} };
const props = { catalog, teams: [], onSaved: async () => {}, identifier: "AL-1", setError: vi.fn(), editing: false, closeEditor: vi.fn() };
const priorityControl = () => screen.getByRole("combobox", { name: "Change priority for AL-1" });

beforeEach(() => { window.history.replaceState(null, "", "/default/issue/AL-1"); });

describe("issue detail request ordering", () => {
  it("preserves text typed after submitting a comment", async () => {
    const post = deferred<Response>();
    vi.stubGlobal("fetch", vi.fn((_url, options) => options?.method === "POST" ? post.promise : Promise.resolve(json(fixtureIssue))));
    render(<IssueDetail {...props} />);
    const textarea = await screen.findByRole("textbox", { name: "Comment" });
    fireEvent.change(textarea, { target: { value: "Submitted" } });
    fireEvent.click(screen.getByRole("button", { name: "Comment" }));
    fireEvent.change(textarea, { target: { value: "Next draft" } });
    await act(async () => post.resolve(json({ body: "Submitted", created_at: fixtureIssue.created_at, author: "" })));
    expect((textarea as HTMLTextAreaElement).value).toBe("Next draft");
  });

  it("does not replace a saved priority with an older focus read", async () => {
    let current = { ...fixtureIssue };
    const oldRead = deferred<Response>();
    let hold = false;
    vi.stubGlobal("fetch", vi.fn((_url, options) => {
      if (options?.method === "PATCH") { current = { ...current, ...JSON.parse(options.body) }; return Promise.resolve(json(current)); }
      if (hold) { hold = false; return oldRead.promise; }
      return Promise.resolve(json(current));
    }));
    render(<IssueDetail {...props} />);
    await screen.findByRole("textbox", { name: "Comment" });
    hold = true;
    fireEvent(window, new Event("focus"));
    fireEvent.keyDown(priorityControl(), { key: "h" });
    await waitFor(() => expect(priorityControl().title).toBe("High"));
    await act(async () => oldRead.resolve(json(fixtureIssue)));
    expect(priorityControl().title).toBe("High");
  });

  it("coordinates status and priority writes for the same issue", async () => {
    let current = { ...fixtureIssue };
    const status = deferred<Response>();
    vi.stubGlobal("fetch", vi.fn((url, options) => {
      if (options?.method === "PATCH") {
        current = { ...current, ...JSON.parse(options.body) };
        if (String(url).endsWith("/status")) return status.promise;
      }
      return Promise.resolve(json(current));
    }));
    render(<IssueDetail {...props} />);
    await screen.findByRole("textbox", { name: "Comment" });
    fireEvent.keyDown(screen.getByRole("combobox", { name: "Change status for AL-1" }), { key: "d" });
    await waitFor(() => expect(current.status).toBe("done"));
    const statusSnapshot = { ...current };
    fireEvent.keyDown(priorityControl(), { key: "h" });
    await act(async () => { await Promise.resolve(); });
    await act(async () => status.resolve(json(statusSnapshot)));
    await waitFor(() => expect(priorityControl().title).toBe("High"));
    expect(current.priority).toBe(2);
  });
});

it("defers focus refresh until the pending comment is saved", async () => {
  const post = deferred<Response>();
  const added = { body: "Submitted", created_at: fixtureIssue.created_at, author: "" };
  let saved = false;
  const fetcher = vi.fn((_url, options) => options?.method === "POST" ? post.promise : Promise.resolve(json({ ...fixtureIssue, comments: saved ? [added] : [] })));
  vi.stubGlobal("fetch", fetcher);
  render(<IssueDetail {...props} />);
  const textarea = await screen.findByRole("textbox", { name: "Comment" });
  fireEvent.change(textarea, { target: { value: "Submitted" } });
  fireEvent.click(screen.getByRole("button", { name: "Comment" }));
  await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2));
  fireEvent(window, new Event("focus"));
  expect(fetcher).toHaveBeenCalledTimes(2);
  saved = true;
  await act(async () => post.resolve(json(added)));
  await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(3));
  expect(screen.getAllByText("Submitted")).toHaveLength(1);
});

it("retains the draft when posting fails", async () => {
  vi.stubGlobal("fetch", vi.fn((_url, options) => Promise.resolve(options?.method === "POST" ? json({ error: "Offline" }, 500) : json(fixtureIssue))));
  const error = vi.fn();
  render(<IssueDetail {...props} setError={error} />);
  const textarea = await screen.findByRole("textbox", { name: "Comment" });
  fireEvent.change(textarea, { target: { value: "Keep me" } });
  fireEvent.click(screen.getByRole("button", { name: "Comment" }));
  await waitFor(() => expect(error).toHaveBeenCalledWith("Offline"));
  expect((textarea as HTMLTextAreaElement).value).toBe("Keep me");
});

it("does not send queued writes after leaving the detail", async () => {
  const status = deferred<Response>();
  const fetcher = vi.fn((_url, options) => options?.method === "PATCH" ? status.promise : Promise.resolve(json(fixtureIssue)));
  vi.stubGlobal("fetch", fetcher);
  const { unmount } = render(<IssueDetail {...props} />);
  await screen.findByRole("textbox", { name: "Comment" });
  fireEvent.keyDown(screen.getByRole("combobox", { name: "Change status for AL-1" }), { key: "d" });
  await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2));
  fireEvent.keyDown(priorityControl(), { key: "h" });
  unmount();
  await act(async () => status.resolve(json({ ...fixtureIssue, status: "done" })));
  expect(fetcher).toHaveBeenCalledTimes(2);
});


it("shows main and child delegation records without replacing the human assignee", async () => {
  const context = { agent: "codex", session_id: "parent-session", hostname: "test-host", actor: { subject: "member", name: "Member", email: "" }, created_at: fixtureIssue.created_at };
  vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(json({
    ...fixtureIssue, assignee: context.actor,
    delegations: [
      { ...context, id: 1 },
      { ...context, id: 2, agent_id: "child-a", agent_type: "implementer" },
      { ...context, id: 3, agent_id: "child-b", agent_type: "reviewer" },
    ],
  }))));
  render(<IssueDetail {...props} />);
  await screen.findByRole("heading", { name: "Agent executions" });
  const history = screen.getByRole("region", { name: "Agent executions" });
  history.scrollIntoView = vi.fn();
  fireEvent.click(screen.getByRole("button", { name: "Codex · 3 executions" }));
  expect(history.scrollIntoView).toHaveBeenCalledWith({ block: "start" });
  expect(screen.getByRole("heading", { name: "Codex · Session" })).toBeTruthy();
  expect(screen.getByRole("heading", { name: "Codex · Subagent · implementer" })).toBeTruthy();
  expect(screen.getByRole("heading", { name: "Codex · Subagent · reviewer" })).toBeTruthy();
  expect(screen.getAllByText("parent-session")).toHaveLength(3);
  expect(screen.getByText("child-a")).toBeTruthy();
  expect((screen.getByRole("combobox", { name: "Change assignee for AL-1" }) as HTMLSelectElement).value).toBe("member");
});

it("lays out properties as aligned Linear-style rows", async () => {
  const team = { private: false, joined: true, accessible: true, can_manage: true, membership_enabled: true, key: "AL", name: "AL", next_number: 2, emoji: "" };
  vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(json({
    ...fixtureIssue, delegations: [{ id: 1, agent: "codex", session_id: "session", hostname: "host", actor: null, created_at: fixtureIssue.created_at }],
  }))));
  render(<IssueDetail {...props} teams={[team]} />);
  const properties = await screen.findByRole("region", { name: "Properties" });
  expect(properties.textContent).not.toMatch(/Created|Updated|Delegated by|No labels|\(AL\)/);
  expect(within(properties).getByText("Set priority")).toBeTruthy();
  const delegation = within(properties).getByRole("button", { name: "Codex · 1 execution" });
  expect(within(properties).getByText("Assignee").parentElement!.contains(delegation)).toBe(true);
  expect(delegation.querySelector('[data-agent="codex"]')).not.toBeNull();
  expect(within(properties).getByRole("button", { name: "Add label" })).toBeTruthy();
});

it.each(["spec", ""])("hides the type row while preserving labels (type=%s)", async (type) => {
  const label = { id: 1, name: "spec", color: "#6366f1", description: "", group_id: null, archived: false };
  vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(json({ ...fixtureIssue, type, labels: [label] }))));
  const team = { private: false, joined: true, accessible: true, can_manage: true, membership_enabled: true, key: "AL", name: "InfTask", next_number: 2, emoji: "" };
  render(<IssueDetail {...props} teams={[team]} />);
  const properties = await screen.findByRole("region", { name: "Properties" });
  const primary = properties.querySelector(".detail-property-list")!;
  expect(primary.textContent).not.toMatch(/Team|InfTask|Type|No type|spec/);
  expect(Array.from(primary.querySelectorAll("dt"), (term) => term.textContent)).toEqual(["Status", "Priority", "Assignee"]);
  expect(within(properties).getByText("spec")).toBeTruthy();
  expect(within(properties).getByRole("heading", { name: "Labels" })).toBeTruthy();
  expect(within(properties).getByRole("button", { name: "Edit labels" })).toBeTruthy();
});

it("renders delegation metadata as text, not HTML", async () => {
  const text = "<img src=x onerror=alert(1)>";
  vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(json({
    ...fixtureIssue, delegations: [{ id: 1, agent: "claude", session_id: text, hostname: text, actor: null, created_at: fixtureIssue.created_at }],
  }))));
  const { container } = render(<IssueDetail {...props} />);
  await screen.findByRole("heading", { name: "Claude · Session" });
  expect(screen.getAllByText(text)).toHaveLength(2);
  expect(container.querySelector("img")).toBeNull();
});

it("clears previously loaded private content after access is revoked", async () => {
  let revoked = false;
  vi.stubGlobal("fetch", vi.fn(() => Promise.resolve(revoked ? json({ error: "issue not found" }, 404) : json(fixtureIssue))));
  render(<IssueDetail {...props} />);
  await screen.findByText("Fixture issue");
  revoked = true;
  fireEvent(window, new Event("focus"));
  await screen.findByText("issue not found");
  expect(screen.queryByText("Fixture issue")).toBeNull();
  expect(screen.getByRole("button", { name: "Retry" })).toBeTruthy();
});

const decision = { id: 7, kind: "decision", summary: "Preserve historical comments", actor: { subject: "member", name: "Member", email: "" }, created_at: fixtureIssue.created_at };
const progress = { ...decision, id: 8, kind: "progress", summary: "Verified the migration" };

it("loads agent records on demand and preserves the human draft across tabs", async () => {
  const fetcher = vi.fn((url) => {
    const path = String(url);
    if (path.endsWith("/work-logs/7")) return Promise.resolve(json({ ...decision, body: "We cannot reliably infer historical authors." }));
    if (path.includes("/work-logs?")) return Promise.resolve(json({ work_logs: [progress, decision], next_before: 0 }));
    return Promise.resolve(json({ ...fixtureIssue, work_log_count: 2, decisions: [decision] }));
  });
  vi.stubGlobal("fetch", fetcher);
  render(<IssueDetail {...props} />);
  const textarea = await screen.findByRole("textbox", { name: "Comment" });
  expect(fetcher).toHaveBeenCalledTimes(1);
  expect(screen.queryByText("Verified the migration")).toBeNull();
  fireEvent.change(textarea, { target: { value: "Human draft" } });
  fireEvent.click(screen.getByRole("button", { name: decision.summary }));
  await screen.findByText("We cannot reliably infer historical authors.");
  expect(fetcher).toHaveBeenCalledTimes(2);
  fireEvent.click(screen.getByRole("tab", { name: /Agent work logs/ }));
  await screen.findByText(progress.summary);
  expect(screen.queryByRole("textbox", { name: "Comment" })).toBeNull();
  fireEvent.keyDown(screen.getByRole("tab", { name: /Agent work logs/ }), { key: "ArrowLeft" });
  expect(screen.getByRole("tab", { name: /Comments/ }).getAttribute("aria-selected")).toBe("true");
  expect((screen.getByRole("textbox", { name: "Comment" }) as HTMLTextAreaElement).value).toBe("Human draft");
});

it("retries a failed log page, loads older summaries and retries a failed body", async () => {
  let failPage = true;
  let failBody = true;
  vi.stubGlobal("fetch", vi.fn((url) => {
    const path = String(url);
    if (path.endsWith("/work-logs/7")) return Promise.resolve(failBody ? json({ error: "Record offline" }, 503) : json({ ...decision, body: "Decision reasons" }));
    if (path.includes("/work-logs?")) {
      if (failPage) return Promise.resolve(json({ error: "Logs offline" }, 503));
      return Promise.resolve(json(path.endsWith("before=8") ? { work_logs: [decision], next_before: 0 } : { work_logs: [progress], next_before: 8 }));
    }
    return Promise.resolve(json({ ...fixtureIssue, work_log_count: 2 }));
  }));
  render(<IssueDetail {...props} />);
  fireEvent.click(await screen.findByRole("tab", { name: /Agent work logs/ }));
  await screen.findByText("Logs offline");
  failPage = false;
  fireEvent.click(screen.getByRole("button", { name: "Retry work logs" }));
  fireEvent.click(await screen.findByRole("button", { name: "Load older records" }));
  fireEvent.click(await screen.findByRole("button", { name: decision.summary }));
  await screen.findByText("Record offline");
  failBody = false;
  fireEvent.click(screen.getByRole("button", { name: "Retry record" }));
  await screen.findByText("Decision reasons");
  expect(screen.getAllByText(progress.summary)).toHaveLength(1);
  expect(screen.queryByRole("button", { name: "Load older records" })).toBeNull();
});

it("ignores a log response after leaving the tab and reloads on return", async () => {
  const delayed = deferred<Response>();
  let first = true;
  vi.stubGlobal("fetch", vi.fn((url) => {
    if (String(url).includes("/work-logs?")) {
      if (first) { first = false; return delayed.promise; }
      return Promise.resolve(json({ work_logs: [progress], next_before: 0 }));
    }
    return Promise.resolve(json({ ...fixtureIssue, work_log_count: 1 }));
  }));
  render(<IssueDetail {...props} />);
  fireEvent.click(await screen.findByRole("tab", { name: /Agent work logs/ }));
  fireEvent.click(screen.getByRole("tab", { name: /Comments/ }));
  await act(async () => delayed.resolve(json({ work_logs: [decision], next_before: 0 })));
  fireEvent.click(screen.getByRole("tab", { name: /Agent work logs/ }));
  await screen.findByText(progress.summary);
  expect(screen.queryByText(decision.summary)).toBeNull();
});
