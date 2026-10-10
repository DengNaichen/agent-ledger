PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS projects (
    name TEXT PRIMARY KEY NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    archived BOOLEAN NOT NULL DEFAULT FALSE CHECK (archived IN (0, 1))
);

CREATE TABLE IF NOT EXISTS issues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    identifier TEXT UNIQUE,
    title TEXT NOT NULL CHECK (length(trim(title)) > 0),
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('triage', 'todo', 'agent_ready', 'in_progress', 'in_review', 'done', 'canceled')),
    type TEXT CHECK (type IN ('spec', 'feature', 'bug', 'improvement', 'chore')),
    project TEXT REFERENCES projects(name),
    parent TEXT REFERENCES issues(identifier) CHECK (parent <> identifier),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS relations (
    issue TEXT NOT NULL REFERENCES issues(identifier),
    kind TEXT NOT NULL CHECK (kind IN ('blocks', 'related')),
    other TEXT NOT NULL REFERENCES issues(identifier),
    PRIMARY KEY (issue, kind, other),
    CHECK (issue <> other),
    CHECK (kind <> 'related' OR issue < other)
);

CREATE TABLE IF NOT EXISTS links (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    issue TEXT NOT NULL REFERENCES issues(identifier),
    title TEXT NOT NULL,
    url TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    issue TEXT NOT NULL REFERENCES issues(identifier),
    body TEXT NOT NULL,
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    issue_id INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('created', 'status_changed')),
    detail TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS issues_status_updated_idx ON issues(status, updated_at DESC);
CREATE INDEX IF NOT EXISTS issues_parent_idx ON issues(parent);
CREATE INDEX IF NOT EXISTS events_issue_created_idx ON events(issue_id, created_at DESC);
