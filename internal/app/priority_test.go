package app

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func TestIssuePriority(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { s.Close() }()
	s = inWorkspace(t, s, "default")
	alTeam(t, s)
	ctx := context.Background()
	var ids []string
	for _, priority := range []int{1, 4, 3, 2, 2, 0} {
		issue, err := s.Create(ctx, CreateIssue{Team: "AL", Title: "Work", Status: new(StatusAgentReady), Priority: priority})
		if err != nil || issue.Priority != priority {
			t.Fatalf("create %d: %#v, %v", priority, issue, err)
		}
		ids = append(ids, issue.Identifier)
	}
	check := func(input ListIssues, want []string) {
		t.Helper()
		issues, err := s.List(ctx, input)
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, issue := range issues {
			got = append(got, issue.Identifier)
		}
		if !slices.Equal(got, want) {
			t.Fatalf("list %#v: %v, want %v", input, got, want)
		}
	}
	check(ListIssues{Status: new(StatusAgentReady)}, []string{ids[0], ids[4], ids[3], ids[2], ids[1], ids[5]})
	check(ListIssues{Status: new(StatusAgentReady), Limit: new(1)}, []string{ids[0]})
	check(ListIssues{}, []string{ids[5], ids[4], ids[3], ids[2], ids[1], ids[0]})
	check(ListIssues{Status: new(StatusAgentReady), Sort: "updated"}, []string{ids[5], ids[4], ids[3], ids[2], ids[1], ids[0]})
	check(ListIssues{Sort: "priority", Limit: new(1)}, []string{ids[0]})
	check(ListIssues{Priority: new(2)}, []string{ids[4], ids[3]})
	check(ListIssues{Priority: new(0), Status: new(StatusAgentReady), Query: "Work"}, []string{ids[5]})
	// SQL tie-breaking must be deterministic even when timestamps match.
	if _, err := s.db.Exec(`UPDATE issues SET updated_at = 'same' WHERE priority = 2`); err != nil {
		t.Fatal(err)
	}
	check(ListIssues{Sort: "priority", Priority: new(2)}, []string{ids[4], ids[3]})
	for _, input := range []UpdateIssue{
		{Identifier: ids[0], Title: new("Edited")},
		{Identifier: ids[0], Priority: new(0)},
	} {
		updated, err := s.Update(ctx, input)
		want := 1
		if input.Priority != nil {
			want = 0
		}
		if err != nil || updated.Priority != want {
			t.Fatalf("update: %#v, %v", updated, err)
		}
	}
	if _, err := s.Update(ctx, UpdateIssue{Identifier: ids[0], Priority: new(2)}); err != nil {
		t.Fatal(err)
	}
	if err := s.Relation(ctx, ids[1], "related", ids[0], false); err != nil {
		t.Fatal(err)
	}
	details, err := s.Details(ctx, ids[1])
	if err != nil || details.Related[0].Priority != 2 {
		t.Fatalf("summary: %#v, %v", details, err)
	}
	for _, bad := range []int{-1, 5} {
		if _, err := s.Create(ctx, CreateIssue{Team: "AL", Title: "Invalid", Priority: bad}); !errors.Is(err, ErrInvalidPriority) {
			t.Fatalf("create %d: %v", bad, err)
		}
		if _, err := s.Update(ctx, UpdateIssue{Identifier: ids[0], Priority: &bad}); !errors.Is(err, ErrInvalidPriority) {
			t.Fatalf("update %d: %v", bad, err)
		}
		if _, err := s.List(ctx, ListIssues{Priority: &bad}); !errors.Is(err, ErrInvalidPriority) {
			t.Fatalf("filter %d: %v", bad, err)
		}
		if _, err := s.db.Exec(`UPDATE issues SET priority = ? WHERE identifier = ?`, bad, ids[0]); err == nil {
			t.Fatalf("database accepted %d", bad)
		}
	}
	if _, err := s.List(ctx, ListIssues{Sort: "title"}); !errors.Is(err, ErrInvalidSort) {
		t.Fatalf("sort: %v", err)
	}
	if got, err := s.Get(ctx, ids[0]); err != nil || got.Priority != 2 {
		t.Fatalf("invalid writes changed priority: %#v, %v", got, err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	scoped, err := s.InWorkspace(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	s = scoped
	if got, err := s.Get(ctx, ids[0]); err != nil || got.Priority != 2 {
		t.Fatalf("reopen: %#v, %v", got, err)
	}
}

func TestMigrateIssuePriority(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// Start from the delivered version-4 schema, before priority existed.
	old, err := os.ReadFile("testdata/schema-before-migration-5.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(old) + `PRAGMA user_version = 4;
 INSERT INTO workspaces(slug, name) VALUES ('default', 'Default');
 INSERT INTO teams(workspace_id, key, name, next_number) VALUES (1, 'AL', 'InfTask', 3);
 INSERT INTO projects(workspace_id, name, description) VALUES (1, 'Core', 'Keep');
 INSERT INTO issues(workspace_id, identifier, team_id, number, title, description, status, project_id, parent_id, created_at, updated_at)
 VALUES (1, 'AL-1', 1, 1, 'Parent', 'Body', 'todo', 1, NULL, 'c1', 'u1'), (1, 'AL-2', 1, 2, 'Child', '', 'agent_ready', 1, 1, 'c2', 'u2');
 INSERT INTO relations VALUES (1, 'blocks', 2);
 INSERT INTO comments(issue_id, body, created_at) VALUES (2, 'Comment', 'c3');
 INSERT INTO labels(workspace_id, name, name_key, color) VALUES (1, 'Keep', 'keep', '#ffffff');
 INSERT INTO issue_labels VALUES (2, 1);`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	root, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	s, err := root.InWorkspace(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`UPDATE issues SET priority = 5`); err == nil {
		t.Fatal("upgraded database accepted invalid priority")
	}
	details, err := s.Details(context.Background(), "AL-2")
	if err != nil || details.Priority != 0 || len(details.BlockedBy) != 1 || len(details.Comments) != 1 || len(details.Labels) != 1 || *details.Parent != "AL-1" || *details.Project != "Core" {
		t.Fatalf("migration lost data: %#v, %v", details, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "backups", "before-migration-5.db")); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(context.Background(), UpdateIssue{Identifier: "AL-2", Priority: new(1)}); err != nil {
		t.Fatal(err)
	}
	if err := migrate(s.db, s.backupDir, migrations); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Get(context.Background(), "AL-2"); err != nil || got.Priority != 1 {
		t.Fatalf("repeated migration: %#v, %v", got, err)
	}
	// Fresh and upgraded schemas enforce the same priority definition.
	fresh, err := Open(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	columns := func(db *sql.DB) []string {
		t.Helper()
		rows, err := db.Query(`SELECT type, "notnull", dflt_value FROM pragma_table_info('issues') WHERE name = 'priority'`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var result []string
		for rows.Next() {
			var typ, notnull, def string
			if err := rows.Scan(&typ, &notnull, &def); err != nil {
				t.Fatal(err)
			}
			result = append(result, typ, notnull, def)
		}
		return result
	}
	if !reflect.DeepEqual(columns(s.db), []string{"INTEGER", "1", "0"}) || !reflect.DeepEqual(columns(s.db), columns(fresh.db)) {
		t.Fatal("priority column differs across schema paths")
	}
}
