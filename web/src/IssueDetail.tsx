import { X } from "lucide-react";
import { FormEvent, useEffect, useRef, useState } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { RequestError, api, columns, Comment, Issue, IssueDetails, IssueSummary, NavLink, Status, StatusIcon, StatusSelect, Priority, PriorityIcon, PrioritySelect } from "./shared";

import { Catalog, LabelPicker } from "./Labels";
import { LabelBadge } from "./shared";

const remarkPlugins = [remarkGfm];

const timestamp = (value: string) => new Date(value).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });

export function IssueDetail({ catalog, identifier, setError, editing, closeEditor }: { catalog: Catalog; identifier: string; setError: (message: string) => void; editing: boolean; closeEditor: () => void }) {
  const [issue, setIssue] = useState<IssueDetails | null>(null);
  const [comment, setComment] = useState("");
  const [labelsOpen, setLabelsOpen] = useState(false);
  const [savingPriority, setSavingPriority] = useState(false);
  const [posting, setPosting] = useState(false);

  useEffect(() => {
    const load = () => api<IssueDetails>(`/api/issues/${identifier}`).then(setIssue, (cause: Error) => setError(cause.message));
    load();
    window.addEventListener("focus", load);
    return () => window.removeEventListener("focus", load);
  }, [identifier]);

  async function changeStatus(status: Status) {
    try {
      const updated = await api<Issue>(`/api/issues/${identifier}/status`, "PATCH", { status });
      setIssue((current) => ({ ...current!, ...updated }));
    } catch (cause) {
      setError((cause as Error).message);
    }
  }

  async function changePriority(priority: Priority) {
    setSavingPriority(true);
    setError("");
    try {
      const updated = await api<Issue>(`/api/issues/${identifier}`, "PATCH", { priority });
      setIssue((current) => ({ ...current!, ...updated }));
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setSavingPriority(false);
    }
  }

  async function postComment(event: FormEvent) {
    event.preventDefault();
    setPosting(true);
    setError("");
    try {
      const added = await api<Comment>(`/api/issues/${identifier}/comments`, "POST", { body: comment });
      setIssue((current) => ({ ...current!, comments: [...current!.comments, added] }));
      setComment("");
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setPosting(false);
    }
  }

  if (!issue) return <p className="empty">Loading…</p>;

  const relations = [
    { label: "Blocks", issues: issue.blocks },
    { label: "Blocked by", issues: issue.blocked_by },
    { label: "Related", issues: issue.related },
  ].filter((group) => group.issues.length > 0);

  const properties = (
    <dl className="properties">
      <dt>Status</dt>
      <dd><StatusSelect issue={issue} onChange={(status) => void changeStatus(status)} /></dd>
      <dt>Priority</dt>
      <dd><PrioritySelect priority={issue.priority} label={`Change priority for ${identifier}`} disabled={savingPriority} onChange={(next) => void changePriority(next)} /></dd>
      <dt>Type</dt>
      <dd>{issue.type ? <span className="type-label">{issue.type}</span> : <span className="none">None</span>}</dd>
      <dt>Project</dt>
      <dd>{issue.project ?? <span className="none">None</span>}</dd>
      <dt>Parent</dt>
      <dd>{issue.parent ? <NavLink to={`/issue/${issue.parent}`}>{issue.parent}</NavLink> : <span className="none">None</span>}</dd>
      <dt>Created</dt>
      <dd>{timestamp(issue.created_at)}</dd>
      <dt>Updated</dt>
      <dd>{timestamp(issue.updated_at)}</dd>
    </dl>
  );

  return (
    <div className="detail">
      <article className="detail-main">
        <div className="detail-content">
          <span className="identifier">{issue.identifier}</span>
          <h1>{issue.title}</h1>
          <div className="detail-inline-properties">{properties}</div>

          <div className="detail-labels">{issue.labels.map((label) => <LabelBadge key={label.id} label={label} />)}<button className="button" onClick={() => setLabelsOpen(true)}>Edit labels</button></div>
          <MarkdownText text={issue.description} />

          {issue.children.length > 0 && (
            <section className="detail-section">
              <h2>Sub-issues</h2>
              <IssueRows issues={issue.children} />
            </section>
          )}

          {relations.length > 0 && (
            <section className="detail-section">
              <h2>Relations</h2>
              {relations.map((group) => (
                <div key={group.label}>
                  <h3>{group.label}</h3>
                  <IssueRows issues={group.issues} />
                </div>
              ))}
            </section>
          )}

          {issue.links.length > 0 && (
            <section className="detail-section">
              <h2>Links</h2>
              <ul className="detail-rows">
                {issue.links.map((link, index) => (
                  <li key={index}>
                    <a href={link.url} target="_blank" rel="noreferrer">{link.title}</a>
                    <small className="link-url">{link.url}</small>
                  </li>
                ))}
              </ul>
            </section>
          )}

          {issue.pull_requests.length > 0 && (
            <section className="detail-section">
              <h2>Pull requests</h2>
              <ul className="detail-rows">
                {issue.pull_requests.map((pull) => (
                  <li key={`${pull.repo}#${pull.number}`}>
                    <a href={pull.url} target="_blank" rel="noreferrer"><span className="pull-ref">{pull.repo}#{pull.number}</span> {pull.title}</a>
                    <span className={`pull-state pull-state-${pull.state}`}>{pull.state}</span>
                  </li>
                ))}
              </ul>
            </section>
          )}

          <section className="detail-section">
            <h2>Comments</h2>
            {issue.comments.map((item, index) => (
              <article className="comment" key={index}>
                <time dateTime={item.created_at}>{timestamp(item.created_at)}</time>
                <MarkdownText text={item.body} />
              </article>
            ))}
            <form className="comment-form" onSubmit={postComment}>
              <textarea rows={3} value={comment} onChange={(event) => setComment(event.target.value)} placeholder="Leave a comment… (Markdown)" aria-label="Comment" required />
              <button type="submit" className="button primary" disabled={posting}>{posting ? "Posting…" : "Comment"}</button>
            </form>
          </section>
        </div>
      </article>

      <aside className="detail-panel" aria-label="Properties">
        <h2>Properties</h2>
        {properties}
      </aside>

      {labelsOpen && <LabelPicker identifiers={[identifier]} current={issue.labels} catalog={catalog} onClose={() => setLabelsOpen(false)} onChanged={async () => { const updated = await api<IssueDetails>(`/api/issues/${identifier}`); setIssue(updated); }} />}
      {editing && <IssueEditor issue={issue} onClose={closeEditor} onSave={(updated) => {
        setIssue({ ...issue, ...updated });
        closeEditor();
      }} />}
    </div>
  );
}

