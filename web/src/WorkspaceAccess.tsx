import { useEffect, useRef, useState } from "react";
import { api, apiPath, navigate, RequestError, Workspace } from "./shared";
import { Session } from "./Session";

export type WorkspaceAccess = { enabled: boolean; role: string; subject: string };
type Member = { subject: string; name: string; email: string; role: "admin" | "member"; status: "active" | "suspended" };

export function MembersSettings() {
  const [access, setAccess] = useState<WorkspaceAccess | null>(null);
  const [members, setMembers] = useState<Member[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [invite, setInvite] = useState("");
  const [message, setMessage] = useState("");
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    void (async () => {
      try {
        const value = await api<WorkspaceAccess>(apiPath("/access"));
        if (!mounted.current) return;
        setAccess(value);
        if (value.enabled) {
          const result = await api<{ members: Member[] }>(apiPath("/members"));
          if (mounted.current) setMembers(result.members);
        }
      } catch (cause) { if (mounted.current) setError((cause as Error).message); }
    })();
    return () => { mounted.current = false; };
  }, []);

  async function update(member: Member, change: Partial<Member>) {
    setBusy(true); setError(""); setMessage("");
    const updated = { ...member, ...change };
    try {
      await api(apiPath("/members"), "PATCH", { subject: member.subject, role: updated.role, status: updated.status });
      if (!mounted.current) return;
      if (member.subject === access!.subject) { window.location.reload(); return; }
      setMembers((items) => items.map((item) => item.subject === member.subject ? updated : item));
      setMessage("Member updated.");
    } catch (cause) { if (mounted.current) setError((cause as Error).message); }
    finally { if (mounted.current) setBusy(false); }
  }

  async function resetInvite() {
    setBusy(true); setError(""); setMessage("");
    try {
      const result = await api<{ token: string }>(apiPath("/invite"), "POST");
      if (mounted.current) { setInvite(`${window.location.origin}/auth/join?token=${encodeURIComponent(result.token)}`); setMessage("New invitation ready. Previous links no longer work."); }
    } catch (cause) { if (mounted.current) setError((cause as Error).message); }
    finally { if (mounted.current) setBusy(false); }
  }

  if (access && !access.enabled) return null;
  const admin = access?.role === "admin";
  return <section className="team-settings members-settings" aria-labelledby="members-title">
    <h2 id="members-title">Members</h2>
    <p>Members collaborate across this workspace. Admins manage access and invitations.</p>
    {!access && !error && <p>Loading members…</p>}
    <div className="member-list">{members.map((member) => <div className="member-row" key={member.subject}>
      <div className="member-identity"><strong>{member.name || member.email || member.subject}{member.subject === access?.subject ? " (you)" : ""}</strong><small>{member.email || member.subject}</small></div>
      {admin ? <select aria-label={`Role for ${member.name || member.subject}`} value={member.role} disabled={busy} onChange={(event) => void update(member, { role: event.target.value === "admin" ? "admin" : "member" })}><option value="member">Member</option><option value="admin">Admin</option></select> : <span>{member.role === "admin" ? "Admin" : "Member"}</span>}
      <span>{member.status === "active" ? "Active" : "Suspended"}</span>
      {admin && <button type="button" className="button" disabled={busy} onClick={() => void update(member, { status: member.status === "active" ? "suspended" : "active" })}>{member.status === "active" ? "Suspend" : "Restore"}</button>}
    </div>)}</div>
    {admin && <div className="invite-settings"><h3>Invite members</h3><p>Anyone with the link can sign in and join as a Member. Creating a new link invalidates the previous one.</p><button type="button" className="button" disabled={busy} onClick={() => void resetInvite()}>{busy ? "Saving…" : "Create new invite link"}</button>{invite && <label>Share this invitation<input aria-label="Invitation link" readOnly value={invite} onFocus={(event) => event.target.select()} /></label>}</div>}
    {error && <RequestError message={error} />}
    <p role="status">{message}</p>
  </section>;
}

export function JoinWorkspace() {
  const token = new URLSearchParams(window.location.search).get("token") || "";
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  async function join() {
    setBusy(true); setError("");
    try {
      const workspace = await api<Workspace>("/api/join", "POST", { token });
      window.history.replaceState(null, "", `/${workspace.slug}/`);
      window.dispatchEvent(new PopStateEvent("popstate"));
    } catch (cause) { setError((cause as Error).message); setBusy(false); }
  }
  return <main className="panel first-workspace"><h1>Join workspace</h1><p>This invitation adds your account as a Member.</p>{!token && <p>This invitation is missing its link. Ask an administrator for a new one.</p>}{error && <RequestError message={error} />}<button className="button primary" disabled={busy || !token} onClick={() => void join()}>{busy ? "Joining…" : "Accept invitation"}</button><button className="button" disabled={busy} onClick={() => navigate("/")}>Your workspaces</button><Session /></main>;
}
