import { FormEvent, useEffect, useMemo, useRef, useState } from "react";

const columns = [
  { status: "triage", label: "Triage", hint: "Worth keeping?" },
  { status: "todo", label: "Todo", hint: "Planned work" },
  { status: "agent_ready", label: "Agent ready", hint: "Clear enough to start" },
  { status: "in_progress", label: "In progress", hint: "One active commitment" },
  { status: "in_review", label: "In review", hint: "Prove it works" },
  { status: "done", label: "Done", hint: "Merged and verified" },
  { status: "canceled", label: "Canceled", hint: "No longer planned" },
] as const;

type Status = (typeof columns)[number]["status"];
type ViewMode = "board" | "list";

type Issue = {
  identifier: string;
  title: string;
  description: string;
  status: Status;
  createdAt: string;
  updatedAt: string;
};

function App() {
  const [issues, setIssues] = useState<Issue[]>([]);
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [view, setView] = useState<ViewMode>("list");
  const [filter, setFilter] = useState<Status | "all">("all");
  const [collapsed, setCollapsed] = useState<Partial<Record<Status, boolean>>>({});
  const [selected, setSelected] = useState<Issue | null>(null);
  const [dropTarget, setDropTarget] = useState<Status | null>(null);
  const titleInput = useRef<HTMLInputElement>(null);

  useEffect(() => {
    void loadIssues();
  }, []);

  const grouped = useMemo(
    () => Object.fromEntries(columns.map(({ status }) => [status, issues.filter((issue) => issue.status === status)])) as Record<Status, Issue[]>,
    [issues],
  );

  const visibleColumns = columns.filter((column) => filter === "all" || column.status === filter);
  const heading = filter === "all" ? "All issues" : visibleColumns[0].label;
  const visibleCount = visibleColumns.reduce((count, column) => count + grouped[column.status].length, 0);

  function selectFilter(status: Status | "all") {
    setFilter(status);
    setCollapsed({});
  }

  async function loadIssues() {
    try {
      const response = await fetch("/api/issues");
      if (!response.ok) throw new Error("Could not load issues");
      const data = (await response.json()) as { issues: Issue[] };
      setIssues(data.issues);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Could not load issues");
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
      const response = await fetch("/api/issues", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ title, description, status: filter === "all" ? "triage" : filter }),
      });
      if (!response.ok) throw new Error("Could not create issue");
      const issue = (await response.json()) as Issue;
      setIssues((current) => [issue, ...current]);
      setCollapsed((current) => ({ ...current, [issue.status]: false }));
      setTitle("");
      setDescription("");
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Could not create issue");
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
        <button type="button" className={`sidebar-link ${filter === "all" ? "active" : ""}`} aria-pressed={filter === "all"} onClick={() => selectFilter("all")}>
          <span className="sidebar-icon">◇</span>
          <span>All issues</span>
          <small>{issues.length}</small>
        </button>

        <div className="sidebar-label">Statuses</div>
        <div className="sidebar-statuses">
          {columns.map((column) => (
            <button type="button" key={column.status} className={filter === column.status ? "active" : ""} aria-pressed={filter === column.status} onClick={() => selectFilter(column.status)}>
              <span className={`status-icon status-icon-${column.status}`} />
              <span>{column.label}</span>
              <small>{grouped[column.status].length}</small>
            </button>
          ))}
        </div>

        <div className="sidebar-footer"><span /> Local workspace</div>
      </aside>

      <main className="workspace">
        <header className="topbar">
          <div className="breadcrumb"><span>Workspace</span><b>/</b>{heading}</div>
          <nav className="view-switch" aria-label="Issue view">
            <button type="button" className={view === "list" ? "active" : ""} aria-pressed={view === "list"} onClick={() => setView("list")}><span>☷</span> List</button>
            <button type="button" className={view === "board" ? "active" : ""} aria-pressed={view === "board"} onClick={() => setView("board")}><span>▦</span> Board</button>
          </nav>
        </header>

        <section className="content">
          <header className="view-heading">
            <div>
              <h1>{heading}</h1>
              <p>{visibleCount} issues across {visibleColumns.length} {visibleColumns.length === 1 ? "status" : "statuses"}</p>
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
                        <button type="button" className="issue-open" onClick={() => setSelected(issue)}>
                          <span className="identifier">{issue.identifier}</span>
                          <span className="card-title">{issue.title}</span>
                        </button>
                        <div className="status-control">
                          <span className={`status-icon status-icon-${issue.status}`} aria-hidden="true" />
                          <select
                            className="status-select"
                            value={issue.status}
                            aria-label={`Change status for ${issue.identifier}`}
                            onChange={(event) => void moveIssue(issue.identifier, event.target.value as Status)}
                          >
                            {columns.map((option) => <option value={option.status} key={option.status}>{option.label}</option>)}
                          </select>
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
                        <div className="status-control">
                          <span className={`status-icon status-icon-${issue.status}`} aria-hidden="true" />
                          <select
                            className="status-select"
                            value={issue.status}
                            aria-label={`Change status for ${issue.identifier}`}
                            onChange={(event) => void moveIssue(issue.identifier, event.target.value as Status)}
                          >
                            {columns.map((option) => <option value={option.status} key={option.status}>{option.label}</option>)}
                          </select>
                        </div>
                        <button type="button" className="identifier issue-open" onClick={() => setSelected(issue)}>{issue.identifier}</button>
                        <button type="button" className="list-copy issue-open" onClick={() => setSelected(issue)}>
                          <span className="list-title">{issue.title}</span>
                          {issue.description && <span className="list-description">{issue.description}</span>}
                        </button>
                      </article>
                    ))}
                  </div>
                </section>
              ))}
            </section>
          )}
        </section>
      </main>
      {selected && <IssueEditor issue={selected} onClose={() => setSelected(null)} onSave={(updated) => {
        setIssues((current) => current.map((issue) => issue.identifier === updated.identifier ? updated : issue));
        setSelected(null);
      }} />}
    </div>
  );
}

