import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { useIssues, IssueQuery } from "./useIssues";
import { emptyLabelFilter } from "./useLabels";
import { deferred, fixtureIssue, json } from "./test/fixtures";

const query: IssueQuery = { team: "", type: "", query: "", priority: "", labelFilter: emptyLabelFilter };
beforeEach(() => { window.history.replaceState(null, "", "/default/"); });

it("keeps the current query when an earlier write completes", async () => {
  const patch = deferred<Response>();
  const other = { ...fixtureIssue, identifier: "AL-2", title: "Other result" };
  vi.stubGlobal("fetch", vi.fn((url, options) => {
    if (options?.method === "PATCH") return patch.promise;
    return Promise.resolve(json({ issues: String(url).includes("query=other") ? [other] : [fixtureIssue] }));
  }));
  const { result, rerender } = renderHook((value) => useIssues(value, vi.fn()), { initialProps: query });
  await waitFor(() => expect(result.current.issues).toHaveLength(1));
  let operation: Promise<void>;
  act(() => { operation = result.current.changePriority("AL-1", 2); });
  rerender({ ...query, query: "other" });
  await act(async () => { patch.resolve(json({ ...fixtureIssue, priority: 2 })); await operation; });
  await waitFor(() => expect(result.current.issues.map((issue) => issue.identifier)).toEqual(["AL-2"]));
});

it("rolls back only status and retains a successful priority change", async () => {
  let current = { ...fixtureIssue };
  const status = deferred<Response>();
  const error = vi.fn();
  vi.stubGlobal("fetch", vi.fn((url, options) => {
    if (String(url).endsWith("/status")) return status.promise;
    if (options?.method === "PATCH") current = { ...current, ...JSON.parse(options.body) };
    return Promise.resolve(json(options?.method === "PATCH" ? current : { issues: [current] }));
  }));
  const { result } = renderHook(() => useIssues(query, error));
  await waitFor(() => expect(result.current.issues).toHaveLength(1));
  let statusSave: Promise<void>, prioritySave: Promise<void>;
  act(() => { statusSave = result.current.moveIssue("AL-1", "done"); });
  act(() => { prioritySave = result.current.changePriority("AL-1", 2); });
  await act(async () => { status.resolve(json({ error: "Could not save status" }, 500)); await Promise.all([statusSave, prioritySave]); });
  expect(result.current.issues[0]).toMatchObject({ status: "todo", priority: 2 });
  expect(error).toHaveBeenCalledWith("Could not save status");
});

it("restores confirmed status when consecutive moves and the refresh fail", async () => {
  const first = deferred<Response>();
  let writes = 0;
  vi.stubGlobal("fetch", vi.fn((_url, options) => {
    if (options?.method === "PATCH") return ++writes === 1 ? first.promise : Promise.resolve(json({ error: "Offline" }, 500));
    return Promise.resolve(writes ? json({ error: "Offline" }, 500) : json({ issues: [fixtureIssue] }));
  }));
  const { result } = renderHook(() => useIssues(query, vi.fn()));
  await waitFor(() => expect(result.current.issues).toHaveLength(1));
  let one: Promise<void>, two: Promise<void>;
  act(() => { one = result.current.moveIssue("AL-1", "in_progress"); });
  act(() => { two = result.current.moveIssue("AL-1", "done"); });
  await act(async () => { first.resolve(json({ error: "Offline" }, 500)); await Promise.all([one, two]); });
  expect(result.current.issues[0].status).toBe("todo");
});

it("ignores an older search response after changing queries", async () => {
  const old = deferred<Response>();
  vi.stubGlobal("fetch", vi.fn((url) => String(url).includes("query=new") ? Promise.resolve(json({ issues: [{ ...fixtureIssue, title: "New result" }] })) : old.promise));
  const { result, rerender } = renderHook((value) => useIssues(value, vi.fn()), { initialProps: query });
  rerender({ ...query, query: "new" });
  await waitFor(() => expect(result.current.issues[0]?.title).toBe("New result"));
  await act(async () => old.resolve(json({ issues: [fixtureIssue] })));
  expect(result.current.issues[0].title).toBe("New result");
});

it("serializes batch label changes with writes to any selected issue", async () => {
  let current = { ...fixtureIssue };
  const status = deferred<Response>();
  const label = { id: 1, name: "Fixture label", group_id: 0, color: "#123456", description: "", archived: false };
  const fetcher = vi.fn((url, options) => {
    if (String(url).endsWith("/status")) { current = { ...current, status: "done" }; return status.promise; }
    return Promise.resolve(json({ issues: [current] }));
  });
  vi.stubGlobal("fetch", fetcher);
  const { result } = renderHook(() => useIssues(query, vi.fn()));
  await waitFor(() => expect(result.current.issues).toHaveLength(1));
  let one: Promise<void>, two: Promise<boolean>;
  act(() => { one = result.current.moveIssue("AL-1", "done"); });
  await waitFor(() => expect(current.status).toBe("done"));
  const batch = vi.fn(async () => { current = { ...current, labels: [label] }; });
  act(() => { two = result.current.writeLabels(["AL-1", "AL-2"], batch, () => { void result.current.loadIssues(); }, vi.fn()); });
  expect(batch).not.toHaveBeenCalled();
  await act(async () => { status.resolve(json(current)); await Promise.all([one, two]); });
  await waitFor(() => expect(result.current.issues[0].labels).toEqual([label]));
  expect(result.current.issues[0].status).toBe("done");
});
