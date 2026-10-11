package app

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestMigrateCommentAuthor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// Start from the delivered version-5 schema, before comments had authors.
	old, err := os.ReadFile("testdata/schema-before-migration-6.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(old) + `PRAGMA user_version = 5;
		INSERT INTO workspaces(slug, name) VALUES ('default', 'Default');
		INSERT INTO teams(workspace_id, key, name, next_number) VALUES (1, 'AL', 'InfTask', 2);
		INSERT INTO issues(workspace_id, identifier, team_id, number, title, status, created_at, updated_at, priority)
		VALUES (1, 'AL-1', 1, 1, 'Keep comments', 'todo', 'c1', 'u1', 2);
		INSERT INTO comments(id, issue_id, body, created_at) VALUES (7, 1, 'First', 'c2'), (8, 1, 'Second', 'c3');`); err != nil {
		t.Fatal(err)
	}
	db.Close()

	root, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	var comments string
	if err := root.db.QueryRow(`SELECT group_concat(id || ' ' || issue_id || ' ' || body || ' ' || created_at || ' [' || author || ']', ', ')
		FROM (SELECT * FROM comments ORDER BY id)`).Scan(&comments); err != nil {
		t.Fatal(err)
	}
	if comments != "7 1 First c2 [], 8 1 Second c3 []" {
		t.Fatalf("migrated comments = %q", comments)
	}
	if version := userVersion(t, root.db); version != len(migrations) {
		t.Fatalf("user_version = %d, want %d", version, len(migrations))
	}
	if _, err := os.Stat(filepath.Join(dir, "backups", "before-migration-6.db")); err != nil {
		t.Fatal(err)
	}
	s, err := root.InWorkspace(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddComment(context.Background(), "AL-1", "Local"); err != nil {
		t.Fatal(err)
	}
	var author string
	if err := root.db.QueryRow(`SELECT author FROM comments WHERE body = 'Local'`).Scan(&author); err != nil || author != "" {
		t.Fatalf("local comment author = %q, err = %v", author, err)
	}

	// A new database and an upgraded one have the same tables, columns,
	// foreign keys, indexes, and triggers.
	fresh, err := Open(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	if upgraded, created := structure(t, root.db), structure(t, fresh.db); !reflect.DeepEqual(upgraded, created) {
		t.Fatalf("schemas differ:\nupgraded = %v\nfresh = %v", upgraded, created)
	}
}

// structure lists every table's columns, foreign keys, and indexes, and every
// trigger's SQL.
func structure(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`
		SELECT 'column ' || m.name || ' ' || c.cid || ' ' || c.name || ' ' || c.type || ' ' || c."notnull" || ' ' || ifnull(c.dflt_value, 'NULL') || ' ' || c.pk
		FROM sqlite_master m, pragma_table_info(m.name) c WHERE m.type = 'table'
		UNION ALL
		SELECT 'foreign key ' || m.name || ' ' || f.id || ' ' || f.seq || ' ' || f."table" || ' ' || f."from" || ' ' || ifnull(f."to", 'NULL') || ' ' || f.on_update || ' ' || f.on_delete
		FROM sqlite_master m, pragma_foreign_key_list(m.name) f WHERE m.type = 'table'
		UNION ALL
		SELECT 'index ' || m.name || ' ' || i.name || ' ' || i."unique" || ' ' || i.origin || ' ' || i.partial
		FROM sqlite_master m, pragma_index_list(m.name) i WHERE m.type = 'table'
		UNION ALL
		SELECT 'trigger ' || name || ' ' || sql FROM sqlite_master WHERE type = 'trigger'
		ORDER BY 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var line string
		if err := rows.Scan(&line); err != nil {
			t.Fatal(err)
		}
		result = append(result, line)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return result
}
