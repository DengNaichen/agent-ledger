import { FormEvent, useState } from "react";
import { api, apiPath, ModalDialog, RequestError } from "./shared";
import { useResource } from "./useResource";
import { useRequestScope } from "./useRequestScope";

export type CycleSettings = { enabled: boolean; timezone: string; start_date: string; duration_weeks: number; upcoming_count: number };
export type Cycle = { timezone: string; id: number; team: string; number: number; name: string; description: string; starts_at: string; ends_at: string; completed_at: string | null; state: "current" | "upcoming" | "completed"; total: number; done: number; canceled: number; rolled_over: number };
export type CyclePage = { settings: CycleSettings; cycles: Cycle[] };
export const emptyCycles: CyclePage = { settings: { enabled: false, timezone: "UTC", start_date: "", duration_weeks: 2, upcoming_count: 2 }, cycles: [] };
export const cycleName = (cycle: Cycle) => cycle.name || `Cycle ${cycle.number}`;
export const cyclePath = (cycle: Cycle) => `/team/${encodeURIComponent(cycle.team)}/cycles/${cycle.id}`;
export function cycleDates(cycle: Cycle) {
  const format = (value: string) => new Date(value).toLocaleDateString(undefined, { timeZone: cycle.timezone, month: "short", day: "numeric", year: "numeric" });
  return `${format(cycle.starts_at)} – ${format(new Date(new Date(cycle.ends_at).getTime() - 1).toISOString())}`;
}

export function CycleSelect({ team, value, onChange, disabled = false, label = "Cycle", filter = false }: { team: string; value: number | ""; onChange: (id: number | "") => void; disabled?: boolean; label?: string; filter?: boolean }) {
  const resource = useResource<CyclePage>(team ? apiPath(`/teams/${encodeURIComponent(team)}/cycles`) : null, emptyCycles);
  const available = resource.data.cycles.filter((c) => filter || (resource.data.settings.enabled && c.state !== "completed") || c.id === value);
  return <span className="cycle-select">
    <select aria-label={label} value={value} disabled={disabled || !team || resource.loading || !!resource.error} onChange={(e) => onChange(e.target.value === "" ? "" : Number(e.target.value))}>
      {filter && <option value="">All cycles</option>}
      <option value={0}>No cycle</option>
      {value !== "" && value !== 0 && !available.some((c) => c.id === value) && <option value={value} disabled>Cycle unavailable</option>}
      {available.map((c) => <option key={c.id} value={c.id}>{cycleName(c)} · {c.state}</option>)}
    </select>
    {resource.error && <><RequestError message={resource.error} /><button type="button" className="ghost" onClick={() => void resource.reload()}>Retry cycles</button></>}
  </span>;
}

export function CycleSettingsDialog({ team, settings, onClose, onSaved }: { team: string; settings: CycleSettings; onClose: () => void; onSaved: (page: CyclePage) => void }) {
  const timezone = settings.start_date ? settings.timezone : Intl.DateTimeFormat().resolvedOptions().timeZone;
  const today = new Intl.DateTimeFormat("en-CA", { timeZone: timezone, year: "numeric", month: "2-digit", day: "2-digit" }).format(new Date());
  const [draft, setDraft] = useState({ ...settings, timezone, start_date: settings.enabled ? settings.start_date : today, enabled: settings.enabled || !settings.start_date });
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const requests = useRequestScope(team);
  async function save(e: FormEvent) {
    e.preventDefault(); setSaving(true); setError("");
    await requests.write(team, () => api<CyclePage>(apiPath(`/teams/${encodeURIComponent(team)}/cycles`), "PUT", draft), onSaved, (cause) => { setError(cause.message); setSaving(false); });
  }
  return <ModalDialog className="issue-dialog" aria-labelledby="cycle-settings-title" restoreFocus onClose={() => { if (!saving) onClose(); }}>
    <form onSubmit={save}>
      <header><h2 id="cycle-settings-title">Cycle settings</h2></header>
      <label className="cycle-toggle"><input type="checkbox" checked={draft.enabled} disabled={saving} onChange={(e) => setDraft({ ...draft, enabled: e.target.checked })} /> Enable cycles</label>
      <label>Timezone<input value={draft.timezone} disabled={saving || settings.enabled} required onChange={(e) => setDraft({ ...draft, timezone: e.target.value })} placeholder="Asia/Shanghai" /></label>
      <label>First day<input type="date" value={draft.start_date} disabled={saving || settings.enabled} required onChange={(e) => setDraft({ ...draft, start_date: e.target.value })} /></label>
      <label>Weeks per cycle<input type="number" min={1} max={8} value={draft.duration_weeks} disabled={saving || settings.enabled} required onChange={(e) => setDraft({ ...draft, duration_weeks: Number(e.target.value) })} /></label>
      <label>Upcoming cycles<input type="number" min={1} max={15} value={draft.upcoming_count} disabled={saving} required onChange={(e) => setDraft({ ...draft, upcoming_count: Number(e.target.value) })} /></label>
      <p className="dialog-hint">Cycles start at midnight in the team timezone. Disable cycles before changing the schedule. Reducing upcoming cycles keeps existing plans.</p>
      {settings.enabled && !draft.enabled && <p className="dialog-hint">Saving will close the current cycle and remove upcoming cycles and their assignments. Completed history is preserved.</p>}
      {error && <RequestError message={error} />}
      <footer><button type="button" className="button" disabled={saving} onClick={onClose}>Cancel</button><button className="button primary" disabled={saving}>{saving ? "Saving…" : "Save changes"}</button></footer>
    </form>
  </ModalDialog>;
}

export function CycleEditor({ cycle, onClose, onSaved }: { cycle: Cycle; onClose: () => void; onSaved: (cycle: Cycle) => void }) {
  const [name, setName] = useState(cycle.name);
  const [description, setDescription] = useState(cycle.description);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const requests = useRequestScope(String(cycle.id));
  async function save(e: FormEvent) { e.preventDefault(); setSaving(true); await requests.write("cycle", () => api<Cycle>(apiPath(`/cycles/${cycle.id}`), "PATCH", { name, description }), onSaved, (cause) => { setError(cause.message); setSaving(false); }); }
  return <ModalDialog className="issue-dialog" aria-labelledby="edit-cycle-title" restoreFocus onClose={() => { if (!saving) onClose(); }}><form onSubmit={save}><header><h2 id="edit-cycle-title">Edit cycle</h2></header><label>Name<input value={name} onChange={(e) => setName(e.target.value)} disabled={saving} placeholder={`Cycle ${cycle.number}`} /></label><label>Description<textarea value={description} onChange={(e) => setDescription(e.target.value)} disabled={saving} rows={4} /></label>{error && <RequestError message={error} />}<footer><button type="button" className="button" disabled={saving} onClick={onClose}>Cancel</button><button className="button primary" disabled={saving}>Save changes</button></footer></form></ModalDialog>;
}