function MarkdownText({ text }: { text: string }) {
  return <div className="markdown"><Markdown remarkPlugins={remarkPlugins}>{text}</Markdown></div>;
}

function IssueRows({ issues }: { issues: IssueSummary[] }) {
  return (
    <ul className="detail-rows">
      {issues.map((item) => (
        <li key={item.identifier}>
          <PriorityIcon priority={item.priority} />
          <StatusIcon status={item.status} />
          <NavLink to={`/issue/${item.identifier}`}><span className="identifier">{item.identifier}</span> {item.title}</NavLink>
          <small>{columns.find((column) => column.status === item.status)!.label}</small>
        </li>
      ))}
    </ul>
  );
}

function IssueEditor({ issue, onClose, onSave }: { issue: Issue; onClose: () => void; onSave: (issue: Issue) => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const titleInput = useRef<HTMLInputElement>(null);
  const [title, setTitle] = useState(issue.title);
  const [description, setDescription] = useState(issue.description);
  const [status, setStatus] = useState(issue.status);
  const [priority, setPriority] = useState(issue.priority);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const element = dialog.current!;
    element.showModal();
    titleInput.current!.focus();
    return () => element.close();
  }, []);

  async function save(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      const response = await fetch(`/api/issues/${issue.identifier}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ title, description, status, priority }),
      });
      if (response.status === 401) window.dispatchEvent(new Event("session-expired"));
      if (response.ok) window.dispatchEvent(new Event("session-active"));
      const result = await response.json();
      if (!response.ok) throw new Error(result.error);
      onSave(result as Issue);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Could not save issue");
    } finally {
      setSaving(false);
    }
  }

  return (
    <dialog ref={dialog} className="issue-dialog" aria-labelledby="issue-dialog-title" onCancel={onClose}>
      <form onSubmit={save}>
        <header>
          <h2 id="issue-dialog-title">{issue.identifier}</h2>
          <button type="button" className="ghost" aria-label="Close issue" onClick={onClose}><X className="icon" /></button>
        </header>
        <label>Title<input ref={titleInput} value={title} onChange={(event) => setTitle(event.target.value)} required /></label>
        <label>Description<textarea rows={8} value={description} onChange={(event) => setDescription(event.target.value)} /></label>
        <label>Status<select value={status} onChange={(event) => setStatus(event.target.value as Status)}>
          {columns.map((column) => <option key={column.status} value={column.status}>{column.label}</option>)}
        </select></label>
        <div className="dialog-priority"><span>Priority</span><PrioritySelect priority={priority} onChange={setPriority} label="Edit issue priority" disabled={saving} /></div>
        {error && <RequestError message={error} />}
        <footer>
          <button type="button" className="button" onClick={onClose}>Cancel</button>
          <button type="submit" className="button primary" disabled={saving}>{saving ? "Saving…" : "Save changes"}</button>
        </footer>
      </form>
    </dialog>
  );
}
