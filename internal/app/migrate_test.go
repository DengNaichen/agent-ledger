package app

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
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
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE,
			description TEXT NOT NULL DEFAULT '',
			archived BOOLEAN NOT NULL DEFAULT FALSE CHECK (archived IN (0, 1)),
			color TEXT NOT NULL DEFAULT 'gray' CHECK (color IN ('gray', 'blue'))
		);
		INSERT INTO projects_new(id, name, description, archived) SELECT id, name, description, archived FROM projects;
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
	if err := service.db.QueryRow(`SELECT group_concat(identifier || ' ' || key || ' ' || number, ', ') FROM (SELECT identifier, key, number FROM issues JOIN teams ON teams.id = issues.team_id ORDER BY issues.id)`).Scan(&backfilled); err != nil {
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
		WHERE list.name = 'issues_team_number_idx' AND list."unique"`).Scan(&columns); err != nil || columns != "team_id,number" {
		t.Fatalf("unique index columns = %q, err = %v", columns, err)
	}
	if _, err := service.db.Exec(`INSERT INTO issues(identifier, team_id, number, title, status, created_at, updated_at)
		VALUES ('AL-1-copy', (SELECT id FROM teams WHERE key = 'AL'), 1, 'Duplicate number', 'triage', 'now', 'now')`); err == nil {
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

func TestMigrateIssueReferencesToIDs(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "old.db")
	old, err := os.ReadFile("testdata/schema-before-migration-2.sql")
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// AL-9 has a lower id than AL-10, but 'AL-10' < 'AL-9' as text, so their
	// related row is stored as (AL-10, AL-9) and must be reordered by id.
	if _, err := db.Exec(string(old) + `
		PRAGMA user_version = 1;
		INSERT INTO teams(key, name, next_number) VALUES ('TOK', 'Tokens', 2), ('AL', 'InfTask', 11);
		INSERT INTO projects(name, description) VALUES ('Core', 'Core work');
		INSERT INTO issues(id, identifier, team, number, title, description, status, type, project, parent, created_at, updated_at) VALUES
			(1, 'AL-1', 'AL', 1, 'Spec', 'Spec body', 'todo', 'spec', 'Core', NULL, 'c1', 'u1'),
			(2, 'AL-9', 'AL', 9, 'Nine', '', 'in_progress', 'feature', 'Core', 'AL-1', 'c2', 'u3'),
			(3, 'AL-10', 'AL', 10, 'Ten', 'Ten body', 'triage', NULL, NULL, 'AL-1', 'c3', 'u2'),
			(4, 'TOK-1', 'TOK', 1, 'Other team', '', 'done', 'chore', NULL, NULL, 'c4', 'u4');
		INSERT INTO relations(issue, kind, other) VALUES ('AL-1', 'blocks', 'TOK-1'), ('AL-10', 'related', 'AL-9');
		INSERT INTO links(issue, title, url) VALUES ('AL-9', 'Design', 'https://example.com/design');
		INSERT INTO comments(issue, body, created_at) VALUES ('AL-10', 'Keep comment', 'c5');
		INSERT INTO events(issue_id, kind, detail, created_at) VALUES (3, 'created', 'triage', 'c3');
		INSERT INTO label_groups(name, name_key) VALUES ('Priority', 'priority');
		INSERT INTO labels(name, name_key, color, group_id) VALUES ('High', 'high', '#d73a4a', 1), ('Risk', 'risk', '#5e6ad2', NULL);
		INSERT INTO issue_labels(issue, label_id) VALUES ('AL-1', 1), ('AL-1', 2), ('TOK-1', 2);
		INSERT INTO pull_requests(repo, number, title, url, head_ref, state, issue_identifier, updated_at)
			VALUES ('owner/repo', 7, 'AL-9 Nine', 'https://github.com/owner/repo/pull/7', 'al-9-nine', 'open', 'AL-9', 'p1');`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	// What every issue looked like before migration 2.
	high := Label{ID: 1, Name: "High", Color: "#d73a4a", GroupID: 1}
	risk := Label{ID: 2, Name: "Risk", Color: "#5e6ad2"}
	spec := Issue{Labels: []Label{high, risk}, Identifier: "AL-1", Title: "Spec", Status: StatusTodo, Type: new("spec"), Project: new("Core"), UpdatedAt: "u1", Description: "Spec body", CreatedAt: "c1"}
	nine := Issue{Labels: []Label{}, Identifier: "AL-9", Title: "Nine", Status: StatusInProgress, Type: new("feature"), Project: new("Core"), Parent: new("AL-1"), UpdatedAt: "u3", CreatedAt: "c2"}
	ten := Issue{Labels: []Label{}, Identifier: "AL-10", Title: "Ten", Status: StatusTriage, Parent: new("AL-1"), UpdatedAt: "u2", Description: "Ten body", CreatedAt: "c3"}
	tok := Issue{Labels: []Label{risk}, Identifier: "TOK-1", Title: "Other team", Status: StatusDone, Type: new("chore"), UpdatedAt: "u4", CreatedAt: "c4"}
	details := func(issue Issue) IssueDetails {
		return IssueDetails{Issue: issue, Comments: []Comment{}, Links: []Link{}, PullRequests: []PullRequest{},
			Children: []IssueSummary{}, Blocks: []IssueSummary{}, BlockedBy: []IssueSummary{}, Related: []IssueSummary{}}
	}
	al1, al9, al10, tok1 := details(spec), details(nine), details(ten), details(tok)
	al1.Children, al1.Blocks = Summaries([]Issue{nine, ten}), Summaries([]Issue{tok})
	al9.Links, al9.Related = []Link{{Title: "Design", URL: "https://example.com/design"}}, Summaries([]Issue{ten})
	al9.PullRequests = []PullRequest{{Repo: "owner/repo", Number: 7, Title: "AL-9 Nine", URL: "https://github.com/owner/repo/pull/7", State: "open", UpdatedAt: "p1"}}
	al10.Comments, al10.Related = []Comment{{Body: "Keep comment", CreatedAt: "c5"}}, Summaries([]Issue{nine})
	tok1.BlockedBy = Summaries([]Issue{spec})
	want := map[string]IssueDetails{"AL-1": al1, "AL-9": al9, "AL-10": al10, "TOK-1": tok1}

	service, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	ctx := context.Background()
	issues, err := service.List(ctx, ListIssues{Limit: new(-1)})
	if err != nil || len(issues) != len(want) {
		t.Fatalf("issues = %#v, err = %v", issues, err)
	}
	for _, listed := range issues {
		issue, err := service.Get(ctx, listed.Identifier)
		if err != nil || !reflect.DeepEqual(issue, want[listed.Identifier].Issue) {
			t.Fatalf("Get(%s) = %#v, err = %v; want %#v", listed.Identifier, issue, err, want[listed.Identifier].Issue)
		}
		detail, err := service.Details(ctx, listed.Identifier)
		if err != nil || !reflect.DeepEqual(detail, want[listed.Identifier]) {
			t.Fatalf("Details(%s) = %#v, err = %v; want %#v", listed.Identifier, detail, err, want[listed.Identifier])
		}
	}
	rows, err := service.db.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	violated := rows.Next()
	rows.Close()
	if violated {
		t.Fatal("foreign_key_check reports violations")
	}
	if _, err := os.Stat(filepath.Join(dir, "backups", "before-migration-2.db")); err != nil {
		t.Fatal(err)
	}
}
