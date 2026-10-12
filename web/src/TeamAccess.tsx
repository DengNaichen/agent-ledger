import { useEffect, useRef, useState } from "react";
import { api, apiPath, NavLink, pagePath, RequestError, Team } from "./shared";

type Member = { workspace_admin: boolean; subject: string; name: string; email: string; role: string; status: string };

type MemberDirectory = { members: Member[] };

export function TeamAccess({ team, onChanged }: { team: Team; onChanged: () => void }) {
  const mounted = useRef(true);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  const [members, setMembers] = useState<Member[]>([]);
  const [candidates, setCandidates] = useState<Member[]>([]);
  const [subject, setSubject] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [confirmJoin, setConfirmJoin] = useState(false);
  const [version, setVersion] = useState(0);
  const path = `/teams/${encodeURIComponent(team.key)}`;
  useEffect(() => {
    let active = true;
    void (async () => {
      try {
        const result = await api<MemberDirectory>(apiPath(path + "/members"));
        if (active) setMembers(result.members);
        if (team.can_manage) {
          const workspace = await api<MemberDirectory>(apiPath("/members"));
          if (active) setCandidates(workspace.members.filter((item) => item.status === "active" && !result.members.some((member) => member.subject === item.subject)));
        }
      } catch (cause) { if (active) setError((cause as Error).message); }
    })();
    return () => { active = false; };
  }, [path, team.can_manage, version]);

  async function save(suffix: string, method: string, input: object) {
    setBusy(true); setError("");
    try {
      await api(apiPath(path + suffix), method, input);
      if (!mounted.current) return;
      setSubject(""); setConfirmJoin(false); setVersion((value) => value + 1); onChanged();
    } catch (cause) { if (mounted.current) setError((cause as Error).message); }
    finally { if (mounted.current) setBusy(false); }
  }
  return <div className="team-access">
    <div className="team-access-actions"><strong>{team.private ? "Private team" : "Public team"}</strong>
      {team.accessible && <NavLink to={pagePath(`/team/${encodeURIComponent(team.key)}/issues`)}>View issues</NavLink>}
      {team.can_manage && <button className="button" disabled={busy} onClick={() => void save("", "PATCH", { private: !team.private })}>{team.private ? "Make public" : "Make private"}</button>}
      <button className="button" disabled={busy} onClick={() => {
        if (team.private && !team.joined) setConfirmJoin(true);
        else void save("/membership", "POST", { leave: team.joined });
      }}>{team.joined ? "Leave team" : "Join team"}</button>
    </div>
    {confirmJoin && <div className="team-join-confirm" role="alert"><p>Joining this private team gives you access to its private issues and projects.</p><button className="button primary" disabled={busy} onClick={() => void save("/membership", "POST", { confirm_private: true })}>Confirm private access</button><button className="button" disabled={busy} onClick={() => setConfirmJoin(false)}>Cancel</button></div>}
    <p>{team.private ? "Only team members can access these issues. Leaving requires an owner to add you again." : "Workspace members can collaborate without joining. Join to keep this team in your sidebar."}</p>
    <h3>Team members</h3>
    {members.length === 0 && <p>No team members.</p>}
    {members.map((member) => <div className="member-row" key={member.subject}>
      <div className="member-identity"><strong>{member.name || member.email || member.subject}</strong><small>{member.status === "suspended" ? "Suspended" : member.email}</small></div>
      <span>{member.workspace_admin ? "Workspace admin" : member.role === "owner" ? "Team owner" : "Member"}</span>
      {team.can_manage && <><button className="button" disabled={busy || member.workspace_admin || member.status !== "active"} onClick={() => void save("/members", "PATCH", { subject: member.subject, role: member.role === "owner" ? "member" : "owner" })}>{member.role === "owner" ? "Make member" : "Make owner"}</button><button className="button" disabled={busy} onClick={() => void save("/members", "PATCH", { subject: member.subject, remove: true })}>Remove</button></>}
    </div>)}
    {team.can_manage && <form className="team-member-add" onSubmit={(event) => { event.preventDefault(); void save("/members", "PATCH", { subject, role: "member" }); }}><label>Add workspace member<select required disabled={busy} value={subject} onChange={(event) => setSubject(event.target.value)}><option value="">Choose a member</option>{candidates.map((candidate) => <option key={candidate.subject} value={candidate.subject}>{candidate.name || candidate.email || candidate.subject}</option>)}</select></label><button className="button" disabled={busy || !subject}>Add member</button></form>}
    {error && <RequestError message={error} />}
  </div>;
}
