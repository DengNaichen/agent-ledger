package app

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestMigrateSkipsMigrationsForNewDatabase(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "new.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := migrate(db, t.TempDir(), []string{"not SQL; it must never run"}); err != nil {
		t.Fatal(err)
	}
	if version := userVersion(t, db); version != 1 {
		t.Fatalf("user_version = %d, want 1", version)
	}
}

func TestMigrateRebuildsReferencedTable(t *testing.T) {
	dir := t.TempDir()
	service, err := Open(filepath.Join(dir, "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	alTeam(t, service)
	ctx := context.Background()
	if _, err := service.CreateProject(ctx, "Core", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(ctx, CreateIssue{Team: "AL", Title: "Keep me", Project: "Core"}); err != nil {
		t.Fatal(err)
	}

	// Changing a CHECK constraint means rebuilding the table; issues references projects.
	rebuild := `
		CREATE TABLE projects_new (
			name TEXT PRIMARY KEY NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			archived BOOLEAN NOT NULL DEFAULT FALSE CHECK (archived IN (0, 1)),
			color TEXT NOT NULL DEFAULT 'gray' CHECK (color IN ('gray', 'blue'))
		);
		INSERT INTO projects_new(name, description, archived) SELECT name, description, archived FROM projects;
		DROP TABLE projects;
		ALTER TABLE projects_new RENAME TO projects;`
	backups := filepath.Join(dir, "backups")
	next := len(migrations) + 1
	for range 2 {
		if err := migrate(service.db, backups, append(slices.Clip(migrations), rebuild)); err != nil {
			t.Fatal(err)
		}
	}

	if version := userVersion(t, service.db); version != next {
		t.Fatalf("user_version = %d, want %d", version, next)
	}
	var color string
	var foreignKeys int
	if err := service.db.QueryRow(`SELECT color FROM projects WHERE name = 'Core'`).Scan(&color); err != nil || color != "gray" {
		t.Fatalf("color = %q, err = %v", color, err)
	}
	if err := service.db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, err = %v", foreignKeys, err)
	}
	if issue, err := service.Get(ctx, "AL-1"); err != nil || *issue.Project != "Core" {
		t.Fatalf("issue = %#v, err = %v", issue, err)
	}
	if _, err := os.Stat(filepath.Join(backups, fmt.Sprintf("before-migration-%d.db", next))); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateRollsBackForeignKeyViolations(t *testing.T) {
	dir := t.TempDir()
	service, err := Open(filepath.Join(dir, "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	alTeam(t, service)
	ctx := context.Background()
	if _, err := service.CreateProject(ctx, "Core", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(ctx, CreateIssue{Team: "AL", Title: "Keep me", Project: "Core"}); err != nil {
		t.Fatal(err)
	}

	if err := migrate(service.db, filepath.Join(dir, "backups"), append(slices.Clip(migrations), `DELETE FROM projects`)); err == nil {
		t.Fatal("migration that orphans issues succeeded")
	}
	if version := userVersion(t, service.db); version != len(migrations) {
		t.Fatalf("user_version = %d, want %d", version, len(migrations))
	}
	if projects, err := service.ListProjects(ctx, true); err != nil || len(projects) != 1 {
		t.Fatalf("projects = %#v, err = %v", projects, err)
	}
}

func userVersion(t *testing.T, db *sql.DB) int {
	t.Helper()
	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		t.Fatal(err)
	}
	return version
}

func TestMigrateBackfillsTeams(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "old.db")
	old, err := os.ReadFile("testdata/schema-before-migration-1.sql")
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// Issue ids were global and -prefix could change between runs, so one
	// database can hold several prefixes, each with gaps in its numbers.
	if _, err := db.Exec(string(old) + `
		INSERT INTO projects(name) VALUES ('Core');
		INSERT INTO issues(id, identifier, title, status, project, created_at, updated_at) VALUES
			(1, 'AL-1', 'Spec', 'todo', 'Core', 'before', 'before'),
			(2, 'AL-2', 'Child', 'done', NULL, 'before', 'before'),
			(3, 'TOK-3', 'After a prefix change', 'triage', NULL, 'before', 'before'),
			(4, 'AL-4', 'Latest', 'in_progress', NULL, 'before', 'before');
		UPDATE issues SET parent = 'AL-1' WHERE identifier = 'AL-2';
		INSERT INTO relations(issue, kind, other) VALUES ('AL-1', 'blocks', 'TOK-3');
		INSERT INTO comments(issue, body, created_at) VALUES ('AL-2', 'Keep comment', 'before');
		INSERT INTO links(issue, title, url) VALUES ('AL-4', 'Design', 'https://example.com/design');
		INSERT INTO events(issue_id, kind, detail, created_at) VALUES (4, 'created', 'in_progress', 'before');
		INSERT INTO labels(name, name_key, color) VALUES ('Risk', 'risk', '#5e6ad2');
		INSERT INTO issue_labels(issue, label_id) VALUES ('AL-1', 1);`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	service, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	ctx := context.Background()
	teams, err := service.ListTeams(ctx)
	if want := []Team{{Key: "AL", Name: "AL", NextNumber: 5}, {Key: "TOK", Name: "TOK", NextNumber: 4}}; err != nil || !slices.Equal(teams, want) {
		t.Fatalf("teams = %#v, err = %v; want %#v", teams, err, want)
	}
	var backfilled string
	if err := service.db.QueryRow(`SELECT group_concat(identifier || ' ' || team || ' ' || number, ', ') FROM (SELECT * FROM issues ORDER BY id)`).Scan(&backfilled); err != nil {
		t.Fatal(err)
	}
	if backfilled != "AL-1 AL 1, AL-2 AL 2, TOK-3 TOK 3, AL-4 AL 4" {
		t.Fatalf("backfilled = %q", backfilled)
	}
	detail, err := service.Details(ctx, "AL-1")
	if err != nil || *detail.Project != "Core" || len(detail.Children) != 1 || detail.Children[0].Identifier != "AL-2" ||
		len(detail.Blocks) != 1 || detail.Blocks[0].Identifier != "TOK-3" || len(detail.Labels) != 1 {
		t.Fatalf("AL-1 lost data: %#v, %v", detail, err)
	}
	if detail, err := service.Details(ctx, "AL-2"); err != nil || len(detail.Comments) != 1 {
		t.Fatalf("AL-2 lost comments: %#v, %v", detail, err)
	}
	if detail, err := service.Details(ctx, "AL-4"); err != nil || len(detail.Links) != 1 {
		t.Fatalf("AL-4 lost links: %#v, %v", detail, err)
	}
	var events int
	if err := service.db.QueryRow(`SELECT count(*) FROM events JOIN issues ON issues.id = events.issue_id WHERE identifier = 'AL-4'`).Scan(&events); err != nil || events != 1 {
		t.Fatalf("AL-4 events = %d, err = %v", events, err)
	}

	// Numbering continues per team, and the unique index guards (team, number).
	for team, want := range map[string]string{"AL": "AL-5", "TOK": "TOK-4"} {
		if issue, err := service.Create(ctx, CreateIssue{Team: team, Title: "After upgrade"}); err != nil || issue.Identifier != want {
			t.Fatalf("issue = %#v, err = %v; want %s", issue, err, want)
		}
	}
	var columns string
	if err := service.db.QueryRow(`SELECT group_concat(info.name, ',') FROM pragma_index_list('issues') list, pragma_index_info(list.name) info
		WHERE list.name = 'issues_team_number_idx' AND list."unique"`).Scan(&columns); err != nil || columns != "team,number" {
		t.Fatalf("unique index columns = %q, err = %v", columns, err)
	}
	if _, err := service.db.Exec(`INSERT INTO issues(identifier, team, number, title, status, created_at, updated_at)
		VALUES ('AL-1-copy', 'AL', 1, 'Duplicate number', 'triage', 'now', 'now')`); err == nil {
		t.Fatal("database accepted a second AL 1")
	}

	if version := userVersion(t, service.db); version != len(migrations) {
		t.Fatalf("user_version = %d, want %d", version, len(migrations))
	}
	var foreignKeys int
	if err := service.db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, err = %v", foreignKeys, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "backups", "before-migration-1.db")); err != nil {
		t.Fatal(err)
	}
}
