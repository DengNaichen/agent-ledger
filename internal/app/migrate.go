package app

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// migrations upgrade an existing database one version at a time, and PRAGMA
// user_version counts how many have run. schema.sql always describes the
// current shape, so a new database starts at len(migrations) without running
// any. Append only; never edit a migration that has shipped.
var migrations = []string{
	// 1: teams. Every identifier so far is <prefix>-<id>, so each prefix
	// becomes a team, the id after the last dash becomes the issue's number,
	// and the team's counter continues after its highest number. Rebuilding
	// issues keeps ids, which events reference, and identifiers, which
	// everything else references.
	`CREATE TABLE teams (
		key TEXT PRIMARY KEY NOT NULL,
		name TEXT NOT NULL,
		next_number INTEGER NOT NULL
	);
	CREATE TABLE issues_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		identifier TEXT NOT NULL UNIQUE,
		team TEXT NOT NULL REFERENCES teams(key),
		number INTEGER NOT NULL,
		title TEXT NOT NULL CHECK (length(trim(title)) > 0),
		description TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL CHECK (status IN ('triage', 'todo', 'agent_ready', 'in_progress', 'in_review', 'done', 'canceled')),
		type TEXT CHECK (type IN ('spec', 'feature', 'bug', 'improvement', 'chore')),
		project TEXT REFERENCES projects(name),
		parent TEXT REFERENCES issues(identifier) CHECK (parent <> identifier),
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	INSERT INTO issues_new(id, identifier, team, number, title, description, status, type, project, parent, created_at, updated_at)
	SELECT id, identifier, substr(head, 1, length(head) - 1), CAST(substr(identifier, length(head) + 1) AS INTEGER),
		title, description, status, type, project, parent, created_at, updated_at
	FROM (SELECT *, rtrim(identifier, '0123456789') AS head FROM issues);
	INSERT INTO teams(key, name, next_number) SELECT team, team, max(number) + 1 FROM issues_new GROUP BY team;
	DROP TABLE issues;
	ALTER TABLE issues_new RENAME TO issues;
	CREATE UNIQUE INDEX issues_team_number_idx ON issues(team, number);
	CREATE INDEX issues_status_updated_idx ON issues(status, updated_at DESC);
	CREATE INDEX issues_parent_idx ON issues(parent);`,

	// 2: references by id. Team keys are to become unique only within a
	// workspace, so tables reference teams, projects, and issues by integer
	// id. Issues, links, and comments keep their ids; teams and projects are
	// numbered in creation order. Related rows kept issue < other as text,
	// where 'AL-10' < 'AL-9', so they are reordered by id.
	`CREATE TABLE teams_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		key TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		next_number INTEGER NOT NULL
	);
	INSERT INTO teams_new(key, name, next_number) SELECT key, name, next_number FROM teams ORDER BY rowid;
	CREATE TABLE projects_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		description TEXT NOT NULL DEFAULT '',
		archived BOOLEAN NOT NULL DEFAULT FALSE CHECK (archived IN (0, 1))
	);
	INSERT INTO projects_new(name, description, archived) SELECT name, description, archived FROM projects ORDER BY rowid;
	CREATE TABLE issues_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		identifier TEXT NOT NULL UNIQUE,
		team_id INTEGER NOT NULL REFERENCES teams(id),
		number INTEGER NOT NULL,
		title TEXT NOT NULL CHECK (length(trim(title)) > 0),
		description TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL CHECK (status IN ('triage', 'todo', 'agent_ready', 'in_progress', 'in_review', 'done', 'canceled')),
		type TEXT CHECK (type IN ('spec', 'feature', 'bug', 'improvement', 'chore')),
		project_id INTEGER REFERENCES projects(id),
		parent_id INTEGER REFERENCES issues(id) CHECK (parent_id <> id),
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);
	INSERT INTO issues_new(id, identifier, team_id, number, title, description, status, type, project_id, parent_id, created_at, updated_at)
	SELECT id, identifier, (SELECT id FROM teams_new WHERE key = issues.team), number, title, description, status, type,
		(SELECT id FROM projects_new WHERE name = issues.project), (SELECT p.id FROM issues p WHERE p.identifier = issues.parent),
		created_at, updated_at
	FROM issues;
	CREATE TABLE relations_new (
		issue_id INTEGER NOT NULL REFERENCES issues(id),
		kind TEXT NOT NULL CHECK (kind IN ('blocks', 'related')),
		other_id INTEGER NOT NULL REFERENCES issues(id),
		PRIMARY KEY (issue_id, kind, other_id),
		CHECK (issue_id <> other_id),
		CHECK (kind <> 'related' OR issue_id < other_id)
	);
	INSERT INTO relations_new(issue_id, kind, other_id)
	SELECT CASE kind WHEN 'related' THEN min(issue_id, other_id) ELSE issue_id END, kind,
		CASE kind WHEN 'related' THEN max(issue_id, other_id) ELSE other_id END
	FROM (SELECT (SELECT id FROM issues WHERE identifier = relations.issue) AS issue_id, kind,
		(SELECT id FROM issues WHERE identifier = relations.other) AS other_id FROM relations);
	CREATE TABLE links_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		issue_id INTEGER NOT NULL REFERENCES issues(id),
		title TEXT NOT NULL,
		url TEXT NOT NULL
	);
	INSERT INTO links_new(id, issue_id, title, url)
	SELECT id, (SELECT id FROM issues WHERE identifier = links.issue), title, url FROM links;
	CREATE TABLE comments_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		issue_id INTEGER NOT NULL REFERENCES issues(id),
		body TEXT NOT NULL,
		created_at TEXT NOT NULL
	);
	INSERT INTO comments_new(id, issue_id, body, created_at)
	SELECT id, (SELECT id FROM issues WHERE identifier = comments.issue), body, created_at FROM comments;
	CREATE TABLE issue_labels_new (
		issue_id INTEGER NOT NULL REFERENCES issues(id),
		label_id INTEGER NOT NULL REFERENCES labels(id),
		PRIMARY KEY (issue_id, label_id)
	);
	INSERT INTO issue_labels_new(issue_id, label_id)
	SELECT (SELECT id FROM issues WHERE identifier = issue_labels.issue), label_id FROM issue_labels;
	DROP TABLE issue_labels;
	DROP TABLE comments;
	DROP TABLE links;
	DROP TABLE relations;
	DROP TABLE issues;
	DROP TABLE projects;
	DROP TABLE teams;
	ALTER TABLE teams_new RENAME TO teams;
	ALTER TABLE projects_new RENAME TO projects;
	ALTER TABLE issues_new RENAME TO issues;
	ALTER TABLE relations_new RENAME TO relations;
	ALTER TABLE links_new RENAME TO links;
	ALTER TABLE comments_new RENAME TO comments;
	ALTER TABLE issue_labels_new RENAME TO issue_labels;
	CREATE UNIQUE INDEX issues_team_number_idx ON issues(team_id, number);
	CREATE INDEX issues_status_updated_idx ON issues(status, updated_at DESC);
	CREATE INDEX issues_parent_idx ON issues(parent_id);
	CREATE INDEX issue_labels_label_idx ON issue_labels(label_id, issue_id);
	CREATE TRIGGER issue_labels_exclusive BEFORE INSERT ON issue_labels
	WHEN EXISTS (
		SELECT 1 FROM issue_labels il JOIN labels old ON old.id = il.label_id
		JOIN labels incoming ON incoming.id = NEW.label_id
		WHERE il.issue_id = NEW.issue_id AND old.group_id = incoming.group_id AND old.id <> incoming.id
	)
	BEGIN SELECT RAISE(ABORT, 'label group is single-select'); END;`,

	// 3: workspaces. Team keys, Project names, label group and label names,
	// and issue identifiers become unique within a workspace, so those tables
	// and the pull request records gain workspace_id. Everything so far moves
	// into workspace default. Every row keeps its id. The trigger is dropped
	// first because it names labels, which is rebuilt.
	`CREATE TABLE workspaces (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		slug TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL
	);
	INSERT INTO workspaces(slug, name) VALUES ('default', 'Default');
	DROP TRIGGER issue_labels_exclusive;
	CREATE TABLE projects_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
		name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		archived BOOLEAN NOT NULL DEFAULT FALSE CHECK (archived IN (0, 1)),
		UNIQUE (workspace_id, name)
	);
	INSERT INTO projects_new(id, workspace_id, name, description, archived)
	SELECT id, (SELECT id FROM workspaces), name, description, archived FROM projects;
	CREATE TABLE teams_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
		key TEXT NOT NULL,
		name TEXT NOT NULL,
		next_number INTEGER NOT NULL,
		UNIQUE (workspace_id, key)
	);
	INSERT INTO teams_new(id, workspace_id, key, name, next_number)
	SELECT id, (SELECT id FROM workspaces), key, name, next_number FROM teams;
	CREATE TABLE issues_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
		identifier TEXT NOT NULL,
		team_id INTEGER NOT NULL REFERENCES teams(id),
		number INTEGER NOT NULL,
		title TEXT NOT NULL CHECK (length(trim(title)) > 0),
		description TEXT NOT NULL DEFAULT '',
		status TEXT NOT NULL CHECK (status IN ('triage', 'todo', 'agent_ready', 'in_progress', 'in_review', 'done', 'canceled')),
		type TEXT CHECK (type IN ('spec', 'feature', 'bug', 'improvement', 'chore')),
		project_id INTEGER REFERENCES projects(id),
		parent_id INTEGER REFERENCES issues(id) CHECK (parent_id <> id),
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		UNIQUE (workspace_id, identifier)
	);
	INSERT INTO issues_new(id, workspace_id, identifier, team_id, number, title, description, status, type, project_id, parent_id, created_at, updated_at)
	SELECT id, (SELECT id FROM workspaces), identifier, team_id, number, title, description, status, type, project_id, parent_id, created_at, updated_at
	FROM issues;
	CREATE TABLE pull_requests_new (
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
	INSERT INTO pull_requests_new(workspace_id, repo, number, title, url, head_ref, state, issue_identifier, updated_at, linkback_posted_at)
	SELECT (SELECT id FROM workspaces), repo, number, title, url, head_ref, state, issue_identifier, updated_at, linkback_posted_at
	FROM pull_requests;
	CREATE TABLE github_sync_new (
		workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
		repo TEXT NOT NULL,
		last_updated_at TEXT NOT NULL,
		PRIMARY KEY (workspace_id, repo)
	);
	INSERT INTO github_sync_new(workspace_id, repo, last_updated_at)
	SELECT (SELECT id FROM workspaces), repo, last_updated_at FROM github_sync;
	CREATE TABLE label_groups_new (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		workspace_id INTEGER NOT NULL REFERENCES workspaces(id),
		name TEXT NOT NULL CHECK (length(trim(name)) > 0),
		name_key TEXT NOT NULL,
		UNIQUE (workspace_id, name_key),
		UNIQUE (workspace_id, id)
	);
	INSERT INTO label_groups_new(id, workspace_id, name, name_key)
	SELECT id, (SELECT id FROM workspaces), name, name_key FROM label_groups;
	CREATE TABLE labels_new (
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
	INSERT INTO labels_new(id, workspace_id, name, name_key, color, description, group_id, archived)
	SELECT id, (SELECT id FROM workspaces), name, name_key, color, description, group_id, archived FROM labels;
	DROP TABLE labels;
	DROP TABLE label_groups;
	DROP TABLE github_sync;
	DROP TABLE pull_requests;
	DROP TABLE issues;
	DROP TABLE teams;
	DROP TABLE projects;
	ALTER TABLE projects_new RENAME TO projects;
	ALTER TABLE teams_new RENAME TO teams;
	ALTER TABLE issues_new RENAME TO issues;
	ALTER TABLE pull_requests_new RENAME TO pull_requests;
	ALTER TABLE github_sync_new RENAME TO github_sync;
	ALTER TABLE label_groups_new RENAME TO label_groups;
	ALTER TABLE labels_new RENAME TO labels;
	CREATE UNIQUE INDEX issues_team_number_idx ON issues(team_id, number);
	CREATE INDEX issues_status_updated_idx ON issues(status, updated_at DESC);
	CREATE INDEX issues_parent_idx ON issues(parent_id);
	CREATE UNIQUE INDEX labels_name_idx ON labels(workspace_id, COALESCE(group_id, 0), name_key);
	CREATE TRIGGER issue_labels_exclusive BEFORE INSERT ON issue_labels
	WHEN EXISTS (
		SELECT 1 FROM issue_labels il JOIN labels old ON old.id = il.label_id
		JOIN labels incoming ON incoming.id = NEW.label_id
		WHERE il.issue_id = NEW.issue_id AND old.group_id = incoming.group_id AND old.id <> incoming.id
	)
	BEGIN SELECT RAISE(ABORT, 'label group is single-select'); END;`,
}

