import { KeyboardEvent, MouseEvent, ReactNode, useEffect, useLayoutEffect, useRef, useState } from "react";
import { Box, Check, UserRound } from "lucide-react";
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

export type Identity = { subject: string; name: string; email: string };
export const personName = (person: Identity) => person.name || person.email || person.subject;

export type IssueSummary = {
  assignee: Identity | null;
  labels: Label[];
  identifier: string;
  title: string;
  status: Status;
  priority: Priority;
  type: string | null;
  project: string | null;
  project_id: number | null;
  parent: string | null;
  created_at: string;
  updated_at: string;
  project_emoji: string;
};

export type Issue = IssueSummary & { description: string };

// Historical team keys may contain hyphens; only the final segment is a number.
export const issueTeamKey = (identifier: string) => identifier.slice(0, identifier.lastIndexOf("-"));

export type Comment = { body: string; created_at: string; author: string };
export type Link = { title: string; url: string };
export type PullRequest = { repo: string; number: number; title: string; url: string; state: "open" | "draft" | "merged" | "closed"; updated_at: string };

export type Delegation = {
  id: number;
  agent: "codex" | "claude";
  session_id: string;
  agent_id?: string;
  agent_type?: string;
  hostname: string;
  actor: Identity | null;
  created_at: string;
};
export const codingAgentName = (agent: Delegation["agent"]) => agent === "codex" ? "Codex" : "Claude";

export type IssueDetails = Issue & {
  delegations: Delegation[];
  assignments: { actor: Identity | null; from: Identity | null; to: Identity | null; created_at: string }[];
  comments: Comment[];
  links: Link[];
  pull_requests: PullRequest[];
  children: IssueSummary[];
  blocks: IssueSummary[];
  blocked_by: IssueSummary[];
  related: IssueSummary[];
};

export type Project = { id: number; teams: string[]; name: string; description: string; archived: boolean; emoji: string };

export type Team = { private: boolean; joined: boolean; accessible: boolean; can_manage: boolean; membership_enabled: boolean; key: string; name: string; next_number: number; emoji: string };

export type Workspace = { slug: string; name: string };

// Pages live under /<workspace slug>/ and the workspace's API under
// /api/w/<workspace slug>/; the slug is the first segment of the page path.
export function workspaceSlug() {
  return window.location.pathname.split("/")[1];
}

export type WorkspaceRoute =
  | { kind: "issues"; team: string }
  | { kind: "projects"; team: string }
  | { kind: "project"; id: number }
  | { kind: "issue"; identifier: string }
  | { kind: "labels" }
  | { kind: "settings" }
  | { kind: "invalid" };

export function workspaceRoute(path: string): WorkspaceRoute {
  const route = "/" + path.split("/").slice(2).join("/");
  if (route === "/") return { kind: "issues", team: "" };
  if (route === "/projects") return { kind: "projects", team: "" };
  if (route === "/settings") return { kind: "settings" };
  if (route === "/labels") return { kind: "labels" };
  const team = route.match(/^\/team\/([^/]+)\/(issues|projects)$/);
  if (team) {
    try { return { kind: team[2] as "issues" | "projects", team: decodeURIComponent(team[1]) }; }
    catch { return { kind: "invalid" }; }
  }
  const project = route.match(/^\/project\/([1-9][0-9]*)$/);
  if (project && Number.isSafeInteger(Number(project[1]))) return { kind: "project", id: Number(project[1]) };
  const issue = route.match(/^\/issue\/(.+)$/);
  if (issue) return { kind: "issue", identifier: issue[1] };
  return { kind: "invalid" };
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

// Closed menus have no option tree. Typeahead on a closed trigger uses the
// small data catalog; Radix handles navigation once the menu is open.
function useLazySelect(value: string, options: { value: string; label: string }[], onChange: (value: string) => void) {
  const [open, setOpen] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null);
  const [container, setContainer] = useState<HTMLDialogElement | null>(null);
  const search = useRef({ text: "", time: 0 });
  useLayoutEffect(() => { setContainer(trigger.current!.closest("dialog")); }, []);
  function onKeyDown(event: KeyboardEvent<HTMLButtonElement>) {
    if (open || event.ctrlKey || event.altKey || event.metaKey || event.key.length !== 1) return;
    const now = Date.now();
    const previous = now - search.current.time < 1000 ? search.current.text : "";
    if (event.key === " " && !previous) return;
    const text = previous + event.key.toLowerCase();
    search.current = { text, time: now };
    const repeated = [...text].every((char) => char === text[0]);
    const prefix = repeated ? text[0] : text;
    const index = options.findIndex((option) => option.value === value);
    const ordered = [...options.slice(index), ...options.slice(0, index)];
    const next = ordered.find((option) => (!repeated || option.value !== value) && option.label.toLowerCase().startsWith(prefix));
    if (next && next.value !== value) onChange(next.value);
    event.preventDefault();
  }
  function changeOpen(next: boolean) {
    search.current = { text: "", time: 0 };
    setOpen(next);
  }
  return { open, setOpen: changeOpen, trigger, container, onKeyDown };
}

