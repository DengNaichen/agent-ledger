PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS projects (
    name TEXT PRIMARY KEY,
    description TEXT NOT NULL DEFAULT '',
    archived BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS issues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    identifier TEXT UNIQUE,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL,
    type TEXT,
    project TEXT REFERENCES projects(name),
    parent TEXT REFERENCES issues(identifier),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS relations (
    issue TEXT NOT NULL REFERENCES issues(identifier),
    kind TEXT NOT NULL,
    other TEXT NOT NULL REFERENCES issues(identifier),
    PRIMARY KEY (issue, kind, other)
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
    kind TEXT NOT NULL,
    detail TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS issues_status_updated_idx ON issues(status, updated_at DESC);
CREATE INDEX IF NOT EXISTS issues_parent_idx ON issues(parent);
CREATE INDEX IF NOT EXISTS events_issue_created_idx ON events(issue_id, created_at DESC);
