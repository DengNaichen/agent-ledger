import { UserRound } from "lucide-react";
import { apiPath, Identity, personName, RequestError } from "./shared";
import { useResource } from "./useResource";

type Candidates = { assignees: Identity[] };
type Members = { members: (Identity & { status: string })[] };

export function AssigneeSelect({ team, value, current = null, onChange, disabled, label }: { team: string; value: string; current?: Identity | null; onChange: (value: string) => void; disabled: boolean; label: string }) {
  const { data, loading, error, reload } = useResource<Candidates>(team ? apiPath(`/teams/${encodeURIComponent(team)}/assignees`) : null, { assignees: [] });
  const unavailable = value && !data.assignees.some((person) => person.subject === value);
  const person = data.assignees.find((candidate) => candidate.subject === value) ?? current;
  return <span className={`assignee-control${value ? "" : " assignee-empty"}`}>{person ? <span className="person-avatar" aria-hidden="true">{Array.from(personName(person))[0].toUpperCase()}</span> : <UserRound className="icon" />}<select aria-label={label} value={value} disabled={disabled || loading || !team || !!error} onChange={(event) => onChange(event.target.value)}>
    <option value="">{loading ? "Loading people…" : "Unassigned"}</option>
    {unavailable && <option value={value} disabled>{current ? personName(current) : value} (unavailable)</option>}
    {data.assignees.map((person) => <option key={person.subject} value={person.subject}>{personName(person)}</option>)}
  </select>{error && <><RequestError message={error} /><button type="button" className="ghost" onClick={() => void reload()}>Retry people</button></>}</span>;
}

export function AssigneeFilter({ value, currentLabel, onChange, enabled }: { value: string; currentLabel: string; onChange: (value: string, label: string) => void; enabled: boolean }) {
  const { data, loading, error, reload } = useResource<Members>(enabled ? apiPath("/members") : null, { members: [] });
  return <><label className="toolbar-field">Assignee<select aria-label="Filter by assignee" value={value} onChange={(event) => onChange(event.target.value, event.target.selectedOptions[0].text)}>
    <option value="">All assignees</option><option value="unassigned">Unassigned</option>{enabled && <option value="me">Assigned to me</option>}
    {value.startsWith("user:") && !data.members.some((person) => `user:${person.subject}` === value) && <option value={value} disabled>{currentLabel}</option>}
    {data.members.map((person) => <option key={person.subject} value={`user:${person.subject}`}>{personName(person)}{person.status === "suspended" ? " (suspended)" : ""}</option>)}
  </select></label>{loading && <p>Loading people…</p>}{error && <><RequestError message={error} /><button type="button" className="ghost" onClick={() => void reload()}>Retry people</button></>}</>;
}