export function StatusSelect({ issue, onChange }: { issue: { identifier: string; status: Status }; onChange: (status: Status) => void }) {
  const { open, setOpen, trigger, container, onKeyDown } = useLazySelect(issue.status, columns.map((option) => ({ value: option.status, label: option.label })), (value) => onChange(value as Status));
  const selected = columns.find((column) => column.status === issue.status)!;
  return <Select.Root open={open} onOpenChange={setOpen} value={issue.status} onValueChange={(value) => onChange(value as Status)}>
    <Select.Trigger ref={trigger} onKeyDown={onKeyDown} className="status-control" aria-label={`Change status for ${issue.identifier}`} title={selected.label}>
      <StatusIcon status={issue.status} />
      <span className="status-select">{selected.label}</span>
    </Select.Trigger>
    {open && <Select.Portal container={container || undefined}>
      <Select.Content className="priority-menu status-menu" position="popper" sideOffset={5} align="start" collisionPadding={8}>
        <Select.Viewport>
          {columns.map((option) => <Select.Item className="priority-option" value={option.status} key={option.status} textValue={option.label}>
            <span className="priority-check"><Select.ItemIndicator><Check className="icon" /></Select.ItemIndicator></span>
            <StatusIcon status={option.status} />
            <Select.ItemText>{option.label}</Select.ItemText>
          </Select.Item>)}
        </Select.Viewport>
      </Select.Content>
    </Select.Portal>}
  </Select.Root>;
}

export function AssigneeBadge({ person }: { person: Identity | null }) {
  return <span className="assignee-badge" title={person ? `Assignee: ${personName(person)}` : "Unassigned"}><UserRound className="icon" /><span>{person ? personName(person) : "Unassigned"}</span></span>;
}

export function IssueMeta({ issue }: { issue: IssueSummary }) {
  return (
    <span className="issue-meta">
      <AssigneeBadge person={issue.assignee} />
      {issue.labels.slice(0, 2).map((label) => <LabelBadge key={label.id} label={label} />)}
      {issue.labels.length > 2 && <span className="label-overflow" title={issue.labels.slice(2).map((l) => l.name).join(", ")}>+{issue.labels.length - 2}</span>}
      {issue.type && <span className="type-label" data-type={issue.type} title={issue.type}><span className="label-dot" />{issue.type}</span>}
      {issue.project && <span className="project-label" title={issue.project}>{issue.project_emoji ? <span className="project-emoji" aria-hidden="true">{issue.project_emoji}</span> : <Box className="icon" />}<span>{issue.project}</span></span>}
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
  }, [restoreFocus]);
  return <dialog ref={ref} {...attributes} onCancel={(event) => { event.preventDefault(); onClose(); }}>{children}</dialog>;
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

export function PrioritySelect({ priority, onChange, label, compact = false, disabled = false, emptyLabel }: { priority: Priority; onChange: (priority: Priority) => void; label: string; compact?: boolean; disabled?: boolean; emptyLabel?: string }) {
  const { open, setOpen, trigger, container, onKeyDown } = useLazySelect(String(priority), priorities.map((option) => ({ value: String(option.value), label: option.label })), (value) => onChange(Number(value) as Priority));
  const selected = priorities[priority];
  return <Select.Root open={open} onOpenChange={setOpen} value={String(priority)} onValueChange={(value) => onChange(Number(value) as Priority)} disabled={disabled}>
    <Select.Trigger ref={trigger} onKeyDown={onKeyDown} className={`priority-control ${compact ? "priority-compact" : ""}`} aria-label={label} title={selected.label}>
      <PriorityIcon priority={priority} />
      {!compact && (priority === 0 && emptyLabel ? <span className="none">{emptyLabel}</span> : <span>{selected.label}</span>)}
    </Select.Trigger>
    {open && <Select.Portal container={container || undefined}>
      <Select.Content className="priority-menu" position="popper" sideOffset={5} align="start" collisionPadding={8}>
        <Select.Viewport>
          {priorities.map((option) => <Select.Item className="priority-option" value={String(option.value)} key={option.value} textValue={option.label}>
            <span className="priority-check"><Select.ItemIndicator><Check className="icon" /></Select.ItemIndicator></span>
            <PriorityIcon priority={option.value} />
            <Select.ItemText>{option.label}</Select.ItemText>
          </Select.Item>)}
        </Select.Viewport>
      </Select.Content>
    </Select.Portal>}
  </Select.Root>;
}
