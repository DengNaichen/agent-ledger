package app

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestInitiativeLifecycle(t *testing.T) {
	s := labelService(t)
	ctx := context.Background()
	p, err := s.CreateProject(ctx, "First", "", "AL")
	if err != nil {
		t.Fatal(err)
	}
	i, err := s.CreateInitiative(ctx, CreateInitiative{Name: " Goal ", ProjectIDs: []int64{p.ID, p.ID}})
	if err != nil || i.Name != "Goal" || len(i.ProjectIDs) != 1 {
		t.Fatal(i, err)
	}
	if _, err = s.CreateInitiative(ctx, CreateInitiative{Name: "Goal"}); !errors.Is(err, ErrInitiativeExists) {
		t.Fatal(err)
	}
	second, err := s.CreateInitiative(ctx, CreateInitiative{Name: "Second", ProjectIDs: []int64{p.ID}})
	if err != nil || len(second.ProjectIDs) != 1 {
		t.Fatal(second, err)
	}
	if _, err = s.UpdateInitiative(ctx, UpdateInitiative{ID: i.ID, Name: new(" "), ProjectIDs: new([]int64{})}); !errors.Is(err, ErrInvalidName) {
		t.Fatal(err)
	}
	if _, err = s.UpdateInitiative(ctx, UpdateInitiative{ID: i.ID, Name: new("Changed"), ProjectIDs: new([]int64{99999})}); !errors.Is(err, ErrProjectNotFound) {
		t.Fatal(err)
	}
	got, err := s.GetInitiative(ctx, i.ID)
	if err != nil || !reflect.DeepEqual(got, i) {
		t.Fatal(got, err)
	}
	_, err = s.UpdateInitiative(ctx, UpdateInitiative{ID: i.ID, Archived: new(true)})
	if err != nil {
		t.Fatal(err)
	}
	items, err := s.ListInitiatives(ctx, false)
	if err != nil || len(items) != 1 {
		t.Fatal(items, err)
	}
	got, err = s.UpdateInitiative(ctx, UpdateInitiative{ID: i.ID, ProjectIDs: new([]int64{p.ID})})
	if err != nil || len(got.ProjectIDs) != 1 {
		t.Fatal(got, err)
	}
	got, err = s.UpdateInitiative(ctx, UpdateInitiative{ID: i.ID, ProjectIDs: new([]int64{})})
	if err != nil || len(got.ProjectIDs) != 0 {
		t.Fatal(got, err)
	}
	if _, err = s.UpdateInitiative(ctx, UpdateInitiative{ID: i.ID, ProjectIDs: new([]int64{p.ID})}); !errors.Is(err, ErrInitiativeArchived) {
		t.Fatal(err)
	}
	if _, err = s.UpdateInitiative(ctx, UpdateInitiative{ID: i.ID, Archived: new(false), ProjectIDs: new([]int64{p.ID})}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.UpdateProject(ctx, UpdateProject{ID: p.ID, Archived: new(true)}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateInitiative(ctx, CreateInitiative{Name: "Rollback", ProjectIDs: []int64{p.ID}}); !errors.Is(err, ErrInitiativeArchived) {
		t.Fatal(err)
	}
	items, err = s.ListInitiatives(ctx, true)
	if err != nil || len(items) != 2 {
		t.Fatal(items, err)
	}
	other := inWorkspace(t, s, "other")
	if _, err = other.GetInitiative(ctx, i.ID); !errors.Is(err, ErrInitiativeNotFound) {
		t.Fatal(err)
	}
	if _, err = other.CreateInitiative(ctx, CreateInitiative{Name: "Cross", ProjectIDs: []int64{p.ID}}); !errors.Is(err, ErrProjectNotFound) {
		t.Fatal(err)
	}
	foreign, err := other.CreateInitiative(ctx, CreateInitiative{Name: "Other"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec("INSERT INTO initiative_projects(initiative_id,project_id) VALUES(?,?)", foreign.ID, p.ID); err == nil {
		t.Fatal("SQL accepted cross workspace association")
	}
	if _, err = s.db.Exec("UPDATE initiative_projects SET initiative_id=? WHERE initiative_id=?", foreign.ID, i.ID); err == nil {
		t.Fatal("SQL update accepted cross workspace association")
	}
}

func TestInitiativeVisibility(t *testing.T) {
	_, s, admin, member := accessFixture(t)
	for _, team := range []CreateTeam{{Key: "PUB", Name: "Public"}, {Key: "SEC", Name: "Secret", Private: true}} {
		if _, err := s.CreateTeam(admin, team); err != nil {
			t.Fatal(err)
		}
	}
	public, err := s.CreateProject(admin, "Public", "", "PUB")
	if err != nil {
		t.Fatal(err)
	}
	secret, err := s.CreateProject(admin, "Secret", "", "SEC")
	if err != nil {
		t.Fatal(err)
	}
	i, err := s.CreateInitiative(admin, CreateInitiative{Name: "Objective", ProjectIDs: []int64{public.ID, secret.ID}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.GetInitiative(member, i.ID)
	if err != nil || !reflect.DeepEqual(got.ProjectIDs, []int64{public.ID}) {
		t.Fatal(got, err)
	}
	if _, err = s.UpdateInitiative(member, UpdateInitiative{ID: i.ID, ProjectIDs: new([]int64{secret.ID})}); !errors.Is(err, ErrProjectNotFound) {
		t.Fatal(err)
	}
	if _, err = s.UpdateProject(member, UpdateProject{ID: secret.ID, StartDate: new("2026-10-01")}); !errors.Is(err, ErrProjectNotFound) {
		t.Fatal(err)
	}
	got, err = s.UpdateInitiative(member, UpdateInitiative{ID: i.ID, ProjectIDs: new([]int64{})})
	if err != nil || len(got.ProjectIDs) != 0 {
		t.Fatal(got, err)
	}
	got, err = s.GetInitiative(admin, i.ID)
	if err != nil || !reflect.DeepEqual(got.ProjectIDs, []int64{secret.ID}) {
		t.Fatal("hidden association removed", got, err)
	}
	outsider := WithIdentity(context.Background(), Identity{Subject: "outsider"})
	if _, err = s.ListInitiatives(outsider, true); !errors.Is(err, ErrAccessDenied) {
		t.Fatal(err)
	}
	if err = s.UpdateMember(admin, UpdateMember{Subject: "member", Role: "member", Status: "suspended"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.GetInitiative(member, i.ID); !errors.Is(err, ErrAccessDenied) {
		t.Fatal("suspended read", err)
	}
	if _, err = s.UpdateInitiative(member, UpdateInitiative{ID: i.ID, Name: new("Forbidden")}); !errors.Is(err, ErrAccessDenied) {
		t.Fatal("suspended write", err)
	}
	if _, err = s.CreateInitiative(member, CreateInitiative{Name: "Forbidden"}); !errors.Is(err, ErrAccessDenied) {
		t.Fatal("suspended create", err)
	}
}
