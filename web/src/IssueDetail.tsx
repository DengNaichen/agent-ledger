import { X } from "lucide-react";
import { FormEvent, useEffect, useRef, useState } from "react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { RequestError, api, apiPath, columns, Comment, Issue, IssueDetails, IssueSummary, ModalDialog, NavLink, pagePath, Status, StatusIcon, StatusSelect, Priority, PriorityIcon, PrioritySelect, Project, Team, issueTeamKey } from "./shared";

import { Catalog, LabelPicker } from "./Labels";
import { IssueProjectSelect } from "./Dialogs";
import { LabelBadge } from "./shared";

const remarkPlugins = [remarkGfm];

const timestamp = (value: string) => new Date(value).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });

export function IssueDetail({ catalog, teams, onSaved, identifier, setError, editing, closeEditor }: { catalog: Catalog; teams: Team[]; onSaved: (issue: Issue) => Promise<void>; identifier: string; setError: (message: string) => void; editing: boolean; closeEditor: () => void }) {
  const [issue, setIssue] = useState<IssueDetails | null>(null);
  const [comment, setComment] = useState("");
  const [labelsOpen, setLabelsOpen] = useState(false);
  const [savingPriority, setSavingPriority] = useState(false);
  const [posting, setPosting] = useState(false);
  const loadVersion = useRef(0);
  const mounted = useRef(true);

  async function loadIssue() {
    const version = ++loadVersion.current;
    try {
      const loaded = await api<IssueDetails>(apiPath(`/issues/${identifier}`));
      if (mounted.current && version === loadVersion.current) setIssue(loaded);
    } catch (cause) {
      if (mounted.current && version === loadVersion.current) setError((cause as Error).message);
    }
  }

  useEffect(() => {
    mounted.current = true;
    const load = () => void loadIssue();
    load();
    window.addEventListener("focus", load);
    return () => { mounted.current = false; ++loadVersion.current; window.removeEventListener("focus", load); };
  }, [identifier]);

  async function changeStatus(status: Status) {
    try {
      const updated = await api<Issue>(apiPath(`/issues/${identifier}/status`), "PATCH", { status });
      setIssue((current) => ({ ...current!, ...updated }));
    } catch (cause) {
      setError((cause as Error).message);
    }
  }

  async function changePriority(priority: Priority) {
    setSavingPriority(true);
    setError("");
    try {
      const updated = await api<Issue>(apiPath(`/issues/${identifier}`), "PATCH", { priority });
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
      const added = await api<Comment>(apiPath(`/issues/${identifier}/comments`), "POST", { body: comment });
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
      <dd>{issue.project_id !== null ? <NavLink to={pagePath(`/project/${issue.project_id}`)}>{issue.project}</NavLink> : <span className="none">None</span>}</dd>
      <dt>Team</dt>
      <dd>{teams.find((team) => team.key === issueTeamKey(issue.identifier))?.name ?? issueTeamKey(issue.identifier)} ({issueTeamKey(issue.identifier)})</dd>
      <dt>Parent</dt>
      <dd>{issue.parent ? <NavLink to={pagePath(`/issue/${issue.parent}`)}>{issue.parent}</NavLink> : <span className="none">None</span>}</dd>
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
                <header className="comment-meta">
                  {item.author && <span className="comment-author">{item.author}</span>}
                  <time dateTime={item.created_at}>{timestamp(item.created_at)}</time>
                </header>
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

      {labelsOpen && <LabelPicker identifiers={[identifier]} current={issue.labels} catalog={catalog} onClose={() => setLabelsOpen(false)} onChanged={loadIssue} />}
      {editing && <IssueEditor issue={issue} teams={teams} onClose={closeEditor} onSave={async (updated) => {
        ++loadVersion.current;
        setIssue((current) => ({ ...current!, ...updated }));
        closeEditor();
        await Promise.all([loadIssue(), onSaved(updated)]);
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
          <NavLink to={pagePath(`/issue/${item.identifier}`)}><span className="identifier">{item.identifier}</span> {item.title}</NavLink>
          <small>{columns.find((column) => column.status === item.status)!.label}</small>
        </li>
      ))}
    </ul>
  );
}

function IssueEditor({ issue, teams, onClose, onSave }: { issue: Issue; teams: Team[]; onClose: () => void; onSave: (issue: Issue) => Promise<void> }) {
  const team = issueTeamKey(issue.identifier);
  const [projectID, setProjectID] = useState(issue.project_id ?? 0);
  const [projects, setProjects] = useState<Project[]>([]);
  const [loadingProjects, setLoadingProjects] = useState(true);
  const [projectError, setProjectError] = useState("");
  const mounted = useRef(true);
  const projectVersion = useRef(0);

  async function loadProjects() {
    const version = ++projectVersion.current;
    setLoadingProjects(true);
    setProjectError("");
    try {
      const data = await api<{ projects: Project[] }>(apiPath(`/projects?include_archived=true&team=${encodeURIComponent(team)}`));
      if (mounted.current && version === projectVersion.current) setProjects(data.projects);
    } catch (cause) {
      if (mounted.current && version === projectVersion.current) setProjectError((cause as Error).message);
    } finally {
      if (mounted.current && version === projectVersion.current) setLoadingProjects(false);
    }
  }
  useEffect(() => {
    mounted.current = true;
    void loadProjects();
    return () => { mounted.current = false; ++projectVersion.current; };
  }, []);
  const [title, setTitle] = useState(issue.title);
  const [description, setDescription] = useState(issue.description);
  const [status, setStatus] = useState(issue.status);
  const [priority, setPriority] = useState(issue.priority);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  async function save(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      const updated = await api<Issue>(apiPath(`/issues/${issue.identifier}`), "PATCH", { title, description, status, priority, project_id: projectID });
      if (mounted.current) await onSave(updated);
    } catch (cause) {
      if (mounted.current) setError((cause as Error).message);
    } finally {
      if (mounted.current) setSaving(false);
    }
  }

  return (
    <ModalDialog className="issue-dialog" aria-labelledby="issue-dialog-title" restoreFocus onClose={() => { if (!saving) onClose(); }}>
      <form onSubmit={save}>
        <header>
          <h2 id="issue-dialog-title">{issue.identifier}</h2>
          <button type="button" className="ghost" aria-label="Close issue" disabled={saving} onClick={onClose}><X className="icon" /></button>
        </header>
        <label>Title<input value={title} onChange={(event) => setTitle(event.target.value)} required disabled={saving} /></label>
        <label>Description<textarea rows={8} value={description} onChange={(event) => setDescription(event.target.value)} disabled={saving} /></label>
        <label>Team<input value={`${teams.find((item) => item.key === team)?.name ?? team} (${team})`} readOnly /></label>
        <p className="dialog-hint">The team and identifier of an existing issue cannot be changed.</p>
        <IssueProjectSelect projects={projects} team={team} value={projectID} onChange={setProjectID} disabled={saving || loadingProjects || !!projectError} retainedID={issue.project_id ?? 0} fallback={issue.project ?? "Unavailable project"} />
        {loadingProjects && <p className="dialog-hint">Loading projects…</p>}
        {projects.find((project) => project.id === projectID)?.archived && <p className="dialog-hint">You can keep this issue in its current archived project or choose an active project.</p>}
        {projectError && <><RequestError message={projectError} /><button type="button" className="button" disabled={saving} onClick={() => void loadProjects()}>Retry projects</button></>}
        <label>Status<select disabled={saving} value={status} onChange={(event) => setStatus(event.target.value as Status)}>
          {columns.map((column) => <option key={column.status} value={column.status}>{column.label}</option>)}
        </select></label>
        <div className="dialog-priority"><span>Priority</span><PrioritySelect priority={priority} onChange={setPriority} label="Edit issue priority" disabled={saving} /></div>
        {error && <RequestError message={error} />}
        <footer>
          <button type="button" className="button" disabled={saving} onClick={onClose}>Cancel</button>
          <button type="submit" className="button primary" disabled={saving || loadingProjects || !!projectError}>{saving ? "Saving…" : "Save changes"}</button>
        </footer>
      </form>
    </ModalDialog>
  );
}
