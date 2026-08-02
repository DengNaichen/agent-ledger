import { FormEvent, useEffect, useMemo, useState } from "react";

const columns = [
  { status: "triage", label: "Triage", hint: "Worth keeping?" },
  { status: "ready", label: "Ready", hint: "Clear enough to start" },
  { status: "in_progress", label: "In progress", hint: "One active commitment" },
  { status: "in_review", label: "In review", hint: "Prove it works" },
  { status: "done", label: "Done", hint: "Merged and verified" },
] as const;

type Status = (typeof columns)[number]["status"];

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

  function shift(issue: Issue, direction: -1 | 1) {
    const index = columns.findIndex((column) => column.status === issue.status);
    const target = columns[index + direction];
    if (target) void moveIssue(issue.identifier, target.status);
  }

  return (
    <main>
      <header className="masthead">
        <div>
          <p className="eyebrow">LOCAL DELIVERY LEDGER</p>
          <h1>Work should move,<br />not disappear.</h1>
        </div>
        <div className="summary" aria-label="Board summary">
          <strong>{issues.length}</strong>
          <span>open ledger entries</span>
        </div>
      </header>

      <form className="composer" onSubmit={createIssue}>
        <div className="composer-copy">
          <span>New issue</span>
          <small>Every delivery starts with a clear outcome.</small>
        </div>
        <label>
          <span>Title</span>
          <input value={title} onChange={(event) => setTitle(event.target.value)} placeholder="Ship the smallest useful thing" required />
        </label>
        <label>
          <span>Description</span>
          <textarea value={description} onChange={(event) => setDescription(event.target.value)} placeholder="Context, scope, and proof." rows={2} />
        </label>
        <button type="submit" disabled={saving}>{saving ? "Adding…" : "Add to triage"}</button>
      </form>

      {error && <p className="error" role="alert">{error}</p>}

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
                <h2>{column.label}</h2>
                <p>{column.hint}</p>
              </div>
              <span>{grouped[column.status].length}</span>
            </header>
            <div className="cards">
              {loading && <p className="empty">Loading…</p>}
              {!loading && grouped[column.status].length === 0 && <p className="empty">Clear</p>}
              {grouped[column.status].map((issue) => {
                const index = columns.findIndex((item) => item.status === issue.status);
                return (
                  <article
                    className="card"
                    draggable
                    key={issue.identifier}
                    onDragStart={(event) => event.dataTransfer.setData("text/plain", issue.identifier)}
                  >
                    <span className="identifier">{issue.identifier}</span>
                    <h3>{issue.title}</h3>
                    {issue.description && <p>{issue.description}</p>}
                    <footer>
                      <button type="button" onClick={() => shift(issue, -1)} disabled={index === 0} aria-label={`Move ${issue.identifier} left`}>←</button>
                      <span>Move</span>
                      <button type="button" onClick={() => shift(issue, 1)} disabled={index === columns.length - 1} aria-label={`Move ${issue.identifier} right`}>→</button>
                    </footer>
                  </article>
                );
              })}
            </div>
          </section>
        ))}
      </section>
    </main>
  );
}

export default App;

