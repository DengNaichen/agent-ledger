import { MouseEvent, ReactNode } from "react";

export const columns = [
  { status: "triage", label: "Triage", hint: "Worth keeping?" },
  { status: "todo", label: "Todo", hint: "Planned work" },
  { status: "agent_ready", label: "Agent ready", hint: "Clear enough to start" },
  { status: "in_progress", label: "In progress", hint: "One active commitment" },
  { status: "in_review", label: "In review", hint: "Prove it works" },
  { status: "done", label: "Done", hint: "Merged and verified" },
  { status: "canceled", label: "Canceled", hint: "No longer planned" },
] as const;

export type Status = (typeof columns)[number]["status"];

export type Issue = {
  identifier: string;
  title: string;
  status: Status;
  type: string | null;
  project: string | null;
  parent: string | null;
  updated_at: string;
  description: string;
  created_at: string;
};

export type IssueSummary = {
  identifier: string;
  title: string;
  status: Status;
  type: string | null;
  project: string | null;
  parent: string | null;
  updated_at: string;
};

export type Comment = { body: string; created_at: string };
export type Link = { title: string; url: string };

export type IssueDetails = Issue & {
  comments: Comment[];
  links: Link[];
  children: IssueSummary[];
  blocks: IssueSummary[];
  blocked_by: IssueSummary[];
  related: IssueSummary[];
};

export type Project = { name: string; description: string; archived: boolean };

export async function api<T>(path: string, method = "GET", body?: object): Promise<T> {
  const response = await fetch(path, { method, headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  const result = await response.json();
  if (!response.ok) throw new Error(result.error);
  return result;
}

export function navigate(path: string) {
  window.history.pushState(null, "", path);
  window.dispatchEvent(new PopStateEvent("popstate"));
}

export function NavLink({ to, className, children }: { to: string; className?: string; children: ReactNode }) {
  function open(event: MouseEvent) {
    if (event.metaKey || event.ctrlKey) return;
    event.preventDefault();
    navigate(to);
  }
  return <a className={className} href={to} onClick={open}>{children}</a>;
}

export function StatusSelect({ issue, onChange }: { issue: { identifier: string; status: Status }; onChange: (status: Status) => void }) {
  return (
    <div className="status-control">
      <span className={`status-icon status-icon-${issue.status}`} aria-hidden="true" />
      <select
        className="status-select"
        value={issue.status}
        aria-label={`Change status for ${issue.identifier}`}
        onChange={(event) => onChange(event.target.value as Status)}
      >
        {columns.map((option) => <option value={option.status} key={option.status}>{option.label}</option>)}
      </select>
    </div>
  );
}

export function IssueMeta({ issue }: { issue: { type: string | null; project: string | null } }) {
  return (
    <span className="issue-meta">
      {issue.type && <span className="type-label">{issue.type}</span>}
      {issue.project && <span className="project-label">{issue.project}</span>}
    </span>
  );
}
