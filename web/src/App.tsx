import { FormEvent, useEffect, useMemo, useRef, useState } from "react";
import { IssueDetail } from "./IssueDetail";
import { api, columns, Issue, IssueMeta, navigate, NavLink, Project, Status, StatusSelect } from "./shared";

type ViewMode = "board" | "list";

function App() {
  const [path, setPath] = useState(window.location.pathname);
  const [issues, setIssues] = useState<Issue[]>([]);
  const [projects, setProjects] = useState<Project[]>([]);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [view, setView] = useState<ViewMode>("list");
  const [filter, setFilter] = useState<Status | "all">("all");
  const [project, setProject] = useState("");
  const [collapsed, setCollapsed] = useState<Partial<Record<Status, boolean>>>({});
  const [dropTarget, setDropTarget] = useState<Status | null>(null);
  const titleInput = useRef<HTMLInputElement>(null);
  const detailIdentifier = path.startsWith("/issue/") ? path.slice("/issue/".length) : "";

  useEffect(() => {
    const sync = () => {
      setPath(window.location.pathname);
      setError("");
    };
    window.addEventListener("popstate", sync);
    return () => window.removeEventListener("popstate", sync);
  }, []);

  useEffect(() => {
    const load = () => void loadIssues();
    load();
    window.addEventListener("focus", load);
    return () => window.removeEventListener("focus", load);
  }, [path, project]);

  const grouped = useMemo(
    () => Object.fromEntries(columns.map(({ status }) => [status, issues.filter((issue) => issue.status === status)])) as Record<Status, Issue[]>,
    [issues],
  );

  const visibleColumns = columns.filter((column) => filter === "all" || column.status === filter);
  const heading = filter === "all" ? project || "All issues" : visibleColumns[0].label;
  const visibleCount = visibleColumns.reduce((count, column) => count + grouped[column.status].length, 0);

  function selectFilter(status: Status | "all", nextProject: string) {
    setFilter(status);
    setProject(nextProject);
    setCollapsed({});
    if (detailIdentifier) navigate("/");
  }

  async function loadIssues() {
    try {
      const [issueData, projectData] = await Promise.all([
        api<{ issues: Issue[] }>(`/api/issues?project=${encodeURIComponent(project)}`),
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

  async function createIssue(event: FormEvent) {
    event.preventDefault();
    if (!title.trim()) return;
    setSaving(true);
    setError("");
    try {
      const issue = await api<Issue>("/api/issues", "POST", { title, description, status: filter === "all" ? "triage" : filter, project });
      setIssues((current) => [issue, ...current]);
      setCollapsed((current) => ({ ...current, [issue.status]: false }));
      setTitle("");
      setDescription("");
    } catch (cause) {
      setError((cause as Error).message);
    } finally {
      setSaving(false);
    }
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
        <button type="button" className={`sidebar-link ${filter === "all" && !project ? "active" : ""}`} aria-pressed={filter === "all" && !project} onClick={() => selectFilter("all", "")}>
          <span className="sidebar-icon">◇</span>
          <span>All issues</span>
          <small>{issues.length}</small>
        </button>

        <div className="sidebar-label">Statuses</div>
        <div className="sidebar-statuses">
          {columns.map((column) => (
            <button type="button" key={column.status} className={filter === column.status ? "active" : ""} aria-pressed={filter === column.status} onClick={() => selectFilter(column.status, project)}>
              <span className={`status-icon status-icon-${column.status}`} />
              <span>{column.label}</span>
              <small>{grouped[column.status].length}</small>
            </button>
          ))}
        </div>

        {projects.length > 0 && <div className="sidebar-label">Projects</div>}
        <div className="sidebar-projects">
          {projects.map((item) => (
            <button type="button" key={item.name} title={item.description} className={project === item.name ? "active" : ""} aria-pressed={project === item.name} onClick={() => selectFilter("all", item.name)}>
              <span className="sidebar-icon">▣</span>
              <span>{item.name}</span>
            </button>
          ))}
        </div>

        <div className="sidebar-footer"><span /> Local workspace</div>
      </aside>

      <main className="workspace">
        <header className="topbar">
          {detailIdentifier ? (
            <div className="breadcrumb"><span>Workspace</span><b>/</b><NavLink to="/">{heading}</NavLink><b>/</b>{detailIdentifier}</div>
          ) : (
            <>
              <div className="breadcrumb"><span>Workspace</span><b>/</b>{heading}</div>
              <nav className="view-switch" aria-label="Issue view">
                <button type="button" className={view === "list" ? "active" : ""} aria-pressed={view === "list"} onClick={() => setView("list")}><span>☷</span> List</button>
                <button type="button" className={view === "board" ? "active" : ""} aria-pressed={view === "board"} onClick={() => setView("board")}><span>▦</span> Board</button>
              </nav>
            </>
          )}
        </header>

        {detailIdentifier ? (
          <section className="content">
            {error && <p className="error" role="alert">{error}</p>}
            <IssueDetail key={detailIdentifier} identifier={detailIdentifier} setError={setError} />
          </section>
        ) : (
          <section className="content">
            <header className="view-heading">
              <div>
                <h1>{heading}</h1>
                <p>{visibleCount} issues across {visibleColumns.length} {visibleColumns.length === 1 ? "status" : "statuses"}{project && ` in ${project}`}</p>
              </div>
              <span className="local-badge">Local</span>
            </header>

            <form className="composer" onSubmit={createIssue}>
              <button type="button" className="composer-icon" aria-label="Focus new issue title" onClick={() => titleInput.current!.focus()}>+</button>
              <label>
                <span>Title</span>
                <input ref={titleInput} value={title} onChange={(event) => setTitle(event.target.value)} placeholder="Create a new issue…" required />
              </label>
              <label>
                <span>Description</span>
                <input value={description} onChange={(event) => setDescription(event.target.value)} placeholder="Optional context" />
              </label>
              <button type="submit" disabled={saving}>{saving ? "Adding…" : "Add issue"}</button>
            </form>

            {error && <p className="error" role="alert">{error}</p>}

            {view === "board" ? (
              <section className="board" aria-label="Delivery board">
                {visibleColumns.map((column) => (
                  <section
                    className={`column column-${column.status} ${dropTarget === column.status ? "drop-target" : ""}`}
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
                    <header>
                      <div>
                        <span className={`status-icon status-icon-${column.status}`} />
                        <h2>{column.label}</h2>
                        <small>{grouped[column.status].length}</small>
                      </div>
                      <span className="column-hint">{column.hint}</span>
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
                          <NavLink to={`/issue/${issue.identifier}`} className="issue-open">
                            <span className="identifier">{issue.identifier}</span>
                            <span className="card-title">{issue.title}</span>
                          </NavLink>
                          <IssueMeta issue={issue} />
                          <StatusSelect issue={issue} onChange={(status) => void moveIssue(issue.identifier, status)} />
                        </article>
                      ))}
                    </div>
                  </section>
                ))}
              </section>
            ) : (
              <section className="list-view" aria-label="Issue list">
                {visibleColumns.map((column) => (
                  <section className={`list-group list-group-${column.status}`} key={column.status}>
                    <header>
                      <button type="button" aria-expanded={!collapsed[column.status]} aria-controls={`rows-${column.status}`} onClick={() => setCollapsed((current) => ({ ...current, [column.status]: !current[column.status] }))}>
                        <span className="chevron">{collapsed[column.status] ? "›" : "⌄"}</span>
                        <span className={`status-icon status-icon-${column.status}`} />
                        <span className="group-title">{column.label}</span>
                        <span className="group-count">{grouped[column.status].length}</span>
                      </button>
                    </header>
                    <div className="list-rows" id={`rows-${column.status}`} hidden={collapsed[column.status]}>
                      {loading && <p className="list-empty">Loading…</p>}
                      {!loading && grouped[column.status].length === 0 && <p className="list-empty">No issues</p>}
                      {grouped[column.status].map((issue) => (
                        <article className="list-row" key={issue.identifier}>
                          <StatusSelect issue={issue} onChange={(status) => void moveIssue(issue.identifier, status)} />
                          <NavLink to={`/issue/${issue.identifier}`} className="identifier issue-open">{issue.identifier}</NavLink>
                          <NavLink to={`/issue/${issue.identifier}`} className="list-copy issue-open">
                            <span className="list-title">{issue.title}</span>
                          </NavLink>
                          <IssueMeta issue={issue} />
                        </article>
                      ))}
                    </div>
                  </section>
                ))}
              </section>
            )}
          </section>
        )}
      </main>
    </div>
  );
}

export default App;
