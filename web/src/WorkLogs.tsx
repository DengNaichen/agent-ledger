import { useCallback, useEffect, useRef, useState } from "react";
import { ChevronDown, GitBranch, MessageSquare, ScrollText } from "lucide-react";
import { api, personName, RequestError, WorkLog, WorkLogPage, WorkLogSummary } from "./shared";
import { MarkdownText } from "./MarkdownText";
import { useResource } from "./useResource";
import { useRequestScope } from "./useRequestScope";

const timestamp = (value: string) => new Date(value).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
type ActivityTab = "comments" | "work-logs";

export function ActivityTabs({ value, onChange, comments, workLogs }: { value: ActivityTab; onChange: (value: ActivityTab) => void; comments: number; workLogs: number }) {
  const buttons = useRef<(HTMLButtonElement | null)[]>([]);
  const tabs = [
    { id: "comments" as const, name: "Comments", count: comments, Icon: MessageSquare },
    { id: "work-logs" as const, name: "Agent work logs", count: workLogs, Icon: ScrollText },
  ];
  return <div className="activity-tabs" role="tablist" aria-label="Issue activity">{tabs.map(({ id, name, count, Icon }, index) => <button
    key={id} ref={(node) => { buttons.current[index] = node; }} type="button" role="tab" id={`tab-${id}`} aria-controls={`activity-${id}`} aria-selected={value === id} tabIndex={value === id ? 0 : -1}
    onClick={() => onChange(id)} onKeyDown={(event) => {
      let next: number;
      if (event.key === "ArrowLeft" || event.key === "ArrowRight") next = 1 - index;
      else if (event.key === "Home") next = 0;
      else if (event.key === "End") next = 1;
      else return;
      event.preventDefault(); onChange(tabs[next].id); buttons.current[next]!.focus();
    }}><Icon className="icon" /><span>{name}</span><span className="activity-count">{count}</span></button>)}</div>;
}

export function WorkLogEntry({ entry, path }: { entry: WorkLogSummary; path: string }) {
  const [open, setOpen] = useState(false);
  return <article className={`work-log work-log-${entry.kind}`}>
    <button className="work-log-heading" type="button" aria-expanded={open} onClick={() => setOpen(!open)}>
      {entry.kind === "decision" ? <GitBranch className="icon" /> : <ScrollText className="icon" />}
      <span>{entry.summary}</span><ChevronDown className="icon work-log-chevron" />
    </button>
    <div className="work-log-meta"><span>{entry.kind === "decision" ? "Decision" : "Progress"}</span><span>#{entry.id}</span><time dateTime={entry.created_at}>{timestamp(entry.created_at)}</time>{entry.actor && <span>Recorded by {personName(entry.actor)}</span>}</div>
    {open && <WorkLogBody path={`${path}/work-logs/${entry.id}`} />}
  </article>;
}

function WorkLogBody({ path }: { path: string }) {
  const record = useResource<WorkLog | null>(path, null);
  return <div className="work-log-body">
    {record.error ? <><RequestError message={record.error} /><button className="button" onClick={() => void record.reload()}>Retry record</button></>
      : record.data ? <MarkdownText text={record.data.body} /> : <p role="status">Loading record…</p>}
  </div>;
}

export function WorkLogList({ path }: { path: string }) {
  const [page, setPage] = useState<WorkLogPage>({ work_logs: [], next_before: 0 });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [retryBefore, setRetryBefore] = useState(0);
  const requests = useRequestScope(path);
  const load = useCallback(async (before = 0) => {
    setLoading(true); setError("");
    await requests.read(() => api<WorkLogPage>(`${path}/work-logs?before=${before}`), (next) => {
      setPage((previous) => ({ work_logs: before ? [...previous.work_logs, ...next.work_logs] : next.work_logs, next_before: next.next_before }));
      setLoading(false);
    }, (cause) => {
      setError(cause.message); setRetryBefore(before); setLoading(false);
      if (cause.message === "issue not found" || cause.message === "workspace access denied") setPage({ work_logs: [], next_before: 0 });
    });
  }, [path, requests]);
  useEffect(() => {
    void load();
    const focus = () => { void load(); };
    window.addEventListener("focus", focus);
    return () => window.removeEventListener("focus", focus);
  }, [load]);
  return <div className="work-log-list">
    <p className="activity-hint">Agent progress, findings and verification. Newest first. Expand a record for details.</p>
    {page.work_logs.map((entry) => <WorkLogEntry key={entry.id} entry={entry} path={path} />)}
    {loading && <p className="activity-hint" role="status">Loading work logs…</p>}
    {error && <div className="work-log-error"><RequestError message={error} /><button className="button" disabled={loading} onClick={() => void load(retryBefore)}>Retry work logs</button></div>}
    {!loading && !error && page.work_logs.length === 0 && <p className="empty">No agent work logs yet.</p>}
    {!error && page.next_before > 0 && <button className="button" disabled={loading} onClick={() => void load(page.next_before)}>Load older records</button>}
  </div>;
}
