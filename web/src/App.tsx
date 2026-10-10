import { Box, ChevronDown, Columns2, List, Search, Tag, X } from "lucide-react";
import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { IssueDetail } from "./IssueDetail";
import { api, columns, Issue, IssueMeta, navigate, NavLink, Project, Status, StatusIcon, StatusSelect } from "./shared";

type ViewMode = "board" | "list";

const issueTypes = ["spec", "feature", "bug", "improvement", "chore"];

function App() {
  const [path, setPath] = useState(window.location.pathname);
  const [issues, setIssues] = useState<Issue[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [view, setView] = useState<ViewMode>("list");
  const [filter, setFilter] = useState<Status | "all">("all");
  const [project, setProject] = useState("");
  const [type, setType] = useState("");
  const [search, setSearch] = useState("");
  const [query, setQuery] = useState("");
  const [collapsed, setCollapsed] = useState<Partial<Record<Status, boolean>>>({});
  const [dropTarget, setDropTarget] = useState<Status | null>(null);
  const [dialog, setDialog] = useState<"new" | "edit" | null>(null);
  const detailIdentifier = path.startsWith("/issue/") ? path.slice("/issue/".length) : "";

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
  }, [path, project, type, query]);

  const grouped = useMemo(
    () => Object.fromEntries(columns.map(({ status }) => [status, issues.filter((issue) => issue.status === status)])) as Record<Status, Issue[]>,
    [issues],
  );

  const visibleColumns = columns.filter((column) => filter === "all" || column.status === filter);
  const heading = project || "All issues";
  const visibleCount = visibleColumns.reduce((count, column) => count + grouped[column.status].length, 0);

  function selectFilter(status: Status | "all", nextProject: string) {
    setFilter(status);
    setProject(nextProject);
    setCollapsed({});
    if (detailIdentifier) navigate("/");
  }

  async function loadIssues() {
    const params = new URLSearchParams({ project, query });
    if (type) params.set("type", type);
    try {
      const [issueData, projectData] = await Promise.all([
        api<{ issues: Issue[] }>(`/api/issues?${params}`),
        api<{ projects: Project[] }>("/api/projects"),
      ]);
      setIssues(issueData.issues);
      setProjects(projectData.projects);
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setLoading(false);
    }
  }

  function addIssue(issue: Issue) {
    setIssues((current) => [issue, ...current]);
    setCollapsed((current) => ({ ...current, [issue.status]: false }));
    setDialog(null);
  }

  async function moveIssue(identifier: string, status: Status) {
    const previous = issues;
    setIssues((current) => current.map((issue) => (issue.identifier === identifier ? { ...issue, status } : issue)));
    try {
      const response = await fetch(`/api/issues/${identifier}/status`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ status }),
      });
      if (!response.ok) throw new Error("Could not move issue");
      const updated = (await response.json()) as Issue;
      setIssues((current) => current.map((issue) => (issue.identifier === identifier ? updated : issue)));
    } catch (cause) {
      setIssues(previous);
      setError(cause instanceof Error ? cause.message : "Could not move issue");
    }
  }

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <div className="brand">
          <span className="brand-mark">AL</span>
          <strong>Agent Ledger</strong>
        </div>

        <div className="sidebar-label">Workspace</div>
        <button type="button" className={`nav-item ${filter === "all" && !project ? "active" : ""}`} aria-pressed={filter === "all" && !project} onClick={() => selectFilter("all", "")}>
          <List className="icon" />
          <span className="nav-label">All issues</span>
          <small>{issues.length}</small>
        </button>

        <div className="sidebar-group">
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

        <div className="sidebar-footer"><span /> Local workspace</div>
      </aside>

      <main className="panel">
        <header className="header-row">
          <nav className="breadcrumb" aria-label="Breadcrumb">
            {detailIdentifier ? (
              <><NavLink to="/">{heading}</NavLink><span className="separator">/</span><span>{detailIdentifier}</span></>
            ) : (
              <span>{heading}</span>
            )}
          </nav>
          {detailIdentifier ? (
            <button type="button" className="button" onClick={() => setDialog("edit")}>Edit</button>
          ) : (
            <button type="button" className="button" onClick={() => setDialog("new")}>New issue</button>
          )}
        </header>

        {!detailIdentifier && (
          <div className="header-row filter-row">
            <div className="chips">
              {filter !== "all" && (
                <span className="chip">
                  <StatusIcon status={filter} />
                  {visibleColumns[0].label}
                  <button type="button" aria-label={`Clear ${visibleColumns[0].label} filter`} onClick={() => selectFilter("all", project)}><X className="icon" /></button>
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
                  }}><X className="icon" /></button>
                </span>
              )}
              <span className="view-count">{visibleCount} issues</span>
            </div>
            <div className="filter-controls">
              <form className="search-box" role="search" onSubmit={(event) => {
                event.preventDefault();
                setQuery(search);
              }}>
                <Search className="icon" />
                <input type="search" value={search} onChange={(event) => setSearch(event.target.value)} placeholder="Search issues" aria-label="Search issues" />
              </form>
              <select className="type-filter" value={type} aria-label="Filter by type" onChange={(event) => setType(event.target.value)}>
                <option value="">All types</option>
                {issueTypes.map((item) => <option key={item} value={item}>{item}</option>)}
              </select>
              <nav className="view-switch" aria-label="Issue view">
                <button type="button" className={view === "list" ? "active" : ""} aria-pressed={view === "list"} onClick={() => setView("list")}><List className="icon" /> List</button>
                <button type="button" className={view === "board" ? "active" : ""} aria-pressed={view === "board"} onClick={() => setView("board")}><Columns2 className="icon" /> Board</button>
              </nav>
            </div>
          </div>
        )}

        {error && <p className="error" role="alert">{error}</p>}

        {detailIdentifier ? (
          <IssueDetail key={detailIdentifier} identifier={detailIdentifier} setError={setError} editing={dialog === "edit"} closeEditor={() => setDialog(null)} />
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
                  const identifier = event.dataTransfer.getData("application/x-agent-ledger-issue");
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
                        event.dataTransfer.setData("application/x-agent-ledger-issue", issue.identifier);
                        event.dataTransfer.effectAllowed = "move";
                      }}
                      onDragEnd={() => setDropTarget(null)}
                    >
                      <span className="drag-handle" draggable title={`Drag ${issue.identifier} to another status`} aria-label={`Drag ${issue.identifier}`}>⠿</span>
                      <NavLink to={`/issue/${issue.identifier}`} className="card-open">
                        <span className="identifier">{issue.identifier}</span>
                        <span className="card-title">{issue.title}</span>
                      </NavLink>
                      <div className="card-meta">
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
              <section key={column.status}>
                <button type="button" className="group-header" aria-expanded={!collapsed[column.status]} aria-controls={`rows-${column.status}`} onClick={() => setCollapsed((current) => ({ ...current, [column.status]: !current[column.status] }))}>
                  <ChevronDown className="icon chevron" />
                  <StatusIcon status={column.status} />
                  <span>{column.label}</span>
                  <small>{grouped[column.status].length}</small>
                </button>
                <div id={`rows-${column.status}`} hidden={collapsed[column.status]}>
                  {loading && <p className="list-empty">Loading…</p>}
                  {!loading && grouped[column.status].length === 0 && <p className="list-empty">No issues</p>}
                  {grouped[column.status].map((issue) => (
                    <article className="list-row" key={issue.identifier}>
                      <StatusSelect issue={issue} onChange={(status) => void moveIssue(issue.identifier, status)} />
                      <NavLink to={`/issue/${issue.identifier}`} className="identifier">{issue.identifier}</NavLink>
                      <NavLink to={`/issue/${issue.identifier}`} className="list-title">{issue.title}</NavLink>
                      <IssueMeta issue={issue} />
                      <time className="row-date" dateTime={issue.updated_at}>{new Date(issue.updated_at).toLocaleDateString(undefined, { month: "short", day: "numeric" })}</time>
                    </article>
                  ))}
                </div>
              </section>
            ))}
          </section>
        )}
      </main>

      {dialog === "new" && <NewIssueDialog status={filter === "all" ? "triage" : filter} project={project} onClose={() => setDialog(null)} onCreate={addIssue} />}
    </div>
  );
}

function NewIssueDialog({ status, project, onClose, onCreate }: { status: Status; project: string; onClose: () => void; onCreate: (issue: Issue) => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const titleInput = useRef<HTMLInputElement>(null);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
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
      onCreate(await api<Issue>("/api/issues", "POST", { title, description, status, project }));
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
        <p className="dialog-meta">
          <span className="chip"><StatusIcon status={status} />{columns.find((column) => column.status === status)!.label}</span>
          {project && <span className="chip"><Box className="icon" />{project}</span>}
        </p>
        {error && <p className="error" role="alert">{error}</p>}
        <footer>
          <button type="button" className="button" onClick={onClose}>Cancel</button>
          <button type="submit" className="button primary" disabled={saving}>{saving ? "Creating…" : "Create issue"}</button>
        </footer>
      </form>
    </dialog>
  );
}

export default App;
