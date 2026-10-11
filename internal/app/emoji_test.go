package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestEntityEmojiLifecycle(t *testing.T) {
	service, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	def := inWorkspace(t, service, "default")
	other := inWorkspace(t, service, "other")
	ctx := context.Background()
	for _, s := range []*Service{def, other} {
		alTeam(t, s)
		if _, err := s.CreateProject(ctx, "Core", "Outcome"); err != nil {
			t.Fatal(err)
		}
	}
	// JSON keeps this test runnable against the original model as Red.
	updateProject := func(s *Service, value string) (map[string]any, error) {
		var input UpdateProject
		if err := json.Unmarshal([]byte(`{"name":"Core","emoji":`+value+`}`), &input); err != nil {
			t.Fatal(err)
		}
		p, err := s.UpdateProject(ctx, input)
		raw, _ := json.Marshal(p)
		var result map[string]any
		json.Unmarshal(raw, &result)
		return result, err
	}
	for _, emoji := range []string{"🚀", "👩🏽‍💻", "🇨🇳", ""} {
		value, _ := json.Marshal(emoji)
		p, err := updateProject(def, string(value))
		if err != nil || p["emoji"] != emoji {
			t.Fatalf("emoji = %#v, error %v; want %q", p["emoji"], err, emoji)
		}
	}
	p, err := updateProject(def, `"🚀"`)
	if err != nil {
		t.Fatal(err)
	}
	var omitted UpdateProject
	json.Unmarshal([]byte(`{"name":"Core","description":"Updated"}`), &omitted)
	kept, err := def.UpdateProject(ctx, omitted)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(kept)
	json.Unmarshal(raw, &p)
	if p["emoji"] != "🚀" {
		t.Fatalf("omitted emoji cleared: %#v", p)
	}
	projects, _ := other.ListProjects(ctx, false)
	raw, _ = json.Marshal(projects[0])
	json.Unmarshal(raw, &p)
	if p["emoji"] != "" {
		t.Fatalf("cross-workspace update: %#v", p)
	}
	for _, value := range []string{`"hello"`, `"🚀🚀"`, `"<script>"`} {
		if _, err := updateProject(def, value); err == nil {
			t.Fatalf("accepted invalid emoji %s", value)
		}
	}
	if _, err := def.UpdateProject(ctx, UpdateProject{Name: "missing"}); !errors.Is(err, ErrProjectNotFound) {
		t.Fatal(err)
	}
	// Team create accepts emoji and must preserve numbering.
	var input CreateTeam
	json.Unmarshal([]byte(`{"key":"DEV","name":"Developers","start_number":42,"emoji":"💻"}`), &input)
	team, err := def.CreateTeam(ctx, input)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(team)
	json.Unmarshal(raw, &p)
	if p["emoji"] != "💻" || team.NextNumber != 42 {
		t.Fatalf("team = %#v", p)
	}
}

func TestMigrateEntityEmoji(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// Reconstruct the previously delivered schema, without the new columns.
	old := strings.ReplaceAll(schema, "    emoji TEXT NOT NULL DEFAULT '',\n", "")
	if _, err := db.Exec(old); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO workspaces(id,slug,name) VALUES(1,'default','Default'); INSERT INTO teams(workspace_id,key,name,next_number) VALUES(1,'AL','InfTask',42); INSERT INTO projects(workspace_id,name,description) VALUES(1,'Core','Keep me'); PRAGMA user_version = 6;`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	service, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	var emoji string
	if err := service.db.QueryRow(`SELECT emoji FROM projects`).Scan(&emoji); err != nil || emoji != "" {
		t.Fatalf("project emoji %q: %v", emoji, err)
	}
	if err := service.db.QueryRow(`SELECT emoji FROM teams`).Scan(&emoji); err != nil || emoji != "" {
		t.Fatalf("team emoji %q: %v", emoji, err)
	}
	def, err := service.InWorkspace(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	teams, _ := def.ListTeams(context.Background())
	projects, _ := def.ListProjects(context.Background(), false)
	if teams[0].NextNumber != 42 || projects[0].Description != "Keep me" {
		t.Fatal("migration lost data")
	}
	if _, err := os.Stat(filepath.Join(dir, "backups", "before-migration-7.db")); err != nil {
		t.Fatal(err)
	}
	fresh, err := Open(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	for _, table := range []string{"projects", "teams"} {
		columns := func(db *sql.DB) []string {
			rows, err := db.Query(`SELECT name || ':' || type || ':' || "notnull" || ':' || COALESCE(dflt_value,'') FROM pragma_table_info(?) ORDER BY name`, table)
			if err != nil {
				t.Fatal(err)
			}
			defer rows.Close()
			var result []string
			for rows.Next() {
				var col string
				if err := rows.Scan(&col); err != nil {
					t.Fatal(err)
				}
				result = append(result, col)
			}
			return result
		}
		if !reflect.DeepEqual(columns(service.db), columns(fresh.db)) {
			t.Fatalf("schema mismatch %s", table)
		}
	}
}
