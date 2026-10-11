import { useEffect, useRef, useState } from "react";
import { LabelFilterValue } from "./Labels";
import { api, apiPath, Issue, IssueSummary, Priority, Project, Status, Team, WorkspaceRoute } from "./shared";

export type IssueQuery = { team: string; type: string; query: string; priority: Priority | ""; labelFilter: LabelFilterValue };

// A ScopeView owns this hook and unmounts on navigation. Reads and writes also
// track the local query so an earlier filter cannot restore its issue snapshot.
export function useIssues(route: WorkspaceRoute, { team, type, query, priority, labelFilter }: IssueQuery, setError: (message: string) => void) {
  const [issues, setIssues] = useState<IssueSummary[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [activeProject, setActiveProject] = useState<Project | null>(null);
  const [teams, setTeams] = useState<Team[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [savingPriorities, setSavingPriorities] = useState<string[]>([]);
  const loadVersion = useRef(0);
  const mounted = useRef(true);
  const queryKey = JSON.stringify({ team, type, query, priority, labelFilter });
  const currentQuery = useRef(queryKey);
  currentQuery.current = queryKey;
  const current = () => mounted.current && currentQuery.current === queryKey;

  useEffect(() => {
    mounted.current = true;
    return () => { mounted.current = false; ++loadVersion.current; };
  }, []);
  useEffect(() => {
    const load = () => void loadIssues();
    load();
    window.addEventListener("focus", load);
    return () => { window.removeEventListener("focus", load); ++loadVersion.current; };
  }, [team, type, query, labelFilter, priority]);

  async function loadIssues() {
    if (!current()) return;
    const version = ++loadVersion.current;
    setLoading(true);
    setLoadError("");
    const params = new URLSearchParams({ query });
    if (route.kind === "project") params.set("project_id", String(route.id));
    if (priority !== "") params.set("priority", String(priority));
    if (team) params.set("team", team);
    if (type) params.set("type", type);
    params.set("label_mode", labelFilter.mode);
    labelFilter.labels.forEach((id) => params.append("label_id", String(id)));
    labelFilter.groups.forEach((id) => params.append("label_group_id", String(id)));
    const latest = () => current() && version === loadVersion.current;
    try {
      const [projectData, teamData] = await Promise.all([
        api<{ projects: Project[] }>(apiPath("/projects")),
        api<{ teams: Team[] }>(apiPath("/teams")),
      ]);
      if (!latest()) return;
      setProjects(projectData.projects);
      setTeams(teamData.teams);
      if ((route.kind === "issues" || route.kind === "projects") && route.team && !teamData.teams.some((item) => item.key === route.team)) throw new Error("Team not found.");
      const project = route.kind === "project" ? await api<Project>(apiPath(`/projects/by-id/${route.id}`)) : null;
      if (!latest()) return;
      setActiveProject(project);
      const issueData = route.kind === "issues" || route.kind === "project" ? await api<{ issues: IssueSummary[] }>(apiPath(`/issues?${params}`)) : { issues: [] };
      if (latest()) {
        setIssues(issueData.issues);
        setError("");
      }
    } catch (cause) {
      if (latest()) setLoadError((cause as Error).message);
    } finally {
      if (latest()) setLoading(false);
    }
  }

  function insertIssue(_issue: IssueSummary) {
    // The server applies the full scope and filters to the created issue.
    if (current()) void loadIssues();
  }

  async function changeProjectEmoji(item: Project, emoji: string) {
    const updated = await api<Project>(apiPath(`/projects/by-id/${item.id}`), "PATCH", { emoji });
    if (!current()) return;
    setActiveProject((project) => project?.id === item.id ? updated : project);
    setProjects((items) => items.map((project) => project.id === item.id ? updated : project));
    void loadIssues();
  }

  function replaceTeam(updated: Team) {
    if (!current()) return;
    setTeams((items) => items.map((team) => team.key === updated.key ? updated : team));
    void loadIssues();
  }

  async function changeTeamEmoji(item: Team, emoji: string) {
    const updated = await api<Team>(apiPath(`/teams/${encodeURIComponent(item.key)}`), "PATCH", { emoji });
    replaceTeam(updated);
  }

  async function moveIssue(identifier: string, status: Status) {
    const previous = issues.find((issue) => issue.identifier === identifier)!;
    setIssues((items) => items.map((issue) => issue.identifier === identifier ? { ...issue, status } : issue));
    try {
      const updated = await api<Issue>(apiPath(`/issues/${identifier}/status`), "PATCH", { status });
      if (current()) setIssues((items) => items.map((issue) => issue.identifier === identifier ? updated : issue));
    } catch (cause) {
      if (current()) {
        setIssues((items) => items.map((issue) => issue.identifier === identifier ? previous : issue));
        setError((cause as Error).message);
      }
    }
  }

  async function changePriority(identifier: string, next: Priority) {
    setSavingPriorities((items) => [...items, identifier]);
    setError("");
    try {
      const updated = await api<Issue>(apiPath(`/issues/${identifier}`), "PATCH", { priority: next });
      if (!current()) return;
      setIssues((items) => items.map((issue) => issue.identifier === identifier ? updated : issue));
      if (priority !== "" && updated.priority !== priority) await loadIssues();
    } catch (cause) {
      if (current()) setError((cause as Error).message);
    } finally {
      if (mounted.current) setSavingPriorities((items) => items.filter((id) => id !== identifier));
    }
  }

  return { issues, projects, activeProject, teams, loading, loadError, savingPriorities, loadIssues, insertIssue, replaceTeam, changeProjectEmoji, changeTeamEmoji, moveIssue, changePriority };
}
