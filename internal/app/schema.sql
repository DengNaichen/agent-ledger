PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    archived BOOLEAN NOT NULL DEFAULT FALSE CHECK (archived IN (0, 1))
);

-- Each team numbers its own issues. A key is fixed once created, because it
-- is the prefix of every identifier the team has issued.
CREATE TABLE IF NOT EXISTS teams (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    key TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    next_number INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS issues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    identifier TEXT NOT NULL UNIQUE,
    team_id INTEGER NOT NULL REFERENCES teams(id),
    number INTEGER NOT NULL,
    title TEXT NOT NULL CHECK (length(trim(title)) > 0),
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('triage', 'todo', 'agent_ready', 'in_progress', 'in_review', 'done', 'canceled')),
    priority INTEGER NOT NULL DEFAULT 0 CHECK (priority IN (0, 1, 2, 3, 4)),
    type TEXT CHECK (type IN ('spec', 'feature', 'bug', 'improvement', 'chore')),
    project_id INTEGER REFERENCES projects(id),
    parent_id INTEGER REFERENCES issues(id) CHECK (parent_id <> id),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS relations (
    issue_id INTEGER NOT NULL REFERENCES issues(id),
    kind TEXT NOT NULL CHECK (kind IN ('blocks', 'related')),
    other_id INTEGER NOT NULL REFERENCES issues(id),
    PRIMARY KEY (issue_id, kind, other_id),
    CHECK (issue_id <> other_id),
    CHECK (kind <> 'related' OR issue_id < other_id)
);

CREATE TABLE IF NOT EXISTS links (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    issue_id INTEGER NOT NULL REFERENCES issues(id),
    title TEXT NOT NULL,
    url TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    issue_id INTEGER NOT NULL REFERENCES issues(id),
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

-- issue_identifier has no foreign key: a pull request may name an issue that
-- is created later.
CREATE TABLE IF NOT EXISTS pull_requests (
    repo TEXT NOT NULL,
    number INTEGER NOT NULL,
    title TEXT NOT NULL,
    url TEXT NOT NULL,
    head_ref TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('open', 'draft', 'merged', 'closed')),
    issue_identifier TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    linkback_posted_at TEXT,
    PRIMARY KEY (repo, number, issue_identifier)
);

CREATE TABLE IF NOT EXISTS github_sync (
    repo TEXT PRIMARY KEY NOT NULL,
    last_updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS issues_team_number_idx ON issues(team_id, number);
CREATE INDEX IF NOT EXISTS issues_status_updated_idx ON issues(status, updated_at DESC);
CREATE INDEX IF NOT EXISTS issues_parent_idx ON issues(parent_id);
CREATE INDEX IF NOT EXISTS events_issue_created_idx ON events(issue_id, created_at DESC);

CREATE TABLE IF NOT EXISTS label_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    name_key TEXT NOT NULL UNIQUE
);
CREATE TABLE IF NOT EXISTS labels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    name_key TEXT NOT NULL,
    color TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    group_id INTEGER REFERENCES label_groups(id),
    archived BOOLEAN NOT NULL DEFAULT FALSE CHECK (archived IN (0, 1))
);
CREATE UNIQUE INDEX IF NOT EXISTS labels_name_idx ON labels(COALESCE(group_id, 0), name_key);
CREATE TABLE IF NOT EXISTS issue_labels (
    issue_id INTEGER NOT NULL REFERENCES issues(id),
    label_id INTEGER NOT NULL REFERENCES labels(id),
    PRIMARY KEY (issue_id, label_id)
);
CREATE INDEX IF NOT EXISTS issue_labels_label_idx ON issue_labels(label_id, issue_id);
CREATE TRIGGER IF NOT EXISTS issue_labels_exclusive BEFORE INSERT ON issue_labels
WHEN EXISTS (
    SELECT 1 FROM issue_labels il JOIN labels old ON old.id = il.label_id
    JOIN labels incoming ON incoming.id = NEW.label_id
    WHERE il.issue_id = NEW.issue_id AND old.group_id = incoming.group_id AND old.id <> incoming.id
)
BEGIN SELECT RAISE(ABORT, 'label group is single-select'); END;
