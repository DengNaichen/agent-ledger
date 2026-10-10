package app

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
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
	service, err := Open(filepath.Join(dir, "ledger.db"), "AL")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	ctx := context.Background()
	if _, err := service.CreateProject(ctx, "Core", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(ctx, CreateIssue{Title: "Keep me", Project: "Core"}); err != nil {
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
	for range 2 {
		if err := migrate(service.db, backups, []string{rebuild}); err != nil {
			t.Fatal(err)
		}
	}

	if version := userVersion(t, service.db); version != 1 {
		t.Fatalf("user_version = %d, want 1", version)
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
	if _, err := os.Stat(filepath.Join(backups, "before-migration-1.db")); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateRollsBackForeignKeyViolations(t *testing.T) {
	dir := t.TempDir()
	service, err := Open(filepath.Join(dir, "ledger.db"), "AL")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	ctx := context.Background()
	if _, err := service.CreateProject(ctx, "Core", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(ctx, CreateIssue{Title: "Keep me", Project: "Core"}); err != nil {
		t.Fatal(err)
	}

	if err := migrate(service.db, filepath.Join(dir, "backups"), []string{`DELETE FROM projects`}); err == nil {
		t.Fatal("migration that orphans issues succeeded")
	}
	if version := userVersion(t, service.db); version != 0 {
		t.Fatalf("user_version = %d, want 0", version)
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
