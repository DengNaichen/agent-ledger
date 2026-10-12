import { useEffect, useMemo, useRef, useState } from "react";
import { CreateTeamDialog, NewIssueDialog } from "./Dialogs";
import { EmojiPicker, EntityEmoji } from "./EmojiPicker";
import { IssueDetail } from "./IssueDetail";
import { IssueFilters } from "./IssueFilters";
import { IssueViews, ViewMode } from "./IssueViews";
import { emptyLabelFilter, LabelFilterValue, LabelManager, LabelPicker, useLabels } from "./Labels";
import { RequestError, api, columns, IssueSummary, navigate, NavLink, pagePath, Status, Team, Project, Workspace, workspaceSlug, Priority, IssueSort, WorkspaceRoute, workspaceRoute } from "./shared";
import { ProjectDialog, ProjectOverview } from "./ProjectManagement";
import { ProjectList } from "./ProjectList";
import { Sidebar } from "./Sidebar";
import { SettingsPage } from "./SettingsPage";
import { useIssues } from "./useIssues";

// The first path segment names the workspace; the root opens the first one.
function App() {
  const [slug, setSlug] = useState(workspaceSlug());

  useEffect(() => {
    const sync = () => setSlug(workspaceSlug());
    window.addEventListener("popstate", sync);
    return () => window.removeEventListener("popstate", sync);
  }, []);

  return slug ? <WorkspaceView key={slug} /> : <FirstWorkspace />;
}

// The root names no workspace: it replaces itself with the first workspace by
// slug, or explains how to create one when there is none.
function FirstWorkspace() {
  const [empty, setEmpty] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    api<{ workspaces: Workspace[] }>("/api/workspaces").then(({ workspaces }) => {
      if (workspaces.length === 0) {
        setEmpty(true);
        return;
      }
      window.history.replaceState(null, "", `/${workspaces[0].slug}/`);
      window.dispatchEvent(new PopStateEvent("popstate"));
    }, (cause: Error) => setError(cause.message));
  }, []);

  return (
    <main className="panel first-workspace">
      {error ? <RequestError message={error} /> : empty ? (
        <p>No workspace yet. Create one with <code>POST /api/workspaces</code> and a body such as <code>{'{"slug":"default","name":"Default"}'}</code>, then reload this page.</p>
      ) : <p className="empty">Loading…</p>}
    </main>
  );
}

// Preferences stay within the workspace; every navigation gets fresh local
// filters, selection, and pending callbacks, even when revisiting the same URL.
function WorkspaceView() {
  const [location, setLocation] = useState({ path: window.location.pathname, visit: 0 });
  const [view, setView] = useState<ViewMode>("list");
  const [sort, setSort] = useState<IssueSort>("auto");
  useEffect(() => {
    const sync = () => setLocation((previous) => ({ path: window.location.pathname, visit: previous.visit + 1 }));
    window.addEventListener("popstate", sync);
    return () => window.removeEventListener("popstate", sync);
  }, []);
  return <ScopeView key={location.visit} route={workspaceRoute(location.path)} view={view} setView={setView} sort={sort} setSort={setSort} />;
}

