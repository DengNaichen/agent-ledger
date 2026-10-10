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
  return <div className="session-bar">
    {session?.user && <><span>{session.user.display_name || session.user.email || session.user.sub}</span><button className="button" type="button" onClick={() => void logout()} disabled={busy}>Sign out</button></>}
    {error && <span role="alert">{error}</span>}
  </div>;
}
