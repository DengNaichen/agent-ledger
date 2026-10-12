import { useCallback, useEffect, useRef, useState } from "react";
import { LabelFilterValue } from "./useLabels";
import { api, apiPath, Issue, IssueSummary, Priority, Status } from "./shared";
import { useRequestScope } from "./useRequestScope";

type IssueList = { issues: IssueSummary[] };

export type IssueQuery = { projectID?: number; team: string; type: string; query: string; priority: Priority | ""; labelFilter: LabelFilterValue };

export function useIssues({ projectID, team, type, query, priority, labelFilter }: IssueQuery, setError: (message: string) => void) {
  const confirmedStatuses = useRef(new Map<string, Status>());
  const [issues, setIssues] = useState<IssueSummary[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [savingPriorities, setSavingPriorities] = useState<string[]>([]);
  const params = new URLSearchParams({ query, label_mode: labelFilter.mode });
  if (projectID !== undefined) params.set("project_id", String(projectID));
  if (team) params.set("team", team);
  if (type) params.set("type", type);
  if (priority !== "") params.set("priority", String(priority));
  labelFilter.labels.forEach((id) => params.append("label_id", String(id)));
  labelFilter.groups.forEach((id) => params.append("label_group_id", String(id)));
  const path = apiPath(`/issues?${params}`);
  // The write queue belongs to the mounted page, across changes to its filters.
  const requests = useRequestScope(apiPath("/issues"));
  const queryScope = useRequestScope(path);

  const loadIssues = useCallback(async () => {
    if (!queryScope.isActive()) return;
    setLoading(true);
    await requests.read(
      () => api<IssueList>(path),
      (data) => { if (!queryScope.isActive()) return; data.issues.forEach((issue) => confirmedStatuses.current.set(issue.identifier, issue.status)); setIssues(data.issues); setLoadError(""); setLoading(false); },
      (cause) => { if (!queryScope.isActive()) return; setLoadError(cause.message); setLoading(false); },
    );
  }, [path, queryScope, requests]);
  useEffect(() => {
    // Changing a filter invalidates reads for the previous query as well.
    requests.invalidate();
    void loadIssues();
    const load = () => { void loadIssues(); };
    window.addEventListener("focus", load);
    return () => { requests.invalidate(); window.removeEventListener("focus", load); };
  }, [loadIssues, requests]);

  async function moveIssue(identifier: string, status: Status) {
    const previous = issues.find((issue) => issue.identifier === identifier)?.status;
    setIssues((items) => items.map((issue) => issue.identifier === identifier ? { ...issue, status } : issue));
    const url = apiPath(`/issues/${identifier}/status`);
    await requests.write(identifier, () => api<Issue>(url, "PATCH", { status }),
      (updated) => { confirmedStatuses.current.set(identifier, updated.status); if (queryScope.isActive()) setIssues((items) => items.map((issue) => issue.identifier === identifier ? updated : issue)); },
      (cause) => {
        if (!queryScope.isActive()) return;
        const restored = confirmedStatuses.current.get(identifier) ?? previous;
        if (restored) setIssues((items) => items.map((issue) => issue.identifier === identifier ? { ...issue, status: restored } : issue));
        setError(cause.message);
      });
    await loadIssues();
  }

  async function changePriority(identifier: string, next: Priority) {
    setSavingPriorities((items) => [...items, identifier]);
    setError("");
    const url = apiPath(`/issues/${identifier}`);
    await requests.write(identifier, () => api<Issue>(url, "PATCH", { priority: next }),
      (updated) => { confirmedStatuses.current.set(identifier, updated.status); if (queryScope.isActive()) setIssues((items) => items.map((issue) => issue.identifier === identifier ? updated : issue)); },
      (cause) => { if (queryScope.isActive()) setError(cause.message); });
    setSavingPriorities((items) => items.filter((id) => id !== identifier));
    await loadIssues();
  }

  return { issues, loading, loadError, savingPriorities, loadIssues, moveIssue, changePriority, writeLabels: requests.write };
}
