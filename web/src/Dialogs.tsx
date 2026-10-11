import { Box, X } from "lucide-react";
import { FormEvent, useState } from "react";
import { EmojiPicker } from "./EmojiPicker";
import { RequestError, api, apiPath, columns, Issue, ModalDialog, Priority, PrioritySelect, Status, StatusIcon, Team } from "./shared";

export function NewIssueDialog({ teams, team: selectedTeam, status, project, onClose, onCreate }: { teams: Team[]; team: string; status: Status; project: string; onClose: () => void; onCreate: (issue: Issue) => void }) {
  // The server has no default team; the dialog preselects the team being viewed, else the first one, and always sends the choice.
  const [team, setTeam] = useState(selectedTeam || (teams.length > 0 ? teams[0].key : ""));
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [priority, setPriority] = useState<Priority>(0);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  async function create(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      onCreate(await api<Issue>(apiPath("/issues"), "POST", { team, title, description, status, project, priority }));
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <ModalDialog className="issue-dialog" aria-labelledby="new-issue-title" onClose={onClose}>
      <form onSubmit={create}>
        <header>
          <h2 id="new-issue-title">New issue</h2>
          <button type="button" className="ghost" aria-label="Close" onClick={onClose}><X className="icon" /></button>
        </header>
        <label>Title<input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="Issue title" required /></label>
        <label>Description<textarea rows={8} value={description} onChange={(event) => setDescription(event.target.value)} placeholder="Add a description… (Markdown)" /></label>
        {teams.length > 0 ? (
          <label>Team<select value={team} onChange={(event) => setTeam(event.target.value)}>
            {teams.map((item) => <option key={item.key} value={item.key}>{item.key} · {item.name}</option>)}
          </select></label>
        ) : (
          <p className="error" role="alert">No teams yet. Create one with + next to Teams in the sidebar, then reopen this dialog.</p>
        )}
        <div className="dialog-priority"><span>Priority</span><PrioritySelect priority={priority} onChange={setPriority} label="New issue priority" disabled={saving} /></div>
        <p className="dialog-meta">
          <span className="chip"><StatusIcon status={status} />{columns.find((column) => column.status === status)!.label}</span>
          {project && <span className="chip"><Box className="icon" />{project}</span>}
        </p>
        {error && <RequestError message={error} />}
        <footer>
          <button type="button" className="button" onClick={onClose}>Cancel</button>
          <button type="submit" className="button primary" disabled={saving || !team}>{saving ? "Creating…" : "Create issue"}</button>
        </footer>
      </form>
    </ModalDialog>
  );
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
