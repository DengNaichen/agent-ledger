import { CycleSelect } from "./Cycles";
import "./cycles.css";
import { AssigneeSelect } from "./Assignee";
import { codingAgentName, personName } from "./shared";
import { ArrowUpRight, Box, CornerUpLeft, Plus, Users, X } from "lucide-react";
import { FormEvent, useCallback, useEffect, useRef, useState } from "react";
import { MarkdownText } from "./MarkdownText";
import { WorkLogEntry, WorkLogList, ActivityTabs } from "./WorkLogs";
import { RequestError, api, apiPath, columns, Comment, Issue, IssueDetails, IssueSummary, ModalDialog, NavLink, pagePath, Status, StatusIcon, StatusSelect, Priority, PriorityIcon, PrioritySelect, Project, Team, issueTeamKey } from "./shared";

import { LabelPicker } from "./Labels";
import { Catalog } from "./useLabels";
import { IssueProjectSelect } from "./Dialogs";
import { LabelBadge } from "./shared";
import { AgentMark } from "./AgentMark";
import { EntityEmoji } from "./EmojiPicker";

import { useResource } from "./useResource";
import { useRequestScope } from "./useRequestScope";

const timestamp = (value: string) => new Date(value).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });

export function IssueDetail({ catalog, teams, onSaved, identifier, setError, editing, closeEditor }: { catalog: Catalog; teams: Team[]; onSaved: (issue: Issue) => Promise<void>; identifier: string; setError: (message: string) => void; editing: boolean; closeEditor: () => void }) {
  const [issue, setIssue] = useState<IssueDetails | null>(null);
  const [activityTab, setActivityTab] = useState<"comments" | "work-logs">("comments");
  const [comment, setComment] = useState("");
  const [labelsOpen, setLabelsOpen] = useState(false);
  const [savingPriority, setSavingPriority] = useState(false);
  const [savingCycle, setSavingCycle] = useState(false);
  const [savingAssignee, setSavingAssignee] = useState(false);
  const [posting, setPosting] = useState(false);
  const requests = useRequestScope(identifier);
  const path = apiPath(`/issues/${identifier}`);
  const [loadError, setLoadError] = useState("");
  const commentRevision = useRef(0);
  const delegationHistory = useRef<HTMLElement>(null);

  const loadIssue = useCallback(async () => {
    await requests.read(() => api<IssueDetails>(path), (loaded) => {
      setIssue(loaded); setLoadError("");
    }, (cause) => {
      setLoadError(cause.message);
      if (cause.message === "issue not found" || cause.message === "workspace access denied") setIssue(null);
    });
  }, [path, requests]);

  useEffect(() => {
    void loadIssue();
    const load = () => { void loadIssue(); };
    window.addEventListener("focus", load);
    return () => window.removeEventListener("focus", load);
  }, [loadIssue]);

  async function changeStatus(status: Status) {
    await requests.write(identifier, () => api<Issue>(`${path}/status`, "PATCH", { status }),
      (updated) => setIssue((current) => ({ ...current!, ...updated })), (cause) => setError(cause.message));
  }

  async function changePriority(priority: Priority) {
    setSavingPriority(true);
    setError("");
    await requests.write(identifier, () => api<Issue>(path, "PATCH", { priority }), (updated) => {
      setIssue((current) => ({ ...current!, ...updated })); setSavingPriority(false);
    }, (cause) => { setError(cause.message); setSavingPriority(false); });
  }

  async function changeCycle(cycle_id: number) {
    setSavingCycle(true); setError("");
    await requests.write(identifier, () => api<Issue>(path, "PATCH", {cycle_id}),
      (updated) => { setIssue((current) => ({...current!, ...updated})); setSavingCycle(false); },
      (cause) => { setError(cause.message); setSavingCycle(false); });
  }
  async function changeAssignee(assignee: string) {
    setSavingAssignee(true); setError("");
    const saved = await requests.write(identifier, () => api<Issue>(path, "PATCH", { assignee }),
      (updated) => { setIssue((current) => ({ ...current!, ...updated })); setSavingAssignee(false); },
      (cause) => { setError(cause.message); setSavingAssignee(false); });
    if (saved) await loadIssue();
  }

  async function postComment(event: FormEvent) {
    event.preventDefault();
    if (posting) return;
    const submittedRevision = commentRevision.current;
    setPosting(true);
    setError("");
    await requests.write(identifier, () => api<Comment>(`${path}/comments`, "POST", { body: comment }), (added) => {
      setIssue((current) => ({ ...current!, comments: [...current!.comments, added] }));
      if (commentRevision.current === submittedRevision) setComment("");
      setPosting(false);
    }, (cause) => { setError(cause.message); setPosting(false); });
  }

  if (!issue) return loadError ? <div className="scope-message"><RequestError message={loadError} /><button className="button" onClick={() => void loadIssue()}>Retry</button></div> : <p className="empty">Loading…</p>;

  const relations = [
    { label: "Blocks", issues: issue.blocks },
    { label: "Blocked by", issues: issue.blocked_by },
    { label: "Related", issues: issue.related },
  ].filter((group) => group.issues.length > 0);

  const teamKey = issueTeamKey(issue.identifier);
  const agents = Array.from(new Set(issue.delegations.map((entry) => entry.agent)));

  return (
    <div className="detail">
      <div className="detail-content">
        {loadError && <RequestError message={loadError} />}
        <header className="detail-heading"><h1>{issue.title}</h1></header>
        <section className="detail-properties" aria-label="Properties">
          <div className="detail-property-group">
            <h2>Properties</h2>
            <dl className="detail-property-list">
              <div><dt className="detail-property-term">Status</dt><dd><StatusSelect issue={issue} onChange={(status) => void changeStatus(status)} /></dd></div>
              <div><dt className="detail-property-term">Priority</dt><dd><PrioritySelect priority={issue.priority} label={`Change priority for ${identifier}`} disabled={savingPriority} emptyLabel="Set priority" onChange={(next) => void changePriority(next)} /></dd></div>
              <div>
                <dt className="detail-property-term">Assignee</dt>
                <dd><AssigneeSelect key={identifier} team={teamKey} value={issue.assignee?.subject ?? ""} current={issue.assignee} disabled={savingAssignee} onChange={(value) => void changeAssignee(value)} label={`Change assignee for ${identifier}`} /></dd>
                {agents.length > 0 && <dd className="detail-property-child"><button className="delegation-link" onClick={() => delegationHistory.current!.scrollIntoView({ block: "start" })}>{agents.map((agent) => <AgentMark key={agent} agent={agent} />)}<span>{agents.map(codingAgentName).join(", ")} · {issue.delegations.length} {issue.delegations.length === 1 ? "execution" : "executions"}</span></button></dd>}
              </div>
              <div><dt className="detail-property-term">Cycle</dt><dd className="detail-cycle-value"><CycleSelect key={identifier} team={teamKey} value={issue.cycle_id ?? 0} disabled={savingCycle} onChange={(id) => void changeCycle(Number(id))} label={`Change cycle for ${identifier}`} />{issue.cycle_id && <NavLink to={pagePath(`/team/${encodeURIComponent(teamKey)}/cycles/${issue.cycle_id}`)}><ArrowUpRight className="icon" role="img" aria-label="View cycle" /></NavLink>}</dd></div>
            </dl>
          </div>
          <div className="detail-property-group">
            <h3>Labels</h3>
            <div className="detail-property-labels">
              {issue.labels.map((label) => <LabelBadge key={label.id} label={label} />)}
              {issue.labels.length > 0
                ? <button className="detail-property-action detail-label-edit" aria-label="Edit labels" title="Edit labels" onClick={() => setLabelsOpen(true)}><Plus className="icon" /></button>
                : <button className="detail-property-action" onClick={() => setLabelsOpen(true)}><Plus className="icon" /><span>Add label</span></button>}
            </div>
          </div>
          <div className="detail-property-group">
            <h3>Project</h3>
            {issue.project_id !== null
              ? <NavLink className="detail-property-action" to={pagePath(`/project/${issue.project_id}`)}><EntityEmoji emoji={issue.project_emoji} /><span>{issue.project}</span></NavLink>
              : <p className="detail-property-row"><Box className="icon" /><span className="none">No project</span></p>}
          </div>
          {issue.parent && <div className="detail-property-group">
            <h3>Parent</h3>
            <NavLink className="detail-property-action" to={pagePath(`/issue/${issue.parent}`)}><CornerUpLeft className="icon" /><span>{issue.parent}</span></NavLink>
          </div>}
        </section>
        <article className="detail-main">
          <MarkdownText text={issue.description} />
          {issue.decisions.length > 0 && <section className="detail-section decision-section" aria-label="Decisions">
            <h2>Decisions <span className="activity-count">{issue.decisions.length}</span></h2>
            <p className="activity-hint">Settled choices and the reasons behind them.</p>
            {issue.decisions.map((entry) => <WorkLogEntry key={entry.id} entry={entry} path={path} />)}
          </section>}
          {issue.delegations.length > 0 && <section ref={delegationHistory} className="detail-section delegation-history" aria-label="Agent executions">
            <h2>Agent executions</h2>
            <p className="none">Recorded participation, not an edit lock or live process status.</p>
            <ol>{issue.delegations.map((entry) => <li key={entry.id}>
              <h3>{codingAgentName(entry.agent)} · {entry.agent_id ? "Subagent" : "Session"}{entry.agent_type && ` · ${entry.agent_type}`}</h3>
              <dl>
                <div><dt>Session ID</dt><dd><code>{entry.session_id}</code></dd></div>
                {entry.agent_id && <div><dt>Agent ID</dt><dd><code>{entry.agent_id}</code></dd></div>}
                <div><dt>Host</dt><dd>{entry.hostname}</dd></div>
                <div><dt>Recorded by</dt><dd>{entry.actor ? personName(entry.actor) : "System"}</dd></div>
                <div><dt>Started</dt><dd><time dateTime={entry.created_at}>{timestamp(entry.created_at)}</time></dd></div>
              </dl>
            </li>)}</ol>
          </section>}
          {issue.assignments.length > 0 && <section className="detail-section assignment-history"><h2>Assignment history</h2><ol>{issue.assignments.map((change, index) => <li key={index}><p><strong>{change.actor ? personName(change.actor) : "System"}</strong>: {change.from ? personName(change.from) : "Unassigned"} → {change.to ? personName(change.to) : "Unassigned"}</p><time dateTime={change.created_at}>{timestamp(change.created_at)}</time></li>)}</ol></section>}

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

          <section className="detail-section issue-activity" aria-label="Activity">
            <ActivityTabs value={activityTab} onChange={setActivityTab} comments={issue.comments.length} workLogs={issue.work_log_count} />
            <div id="activity-comments" role="tabpanel" aria-labelledby="tab-comments" hidden={activityTab !== "comments"}>
              {issue.comments.length === 0 && <p className="activity-hint">Questions and feedback from people belong here.</p>}
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
                <textarea rows={3} value={comment} onChange={(event) => { commentRevision.current++; setComment(event.target.value); }} placeholder="Leave a comment… (Markdown)" aria-label="Comment" required />
                <button type="submit" className="button primary" disabled={posting}>{posting ? "Posting…" : "Comment"}</button>
              </form>
            </div>
            <div id="activity-work-logs" role="tabpanel" aria-labelledby="tab-work-logs" hidden={activityTab !== "work-logs"}>
              {activityTab === "work-logs" && <WorkLogList key={identifier} path={path} />}
            </div>
          </section>
        </article>
      </div>

      {labelsOpen && <LabelPicker identifiers={[identifier]} current={issue.labels} catalog={catalog} onClose={() => setLabelsOpen(false)} onChanged={loadIssue} write={(save, success, failure) => requests.write(identifier, save, success, failure)} />}
      {editing && <IssueEditor issue={issue} teams={teams} onClose={closeEditor} write={(save, success, failure) => requests.write(identifier, save, success, failure)} onSave={async (updated) => {
        setIssue((current) => ({ ...current!, ...updated }));
        closeEditor();
        await Promise.all([loadIssue(), onSaved(updated)]);
      }} />}
    </div>
  );
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

function IssueEditor({ issue, teams, onClose, onSave, write }: { issue: Issue; teams: Team[]; onClose: () => void; onSave: (issue: Issue) => Promise<void>; write: (save: () => Promise<Issue>, success: (issue: Issue) => void, failure: (cause: Error) => void) => Promise<boolean> }) {
  const team = issueTeamKey(issue.identifier);
  const [projectID, setProjectID] = useState(issue.project_id ?? 0);
  const { data, loading: loadingProjects, error: projectError, reload: loadProjects } = useResource<{ projects: Project[] }>(apiPath(`/projects?include_archived=true&team=${encodeURIComponent(team)}`), { projects: [] });
  const projects = data.projects;
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
    const path = apiPath(`/issues/${issue.identifier}`);
    await write(() => api<Issue>(path, "PATCH", { title, description, status, priority, project_id: projectID }),
      (updated) => { void onSave(updated); },
      (cause) => { setError(cause.message); setSaving(false); });
  }

  return (
    <ModalDialog className="issue-dialog issue-composer" aria-labelledby="issue-dialog-title" restoreFocus onClose={() => { if (!saving) onClose(); }}>
      <form onSubmit={save}>
        <header>
          <h2 id="issue-dialog-title">{issue.identifier}</h2>
          <button type="button" className="ghost" aria-label="Close issue" disabled={saving} onClick={onClose}><X className="icon" /></button>
        </header>
        <label className="composer-title"><span className="composer-field-label">Title</span><input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="Issue title" required disabled={saving} /></label>
        <label className="composer-description"><span className="composer-field-label">Description</span><textarea rows={5} value={description} onChange={(event) => setDescription(event.target.value)} placeholder="Add a description… (Markdown)" disabled={saving} /></label>
        <div className="composer-properties">
          <label className="composer-property"><Users className="icon" /><span className="composer-property-name">Team</span><input value={`${teams.find((item) => item.key === team)?.name ?? team} (${team})`} readOnly /></label>
          <IssueProjectSelect projects={projects} team={team} value={projectID} onChange={setProjectID} disabled={saving || loadingProjects || !!projectError} retainedID={issue.project_id ?? 0} fallback={issue.project ?? "Unavailable project"} />
          <label className="composer-property"><StatusIcon status={status} /><span className="composer-property-name">Status</span><select disabled={saving} value={status} onChange={(event) => setStatus(event.target.value as Status)}>
            {columns.map((column) => <option key={column.status} value={column.status}>{column.label}</option>)}
          </select></label>
          <div className="composer-property composer-priority"><span className="composer-property-name">Priority</span><PrioritySelect priority={priority} onChange={setPriority} label="Edit issue priority" disabled={saving} /></div>
        </div>
        <p className="dialog-hint">The team and identifier of an existing issue cannot be changed.</p>
        {loadingProjects && <p className="dialog-hint">Loading projects…</p>}
        {projects.find((project) => project.id === projectID)?.archived && <p className="dialog-hint">You can keep this issue in its current archived project or choose an active project.</p>}
        {projectError && <><RequestError message={projectError} /><button type="button" className="button" disabled={saving} onClick={() => void loadProjects()}>Retry projects</button></>}
        {error && <RequestError message={error} />}
        <footer>
          <button type="button" className="button" disabled={saving} onClick={onClose}>Cancel</button>
          <button type="submit" className="button primary" disabled={saving || loadingProjects || !!projectError}>{saving ? "Saving…" : "Save changes"}</button>
        </footer>
      </form>
    </ModalDialog>
  );
}
