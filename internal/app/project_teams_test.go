package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestProjectTeamsLifecycle(t *testing.T) {
	s := labelService(t)
	ctx := context.Background()
	if _, err := s.CreateTeam(ctx, CreateTeam{Key: "BB", Name: "B"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProject(ctx, "Missing", ""); !errors.Is(err, ErrProjectTeamsRequired) {
		t.Fatalf("missing teams: %v", err)
	}
	p, err := s.CreateProject(ctx, "Empty", "", "AL")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID == 0 || !reflect.DeepEqual(p.Teams, []string{"AL"}) {
		t.Fatalf("project: %#v", p)
	}
	for _, tc := range []struct {
		team  string
		count int
	}{{"AL", 1}, {"BB", 0}} {
		ps, err := s.ListProjects(ctx, false, tc.team)
		if err != nil || len(ps) != tc.count {
			t.Fatalf("filter %s: %#v %v", tc.team, ps, err)
		}
	}
	teams := []string{"BB", "AL"}
	p, err = s.UpdateProject(ctx, UpdateProject{ID: p.ID, Teams: &teams})
	if err != nil || !reflect.DeepEqual(p.Teams, []string{"AL", "BB"}) {
		t.Fatalf("share: %#v %v", p, err)
	}
	bad := []string{"AL", "ZZ"}
	if _, err = s.UpdateProject(ctx, UpdateProject{ID: p.ID, Description: new("changed"), Teams: &bad}); !errors.Is(err, ErrTeamNotFound) {
		t.Fatal(err)
	}
	p, err = s.GetProject(ctx, ProjectRef{ID: p.ID})
	if err != nil || p.Description != "" || len(p.Teams) != 2 {
		t.Fatalf("rollback: %#v %v", p, err)
	}
	if _, err = s.GetProject(ctx, ProjectRef{ID: p.ID, Name: "other"}); !errors.Is(err, ErrProjectReference) {
		t.Fatal(err)
	}
	other := inWorkspace(t, s, "other")
	if _, err := other.CreateTeam(ctx, CreateTeam{Key: "ZZ", Name: "Other"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProject(ctx, "Cross", "", "ZZ"); !errors.Is(err, ErrTeamNotFound) {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO project_teams(project_id,team_id) SELECT ?,id FROM teams WHERE key='ZZ'`, p.ID); err == nil {
		t.Fatal("cross-workspace association accepted")
	}

	if _, err = other.GetProject(ctx, ProjectRef{ID: p.ID}); !errors.Is(err, ErrProjectNotFound) {
		t.Fatal(err)
	}
}

func TestProjectTeamsIssueValidation(t *testing.T) {
	s := labelService(t)
	ctx := context.Background()
	_, err := s.CreateTeam(ctx, CreateTeam{Key: "BB", Name: "B"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := s.CreateProject(ctx, "Only A", "", "AL")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Create(ctx, CreateIssue{Team: "BB", Title: "invalid", ProjectID: new(p.ID)}); !errors.Is(err, ErrProjectTeam) {
		t.Fatal(err)
	}
	i, err := s.Create(ctx, CreateIssue{Team: "BB", Title: "valid"})
	if err != nil || i.Identifier != "BB-1" {
		t.Fatalf("number: %#v %v", i, err)
	}
	if _, err = s.Update(ctx, UpdateIssue{Identifier: i.Identifier, ProjectID: new(p.ID)}); !errors.Is(err, ErrProjectTeam) {
		t.Fatal(err)
	}
	done := StatusDone
	_, err = s.Create(ctx, CreateIssue{Team: "AL", Title: "closed", Status: &done, Project: new(p.Name)})
	if err != nil {
		t.Fatal(err)
	}
	teams := []string{"BB"}
	if _, err = s.UpdateProject(ctx, UpdateProject{ID: p.ID, Teams: &teams}); !errors.Is(err, ErrProjectTeamInUse) {
		t.Fatal(err)
	}
	_, err = s.UpdateProject(ctx, UpdateProject{ID: p.ID, Archived: new(true)})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Create(ctx, CreateIssue{Team: "AL", Title: "archived", ProjectID: new(p.ID)}); !errors.Is(err, ErrProjectArchived) {
		t.Fatal(err)
	}
	if _, err = s.Update(ctx, UpdateIssue{Identifier: "AL-1", Title: new("still editable"), ProjectID: new(p.ID)}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Update(ctx, UpdateIssue{Identifier: "AL-1", Project: new("")}); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateProjectTeams(t *testing.T) {
	s := labelService(t)
	ctx := context.Background()
	if _, err := s.db.Exec(`DROP TABLE IF EXISTS project_teams; INSERT INTO projects(workspace_id,name,description,emoji) VALUES (1,'Used','Outcome','🚀'),(1,'Empty','',''); INSERT INTO issues(workspace_id,identifier,team_id,number,title,status,project_id,created_at,updated_at) VALUES (1,'AL-9',1,9,'Keep','done',1,'created','updated'); UPDATE teams SET next_number=10;`); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, len(migrations)-1)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT INTO teams(id,workspace_id,key,name,next_number) VALUES(2,1,'BB','B',8);
 INSERT INTO issues(workspace_id,identifier,team_id,number,title,status,project_id,parent_id,created_at,updated_at) VALUES (1,'BB-7',2,7,'Related','canceled',1,1,'c2','u2');
 INSERT INTO comments(issue_id,body,created_at,author) VALUES(1,'Comment','c3','Author');
 INSERT INTO links(issue_id,title,url) VALUES(1,'Link','https://example.com');
 INSERT INTO relations(issue_id,kind,other_id) VALUES(1,'blocks',2);
 INSERT INTO events(issue_id,kind,detail,created_at) VALUES(1,'created','done','created');
 INSERT INTO pull_requests(workspace_id,repo,number,title,url,head_ref,state,issue_identifier,updated_at) VALUES(1,'owner/repo',1,'AL-9','https://example.com/pr','al-9','open','AL-9','p1');`); err != nil {
		t.Fatal(err)
	}
	before, err := s.Details(ctx, "AL-9")
	if err != nil {
		t.Fatal(err)
	}
	backups := t.TempDir()
	if err := migrate(s.db, backups, migrations); err != nil {
		t.Fatal(err)
	}
	after, err := s.Details(ctx, "AL-9")
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("references changed: %#v %#v %v", before, after, err)
	}
	fresh := labelService(t)
	if !reflect.DeepEqual(structure(t, s.db), structure(t, fresh.db)) {
		t.Fatal("fresh and upgraded schemas differ")
	}
	if _, err := os.Stat(filepath.Join(backups, fmt.Sprintf("before-migration-%d.db", len(migrations)))); err != nil {
		t.Fatal(err)
	}
	var integrity string
	if err := s.db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatalf("integrity: %s %v", integrity, err)
	}
	rows, err := s.db.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	violated := rows.Next()
	rows.Close()
	if violated {
		t.Fatal("foreign key violation")
	}
	ps, err := s.ListProjects(ctx, true, "AL")
	if err != nil || len(ps) != 1 || ps[0].Name != "Used" || ps[0].Emoji != "🚀" || !reflect.DeepEqual(ps[0].Teams, []string{"AL", "BB"}) {
		t.Fatalf("backfill: %#v %v", ps, err)
	}
	all, err := s.ListProjects(ctx, true)
	if err != nil || len(all) != 2 || len(all[0].Teams) != 0 {
		t.Fatalf("empty: %#v %v", all, err)
	}
	issue, err := s.Get(ctx, "AL-9")
	if err != nil || issue.CreatedAt != "created" || issue.UpdatedAt != "updated" {
		t.Fatalf("preserve: %#v %v", issue, err)
	}
	if _, err = s.Create(ctx, CreateIssue{Team: "AL", Title: "unassigned", Project: new("Empty")}); !errors.Is(err, ErrProjectTeam) {
		t.Fatal(err)
	}
}
