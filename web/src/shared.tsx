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

export type Label = { id: number; name: string; color: string; description: string; group_id: number; archived: boolean };
export type LabelGroup = { id: number; name: string };

export type Issue = {
  labels: Label[];
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
  labels: Label[];
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
export type PullRequest = { repo: string; number: number; title: string; url: string; state: "open" | "draft" | "merged" | "closed"; updated_at: string };

export type IssueDetails = Issue & {
  comments: Comment[];
  links: Link[];
  pull_requests: PullRequest[];
  children: IssueSummary[];
  blocks: IssueSummary[];
  blocked_by: IssueSummary[];
  related: IssueSummary[];
};

export type Project = { name: string; description: string; archived: boolean };

export async function api<T>(path: string, method = "GET", body?: object): Promise<T> {
  const response = await fetch(path, { method, headers: { "Content-Type": "application/json" }, body: JSON.stringify(body) });
  if (response.status === 401) window.dispatchEvent(new Event("session-expired"));
  if (response.ok) window.dispatchEvent(new Event("session-active"));
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

const statusColors: Record<Status, string> = {
  triage: "#bec2c8",
  todo: "#e2e2e2",
  agent_ready: "#b49bdd",
  in_progress: "#f2c94c",
  in_review: "#4cb782",
  done: "#5e6ad2",
  canceled: "#95a2b3",
};

// Open states are rings, progress fills a pie inside the ring, and closed
// states are solid discs with a mark drawn in the panel color.
const ring = <circle cx="7" cy="7" r="6" fill="none" stroke="currentColor" strokeWidth="1.5" />;
const disc = <circle cx="7" cy="7" r="7" fill="currentColor" />;
const statusGlyphs: Record<Status, ReactNode> = {
  triage: <circle cx="7" cy="7" r="6" fill="none" stroke="currentColor" strokeWidth="1.5" pathLength="24" strokeDasharray="1.5 1.5" />,
  todo: ring,
  agent_ready: <>{ring}<circle cx="7" cy="7" r="2" fill="currentColor" /></>,
  in_progress: <>{ring}<path d="M7 7V3.5a3.5 3.5 0 0 1 0 7z" fill="currentColor" /></>,
  in_review: <>{ring}<path d="M7 7V3.5a3.5 3.5 0 1 1-3.5 3.5z" fill="currentColor" /></>,
  done: <>{disc}<path className="status-mark" d="M4.25 7.25l1.75 1.75 3.75-4" /></>,
  canceled: <>{disc}<path className="status-mark" d="M5 5l4 4M9 5l-4 4" /></>,
};

export function StatusIcon({ status }: { status: Status }) {
  return (
    <svg className="status-icon" width="14" height="14" viewBox="0 0 14 14" color={statusColors[status]} aria-hidden="true">
      {statusGlyphs[status]}
    </svg>
  );
}

export function StatusSelect({ issue, onChange }: { issue: { identifier: string; status: Status }; onChange: (status: Status) => void }) {
  return (
    <div className="status-control">
      <StatusIcon status={issue.status} />
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

export function IssueMeta({ issue }: { issue: { type: string | null; project: string | null; labels: Label[] } }) {
  return (
    <span className="issue-meta">
      {issue.labels.slice(0, 2).map((label) => <LabelBadge key={label.id} label={label} />)}
      {issue.labels.length > 2 && <span className="label-overflow" title={issue.labels.slice(2).map((l) => l.name).join(", ")}>+{issue.labels.length - 2}</span>}
      {issue.type && <span className="type-label">{issue.type}</span>}
      {issue.project && <span className="project-label">{issue.project}</span>}
    </span>
  );
}

export function LabelBadge({ label }: { label: Label }) {
 return <span className="label-badge" title={`${label.name}${label.archived ? " (archived)" : ""}${label.description ? ` — ${label.description}` : ""}`}><span className="label-dot" style={{ backgroundColor: label.color }} /><span>{label.name}</span>{label.archived && <small>archived</small>}</span>;
}

export function RequestError({ message }: { message: string }) {
  return <p className="error" role="alert">{message === "unauthorized" ? <>Your session expired. <a href="/auth/login" target="_blank" rel="noopener noreferrer">Sign in again</a>, then retry. Your text stays here.</> : message}</p>;
}