function IssueEditor({ issue, onClose, onSave }: { issue: Issue; onClose: () => void; onSave: (issue: Issue) => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [title, setTitle] = useState(issue.title);
  const [description, setDescription] = useState(issue.description);
  const [status, setStatus] = useState(issue.status);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    const element = dialog.current!;
    element.showModal();
    return () => element.close();
  }, []);

  async function save(event: FormEvent) {
    event.preventDefault();
    setSaving(true);
    setError("");
    try {
      const response = await fetch(`/api/issues/${issue.identifier}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ title, description, status }),
      });
      const result = await response.json();
      if (!response.ok) throw new Error(result.error);
      onSave(result as Issue);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Could not save issue");
    } finally {
      setSaving(false);
    }
  }

  return (
    <dialog ref={dialog} className="issue-dialog" aria-labelledby="issue-dialog-title" onCancel={onClose}>
      <form onSubmit={save}>
        <header>
          <h2 id="issue-dialog-title">{issue.identifier}</h2>
          <button type="button" aria-label="Close issue" onClick={onClose}>×</button>
        </header>
        <label>Title<input value={title} onChange={(event) => setTitle(event.target.value)} required autoFocus /></label>
        <label>Description<textarea rows={8} value={description} onChange={(event) => setDescription(event.target.value)} /></label>
        <label>Status<select value={status} onChange={(event) => setStatus(event.target.value as Status)}>
          {columns.map((column) => <option key={column.status} value={column.status}>{column.label}</option>)}
        </select></label>
        {error && <p className="error" role="alert">{error}</p>}
        <footer>
          <button type="button" onClick={onClose}>Cancel</button>
          <button type="submit" disabled={saving}>{saving ? "Saving…" : "Save changes"}</button>
        </footer>
      </form>
    </dialog>
  );
}

export default App;
