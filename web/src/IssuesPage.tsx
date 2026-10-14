import { Cycle, CyclePage, CycleEditor, CycleSelect, cycleDates, cycleName, emptyCycles } from "./Cycles";
import "./cycles.css";
import { useMemo, useState } from "react";
import { NewIssueDialog } from "./Dialogs";
import { EmojiPicker, EntityEmoji } from "./EmojiPicker";
import { IssueFilters } from "./IssueFilters";
import { IssueViews, ViewMode } from "./IssueViews";
import { LabelPicker } from "./Labels";
import { emptyLabelFilter, LabelFilterValue, useLabels } from "./useLabels";
import { PageHeader } from "./PageHeader";
import { ProjectDialog, ProjectOverview } from "./ProjectManagement";
import { api, apiPath, columns, IssueSummary, NavLink, pagePath, RequestError, Status, Team, Project, Priority, IssueSort, WorkspaceRoute } from "./shared";
import { useIssues } from "./useIssues";
import { useResource } from "./useResource";

type IssueRoute = Extract<WorkspaceRoute, { kind: "issues" | "project" | "cycle" }>;
export default function IssuesPage({ route, teams, teamsLoading, onCreateTeam, view, setView, sort, setSort }: { route: IssueRoute; teams: Team[]; teamsLoading: boolean; onCreateTeam: () => void; view: ViewMode; setView: (view: ViewMode) => void; sort: IssueSort; setSort: (sort: IssueSort) => void }) {
  const catalog = useLabels();
  const [labelFilter, setLabelFilter] = useState<LabelFilterValue>(emptyLabelFilter);
  const [selected, setSelected] = useState<string[]>([]);
  const [batchLabels, setBatchLabels] = useState(false);
  const [error, setError] = useState("");
  const [filter, setFilter] = useState<Status | "all">("all");
  const [teamFilter, setTeamFilter] = useState("");
  const [priority, setPriority] = useState<Priority | "">("");
  const [type, setType] = useState("");
  const [assignee, setAssignee] = useState("");
  const [query, setQuery] = useState("");
  const [collapsed, setCollapsed] = useState<Partial<Record<Status, boolean>>>({});
  const [projectDialog, setProjectDialog] = useState(false);
  const [savedIssue, setSavedIssue] = useState<IssueSummary | null>(null);
  const [cycleFilter, setCycleFilter] = useState<number | "">("");
  const [editingCycle, setEditingCycle] = useState(false);
  const cycle = useResource<Cycle | null>(route.kind === "cycle" ? apiPath(`/cycles/${route.id}`) : null, null);
  const cycleTeam = useResource<CyclePage>(route.kind === "cycle" ? apiPath(`/teams/${encodeURIComponent(route.team)}/cycles`) : null, emptyCycles);
  const [creating, setCreating] = useState(false);
  const scopeTeam = (route.kind === "issues" || route.kind === "cycle") ? route.team : "";
  const team = scopeTeam || teamFilter;
  const project = useResource<Project | null>(route.kind === "project" ? apiPath(`/projects/by-id/${route.id}`) : null, null);
  const projects = useResource<{ projects: Project[] }>(apiPath("/projects"), { projects: [] });
  const activeProject = project.data;
  const { issues, loading: issuesLoading, loadError, savingPriorities, loadIssues, moveIssue: moveIssueRequest, changePriority, writeLabels } = useIssues({ cycleID: route.kind === "cycle" ? route.id : cycleFilter === "" ? undefined : cycleFilter, projectID: route.kind === "project" ? route.id : undefined, team, type, query, priority, labelFilter, assignee }, setError);
  const loading = issuesLoading || project.loading || cycle.loading || teamsLoading;
  const grouped = useMemo(
    () => Object.fromEntries(columns.map(({ status }) => [status, issues.filter((issue) => issue.status === status).sort((a, b) => {
      if (sort === "priority" || (sort === "auto" && status === "agent_ready")) {
        const difference = (a.priority === 0 ? 5 : a.priority) - (b.priority === 0 ? 5 : b.priority);
        if (difference) return difference;
      }
      return new Date(b.updated_at).getTime() - new Date(a.updated_at).getTime();
    })])) as Record<Status, IssueSummary[]>,
    [issues, sort],
  );

  const visibleColumns = columns.filter((column) => filter === "all" || column.status === filter);
  // Listing every status without a search starts with Done and Canceled collapsed; a header click overrides the default until filters change.
  const defaultCollapsed = (status: Status) => filter === "all" && !query && (status === "done" || status === "canceled");
  const isCollapsed = (status: Status) => collapsed[status] ?? defaultCollapsed(status);
  const activeTeam = teams.find((item) => item.key === scopeTeam);
  const heading = (cycle.data ? cycleName(cycle.data) : "") || activeProject?.name || (scopeTeam ? `${activeTeam?.name ?? scopeTeam} / Issues` : "All issues");
  const scopeError = cycle.error || (cycle.data && route.kind === "cycle" && cycle.data.team !== route.team ? "Cycle not found." : "") || project.error || loadError || (!teamsLoading && scopeTeam && !activeTeam ? "Team not found." : "");
  const canCreate = (route.kind !== "cycle" || (cycle.data?.state !== "completed" && cycleTeam.data.settings.enabled && !cycleTeam.error && !cycleTeam.loading)) && !loading && !scopeError && !projects.loading && !projects.error && !activeProject?.archived && (activeProject === null || activeProject.teams.length > 0);
  const selectedVisible = useMemo(() => {
    const visible = new Set(issues.filter((issue) => filter === "all" || issue.status === filter).map((issue) => issue.identifier));
    return selected.filter((id) => visible.has(id));
  }, [selected, issues, filter]);
  function toggleIssue(id: string) { setSelected((ids) => ids.includes(id) ? ids.filter((v) => v !== id) : [...ids, id]); }
  const visibleCount = visibleColumns.reduce((count, column) => count + grouped[column.status].length, 0);

  function changeStatusFilter(status: Status | "all") {
    setFilter(status);
    setCollapsed({});
    setSelected([]);
  }

  function changeTeamFilter(value: string) {
    setTeamFilter(value);
    setCycleFilter("");
    setCollapsed({});
    setSelected([]);
  }

  function submitQuery(value: string) {
    setQuery(value);
    setCollapsed({});
  }


  async function moveIssue(identifier: string, status: Status) {
    await moveIssueRequest(identifier, status);
    await cycle.reload();
  }
  async function addIssue(issue: IssueSummary) {
    setSavedIssue(issue);
    await loadIssues();
    await cycle.reload();
    setCollapsed((current) => ({ ...current, [issue.status]: false }));
    setCreating(false);
  }
  async function saveProject() {
    await Promise.all([project.reload(), projects.reload(), loadIssues()]);
    setProjectDialog(false);
  }
  return <>
    <PageHeader actions={!scopeError && <button className="button" disabled={!canCreate} onClick={() => setCreating(true)}>New issue</button>}>
      {activeProject ? <EmojiPicker value={activeProject.emoji} label={`Change emoji for project ${activeProject.name}`} onChange={async (emoji) => {
        const updated = await api<Project>(apiPath(`/projects/by-id/${activeProject.id}`), "PATCH", { emoji });
        project.update(() => updated);
      }} /> : activeTeam ? <EntityEmoji emoji={activeTeam.emoji} teamKey={activeTeam.key} /> : null}<span>{heading}</span>
    </PageHeader>
        {!scopeError && (
          <IssueFilters assignee={assignee} setAssignee={setAssignee} membershipEnabled={teams.some((item) => item.membership_enabled)} filter={filter} setFilter={changeStatusFilter} team={teamFilter} teams={route.kind === "project" && activeProject ? teams.filter((item) => activeProject.teams.includes(item.key)) : []} setTeam={changeTeamFilter}
            type={type} setType={setType} query={query} submitQuery={submitQuery}
            priority={priority} setPriority={setPriority} sort={sort} setSort={setSort} labelFilter={labelFilter} setLabelFilter={setLabelFilter}
            catalog={catalog} clearSelection={() => setSelected([])} view={view} setView={setView} visibleCount={visibleCount} selectionDisabled={loading} selectVisible={() => setSelected(issues.filter((issue) => filter === "all" || issue.status === filter).map((issue) => issue.identifier))} />
        )}

    {cycle.data && !scopeError && <div className="cycle-overview"><NavLink to={pagePath(`/team/${encodeURIComponent(cycle.data.team)}/cycles`)}>All cycles</NavLink><p>{cycleDates(cycle.data)} · {cycle.data.timezone}</p><p>{cycle.data.state} · {cycle.data.done} / {cycle.data.total} done · {cycle.data.canceled} canceled · {cycle.data.rolled_over} carried over</p>{cycle.data.description && <p>{cycle.data.description}</p>}{cycle.data.state === "completed" && <p>Counts are a completion snapshot. The issues below reflect their current cycle assignment and status.</p>}<button className="button" onClick={() => setEditingCycle(true)}>Edit cycle</button></div>}
    {route.kind !== "cycle" && team && <label className="cycle-filter">Cycle <CycleSelect key={team} team={team} value={cycleFilter} filter onChange={(id) => { setCycleFilter(id); setSelected([]); }} /></label>}
    {editingCycle && cycle.data && <CycleEditor cycle={cycle.data} onClose={() => setEditingCycle(false)} onSaved={(updated) => { cycle.update(() => updated); setEditingCycle(false); }} />}
    {activeProject && <ProjectOverview project={activeProject} teams={teams} onEdit={() => setProjectDialog(true)} onSaved={saveProject} />}
    {cycleTeam.error && <div className="scope-message"><RequestError message={cycleTeam.error} /><button className="button" onClick={() => void cycleTeam.reload()}>Retry cycle settings</button></div>}
    {projects.error && <div className="scope-message"><RequestError message={projects.error} /><button className="button" onClick={() => void projects.reload()}>Retry projects</button></div>}
    {scopeError && <div className="scope-message"><RequestError message={scopeError} /><button className="button" onClick={() => { void loadIssues(); void project.reload(); void cycle.reload(); }}>Retry</button> <NavLink to={pagePath("/")}>Back to all issues</NavLink></div>}
    {savedIssue && <p className="scope-description" role="status">Created {savedIssue.identifier}. <NavLink to={pagePath(`/issue/${savedIssue.identifier}`)}>View issue</NavLink></p>}
    {error && <RequestError message={error} />}
        {!scopeError && !loading && selectedVisible.length > 0 && <div className="selection-toolbar" role="toolbar" aria-label="Selected issues"><button type="button" className="ghost" onClick={() => setSelected(issues.filter((issue) => filter === "all" || issue.status === filter).map((issue) => issue.identifier))}>Select visible</button><span>{selectedVisible.length} selected</span><button className="button" onClick={() => setBatchLabels(true)}>Change labels</button><button className="ghost" onClick={() => setSelected([])}>Clear selection</button></div>}

    {!scopeError && (!loading && teams.length === 0 ? <div className="scope-message"><p>No teams yet. Create a team to start adding issues.</p><button className="button" onClick={onCreateTeam}>Create team</button></div> :
      <IssueViews view={view} visibleColumns={visibleColumns} grouped={grouped} loading={loading} isCollapsed={isCollapsed}
        toggleGroup={(status) => setCollapsed((current) => ({ ...current, [status]: !(current[status] ?? defaultCollapsed(status)) }))}
        actions={{ selected: selectedVisible, toggle: toggleIssue, savingPriorities, changePriority, moveIssue }} />)}
    {projectDialog && activeProject && <ProjectDialog project={activeProject} teams={teams} onClose={() => setProjectDialog(false)} onSaved={saveProject} />}
    {batchLabels && <LabelPicker write={(save, success, failure) => writeLabels(selectedVisible, save, success, failure)} identifiers={selectedVisible} current={null} catalog={catalog} onClose={() => setBatchLabels(false)} onChanged={async () => { await loadIssues(); setSelected([]); }} />}
    {creating && <NewIssueDialog cycleID={route.kind === "cycle" ? route.id : undefined} teams={teams} team={scopeTeam} projects={projects.data.projects} status={filter === "all" ? "triage" : filter} project={activeProject} onClose={() => setCreating(false)} onCreate={addIssue} />}
  </>;
}
