import { useEffect, useRef, useState } from "react";
import { LabelFilterValue } from "./Labels";
import { api, apiPath, Issue, IssueSummary, Priority, Project, Status, Team } from "./shared";

export type IssueQuery = { team: string; project: string; type: string; query: string; priority: Priority | ""; labelFilter: LabelFilterValue };

// useIssues keeps the page's copy of the workspace's issues matching filters,
// with its Projects and teams. It reloads when path or a filter changes and
// when the window regains focus, and applies writes to that copy.
export function useIssues(path: string, { team, project, type, query, priority, labelFilter }: IssueQuery, setError: (message: string) => void) {
  const [issues, setIssues] = useState<IssueSummary[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [teams, setTeams] = useState<Team[]>([]);
  const [loading, setLoading] = useState(true);
  const [savingPriorities, setSavingPriorities] = useState<string[]>([]);
  const loadVersion = useRef(0);

  useEffect(() => {
    const load = () => void loadIssues();
    load();
    window.addEventListener("focus", load);
    return () => window.removeEventListener("focus", load);
  }, [path, team, project, type, query, labelFilter, priority]);

  async function loadIssues() {
    const version = ++loadVersion.current;
    const params = new URLSearchParams({ project, query });
    if (priority !== "") params.set("priority", String(priority));
    if (team) params.set("team", team);
    if (type) params.set("type", type);
    params.set("label_mode", labelFilter.mode);
    labelFilter.labels.forEach((id) => params.append("label_id", String(id)));
    labelFilter.groups.forEach((id) => params.append("label_group_id", String(id)));
    try {
      const [issueData, projectData, teamData] = await Promise.all([
        api<{ issues: IssueSummary[] }>(apiPath(`/issues?${params}`)),
        api<{ projects: Project[] }>(apiPath("/projects")),
        api<{ teams: Team[] }>(apiPath("/teams")),
      ]);
      if (version === loadVersion.current) {
        setIssues(issueData.issues);
        setProjects(projectData.projects);
        setTeams(teamData.teams);
        setError("");
      }
    } catch (cause) {
      if (version === loadVersion.current) setError((cause as Error).message);
    } finally {
      if (version === loadVersion.current) setLoading(false);
    }
  }

  // A new issue outside the team, priority, or specific label filter reloads so the filters decide whether it shows.
  function insertIssue(issue: IssueSummary) {
    const inTeam = !team || issue.identifier.startsWith(`${team}-`);
    if (inTeam && (priority === "" || issue.priority === priority) && (labelFilter.mode === "unlabeled" || (labelFilter.labels.length === 0 && labelFilter.groups.length === 0))) setIssues((current) => [issue, ...current]);
    else void loadIssues();
  }

  function insertTeam(created: Team) {
    setTeams((current) => [...current, created].sort((a, b) => a.key.localeCompare(b.key)));
  }

  async function changeProjectEmoji(item: Project, emoji: string) {
    const updated = await api<Project>(apiPath(`/projects/${encodeURIComponent(item.name)}`), "PATCH", { emoji });
    setProjects((current) => current.map((project) => project.name === item.name ? updated : project));
  }

  async function changeTeamEmoji(item: Team, emoji: string) {
    const updated = await api<Team>(apiPath(`/teams/${encodeURIComponent(item.key)}`), "PATCH", { emoji });
    setTeams((current) => current.map((team) => team.key === item.key ? updated : team));
  }

  // The status shows at once; a failed write restores the issues as they were before it.
  async function moveIssue(identifier: string, status: Status) {
    const previous = issues;
    setIssues((current) => current.map((issue) => (issue.identifier === identifier ? { ...issue, status } : issue)));
    try {
      const updated = await api<Issue>(apiPath(`/issues/${identifier}/status`), "PATCH", { status });
      setIssues((current) => current.map((issue) => (issue.identifier === identifier ? updated : issue)));
    } catch (cause) {
      setIssues(previous);
      setError((cause as Error).message);
    }
  }

  async function changePriority(identifier: string, next: Priority) {
    setSavingPriorities((current) => [...current, identifier]);
    setError("");
    try {
      const updated = await api<Issue>(apiPath(`/issues/${identifier}`), "PATCH", { priority: next });
      setIssues((current) => current.map((issue) => issue.identifier === identifier ? updated : issue));
      if (priority !== "" && updated.priority !== priority) await loadIssues();
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setSavingPriorities((current) => current.filter((id) => id !== identifier));
    }
  }

  return { issues, projects, teams, loading, savingPriorities, loadIssues, insertIssue, insertTeam, changeProjectEmoji, changeTeamEmoji, moveIssue, changePriority };
}
