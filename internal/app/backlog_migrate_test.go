package app

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMigrateBacklog(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "version-3.db")
	old, err := os.ReadFile("testdata/schema-before-migration-4.sql")
	if err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(old) + `
		PRAGMA user_version = 3;
		INSERT INTO workspaces(id, slug, name) VALUES (1, 'default', 'Default'), (2, 'acme', 'Acme');
		INSERT INTO teams(id, workspace_id, key, name, next_number) VALUES (1, 1, 'AL', 'InfTask', 8), (2, 2, 'AL', 'Acme', 2);
		INSERT INTO projects(id, workspace_id, name, description, archived) VALUES (1, 1, 'Core', 'Keep context', 1);
		INSERT INTO issues(id, workspace_id, identifier, team_id, number, title, description, status, type, project_id, parent_id, created_at, updated_at) VALUES
			(1, 1, 'AL-1', 1, 1, 'Triage', 'Undecided', 'triage', 'spec', 1, NULL, 'c1', 'u1'),
			(2, 1, 'AL-2', 1, 2, 'Todo', 'Committed', 'todo', 'feature', 1, 1, 'c2', 'u2'),
			(3, 1, 'AL-3', 1, 3, 'Ready', '', 'agent_ready', 'bug', NULL, NULL, 'c3', 'u3'),
			(4, 1, 'AL-4', 1, 4, 'Active', '', 'in_progress', 'improvement', NULL, NULL, 'c4', 'u4'),
			(5, 1, 'AL-5', 1, 5, 'Review', '', 'in_review', 'chore', NULL, NULL, 'c5', 'u5'),
			(6, 1, 'AL-6', 1, 6, 'Done', '', 'done', NULL, NULL, NULL, 'c6', 'u6'),
			(7, 1, 'AL-7', 1, 7, 'Canceled', '', 'canceled', NULL, NULL, NULL, 'c7', 'u7'),
			(8, 2, 'AL-1', 2, 1, 'Other workspace', 'Same identifier', 'todo', 'spec', NULL, NULL, 'c8', 'u8'),
			(100, 1, 'AL-100', 1, 100, 'Deleted', '', 'todo', NULL, NULL, NULL, 'c100', 'u100');
		DELETE FROM issues WHERE id = 100;
		INSERT INTO relations(issue_id, kind, other_id) VALUES (1, 'blocks', 2), (2, 'related', 3);
		INSERT INTO links(id, issue_id, title, url) VALUES (9, 2, 'Design', 'https://example.com/design');
		INSERT INTO comments(id, issue_id, body, created_at) VALUES (10, 1, 'Keep comment', 'c9');
		INSERT INTO events(id, issue_id, kind, detail, created_at) VALUES (11, 1, 'created', 'triage', 'c1'), (12, 2, 'status_changed', 'triage -> todo', 'u2');
		INSERT INTO label_groups(id, workspace_id, name, name_key) VALUES (1, 1, 'Risk', 'risk');
		INSERT INTO labels(id, workspace_id, name, name_key, color, description, group_id, archived) VALUES (1, 1, 'High', 'high', '#fc7840', 'Keep label', 1, 1);
		INSERT INTO issue_labels(issue_id, label_id) VALUES (1, 1);
		INSERT INTO pull_requests(workspace_id, repo, number, title, url, head_ref, state, issue_identifier, updated_at, linkback_posted_at) VALUES
			(1, 'owner/repo', 7, 'AL-2 Todo', 'https://example.com/pr/7', 'al-2-todo', 'open', 'AL-2', 'p1', 'l1');
		INSERT INTO github_sync(workspace_id, repo, last_updated_at) VALUES (1, 'owner/repo', 'p1');`); err != nil {
		t.Fatal(err)
	}

	// These fixed tables include every issue reference and all stored fields.
	snapshot := func(db *sql.DB) map[string][][]any {
		t.Helper()
		result := map[string][][]any{}
		for _, table := range []string{"workspaces", "teams", "projects", "issues", "relations", "links", "comments", "events", "label_groups", "labels", "issue_labels", "pull_requests", "github_sync"} {
			rows, err := db.Query("SELECT * FROM " + table + " ORDER BY rowid")
			if err != nil {
				t.Fatal(err)
			}
			cols, err := rows.Columns()
			if err != nil {
				t.Fatal(err)
			}
			for rows.Next() {
				values := make([]any, len(cols))
				pointers := make([]any, len(cols))
				for i := range values {
					pointers[i] = &values[i]
				}
				if err := rows.Scan(pointers...); err != nil {
					t.Fatal(err)
				}
				result[table] = append(result[table], values)
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			rows.Close()
		}
		return result
	}
	before := snapshot(db)
	db.Close()
	service, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	// Migration 5 then appends priority, 0 for every existing issue, and
	// migration 6 appends author, and the entity icon migration appends
	// emoji to teams and projects, empty for every existing record.
	after := snapshot(service.db)
	for i, row := range after["projects"] {
		if row[len(row)-1] != nil || row[len(row)-2] != nil {
			t.Fatal("historical project scheduled")
		}
		after["projects"][i] = row[:len(row)-2]
	}
	for i, row := range after["issues"] {
		if row[len(row)-1] != nil {
			t.Fatal("existing issue gained cycle")
		}
		after["issues"][i] = row[:len(row)-1]
	}
	for i, row := range after["issues"] {
		if row[len(row)-1] != nil {
			t.Fatal("existing issue assigned")
		}
		after["issues"][i] = row[:len(row)-1]
	}
	for i, row := range after["teams"] {
		if row[len(row)-1] != int64(0) {
			t.Fatal("existing team became private")
		}
		after["teams"][i] = row[:len(row)-1]
	}
	for table, added := range map[string]any{"issues": int64(0), "comments": "", "teams": "", "projects": ""} {
		for i, row := range after[table] {
			if row[len(row)-1] != added {
				t.Fatalf("migrated %s = %#v", table, row)
			}
			after[table][i] = row[:len(row)-1]
		}
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("migration changed stored rows:\nbefore = %#v\nafter = %#v", before, after)
	}
	ctx := context.Background()
	def, err := service.InWorkspace(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	issue, err := def.Update(ctx, UpdateIssue{Identifier: "AL-2", Status: new(StatusBacklog)})
	if err != nil || issue.Status != StatusBacklog {
		t.Fatalf("upgraded backlog update: %#v, %v", issue, err)
	}
	if version := userVersion(t, service.db); version != len(migrations) {
		t.Fatalf("user_version = %d, want %d", version, len(migrations))
	}
	backup, err := sql.Open("sqlite", filepath.Join(dir, "backups", "before-migration-4.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer backup.Close()
	if userVersion(t, backup) != 3 || !reflect.DeepEqual(before, snapshot(backup)) {
		t.Fatal("pre-migration backup does not preserve version 3 rows")
	}
	var sequence int
	if err := service.db.QueryRow(`SELECT seq FROM sqlite_sequence WHERE name = 'issues'`).Scan(&sequence); err != nil || sequence != 100 {
		t.Fatalf("issues sequence = %d, err = %v", sequence, err)
	}
	for _, slug := range []string{"default", "acme"} {
		scoped, err := service.InWorkspace(ctx, slug)
		if err != nil {
			t.Fatal(err)
		}
		created, err := scoped.Create(ctx, CreateIssue{Team: "AL", Title: "Keep for later", Status: new(StatusBacklog)})
		if err != nil || created.Status != StatusBacklog {
			t.Fatalf("%s backlog create: %#v, %v", slug, created, err)
		}
		listed, err := scoped.List(ctx, ListIssues{Status: new(StatusBacklog)})
		want := 1
		if slug == "default" {
			want = 2
		}
		if err != nil || len(listed) != want {
			t.Fatalf("%s backlog filter: %#v, %v", slug, listed, err)
		}
	}
	var nextID int
	if err := service.db.QueryRow(`SELECT id FROM issues WHERE workspace_id = 1 AND identifier = 'AL-8'`).Scan(&nextID); err != nil || nextID != 101 {
		t.Fatalf("next id = %d, err = %v", nextID, err)
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
	var foreignKeys int
	if err := service.db.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys); err != nil || foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d, err = %v", foreignKeys, err)
	}
}
