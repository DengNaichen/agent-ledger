import { FormEvent, useEffect, useRef, useState } from "react";
import { PageHeader } from "./PageHeader";
import { NavLink, pagePath } from "./shared";
import { EmojiPicker } from "./EmojiPicker";
import { TeamAccess } from "./TeamAccess";
import { MembersSettings } from "./WorkspaceAccess";
import { api, apiPath, RequestError, Team } from "./shared";

export function SettingsPage({ teams, loading, onSaved }: { teams: Team[]; loading: boolean; onSaved: (team: Team) => void }) {
  const [directory, setDirectory] = useState<Team[] | null>(null);
  const [directoryError, setDirectoryError] = useState("");
  useEffect(() => { let active = true; api<{ teams: Team[] }>(apiPath("/team-directory")).then((result) => { if (active) setDirectory(result.teams); }, (cause: Error) => { if (active) setDirectoryError(cause.message); }); return () => { active = false; }; }, []);
  const entries = directory ?? teams;
  return <><PageHeader actions={<NavLink to={pagePath("/")}>Back to issues</NavLink>}><span>Settings</span></PageHeader><div className="settings-page">
    <MembersSettings />
    <header className="settings-heading"><h1>Teams</h1><p>Manage team names and icons in this workspace.</p></header>
    {directoryError && <RequestError message={directoryError} />}
    {entries.length === 0 ? <p className="empty">{loading ? "Loading teams…" : "No teams yet. Use + beside Teams to create one."}</p> : entries.map((team) => <TeamSettings key={team.key} team={team} onSaved={(updated) => { setDirectory((items) => (items ?? teams).map((item) => item.key === updated.key ? updated : item)); onSaved(updated); }} />)}
  </div></>;
}

function TeamSettings({ team, onSaved }: { team: Team; onSaved: (team: Team) => void }) {
  const [draft, setDraft] = useState<{ name: string; emoji: string } | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(false);
  const mounted = useRef(true);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  const value = draft ?? team;
  const dirty = value.name !== team.name || value.emoji !== team.emoji;

  function edit(change: Partial<{ name: string; emoji: string }>) {
    setDraft({ name: value.name, emoji: value.emoji, ...change });
    setSaved(false);
  }

  async function save(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError("");
    setSaved(false);
    try {
      const updated = await api<Team>(apiPath(`/teams/${encodeURIComponent(team.key)}`), "PATCH", { name: value.name, emoji: value.emoji });
      if (!mounted.current) return;
      onSaved(updated);
      setDraft(null);
      setSaved(true);
    } catch (cause) {
      if (mounted.current) setError((cause as Error).message);
    } finally {
      if (mounted.current) setSaving(false);
    }
  }

  return <section className="team-settings" aria-labelledby={`settings-team-${team.key}`}>
    <h2 id={`settings-team-${team.key}`}>{team.name}</h2>
    <form onSubmit={save}>
      <div className="settings-field"><label htmlFor={`team-name-${team.key}`}>Name</label><div className="settings-name"><EmojiPicker value={value.emoji} teamKey={team.key} label={`Choose icon for ${team.name}`} onChange={(emoji) => edit({ emoji })} disabled={saving} /><input id={`team-name-${team.key}`} value={value.name} onChange={(event) => edit({ name: event.target.value })} required disabled={saving} /></div></div>
      <div className="settings-field"><div><span>Identifier</span><p>Used in issue IDs. Cannot be changed.</p></div><span className="settings-identifier">{team.key}</span></div>
      {error && <RequestError message={error} />}
      <footer><span role="status">{saved ? "Changes saved." : dirty ? "Unsaved changes" : ""}</span><button type="button" className="button" disabled={saving || !dirty} onClick={() => { setDraft(null); setError(""); setSaved(false); }}>Reset</button><button type="submit" className="button primary" disabled={saving || !dirty || !value.name.trim()}>{saving ? "Saving…" : "Save changes"}</button></footer>
    </form>
    {team.membership_enabled && <TeamAccess team={team} onChanged={() => window.location.reload()} />}
  </section>;
}

export default SettingsPage;
