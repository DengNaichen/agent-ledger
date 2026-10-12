import { useCallback, useEffect, useState } from "react";
import { api } from "./shared";
import { useRequestScope } from "./useRequestScope";

// A resource has its own loading/error boundary and refreshes on window focus.
export function useResource<T>(path: string | null, initial: T) {
  const [data, setData] = useState(initial);
  const [loading, setLoading] = useState(path !== null);
  const [error, setError] = useState("");
  const requests = useRequestScope(path ?? "");
  const reload = useCallback(async () => {
    if (path === null) return;
    setLoading(true);
    await requests.read(() => api<T>(path), (value) => {
      setData(value); setError(""); setLoading(false);
    }, (cause) => { setError(cause.message); setLoading(false); });
  }, [path, requests]);
  useEffect(() => {
    void reload();
    const focus = () => { void reload(); };
    window.addEventListener("focus", focus);
    return () => window.removeEventListener("focus", focus);
  }, [reload]);
  const update = useCallback((change: (current: T) => T) => {
    requests.invalidate();
    setData(change);
  }, [requests]);
  return { data, loading, error, reload, update };
}
