import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { IssueDetail } from "./IssueDetail";
import { fixtureIssue, deferred, json } from "./test/fixtures";

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
