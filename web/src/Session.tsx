import * as Popover from "@radix-ui/react-popover";
import { ChevronUp, LogOut, UserRound } from "lucide-react";
import { useEffect, useState } from "react";

type Session = { enabled: boolean; user?: { display_name: string; email: string; sub: string } };

export function Session() {
  const [session, setSession] = useState<Session | null>(null);
  const [expired, setExpired] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    const load = async () => {
      try {
        const response = await fetch("/auth/session");
        if (response.status === 401) { setExpired(true); return; }
        if (!response.ok) throw new Error("Could not check your session");
        setSession(await response.json() as Session);
        setExpired(false);
        setError("");
      } catch (cause) { setError((cause as Error).message); }
    };
    const invalidate = () => setExpired(true);
    const recovered = () => setExpired(false);
    void load();
    window.addEventListener("focus", load);
    window.addEventListener("session-expired", invalidate);
    window.addEventListener("session-active", recovered);
    return () => { window.removeEventListener("focus", load); window.removeEventListener("session-expired", invalidate); window.removeEventListener("session-active", recovered); };
  }, []);
  async function logout() {
    setBusy(true);
    try {
      const response = await fetch("/auth/logout", { method: "POST" });
      if (!response.ok) throw new Error("Could not sign out. Please try again.");
      window.location.assign("/auth/signed-out");
    } catch (cause) { setError((cause as Error).message); setBusy(false); }
  }
  if (expired) return <div className="session-bar" role="alert">Session expired. Your unsaved text stays here. <a href={`/auth/login?return_to=${encodeURIComponent(window.location.pathname)}`} target="_blank" rel="noopener noreferrer">Sign in again</a></div>;
  if (!session?.enabled && !error) return null;
  const name = session?.user && (session.user.display_name || session.user.email || session.user.sub);
  return <div className="sidebar-account">
    {session?.user && <Popover.Root>
      <Popover.Trigger asChild><button type="button" className="account-trigger" title={name} aria-label={`Account: ${name}`}><span className="account-avatar" aria-hidden="true"><UserRound className="icon" /></span><span>{name}</span><ChevronUp className="icon" /></button></Popover.Trigger>
      <Popover.Portal><Popover.Content className="account-menu" side="top" align="start" sideOffset={6} collisionPadding={8} aria-label="Account">
        <div className="account-identity"><strong>{name}</strong>{session.user.email && session.user.email !== name && <small>{session.user.email}</small>}</div>
        <button type="button" className="toolbar-menu-action" onClick={() => void logout()} disabled={busy}><LogOut className="icon" />{busy ? "Signing out…" : "Sign out"}</button>
      </Popover.Content></Popover.Portal>
    </Popover.Root>}
    {error && <p className="account-error" role="alert">{error}</p>}
  </div>;
}
