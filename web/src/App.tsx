import { Pencil } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { CreateTeamDialog, NewIssueDialog, RenameTeamDialog } from "./Dialogs";
import { EmojiPicker } from "./EmojiPicker";
import { IssueDetail } from "./IssueDetail";
import { IssueFilters } from "./IssueFilters";
import { IssueViews, ViewMode } from "./IssueViews";
import { emptyLabelFilter, LabelFilterValue, LabelManager, LabelPicker, useLabels } from "./Labels";
import { RequestError, api, columns, IssueSummary, navigate, NavLink, pagePath, Status, Team, Workspace, workspaceSlug, Priority, IssueSort } from "./shared";
import { Sidebar } from "./Sidebar";
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

// WorkspaceView shows the workspace named by the first path segment; the rest
// of the path selects the list, an issue, or the label manager. It holds the
// filter, view, selection, and dialog state and passes it to the page parts.
function WorkspaceView() {
  const catalog = useLabels();
  const [labelFilter, setLabelFilter] = useState<LabelFilterValue>(emptyLabelFilter);
  const [selected, setSelected] = useState<string[]>([]);
  const [batchLabels, setBatchLabels] = useState(false);
  const [path, setPath] = useState(window.location.pathname);
  const [error, setError] = useState("");
  const [view, setView] = useState<ViewMode>("list");
  const [filter, setFilter] = useState<Status | "all">("all");
  const [project, setProject] = useState("");
  const [renamingTeam, setRenamingTeam] = useState<Team | null>(null);
  const [team, setTeam] = useState("");
  const [priority, setPriority] = useState<Priority | "">("");
  const [sort, setSort] = useState<IssueSort>("auto");
  const [type, setType] = useState("");
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [collapsed, setCollapsed] = useState<Partial<Record<Status, boolean>>>({});
  const [dialog, setDialog] = useState<"new" | "edit" | "team" | null>(null);
  const route = "/" + path.split("/").slice(2).join("/");
  const managingLabels = route === "/labels";
  const detailIdentifier = route.startsWith("/issue/") ? route.slice("/issue/".length) : "";

  useEffect(() => {
    const sync = () => {
      setPath(window.location.pathname);
      setError("");
      setDialog(null);
    };
    window.addEventListener("popstate", sync);
    return () => window.removeEventListener("popstate", sync);
  }, []);

  const { issues, projects, teams, loading, savingPriorities, loadIssues, insertIssue, insertTeam, replaceTeam, changeProjectEmoji, changeTeamEmoji, moveIssue, changePriority } = useIssues(path, { team, project, type, query, priority, labelFilter }, setError);

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
  const heading = managingLabels ? "Labels" : project || (team && teams.find((item) => item.key === team)!.name) || "All issues";
  const selectedVisible = selected.filter((id) => issues.some((issue) => issue.identifier === id && (filter === "all" || issue.status === filter)));
  function toggleIssue(id: string) { setSelected((ids) => ids.includes(id) ? ids.filter((v) => v !== id) : [...ids, id]); }
  const visibleCount = visibleColumns.reduce((count, column) => count + grouped[column.status].length, 0);

  function selectFilter(status: Status | "all", nextProject: string, nextTeam = team) {
    setFilter(status);
    setProject(nextProject);
    setTeam(nextTeam);
    setCollapsed({});
    setSelected([]);
    if (route !== "/") navigate(pagePath("/"));
  }

  function submitQuery(value: string) {
    setQuery(value);
    setCollapsed({});
  }

  function addIssue(issue: IssueSummary) {
    insertIssue(issue);
    setCollapsed((current) => ({ ...current, [issue.status]: false }));
    setDialog(null);
  }

  const activeProject = projects.find((item) => item.name === project);
  const activeTeam = teams.find((item) => item.key === team);

  function addTeam(created: Team) {
    insertTeam(created);
    selectFilter("all", "", created.key);
    setDialog(null);
  }

  return (
    <div className="app-shell">
      <Sidebar issueCount={issues.length} grouped={grouped} teams={teams} projects={projects} filter={filter} project={project} team={team} managingLabels={managingLabels} selectFilter={selectFilter} onCreateTeam={() => setDialog("team")} onRenameTeam={setRenamingTeam} changeTeamEmoji={changeTeamEmoji} changeProjectEmoji={changeProjectEmoji} />

      <main className="panel">
        <header className="header-row">
          <nav className="breadcrumb" aria-label="Breadcrumb">
            {!managingLabels && (activeProject ? <EmojiPicker value={activeProject.emoji} label={`Change emoji for project ${activeProject.name}`} onChange={(emoji) => changeProjectEmoji(activeProject, emoji)} /> : activeTeam ? <EmojiPicker value={activeTeam.emoji} teamKey={activeTeam.key} label={`Change emoji for team ${activeTeam.name}`} onChange={(emoji) => changeTeamEmoji(activeTeam, emoji)} /> : null)}
            {detailIdentifier ? (
              <><NavLink to={pagePath("/")}>{heading}</NavLink><span className="separator">/</span><span>{detailIdentifier}</span></>
            ) : (
              <span>{heading}</span>
            )}
            {!managingLabels && activeTeam && !activeProject && <button type="button" className="ghost" aria-label={`Rename team ${activeTeam.name}`} title="Rename team" onClick={() => setRenamingTeam(activeTeam)}><Pencil className="icon" /></button>}
          </nav>
          {managingLabels ? <NavLink to={pagePath("/")}>Back to issues</NavLink> : detailIdentifier ? (
            <button type="button" className="button" onClick={() => setDialog("edit")}>Edit</button>
          ) : (
            <button type="button" className="button" onClick={() => setDialog("new")}>New issue</button>
          )}
        </header>

        {!detailIdentifier && !managingLabels && (
          <IssueFilters filter={filter} team={team} activeTeam={activeTeam} project={project} activeProject={activeProject} selectFilter={selectFilter}
            type={type} setType={setType} search={search} setSearch={setSearch} query={query} submitQuery={submitQuery}
            priority={priority} setPriority={setPriority} sort={sort} setSort={setSort} labelFilter={labelFilter} setLabelFilter={setLabelFilter}
            catalog={catalog} clearSelection={() => setSelected([])} view={view} setView={setView} visibleCount={visibleCount} />
        )}

        {error && <RequestError message={error} />}

        {!detailIdentifier && !managingLabels && <div className="label-bulk-bar"><button className="ghost" onClick={() => setSelected(issues.filter((issue) => filter === "all" || issue.status === filter).map((issue) => issue.identifier))}>Select visible</button>{selectedVisible.length > 0 && <><span>{selectedVisible.length} selected</span><button className="button" onClick={() => setBatchLabels(true)}>Change labels</button><button className="ghost" onClick={() => setSelected([])}>Clear selection</button></>}</div>}
        {managingLabels ? <LabelManager catalog={catalog} onMerged={() => setLabelFilter(emptyLabelFilter)} /> : detailIdentifier ? (
          <IssueDetail catalog={catalog} key={detailIdentifier} identifier={detailIdentifier} setError={setError} editing={dialog === "edit"} closeEditor={() => setDialog(null)} />
        ) : (
          <IssueViews view={view} visibleColumns={visibleColumns} grouped={grouped} loading={loading} isCollapsed={isCollapsed}
            toggleGroup={(status) => setCollapsed((current) => ({ ...current, [status]: !(current[status] ?? defaultCollapsed(status)) }))}
            actions={{ selected: selectedVisible, toggle: toggleIssue, savingPriorities, changePriority, moveIssue }} />
        )}
      </main>

      {batchLabels && <LabelPicker identifiers={selectedVisible} current={null} catalog={catalog} onClose={() => setBatchLabels(false)} onChanged={async () => { await loadIssues(); setSelected([]); }} />}
      {dialog === "new" && <NewIssueDialog teams={teams} team={team} status={filter === "all" ? "triage" : filter} project={project} onClose={() => setDialog(null)} onCreate={addIssue} />}
      {renamingTeam && <RenameTeamDialog team={renamingTeam} onClose={() => setRenamingTeam(null)} onSaved={(updated) => { replaceTeam(updated); setRenamingTeam(null); }} />}
      {dialog === "team" && <CreateTeamDialog onClose={() => setDialog(null)} onCreate={addTeam} />}
    </div>
  );
}

export default App;
