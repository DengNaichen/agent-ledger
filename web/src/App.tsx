import { Box, ChevronDown, Columns2, List, Plus, Search, Tag, X } from "lucide-react";
import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { IssueDetail } from "./IssueDetail";
import { RequestError, api, apiPath, columns, Issue, IssueMeta, IssueSummary, navigate, NavLink, pagePath, Project, Status, StatusIcon, StatusSelect, Team, Workspace, workspaceSlug, priorities, Priority, PrioritySelect, IssueSort } from "./shared";

import { Session } from "./Session";

import { emptyLabelFilter, LabelFilter, LabelFilterValue, LabelManager, LabelPicker, useLabels } from "./Labels";

type ViewMode = "board" | "list";

const issueTypes = ["spec", "feature", "bug", "improvement", "chore"];

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
// of the path selects the list, an issue, or the label manager.
function WorkspaceView() {
  const catalog = useLabels();
  const [labelFilter, setLabelFilter] = useState<LabelFilterValue>(emptyLabelFilter);
  const [selected, setSelected] = useState<string[]>([]);
  const [batchLabels, setBatchLabels] = useState(false);
  const [path, setPath] = useState(window.location.pathname);
  const [issues, setIssues] = useState<IssueSummary[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [teams, setTeams] = useState<Team[]>([]);
  const [loading, setLoading] = useState(true);
  const loadVersion = useRef(0);
  const [error, setError] = useState("");
  const [view, setView] = useState<ViewMode>("list");
  const [filter, setFilter] = useState<Status | "all">("all");
  const [project, setProject] = useState("");
  const [team, setTeam] = useState("");
  const [priority, setPriority] = useState<Priority | "">("");
  const [sort, setSort] = useState<IssueSort>("auto");
  const [savingPriorities, setSavingPriorities] = useState<string[]>([]);
  const [type, setType] = useState("");
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [collapsed, setCollapsed] = useState<Partial<Record<Status, boolean>>>({});
  const [dropTarget, setDropTarget] = useState<Status | null>(null);
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

  useEffect(() => {
    const load = () => void loadIssues();
    load();
    window.addEventListener("focus", load);
    return () => window.removeEventListener("focus", load);
  }, [path, team, project, type, query, labelFilter, priority]);

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

  function addIssue(issue: IssueSummary) {
    const inTeam = !team || issue.identifier.startsWith(`${team}-`);
    if (inTeam && (priority === "" || issue.priority === priority) && (labelFilter.mode === "unlabeled" || (labelFilter.labels.length === 0 && labelFilter.groups.length === 0))) setIssues((current) => [issue, ...current]);
    else void loadIssues();
    setCollapsed((current) => ({ ...current, [issue.status]: false }));
    setDialog(null);
  }

  function addTeam(created: Team) {
    setTeams((current) => [...current, created].sort((a, b) => a.key.localeCompare(b.key)));
    selectFilter("all", "", created.key);
    setDialog(null);
  }

  async function moveIssue(identifier: string, status: Status) {
    const previous = issues;
    setIssues((current) => current.map((issue) => (issue.identifier === identifier ? { ...issue, status } : issue)));
    try {
      const response = await fetch(apiPath(`/issues/${identifier}/status`), {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ status }),
      });
      if (response.status === 401) window.dispatchEvent(new Event("session-expired"));
      if (response.ok) window.dispatchEvent(new Event("session-active"));
      if (!response.ok) throw new Error(response.status === 401 ? "unauthorized" : "Could not move issue");
      const updated = (await response.json()) as Issue;
      setIssues((current) => current.map((issue) => (issue.identifier === identifier ? updated : issue)));
    } catch (cause) {
      setIssues(previous);
      setError(cause instanceof Error ? cause.message : "Could not move issue");
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

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">IT</span>
          <strong>InfTask</strong>
        </div>

        <Session />
        <div className="sidebar-label">Workspace</div>
        <button type="button" className={`nav-item ${filter === "all" && !project && !team && !managingLabels ? "active" : ""}`} aria-pressed={filter === "all" && !project && !team && !managingLabels} onClick={() => selectFilter("all", "", "")}>
          <List className="icon" />
          <span className="nav-label">All issues</span>
          <small>{issues.length}</small>
        </button>

        <button type="button" className={`nav-item ${managingLabels ? "active" : ""}`} onClick={() => navigate(pagePath("/labels"))}><Tag className="icon" /><span className="nav-label">Labels</span></button>

        <div className="sidebar-group">
          <div className="sidebar-label sidebar-heading">
            <span>Teams</span>
            <button type="button" className="ghost" aria-label="Create team" title="Create team" onClick={() => setDialog("team")}><Plus className="icon" /></button>
          </div>
          {teams.map((item) => (
            <button type="button" key={item.key} className={`nav-item ${team === item.key && !managingLabels ? "active" : ""}`} aria-pressed={team === item.key && !managingLabels} onClick={() => selectFilter("all", "", item.key)}>
              <span className="team-mark">{item.key.slice(0, 2)}</span>
              <span className="nav-label">{item.name}</span>
              <small>{item.key}</small>
            </button>
          ))}

          <div className="sidebar-label">Statuses</div>
          {columns.map((column) => (
            <button type="button" key={column.status} className={`nav-item ${filter === column.status ? "active" : ""}`} aria-pressed={filter === column.status} onClick={() => selectFilter(column.status, project)}>
              <StatusIcon status={column.status} />
              <span className="nav-label">{column.label}</span>
              <small>{grouped[column.status].length}</small>
            </button>
          ))}

          {projects.length > 0 && <div className="sidebar-label">Projects</div>}
          {projects.map((item) => (
            <button type="button" key={item.name} title={item.description} className={`nav-item ${project === item.name ? "active" : ""}`} aria-pressed={project === item.name} onClick={() => selectFilter("all", item.name)}>
              <Box className="icon" />
              <span className="nav-label">{item.name}</span>
            </button>
          ))}
        </div>

        <div className="sidebar-footer"><span /> Shared workspace</div>
      </aside>

      <main className="panel">
        <header className="header-row">
          <nav className="breadcrumb" aria-label="Breadcrumb">
            {detailIdentifier ? (
              <><NavLink to={pagePath("/")}>{heading}</NavLink><span className="separator">/</span><span>{detailIdentifier}</span></>
            ) : (
              <span>{heading}</span>
            )}
          </nav>
          {managingLabels ? <NavLink to={pagePath("/")}>Back to issues</NavLink> : detailIdentifier ? (
            <button type="button" className="button" onClick={() => setDialog("edit")}>Edit</button>
          ) : (
            <button type="button" className="button" onClick={() => setDialog("new")}>New issue</button>
          )}
        </header>

        {!detailIdentifier && !managingLabels && (
          <div className="header-row filter-row">
            <div className="chips">
              {filter !== "all" && (
                <span className="chip">
                  <StatusIcon status={filter} />
                  {visibleColumns[0].label}
                  <button type="button" aria-label={`Clear ${visibleColumns[0].label} filter`} onClick={() => selectFilter("all", project)}><X className="icon" /></button>
                </span>
              )}
              {team && (
                <span className="chip">
                  <span className="team-mark">{team.slice(0, 2)}</span>
                  {team}
                  <button type="button" aria-label={`Clear ${team} filter`} onClick={() => selectFilter(filter, project, "")}><X className="icon" /></button>
                </span>
              )}
              {project && (
                <span className="chip">
                  <Box className="icon" />
                  {project}
                  <button type="button" aria-label={`Clear ${project} filter`} onClick={() => selectFilter(filter, "")}><X className="icon" /></button>
                </span>
              )}
              {type && (
                <span className="chip">
                  <Tag className="icon" />
                  {type}
                  <button type="button" aria-label={`Clear ${type} filter`} onClick={() => setType("")}><X className="icon" /></button>
                </span>
              )}
              {query && (
                <span className="chip">
                  <Search className="icon" />
                  “{query}”
                  <button type="button" aria-label="Clear search" onClick={() => {
                    setSearch("");
                    setQuery("");
                    setCollapsed({});
                  }}><X className="icon" /></button>
                </span>
              )}
              <span className="view-count">{visibleCount} issues</span>
            </div>
            <div className="filter-controls">
              <form className="search-box" role="search" onSubmit={(event) => {
                event.preventDefault();
                setQuery(search);
                setCollapsed({});
              }}>
                <Search className="icon" />
                <input type="search" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search issues" aria-label="Search issues" />
              </form>
              <select className="type-filter" value={type} aria-label="Filter by type" onChange={(event) => setType(event.target.value)}>
                <option value="">All types</option>
                {issueTypes.map((item) => <option key={item} value={item}>{item}</option>)}
              </select>
              <select className="type-filter" value={priority} aria-label="Filter by priority" onChange={(event) => { setPriority(event.target.value === "" ? "" : Number(event.target.value) as Priority); setSelected([]); }}>
                <option value="">All priorities</option>
                {priorities.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
              </select>
              <select className="type-filter" value={sort} aria-label="Issue ordering" onChange={(event) => setSort(event.target.value as IssueSort)}>
                <option value="auto">Default ordering</option>
                <option value="updated">Last updated</option>
                <option value="priority">Priority</option>
              </select>
              <LabelFilter value={labelFilter} onChange={(value) => { setLabelFilter(value); setSelected([]); }} catalog={catalog} />
              <nav className="view-switch" aria-label="Issue view">
                <button type="button" className={view === "list" ? "active" : ""} aria-pressed={view === "list"} onClick={() => setView("list")}><List className="icon" /> List</button>
                <button type="button" className={view === "board" ? "active" : ""} aria-pressed={view === "board"} onClick={() => setView("board")}><Columns2 className="icon" /> Board</button>
              </nav>
            </div>
          </div>
        )}

        {error && <RequestError message={error} />}

        {!detailIdentifier && !managingLabels && <div className="label-bulk-bar"><button className="ghost" onClick={() => setSelected(issues.filter((issue) => filter === "all" || issue.status === filter).map((issue) => issue.identifier))}>Select visible</button>{selectedVisible.length > 0 && <><span>{selectedVisible.length} selected</span><button className="button" onClick={() => setBatchLabels(true)}>Change labels</button><button className="ghost" onClick={() => setSelected([])}>Clear selection</button></>}</div>}
        {managingLabels ? <LabelManager catalog={catalog} onMerged={() => setLabelFilter(emptyLabelFilter)} /> : detailIdentifier ? (
          <IssueDetail catalog={catalog} key={detailIdentifier} identifier={detailIdentifier} setError={setError} editing={dialog === "edit"} closeEditor={() => setDialog(null)} />
        ) : view === "board" ? (
          <section className="board" aria-label="Delivery board">
            {visibleColumns.map((column) => (
              <section
                className={`column ${dropTarget === column.status ? "drop-target" : ""}`}
                key={column.status}
                onDragOver={(event) => {
                  event.preventDefault();
                  event.dataTransfer.dropEffect = "move";
                  setDropTarget(column.status);
                }}
                onDrop={(event) => {
                  event.preventDefault();
                  setDropTarget(null);
                  const identifier = event.dataTransfer.getData("application/x-inftask-issue");
                  if (identifier) void moveIssue(identifier, column.status);
                }}
              >
                <header title={column.hint}>
                  <StatusIcon status={column.status} />
                  <h2>{column.label}</h2>
                  <small>{grouped[column.status].length}</small>
                </header>
                <div className="cards">
                  {loading && <p className="empty">Loading…</p>}
                  {!loading && grouped[column.status].length === 0 && <p className="empty">No issues</p>}
                  {grouped[column.status].map((issue) => (
                    <article
                      className="card"
                      draggable
                      key={issue.identifier}
                      onDragStart={(event) => {
                        event.dataTransfer.setData("application/x-inftask-issue", issue.identifier);
                        event.dataTransfer.effectAllowed = "move";
                      }}
                      onDragEnd={() => setDropTarget(null)}
                    >
                      <input className="issue-selection" type="checkbox" aria-label={`Select ${issue.identifier}`} checked={selectedVisible.includes(issue.identifier)} onChange={() => toggleIssue(issue.identifier)} />
                      <span className="drag-handle" draggable title={`Drag ${issue.identifier} to another status`} aria-label={`Drag ${issue.identifier}`}>⠿</span>
                      <NavLink to={pagePath(`/issue/${issue.identifier}`)} className="card-open">
                        <span className="identifier">{issue.identifier}</span>
                        <span className="card-title">{issue.title}</span>
                      </NavLink>
                      <div className="card-meta">
                        <PrioritySelect priority={issue.priority} label={`Change priority for ${issue.identifier}`} compact disabled={savingPriorities.includes(issue.identifier)} onChange={(next) => void changePriority(issue.identifier, next)} />
                        <StatusSelect issue={issue} onChange={(status) => void moveIssue(issue.identifier, status)} />
                        <IssueMeta issue={issue} />
                      </div>
                    </article>
                  ))}
                </div>
              </section>
            ))}
          </section>
        ) : (
          <section className="list-view" aria-label="Issue list">
            {visibleColumns.map((column) => (
              <section className="list-group" key={column.status}>
                <button type="button" className="group-header" aria-expanded={!isCollapsed(column.status)} aria-controls={`rows-${column.status}`} onClick={() => setCollapsed((current) => ({ ...current, [column.status]: !(current[column.status] ?? defaultCollapsed(column.status)) }))}>
                  <ChevronDown className="icon chevron" />
                  <StatusIcon status={column.status} />
                  <span>{column.label}</span>
                  <small>{grouped[column.status].length}</small>
                </button>
                {/* A collapsed group renders no rows; the empty container keeps the header's aria-controls target. */}
                <div className="list-rows" id={`rows-${column.status}`} hidden={isCollapsed(column.status)}>
                  {!isCollapsed(column.status) && <>
                    {loading && <p className="list-empty">Loading…</p>}
                    {!loading && grouped[column.status].length === 0 && <p className="list-empty">No issues</p>}
                    {grouped[column.status].map((issue) => (
                      <article className="list-row" key={issue.identifier}>
                        <input className="issue-selection" type="checkbox" aria-label={`Select ${issue.identifier}`} checked={selectedVisible.includes(issue.identifier)} onChange={() => toggleIssue(issue.identifier)} />
                        <PrioritySelect priority={issue.priority} label={`Change priority for ${issue.identifier}`} compact disabled={savingPriorities.includes(issue.identifier)} onChange={(next) => void changePriority(issue.identifier, next)} />
                        <StatusSelect issue={issue} onChange={(status) => void moveIssue(issue.identifier, status)} />
                        <NavLink to={pagePath(`/issue/${issue.identifier}`)} className="identifier">{issue.identifier}</NavLink>
                        <NavLink to={pagePath(`/issue/${issue.identifier}`)} className="list-title">{issue.title}</NavLink>
                        <IssueMeta issue={issue} />
                        <time className="row-date" dateTime={issue.updated_at}>{new Date(issue.updated_at).toLocaleDateString(undefined, { month: "short", day: "numeric" })}</time>
                      </article>
                    ))}
                  </>}
                </div>
              </section>
            ))}
          </section>
        )}
      </main>

      {batchLabels && <LabelPicker identifiers={selectedVisible} current={null} catalog={catalog} onClose={() => setBatchLabels(false)} onChanged={async () => { await loadIssues(); setSelected([]); }} />}
      {dialog === "new" && <NewIssueDialog teams={teams} team={team} status={filter === "all" ? "triage" : filter} project={project} onClose={() => setDialog(null)} onCreate={addIssue} />}
      {dialog === "team" && <CreateTeamDialog onClose={() => setDialog(null)} onCreate={addTeam} />}
    </div>
  );
}

function NewIssueDialog({ teams, team: selectedTeam, status, project, onClose, onCreate }: { teams: Team[]; team: string; status: Status; project: string; onClose: () => void; onCreate: (issue: Issue) => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const titleInput = useRef<HTMLInputElement>(null);
  // The server has no default team; the dialog preselects the team being viewed, else the first one, and always sends the choice.
  const [team, setTeam] = useState(selectedTeam || (teams.length > 0 ? teams[0].key : ""));
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [priority, setPriority] = useState<Priority>(0);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const element = dialog.current!;
    element.showModal();
    titleInput.current!.focus();
    return () => element.close();
  }, []);

  async function create(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      onCreate(await api<Issue>(apiPath("/issues"), "POST", { team, title, description, status, project, priority }));
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <dialog ref={dialog} className="issue-dialog" aria-labelledby="new-issue-title" onCancel={onClose}>
      <form onSubmit={create}>
        <header>
          <h2 id="new-issue-title">New issue</h2>
          <button type="button" className="ghost" aria-label="Close" onClick={onClose}><X className="icon" /></button>
        </header>
        <label>Title<input ref={titleInput} value={title} onChange={(event) => setTitle(event.target.value)} placeholder="Issue title" required /></label>
        <label>Description<textarea rows={8} value={description} onChange={(event) => setDescription(event.target.value)} placeholder="Add a description… (Markdown)" /></label>
        {teams.length > 0 ? (
          <label>Team<select value={team} onChange={(event) => setTeam(event.target.value)}>
            {teams.map((item) => <option key={item.key} value={item.key}>{item.key} · {item.name}</option>)}
          </select></label>
        ) : (
          <p className="error" role="alert">No teams yet. Create one with + next to Teams in the sidebar, then reopen this dialog.</p>
        )}
        <div className="dialog-priority"><span>Priority</span><PrioritySelect priority={priority} onChange={setPriority} label="New issue priority" disabled={saving} /></div>
        <p className="dialog-meta">
          <span className="chip"><StatusIcon status={status} />{columns.find((column) => column.status === status)!.label}</span>
          {project && <span className="chip"><Box className="icon" />{project}</span>}
        </p>
        {error && <RequestError message={error} />}
        <footer>
          <button type="button" className="button" onClick={onClose}>Cancel</button>
          <button type="submit" className="button primary" disabled={saving || !team}>{saving ? "Creating…" : "Create issue"}</button>
        </footer>
      </form>
    </dialog>
  );
}

