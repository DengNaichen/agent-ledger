import { MouseEvent, ReactNode, useEffect, useLayoutEffect, useRef, useState } from "react";
import { Check } from "lucide-react";
import * as Select from "@radix-ui/react-select";

export const columns = [
  { status: "triage", label: "Triage", hint: "Worth keeping?" },
  { status: "backlog", label: "Backlog", hint: "Not planned yet" },
  { status: "todo", label: "Todo", hint: "Planned work" },
  { status: "agent_ready", label: "Agent ready", hint: "Clear enough to start" },
  { status: "in_progress", label: "In progress", hint: "One active commitment" },
  { status: "in_review", label: "In review", hint: "Prove it works" },
  { status: "done", label: "Done", hint: "Merged and verified" },
  { status: "canceled", label: "Canceled", hint: "No longer planned" },
] as const;

export const priorities = [
  { value: 0, label: "No priority" },
  { value: 1, label: "Urgent" },
  { value: 2, label: "High" },
  { value: 3, label: "Medium" },
  { value: 4, label: "Low" },
] as const;
export type Priority = (typeof priorities)[number]["value"];
export type IssueSort = "auto" | "updated" | "priority";

export const issueTypes = ["spec", "feature", "bug", "improvement", "chore"];

export type Status = (typeof columns)[number]["status"];

export type Label = { id: number; name: string; color: string; description: string; group_id: number; archived: boolean };
export type LabelGroup = { id: number; name: string };

export type IssueSummary = {
  labels: Label[];
  identifier: string;
  title: string;
  status: Status;
  priority: Priority;
  type: string | null;
  project: string | null;
  parent: string | null;
  updated_at: string;
};

export type Issue = IssueSummary & { description: string; created_at: string };

export type Comment = { body: string; created_at: string; author: string };
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

export type Project = { name: string; description: string; archived: boolean; emoji: string };

export type Team = { key: string; name: string; next_number: number; emoji: string };

export type Workspace = { slug: string; name: string };

// Pages live under /<workspace slug>/ and the workspace's API under
// /api/w/<workspace slug>/; the slug is the first segment of the page path.
export function workspaceSlug() {
  return window.location.pathname.split("/")[1];
}

export const pagePath = (path: string) => `/${workspaceSlug()}${path}`;

export const apiPath = (path: string) => `/api/w/${workspaceSlug()}${path}`;

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
  triage: "var(--status-triage)",
  backlog: "var(--status-backlog)",
  todo: "var(--status-todo)",
  agent_ready: "var(--status-agent-ready)",
  in_progress: "var(--status-in-progress)",
  in_review: "var(--status-in-review)",
  done: "var(--status-done)",
  canceled: "var(--status-canceled)",
};

// Backlog and planned states are rings; progress fills a pie inside the ring.
// Triage and closed states use solid discs with a mark in the panel color.
const ring = <circle cx="7" cy="7" r="6" fill="none" stroke="currentColor" strokeWidth="1.5" />;
const disc = <circle cx="7" cy="7" r="7" fill="currentColor" />;
const statusGlyphs: Record<Status, ReactNode> = {
  triage: <>{disc}<path className="status-mark" d="M3.5 5h7M8.5 3l2 2-2 2M10.5 9h-7M5.5 7l-2 2 2 2" /></>,
  backlog: <circle cx="7" cy="7" r="6" fill="none" stroke="currentColor" strokeWidth="1.5" pathLength="24" strokeDasharray="1.5 1.5" />,
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

// ModalDialog shows a native modal dialog while mounted and focuses its first
// text field or select. restoreFocus returns focus to the element that had it
// when the dialog opened.
export function ModalDialog({ restoreFocus = false, onClose, children, ...attributes }: { className: string; "aria-label"?: string; "aria-labelledby"?: string; restoreFocus?: boolean; onClose: () => void; children: ReactNode }) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const element = ref.current!;
    const opener = document.activeElement as HTMLElement;
    element.showModal();
    element.querySelector<HTMLElement>('input:not([type="checkbox"]), select, textarea')!.focus();
    return () => {
      element.close();
      if (restoreFocus) opener.focus();
    };
  }, []);
  return <dialog ref={ref} {...attributes} onCancel={onClose}>{children}</dialog>;
}

export function RequestError({ message }: { message: string }) {
  return <p className="error" role="alert">{message === "unauthorized" ? <>Your session expired. <a href="/auth/login" target="_blank" rel="noopener noreferrer">Sign in again</a>, then retry. Your text stays here.</> : message}</p>;
}

// The three bars retain their silhouette at every level; inactive bars are dimmed.
export function PriorityIcon({ priority }: { priority: Priority }) {
  return <svg className={`priority-icon priority-${priority}`} width="16" height="16" viewBox="0 0 16 16" aria-hidden="true">
    {priority === 0 ? <path d="M2 8h2M7 8h2M12 8h2" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" /> : priority === 1 ? <><rect x="1" y="1" width="14" height="14" rx="3" fill="currentColor" /><path d="M8 4v5" className="priority-mark" /><circle cx="8" cy="11.5" r=".8" className="priority-mark-dot" /></> : [0, 1, 2].map((bar) => <rect key={bar} x={2 + bar * 4.5} y={9 - bar * 3} width="3" height={5 + bar * 3} rx="1" fill="currentColor" className={bar < 5 - priority ? undefined : "priority-bar-muted"} />)}
  </svg>;
}

export function PrioritySelect({ priority, onChange, label, compact = false, disabled = false }: { priority: Priority; onChange: (priority: Priority) => void; label: string; compact?: boolean; disabled?: boolean }) {
  const trigger = useRef<HTMLButtonElement>(null);
  const [container, setContainer] = useState<HTMLDialogElement | null>(null);
  // A portal inside a native modal must stay inside that dialog's top layer.
  useLayoutEffect(() => { setContainer(trigger.current!.closest("dialog")); }, []);
  const selected = priorities[priority];
  return <Select.Root value={String(priority)} onValueChange={(value) => onChange(Number(value) as Priority)} disabled={disabled}>
    <Select.Trigger ref={trigger} className={`priority-control ${compact ? "priority-compact" : ""}`} aria-label={label} title={selected.label}>
      <PriorityIcon priority={priority} />
      {!compact && <span>{selected.label}</span>}
    </Select.Trigger>
    <Select.Portal container={container || undefined}>
      <Select.Content className="priority-menu" position="popper" sideOffset={5} align="start" collisionPadding={8}>
        <Select.Viewport>
          {priorities.map((option) => <Select.Item className="priority-option" value={String(option.value)} key={option.value} textValue={option.label}>
            <span className="priority-check"><Select.ItemIndicator><Check className="icon" /></Select.ItemIndicator></span>
            <PriorityIcon priority={option.value} />
            <Select.ItemText>{option.label}</Select.ItemText>
          </Select.Item>)}
        </Select.Viewport>
      </Select.Content>
    </Select.Portal>
  </Select.Root>;
}
