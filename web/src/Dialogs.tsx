import { X } from "lucide-react";
import { FormEvent, useEffect, useRef, useState } from "react";
import { EmojiPicker } from "./EmojiPicker";
import { RequestError, api, apiPath, columns, Issue, ModalDialog, Priority, PrioritySelect, Project, Status, StatusIcon, Team } from "./shared";

export function NewIssueDialog({ teams, team: scopeTeam, status, project: initialProject, projects, onClose, onCreate }: { teams: Team[]; team: string; status: Status; project: Project | null; projects: Project[]; onClose: () => void; onCreate: (issue: Issue) => Promise<void> }) {
  const [team, setTeam] = useState(initialProject ? (initialProject.teams.length === 1 ? initialProject.teams[0] : "") : scopeTeam);
  const [projectID, setProjectID] = useState(initialProject?.id ?? 0);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [priority, setPriority] = useState<Priority>(0);
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
      const created = await api<Issue>(apiPath("/issues"), "POST", { team, title, description, status, project_id: projectID, priority });
      if (mounted.current) await onCreate(created);
    } catch (cause) {
      if (mounted.current) setError((cause as Error).message);
    } finally {
      if (mounted.current) setSaving(false);
    }
  }

  return (
    <ModalDialog className="issue-dialog" aria-labelledby="new-issue-title" restoreFocus onClose={() => { if (!saving) onClose(); }}>
      <form onSubmit={create}>
        <header>
          <h2 id="new-issue-title">New issue</h2>
          <button type="button" className="ghost" aria-label="Close" disabled={saving} onClick={onClose}><X className="icon" /></button>
        </header>
        <label>Title<input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="Issue title" required disabled={saving} /></label>
        <label>Description<textarea rows={8} value={description} onChange={(event) => setDescription(event.target.value)} placeholder="Add a description… (Markdown)" disabled={saving} /></label>
        {teams.length > 0 ? (
          <label>Team<select value={team} onChange={(event) => setTeam(event.target.value)} required disabled={saving}>
            <option value="">Choose a team…</option>
            {teams.map((item) => <option key={item.key} value={item.key}>{item.name} ({item.key})</option>)}
          </select></label>
        ) : (
          <p className="error" role="alert">No teams yet. Create a team from Navigation, then reopen this dialog.</p>
        )}
        <IssueProjectSelect projects={projects} team={team} value={projectID} onChange={setProjectID} disabled={saving || !team} fallback={initialProject?.name ?? "Unavailable project"} />
        {!team && <p className="dialog-hint">Choose the team responsible for this issue before selecting a project.</p>}
        {incompatible && <p className="error" role="alert">This project is not available for the selected team. Clear it or choose another project.</p>}
        <div className="dialog-priority"><span>Priority</span><PrioritySelect priority={priority} onChange={setPriority} label="New issue priority" disabled={saving} /></div>
        <p className="dialog-meta">
          <span className="chip"><StatusIcon status={status} />{columns.find((column) => column.status === status)!.label}</span>
        </p>
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
  return <label>Project<select value={value} onChange={(event) => onChange(Number(event.target.value))} disabled={disabled}>
    <option value={0}>No project</option>
    {unavailable && <option value={value} disabled>{projects.find((project) => project.id === value)?.name ?? fallback}{team ? " (not available for this team)" : " (choose a team)"}</option>}
    {available.map((project) => <option key={project.id} value={project.id}>{project.name}{project.archived ? " (archived, current project)" : ""}</option>)}
  </select></label>;
}

export function CreateTeamDialog({ onClose, onCreate }: { onClose: () => void; onCreate: (team: Team) => void }) {
  const [key, setKey] = useState("");
  const [name, setName] = useState("");
  const [emoji, setEmoji] = useState("");
  const [startNumber, setStartNumber] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  async function create(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      onCreate(await api<Team>(apiPath("/teams"), "POST", { key, name, emoji, start_number: startNumber ? Number(startNumber) : undefined }));
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <ModalDialog className="issue-dialog" aria-labelledby="new-team-title" onClose={onClose}>
      <form onSubmit={create}>
        <header>
          <h2 id="new-team-title">Create team</h2>
          <button type="button" className="ghost" aria-label="Close" onClick={onClose}><X className="icon" /></button>
        </header>
        <label>Key<input value={key} onChange={(event) => setKey(event.target.value.toUpperCase())} placeholder="TOK" pattern="[A-Z][A-Z0-9]{1,5}" maxLength={6} required /></label>
        <div className="dialog-priority"><span>Emoji</span><EmojiPicker value={emoji} teamKey={key || "TE"} label="New team emoji" onChange={setEmoji} disabled={saving} /></div>
        <label>Name<input value={name} onChange={(event) => setName(event.target.value)} placeholder="Team name" required /></label>
        <label>Start number<input type="number" min={1} step={1} value={startNumber} onChange={(event) => setStartNumber(event.target.value)} placeholder="1" /></label>
        <p className="dialog-hint">Issues are numbered {key || "KEY"}-{startNumber || "1"}, {key || "KEY"}-{startNumber ? Number(startNumber) + 1 : "2"}, and so on. Set a start number above numbers the team already used elsewhere. The key cannot be changed later.</p>
        {error && <RequestError message={error} />}
        <footer>
          <button type="button" className="button" onClick={onClose}>Cancel</button>
          <button type="submit" className="button primary" disabled={saving}>{saving ? "Creating…" : "Create team"}</button>
        </footer>
      </form>
    </ModalDialog>
  );
}

export function RenameTeamDialog({ team, onClose, onSaved }: { team: Team; onClose: () => void; onSaved: (team: Team) => void }) {
  const [name, setName] = useState(team.name);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; };
  }, []);

  async function save(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      const saved = await api<Team>(apiPath(`/teams/${encodeURIComponent(team.key)}`), "PATCH", { name });
      if (mounted.current) onSaved(saved);
    } catch (cause) {
      if (mounted.current) setError((cause as Error).message);
    } finally {
      if (mounted.current) setSaving(false);
    }
  }

  return <ModalDialog className="issue-dialog" aria-labelledby="rename-team-title" restoreFocus onClose={() => { if (!saving) onClose(); }}>
    <form onSubmit={save}>
      <header><h2 id="rename-team-title">Rename team</h2><button type="button" className="ghost" aria-label="Close" disabled={saving} onClick={onClose}><X className="icon" /></button></header>
      <label>Name<input value={name} onChange={(event) => setName(event.target.value)} required disabled={saving} /></label>
      <p className="dialog-hint">The key {team.key} and existing issue identifiers stay unchanged.</p>
      {error && <RequestError message={error} />}
      <footer><button type="button" className="button" disabled={saving} onClick={onClose}>Cancel</button><button type="submit" className="button primary" disabled={saving}>{saving ? "Saving…" : "Save name"}</button></footer>
    </form>
  </ModalDialog>;
}
