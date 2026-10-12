import { IssueDetails } from "../shared";

export const fixtureIssue: IssueDetails = {
  assignee: null, assignments: [], delegations: [], identifier: "AL-1", title: "Fixture issue", description: "Test description", status: "todo", priority: 0,
  type: "bug", project: null, project_id: null, project_emoji: "", parent: null, labels: [],
  created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z",
  comments: [], links: [], pull_requests: [], children: [], blocks: [], blocked_by: [], related: [],
};
export function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
export const json = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { "Content-Type": "application/json" } });
