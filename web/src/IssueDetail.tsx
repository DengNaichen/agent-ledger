import { FormEvent, useEffect, useRef, useState } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { api, columns, Comment, Issue, IssueDetails, IssueMeta, IssueSummary, NavLink, Status, StatusSelect } from "./shared";

const remarkPlugins = [remarkGfm];

export function IssueDetail({ identifier, setError }: { identifier: string; setError: (message: string) => void }) {
  const [issue, setIssue] = useState<IssueDetails | null>(null);
  const [editing, setEditing] = useState(false);
  const [comment, setComment] = useState("");
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

  return (
    <article className="detail">
      <header className="detail-header">
        <div className="detail-topline">
          <span className="identifier">{issue.identifier}</span>
          <button type="button" className="detail-button" onClick={() => setEditing(true)}>Edit</button>
        </div>
        <h1>{issue.title}</h1>
        <div className="detail-properties">
          <StatusSelect issue={issue} onChange={(status) => void changeStatus(status)} />
          <IssueMeta issue={issue} />
          {issue.parent && <span>Parent <NavLink to={`/issue/${issue.parent}`}>{issue.parent}</NavLink></span>}
        </div>
      </header>

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

      {issue.comments.length > 0 && (
        <section className="detail-section">
          <h2>Comments</h2>
          {issue.comments.map((item, index) => (
            <article className="comment" key={index}>
              <time dateTime={item.created_at}>{new Date(item.created_at).toLocaleString()}</time>
              <MarkdownText text={item.body} />
            </article>
          ))}
        </section>
      )}

      <form className="comment-form" onSubmit={postComment}>
        <textarea rows={4} value={comment} onChange={(event) => setComment(event.target.value)} placeholder="Leave a comment… (Markdown)" aria-label="Comment" required />
        <button type="submit" className="detail-button" disabled={posting}>{posting ? "Posting…" : "Comment"}</button>
      </form>

      {editing && <IssueEditor issue={issue} onClose={() => setEditing(false)} onSave={(updated) => {
        setIssue({ ...issue, ...updated });
        setEditing(false);
      }} />}
    </article>
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
          <span className={`status-icon status-icon-${item.status}`} aria-hidden="true" />
          <NavLink to={`/issue/${item.identifier}`}><span className="identifier">{item.identifier}</span> {item.title}</NavLink>
          <small>{columns.find((column) => column.status === item.status)!.label}</small>
        </li>
      ))}
    </ul>
  );
}

function IssueEditor({ issue, onClose, onSave }: { issue: Issue; onClose: () => void; onSave: (issue: Issue) => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [title, setTitle] = useState(issue.title);
  const [description, setDescription] = useState(issue.description);
  const [status, setStatus] = useState(issue.status);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const element = dialog.current!;
    element.showModal();
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
        body: JSON.stringify({ title, description, status }),
      });
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
          <button type="button" aria-label="Close issue" onClick={onClose}>×</button>
        </header>
        <label>Title<input value={title} onChange={(event) => setTitle(event.target.value)} required autoFocus /></label>
        <label>Description<textarea rows={8} value={description} onChange={(event) => setDescription(event.target.value)} /></label>
        <label>Status<select value={status} onChange={(event) => setStatus(event.target.value as Status)}>
          {columns.map((column) => <option key={column.status} value={column.status}>{column.label}</option>)}
        </select></label>
        {error && <p className="error" role="alert">{error}</p>}
        <footer>
          <button type="button" onClick={onClose}>Cancel</button>
          <button type="submit" disabled={saving}>{saving ? "Saving…" : "Save changes"}</button>
        </footer>
      </form>
    </dialog>
  );
}