function ScopeView({ route, view, setView, sort, setSort }: { route: WorkspaceRoute; view: ViewMode; setView: (view: ViewMode) => void; sort: IssueSort; setSort: (sort: IssueSort) => void }) {
  const catalog = useLabels();
  const [labelFilter, setLabelFilter] = useState<LabelFilterValue>(emptyLabelFilter);
  const [selected, setSelected] = useState<string[]>([]);
  const [batchLabels, setBatchLabels] = useState(false);
  const [error, setError] = useState("");
  const [filter, setFilter] = useState<Status | "all">("all");
  const [teamFilter, setTeamFilter] = useState("");
  const [priority, setPriority] = useState<Priority | "">("");
  const [type, setType] = useState("");
  const [query, setQuery] = useState("");
  const [collapsed, setCollapsed] = useState<Partial<Record<Status, boolean>>>({});
  const [projectDialog, setProjectDialog] = useState<Project | "new" | null>(null);
  const [archivedProjects, setArchivedProjects] = useState(false);
  const [createdProject, setCreatedProject] = useState<Project | null>(null);
  const [savedIssue, setSavedIssue] = useState<{ issue: IssueSummary; action: "Created" | "Updated" } | null>(null);
  const [dialog, setDialog] = useState<"new" | "edit" | "team" | null>(null);
  const mounted = useRef(true);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; }; }, []);
  const scopeTeam = route.kind === "issues" || route.kind === "projects" ? route.team : "";
  const team = scopeTeam || teamFilter;
  const managingSettings = route.kind === "settings";
  const managingLabels = route.kind === "labels";
  const listingProjects = route.kind === "projects";
  const listingIssues = route.kind === "issues" || route.kind === "project";
  const detailIdentifier = route.kind === "issue" ? route.identifier : "";

  const { issues, projects, activeProject, teams, loading, loadError, savingPriorities, loadIssues, insertIssue, replaceTeam, changeProjectEmoji, moveIssue, changePriority } = useIssues(route, { archivedProjects, team, type, query, priority, labelFilter }, setError);

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
  const heading = managingSettings ? "Settings" : managingLabels ? "Labels" : activeProject?.name || (scopeTeam ? `${activeTeam?.name ?? scopeTeam} / ${listingProjects ? "Projects" : "Issues"}` : listingProjects ? "All projects" : "All issues");
  const scopeError = route.kind === "invalid" ? "This page does not exist." : loadError;
  const blockingError = !detailIdentifier && !managingLabels && !(managingSettings && teams.length > 0) ? scopeError : "";
  const canCreate = listingIssues && !loading && !scopeError && !activeProject?.archived && (activeProject === null || activeProject.teams.length > 0);
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
    setCollapsed({});
    setSelected([]);
  }

  function submitQuery(value: string) {
    setQuery(value);
    setCollapsed({});
  }

  async function addIssue(issue: IssueSummary) {
    setSavedIssue({ issue, action: "Created" });
    await insertIssue(issue);
    if (!mounted.current) return;
    setCollapsed((current) => ({ ...current, [issue.status]: false }));
    setDialog(null);
  }

  function addTeam(created: Team) {
    if (mounted.current) navigate(pagePath(`/team/${encodeURIComponent(created.key)}/issues`));
  }

  return (
    <div className="app-shell">
      <Sidebar route={route} teams={teams} onCreateTeam={() => setDialog("team")} />

      <main className="panel">
        <header className="header-row">
          <nav className="breadcrumb" aria-label="Breadcrumb">
            {!managingLabels && (activeProject ? <EmojiPicker value={activeProject.emoji} label={`Change emoji for project ${activeProject.name}`} onChange={(emoji) => changeProjectEmoji(activeProject, emoji)} /> : activeTeam ? <EntityEmoji emoji={activeTeam.emoji} teamKey={activeTeam.key} /> : null)}
            {detailIdentifier ? (
              <><NavLink to={pagePath("/")}>{heading}</NavLink><span className="separator">/</span><span>{detailIdentifier}</span></>
            ) : (
              <span>{heading}</span>
            )}
          </nav>
          {blockingError ? null : managingLabels || managingSettings ? <NavLink to={pagePath("/")}>Back to issues</NavLink> : detailIdentifier ? (
            <button type="button" className="button" onClick={() => setDialog("edit")}>Edit</button>
          ) : listingIssues ? (
            <button type="button" className="button" disabled={!canCreate} onClick={() => setDialog("new")}>New issue</button>
          ) : listingProjects ? <button type="button" className="button" disabled={loading} onClick={() => setProjectDialog("new")}>New project</button> : null}
        </header>

        {listingIssues && !scopeError && (
          <IssueFilters filter={filter} setFilter={changeStatusFilter} team={teamFilter} teams={route.kind === "project" && activeProject ? teams.filter((item) => activeProject.teams.includes(item.key)) : []} setTeam={changeTeamFilter}
            type={type} setType={setType} query={query} submitQuery={submitQuery}
            priority={priority} setPriority={setPriority} sort={sort} setSort={setSort} labelFilter={labelFilter} setLabelFilter={setLabelFilter}
            catalog={catalog} clearSelection={() => setSelected([])} view={view} setView={setView} visibleCount={visibleCount} selectionDisabled={loading} selectVisible={() => setSelected(issues.filter((issue) => filter === "all" || issue.status === filter).map((issue) => issue.identifier))} />
        )}
        {activeProject && <ProjectOverview project={activeProject} teams={teams} onEdit={() => setProjectDialog(activeProject)} onSaved={loadIssues} />}
        {listingProjects && <div className="project-actions project-list-toolbar"><button type="button" className="button" aria-pressed={!archivedProjects} onClick={() => setArchivedProjects(false)}>Active projects</button><button type="button" className="button" aria-pressed={archivedProjects} onClick={() => setArchivedProjects(true)}>Archived projects</button></div>}
        {createdProject && <p className="scope-description" role="status">Created {createdProject.name}. <NavLink to={pagePath(`/project/${createdProject.id}`)}>View project</NavLink></p>}

        {scopeError && <div className="scope-message"><RequestError message={scopeError} /><button type="button" className="button" onClick={() => void loadIssues()}>Retry</button> <NavLink to={pagePath("/")}>Back to all issues</NavLink></div>}

        {savedIssue && <p className="scope-description" role="status">{savedIssue.action} {savedIssue.issue.identifier}. <NavLink to={pagePath(`/issue/${savedIssue.issue.identifier}`)}>View issue</NavLink></p>}
        {error && <RequestError message={error} />}

        {listingIssues && !scopeError && !loading && selectedVisible.length > 0 && <div className="selection-toolbar" role="toolbar" aria-label="Selected issues"><button type="button" className="ghost" onClick={() => setSelected(issues.filter((issue) => filter === "all" || issue.status === filter).map((issue) => issue.identifier))}>Select visible</button><span>{selectedVisible.length} selected</span><button className="button" onClick={() => setBatchLabels(true)}>Change labels</button><button className="ghost" onClick={() => setSelected([])}>Clear selection</button></div>}
        {blockingError ? null : managingSettings ? <SettingsPage teams={teams} loading={loading} onSaved={replaceTeam} /> : managingLabels ? <LabelManager catalog={catalog} onMerged={() => setLabelFilter(emptyLabelFilter)} /> : detailIdentifier ? (
          <IssueDetail teams={teams} onSaved={async (issue) => { setSavedIssue({ issue, action: "Updated" }); await loadIssues(); }} catalog={catalog} key={detailIdentifier} identifier={detailIdentifier} setError={setError} editing={dialog === "edit"} closeEditor={() => setDialog(null)} />
        ) : listingProjects ? (
          <ProjectList projects={projects.filter((item) => item.archived === archivedProjects && (!scopeTeam || item.teams.includes(scopeTeam)))} teams={teams} loading={loading} archived={archivedProjects} />
        ) : listingIssues && !loading && teams.length === 0 ? (
          <div className="scope-message"><p>No teams yet. Create a team to start adding issues.</p><button type="button" className="button" onClick={() => setDialog("team")}>Create team</button></div>
        ) : (
          <IssueViews view={view} visibleColumns={visibleColumns} grouped={grouped} loading={loading} isCollapsed={isCollapsed}
            toggleGroup={(status) => setCollapsed((current) => ({ ...current, [status]: !(current[status] ?? defaultCollapsed(status)) }))}
            actions={{ selected: selectedVisible, toggle: toggleIssue, savingPriorities, changePriority, moveIssue }} />
        )}
      </main>

      {projectDialog && <ProjectDialog project={projectDialog === "new" ? undefined : projectDialog} initialTeam={scopeTeam} teams={teams} onClose={() => setProjectDialog(null)} onSaved={async (saved) => {
        if (projectDialog === "new") { setCreatedProject(saved); setArchivedProjects(false); }
        await loadIssues();
        if (mounted.current) setProjectDialog(null);
      }} />}
      {batchLabels && <LabelPicker identifiers={selectedVisible} current={null} catalog={catalog} onClose={() => setBatchLabels(false)} onChanged={async () => { await loadIssues(); setSelected([]); }} />}
      {dialog === "new" && <NewIssueDialog teams={teams} team={scopeTeam} projects={projects} status={filter === "all" ? "triage" : filter} project={activeProject} onClose={() => setDialog(null)} onCreate={addIssue} />}
      {dialog === "team" && <CreateTeamDialog onClose={() => setDialog(null)} onCreate={addTeam} />}
    </div>
  );
}

export default App;
