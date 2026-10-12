import { AssigneeSelect } from "./Assignee";
import { Box, Users, X } from "lucide-react";
import { FormEvent, useEffect, useRef, useState } from "react";
import { EmojiPicker } from "./EmojiPicker";
import { RequestError, api, apiPath, columns, Issue, ModalDialog, Priority, PrioritySelect, Project, Status, StatusIcon, Team } from "./shared";

export function NewIssueDialog({ teams, team: scopeTeam, status, project: initialProject, projects, onClose, onCreate }: { teams: Team[]; team: string; status: Status; project: Project | null; projects: Project[]; onClose: () => void; onCreate: (issue: Issue) => Promise<void> }) {
  const [team, setTeam] = useState(initialProject ? (initialProject.teams.length === 1 ? initialProject.teams[0] : "") : scopeTeam);
  const [projectID, setProjectID] = useState(initialProject?.id ?? 0);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [priority, setPriority] = useState<Priority>(0);
  const [assignee, setAssignee] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const mounted = useRef(true);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  const selectedProject = projects.find((item) => item.id === projectID);
  const incompatible = projectID !== 0 && !!team && (!selectedProject || selectedProject.archived || !selectedProject.teams.includes(team));

  async function create(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      const created = await api<Issue>(apiPath("/issues"), "POST", { team, title, description, status, project_id: projectID, priority, assignee });
      if (mounted.current) await onCreate(created);
    } catch (cause) {
      if (mounted.current) setError((cause as Error).message);
    } finally {
      if (mounted.current) setSaving(false);
    }
  }

  return (
    <ModalDialog className="issue-dialog issue-composer" aria-labelledby="new-issue-title" restoreFocus onClose={() => { if (!saving) onClose(); }}>
      <form onSubmit={create}>
        <header>
          <h2 id="new-issue-title">New issue</h2>
          <button type="button" className="ghost" aria-label="Close" disabled={saving} onClick={onClose}><X className="icon" /></button>
        </header>
        <label className="composer-title"><span className="composer-field-label">Title</span><input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="Issue title" required disabled={saving} /></label>
        <label className="composer-description"><span className="composer-field-label">Description</span><textarea rows={5} value={description} onChange={(event) => setDescription(event.target.value)} placeholder="Add a description… (Markdown)" disabled={saving} /></label>
        <div className="composer-properties">
          {teams.length > 0 ? (
            <label className="composer-property"><Users className="icon" /><span className="composer-property-name">Team <span title="Required">*</span></span><select value={team} onChange={(event) => { setTeam(event.target.value); setAssignee(""); }} required disabled={saving}>
              <option value="">Choose a team…</option>
              {teams.map((item) => <option key={item.key} value={item.key}>{item.name} ({item.key})</option>)}
            </select></label>
          ) : (
            <p className="error" role="alert">No teams yet. Create a team from Navigation, then reopen this dialog.</p>
          )}
          <label className="composer-property"><span className="composer-property-name">Assignee</span><AssigneeSelect key={team} team={team} value={assignee} onChange={setAssignee} disabled={saving} label="New issue assignee" /></label>
          <IssueProjectSelect projects={projects} team={team} value={projectID} onChange={setProjectID} disabled={saving || !team} fallback={initialProject?.name ?? "Unavailable project"} />
          <div className="composer-property composer-priority"><span className="composer-property-name">Priority</span><PrioritySelect priority={priority} onChange={setPriority} label="New issue priority" disabled={saving} /></div>
          <span className="composer-property composer-status" title="Status"><StatusIcon status={status} />{columns.find((column) => column.status === status)!.label}</span>
        </div>
        {!team && <p className="dialog-hint">Choose the team responsible for this issue before selecting a project.</p>}
        {incompatible && <p className="error" role="alert">This project is not available for the selected team. Clear it or choose another project.</p>}
        {error && <RequestError message={error} />}
        <footer>
          <button type="button" className="button" disabled={saving} onClick={onClose}>Cancel</button>
          <button type="submit" className="button primary" disabled={saving || !team || incompatible}>{saving ? "Creating…" : "Create issue"}</button>
        </footer>
      </form>
    </ModalDialog>
  );
}

export function IssueProjectSelect({ projects, team, value, onChange, disabled, retainedID = 0, fallback }: { projects: Project[]; team: string; value: number; onChange: (id: number) => void; disabled: boolean; retainedID?: number; fallback: string }) {
  const available = projects.filter((project) => project.teams.includes(team) && (!project.archived || project.id === retainedID));
  const unavailable = value !== 0 && !available.some((project) => project.id === value);
  return <label className="composer-property"><Box className="icon" /><span className="composer-property-name">Project</span><select value={value} onChange={(event) => onChange(Number(event.target.value))} disabled={disabled}>
    <option value={0}>No project</option>
    {unavailable && <option value={value} disabled>{projects.find((project) => project.id === value)?.name ?? fallback}{team ? " (not available for this team)" : " (choose a team)"}</option>}
    {available.map((project) => <option key={project.id} value={project.id}>{project.name}{project.archived ? " (archived, current project)" : ""}</option>)}
  </select></label>;
}

export function CreateTeamDialog({ onClose, onCreate }: { onClose: () => void; onCreate: (team: Team) => void }) {
  const [privateTeam, setPrivateTeam] = useState(false);
  const [key, setKey] = useState("");
  const [name, setName] = useState("");
  const [emoji, setEmoji] = useState("");
  const [startNumber, setStartNumber] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  const mounted = useRef(true);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);

  async function create(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      const created = await api<Team>(apiPath("/teams"), "POST", { key, name, emoji, private: privateTeam, start_number: startNumber ? Number(startNumber) : undefined });
      if (mounted.current) onCreate(created);
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <ModalDialog className="issue-dialog" aria-labelledby="new-team-title" onClose={() => { if (!saving) onClose(); }}>
      <form onSubmit={create}>
        <header>
          <h2 id="new-team-title">Create team</h2>
          <button type="button" className="ghost" aria-label="Close" disabled={saving} onClick={onClose}><X className="icon" /></button>
        </header>
        <label>Key<input disabled={saving} value={key} onChange={(event) => setKey(event.target.value.toUpperCase())} placeholder="TOK" pattern="[A-Z][A-Z0-9]{1,5}" maxLength={6} required /></label>
        <div className="dialog-priority"><span>Emoji</span><EmojiPicker value={emoji} teamKey={key || "TE"} label="New team emoji" onChange={setEmoji} disabled={saving} /></div>
        <label>Name<input disabled={saving} value={name} onChange={(event) => setName(event.target.value)} placeholder="Team name" required /></label>
        <label>Start number<input disabled={saving} type="number" min={1} step={1} value={startNumber} onChange={(event) => setStartNumber(event.target.value)} placeholder="1" /></label>
        <p className="dialog-hint">Issues are numbered {key || "KEY"}-{startNumber || "1"}, {key || "KEY"}-{startNumber ? Number(startNumber) + 1 : "2"}, and so on. Set a start number above numbers the team already used elsewhere. The key cannot be changed later.</p>
        <label><input type="checkbox" checked={privateTeam} onChange={(event) => setPrivateTeam(event.target.checked)} disabled={saving} />Make team private</label>
        {error && <RequestError message={error} />}
        <footer>
          <button type="button" className="button" disabled={saving} onClick={onClose}>Cancel</button>
          <button type="submit" className="button primary" disabled={saving}>{saving ? "Creating…" : "Create team"}</button>
        </footer>
      </form>
    </ModalDialog>
  );
}
