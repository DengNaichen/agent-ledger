package app

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestEntityRenameLifecycle(t *testing.T) {
	s := labelService(t)
	ctx := context.Background()
	p, err := s.CreateProject(ctx, "Old", "Outcome", "AL")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateTeam(ctx, CreateTeam{Key: "BB", Name: "Other"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateProject(ctx, "Taken", "", "BB"); err != nil {
		t.Fatal(err)
	}
	i, err := s.Create(ctx, CreateIssue{Team: "AL", Title: "Keep", ProjectID: &p.ID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AddComment(ctx, i.Identifier, "Keep comment"); err != nil {
		t.Fatal(err)
	}
	before, err := s.Details(ctx, i.Identifier)
	if err != nil {
		t.Fatal(err)
	}
	var teamID int64
	if err = s.db.QueryRow(`SELECT id FROM teams WHERE key='AL'`).Scan(&teamID); err != nil {
		t.Fatal(err)
	}
	team, err := s.UpdateTeam(ctx, UpdateTeam{Key: "AL", Name: new(" 产品 A ")})
	if err != nil || team.Name != "产品 A" || team.NextNumber != 2 || team.Key != "AL" {
		t.Fatalf("team: %#v %v", team, err)
	}
	renamed, err := s.UpdateProject(ctx, UpdateProject{ID: p.ID, NewName: new(" New ")})
	if err != nil {
		t.Fatal(err)
	}
	p.Name = "New"
	if !reflect.DeepEqual(p, renamed) {
		t.Fatalf("project changed: %#v %#v", p, renamed)
	}
	after, err := s.Details(ctx, i.Identifier)
	before.Project = new("New")
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("references changed: %#v %v", after, err)
	}
	var afterID int64
	s.db.QueryRow(`SELECT id FROM teams WHERE key='AL'`).Scan(&afterID)
	if afterID != teamID {
		t.Fatal("team id changed")
	}
	if _, err = s.GetProject(ctx, ProjectRef{Name: "Old"}); !errors.Is(err, ErrProjectNotFound) {
		t.Fatal(err)
	}
	byName, err := s.GetProject(ctx, ProjectRef{Name: "New"})
	if err != nil || byName.ID != p.ID {
		t.Fatal(err)
	}
	if _, err = s.UpdateProject(ctx, UpdateProject{ID: p.ID, NewName: new("Taken"), Teams: &[]string{"AL", "BB"}, Description: new("bad")}); !errors.Is(err, ErrProjectExists) {
		t.Fatal(err)
	}
	kept, err := s.GetProject(ctx, ProjectRef{ID: p.ID})
	if err != nil || !reflect.DeepEqual(kept, p) {
		t.Fatalf("partial write: %#v %v", kept, err)
	}
	for _, name := range []string{"", " \t "} {
		if _, err = s.UpdateTeam(ctx, UpdateTeam{Key: "AL", Name: &name, Emoji: new("🚀")}); !errors.Is(err, ErrInvalidName) {
			t.Fatal(err)
		}
		if _, err = s.UpdateProject(ctx, UpdateProject{ID: p.ID, NewName: &name}); !errors.Is(err, ErrInvalidName) {
			t.Fatal(err)
		}
	}
	other := inWorkspace(t, s, "other")
	if _, err = other.UpdateProject(ctx, UpdateProject{ID: p.ID, NewName: new("Cross")}); !errors.Is(err, ErrProjectNotFound) {
		t.Fatal(err)
	}
	if _, err = other.UpdateTeam(ctx, UpdateTeam{Key: "AL", Name: new("Cross")}); !errors.Is(err, ErrTeamNotFound) {
		t.Fatal(err)
	}
	next, err := s.Create(ctx, CreateIssue{Team: "AL", Title: "Next", ProjectID: &p.ID})
	if err != nil || next.Identifier != "AL-2" {
		t.Fatalf("number changed: %#v %v", next, err)
	}
}