function CreateTeamDialog({ onClose, onCreate }: { onClose: () => void; onCreate: (team: Team) => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const keyInput = useRef<HTMLInputElement>(null);
  const [key, setKey] = useState("");
  const [name, setName] = useState("");
  const [startNumber, setStartNumber] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const element = dialog.current!;
    element.showModal();
    keyInput.current!.focus();
    return () => element.close();
  }, []);

  async function create(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      onCreate(await api<Team>(apiPath("/teams"), "POST", { key, name, start_number: startNumber ? Number(startNumber) : undefined }));
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setSaving(false);
    }
  }

  return (
    <dialog ref={dialog} className="issue-dialog" aria-labelledby="new-team-title" onCancel={onClose}>
      <form onSubmit={create}>
        <header>
          <h2 id="new-team-title">Create team</h2>
          <button type="button" className="ghost" aria-label="Close" onClick={onClose}><X className="icon" /></button>
        </header>
        <label>Key<input ref={keyInput} value={key} onChange={(event) => setKey(event.target.value.toUpperCase())} placeholder="TOK" pattern="[A-Z][A-Z0-9]{1,5}" maxLength={6} required /></label>
        <label>Name<input value={name} onChange={(event) => setName(event.target.value)} placeholder="Team name" required /></label>
        <label>Start number<input type="number" min={1} step={1} value={startNumber} onChange={(event) => setStartNumber(event.target.value)} placeholder="1" /></label>
        <p className="dialog-hint">Issues are numbered {key || "KEY"}-{startNumber || "1"}, {key || "KEY"}-{startNumber ? Number(startNumber) + 1 : "2"}, and so on. Set a start number above numbers the team already used elsewhere. The key cannot be changed later.</p>
        {error && <RequestError message={error} />}
        <footer>
          <button type="button" className="button" onClick={onClose}>Cancel</button>
          <button type="submit" className="button primary" disabled={saving}>{saving ? "Creating…" : "Create team"}</button>
        </footer>
      </form>
    </dialog>
  );
}

export default App;
