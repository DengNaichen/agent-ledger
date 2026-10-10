import { FormEvent, useEffect, useMemo, useState } from "react";

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

  useEffect(() => {
    void loadIssues();
  }, []);

  const grouped = useMemo(
    () => Object.fromEntries(columns.map(({ status }) => [status, issues.filter((issue) => issue.status === status)])) as Record<Status, Issue[]>,
    [issues],
  );

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
        body: JSON.stringify({ title, description }),
      });
      if (!response.ok) throw new Error("Could not create issue");
      const issue = (await response.json()) as Issue;
      setIssues((current) => [issue, ...current]);
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
        <div className="sidebar-link active">
          <span className="sidebar-icon">◇</span>
          <span>All issues</span>
          <small>{issues.length}</small>
        </div>

        <div className="sidebar-label">Statuses</div>
        <div className="sidebar-statuses">
          {columns.map((column) => (
            <div key={column.status}>
              <span className={`status-icon status-icon-${column.status}`} />
              <span>{column.label}</span>
              <small>{grouped[column.status].length}</small>
            </div>
          ))}
        </div>

        <div className="sidebar-footer"><span /> Local workspace</div>
      </aside>

      <main className="workspace">
        <header className="topbar">
          <div className="breadcrumb"><span>Workspace</span><b>/</b>All issues</div>
          <nav className="view-switch" aria-label="Issue view">
            <button type="button" className={view === "list" ? "active" : ""} aria-pressed={view === "list"} onClick={() => setView("list")}><span>☷</span> List</button>
            <button type="button" className={view === "board" ? "active" : ""} aria-pressed={view === "board"} onClick={() => setView("board")}><span>▦</span> Board</button>
          </nav>
        </header>

        <section className="content">
          <header className="view-heading">
            <div>
              <h1>All issues</h1>
              <p>{issues.length} issues across {columns.length} statuses</p>
            </div>
            <span className="local-badge">Local</span>
          </header>

          <form className="composer" onSubmit={createIssue}>
            <span className="composer-icon">+</span>
            <label>
              <span>Title</span>
              <input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="Create a new issue…" required />
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
              {columns.map((column) => (
                <section
                  className={`column column-${column.status}`}
                  key={column.status}
                  onDragOver={(event) => event.preventDefault()}
                  onDrop={(event) => {
                    const identifier = event.dataTransfer.getData("text/plain");
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
                        onDragStart={(event) => event.dataTransfer.setData("text/plain", issue.identifier)}
                      >
                        <span className="identifier">{issue.identifier}</span>
                        <h3>{issue.title}</h3>
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
              {columns.map((column) => (
                <section className={`list-group list-group-${column.status}`} key={column.status}>
                  <header>
                    <div>
                      <span className="chevron">⌄</span>
                      <span className={`status-icon status-icon-${column.status}`} />
                      <h2>{column.label}</h2>
                      <span className="group-count">{grouped[column.status].length}</span>
                    </div>
                  </header>
                  <div className="list-rows">
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
                        <span className="identifier">{issue.identifier}</span>
                        <div className="list-copy">
                          <h3>{issue.title}</h3>
                          {issue.description && <p>{issue.description}</p>}
                        </div>
                      </article>
                    ))}
                  </div>
                </section>
              ))}
            </section>
          )}
        </section>
      </main>
    </div>
  );
}

export default App;