func migrate(db *sql.DB, backupDir string, migrations []string) error {
	var tables int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'issues'`).Scan(&tables); err != nil {
		return err
	}
	if tables == 0 {
		if _, err := db.Exec(schema); err != nil {
			return err
		}
		_, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, len(migrations)))
		return err
	}

	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version < len(migrations) {
		if err := os.MkdirAll(backupDir, 0o755); err != nil {
			return err
		}
		if _, err := db.Exec(`VACUUM INTO ?`, filepath.Join(backupDir, fmt.Sprintf("before-migration-%d.db", version+1))); err != nil {
			return err
		}
		// Rebuilding a table, which changing a CHECK constraint requires, only
		// works with foreign keys off outside the transaction.
		if _, err := db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
			return err
		}
		for ; version < len(migrations); version++ {
			if err := runMigration(db, migrations[version], version+1); err != nil {
				return fmt.Errorf("migration %d: %w", version+1, err)
			}
		}
		if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
			return err
		}
	}
	_, err := db.Exec(schema)
	return err
}

func runMigration(db *sql.DB, statements string, version int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(statements); err != nil {
		return err
	}
	rows, err := tx.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	violated := rows.Next()
	rows.Close()
	if violated {
		return errors.New("leaves foreign key violations")
	}
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, version)); err != nil {
		return err
	}
	return tx.Commit()
}
