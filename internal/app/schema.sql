PRAGMA foreign_keys = ON;

-- A workspace groups teams, Projects, and labels. Its slug names it in URLs
-- and cannot change.
CREATE TABLE IF NOT EXISTS workspaces (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS projects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    archived BOOLEAN NOT NULL DEFAULT FALSE CHECK (archived IN (0, 1)),
    emoji TEXT NOT NULL DEFAULT '',
    UNIQUE (workspace_id, name)
);

-- Each team numbers its own issues. A key is fixed once created, because it
-- is the prefix of every identifier the team has issued.
CREATE TABLE IF NOT EXISTS teams (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
    key TEXT NOT NULL,
    name TEXT NOT NULL,
    next_number INTEGER NOT NULL,
    emoji TEXT NOT NULL DEFAULT '',
    private BOOLEAN NOT NULL DEFAULT FALSE CHECK(private IN (0,1)),
    UNIQUE (workspace_id, key)
);

-- workspace_id is the team's workspace; identifiers are unique within it.
CREATE TABLE IF NOT EXISTS issues (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
    identifier TEXT NOT NULL,
    team_id INTEGER NOT NULL REFERENCES teams(id),
    number INTEGER NOT NULL,
    title TEXT NOT NULL CHECK (length(trim(title)) > 0),
    description TEXT NOT NULL DEFAULT '',
    status TEXT NOT NULL CHECK (status IN ('triage', 'backlog', 'todo', 'agent_ready', 'in_progress', 'in_review', 'done', 'canceled')),
    type TEXT CHECK (type IN ('spec', 'feature', 'bug', 'improvement', 'chore')),
    project_id INTEGER REFERENCES projects(id),
    parent_id INTEGER REFERENCES issues(id) CHECK (parent_id <> id),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    priority INTEGER NOT NULL DEFAULT 0 CHECK (priority IN (0, 1, 2, 3, 4)),
    assignee_subject TEXT REFERENCES users(subject),
    UNIQUE (workspace_id, identifier)
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

-- author is who wrote a comment imported from another tracker; comments
-- added here have none.
CREATE TABLE IF NOT EXISTS comments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    issue_id INTEGER NOT NULL REFERENCES issues(id),
    body TEXT NOT NULL,
    created_at TEXT NOT NULL,
    author TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    issue_id INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
    kind TEXT NOT NULL CHECK (kind IN ('created', 'status_changed')),
    detail TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL
);

-- issue_identifier has no foreign key: a pull request may name an issue that
-- is created later. It names an issue in workspace_id, the workspace the
-- repository is bound to.
CREATE TABLE IF NOT EXISTS pull_requests (
    workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
    repo TEXT NOT NULL,
    number INTEGER NOT NULL,
    title TEXT NOT NULL,
    url TEXT NOT NULL,
    head_ref TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('open', 'draft', 'merged', 'closed')),
    issue_identifier TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    linkback_posted_at TEXT,
    PRIMARY KEY (workspace_id, repo, number, issue_identifier)
);

CREATE TABLE IF NOT EXISTS github_sync (
    workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
    repo TEXT NOT NULL,
    last_updated_at TEXT NOT NULL,
    PRIMARY KEY (workspace_id, repo)
);

CREATE UNIQUE INDEX IF NOT EXISTS issues_team_number_idx ON issues(team_id, number);
CREATE INDEX IF NOT EXISTS issues_status_updated_idx ON issues(status, updated_at DESC);
CREATE INDEX IF NOT EXISTS issues_parent_idx ON issues(parent_id);
CREATE INDEX IF NOT EXISTS events_issue_created_idx ON events(issue_id, created_at DESC);

-- A label's group must be in the label's workspace: (workspace_id, id) is
-- unique so that labels can reference both.
CREATE TABLE IF NOT EXISTS label_groups (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    name_key TEXT NOT NULL,
    UNIQUE (workspace_id, name_key),
    UNIQUE (workspace_id, id)
);
CREATE TABLE IF NOT EXISTS labels (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
    name TEXT NOT NULL CHECK (length(trim(name)) > 0),
    name_key TEXT NOT NULL,
    color TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    group_id INTEGER,
    archived BOOLEAN NOT NULL DEFAULT FALSE CHECK (archived IN (0, 1)),
    FOREIGN KEY (workspace_id, group_id) REFERENCES label_groups(workspace_id, id)
);
CREATE UNIQUE INDEX IF NOT EXISTS labels_name_idx ON labels(workspace_id, COALESCE(group_id, 0), name_key);
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

CREATE TABLE IF NOT EXISTS project_teams (
    project_id INTEGER NOT NULL REFERENCES projects(id),
    team_id INTEGER NOT NULL REFERENCES teams(id),
    PRIMARY KEY (project_id, team_id)
);
CREATE INDEX IF NOT EXISTS project_teams_team_idx ON project_teams(team_id, project_id);
CREATE TRIGGER IF NOT EXISTS project_teams_workspace BEFORE INSERT ON project_teams
WHEN (SELECT workspace_id FROM projects WHERE id=NEW.project_id) <> (SELECT workspace_id FROM teams WHERE id=NEW.team_id)
BEGIN SELECT RAISE(ABORT, 'project and team must belong to the same workspace'); END;

CREATE TABLE IF NOT EXISTS users (
 subject TEXT PRIMARY KEY NOT NULL,
 name TEXT NOT NULL,
 email TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS workspace_members (
 workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
 subject TEXT NOT NULL REFERENCES users(subject),
 role TEXT NOT NULL CHECK(role IN ('admin','member')),
 status TEXT NOT NULL CHECK(status IN ('active','suspended')),
 PRIMARY KEY(workspace_id,subject)
);
CREATE INDEX IF NOT EXISTS workspace_members_subject_idx ON workspace_members(subject,workspace_id);
CREATE TABLE IF NOT EXISTS workspace_invites (
 workspace_id INTEGER PRIMARY KEY REFERENCES workspaces(id),
 digest BLOB NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS team_members (
 team_id INTEGER NOT NULL REFERENCES teams(id),
 subject TEXT NOT NULL REFERENCES users(subject),
 role TEXT NOT NULL CHECK(role IN ('member','owner')),
 PRIMARY KEY(team_id,subject)
);
CREATE TRIGGER IF NOT EXISTS team_members_workspace BEFORE INSERT ON team_members
WHEN NOT EXISTS(SELECT 1 FROM workspace_members m JOIN teams t ON t.workspace_id=m.workspace_id WHERE t.id=NEW.team_id AND m.subject=NEW.subject)
BEGIN SELECT RAISE(ABORT, 'team member must belong to workspace'); END;

CREATE TABLE IF NOT EXISTS issue_assignments (
 id INTEGER PRIMARY KEY AUTOINCREMENT,
 issue_id INTEGER NOT NULL REFERENCES issues(id) ON DELETE CASCADE,
 actor_subject TEXT REFERENCES users(subject),
 from_subject TEXT REFERENCES users(subject),
 to_subject TEXT REFERENCES users(subject),
 created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS issue_assignments_issue_idx ON issue_assignments(issue_id,id);
CREATE INDEX IF NOT EXISTS issues_assignee_idx ON issues(workspace_id,assignee_subject);
