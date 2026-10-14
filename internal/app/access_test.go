package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func accessFixture(t *testing.T) (*Service, *Service, context.Context, context.Context) {
	t.Helper()
	root, err := Open(filepath.Join(t.TempDir(), "access.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	root = root.ForRequests(true)
	admin := WithIdentity(context.Background(), Identity{Subject: "admin", Name: "Admin", Email: "admin@example.test"})
	member := WithIdentity(context.Background(), Identity{Subject: "member", Name: "Member", Email: "member@example.test"})
	if _, err = root.CreateWorkspace(admin, Workspace{Slug: "alpha", Name: "Alpha"}); err != nil {
		t.Fatal(err)
	}
	scoped, err := root.InWorkspace(admin, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	token, err := scoped.ResetInvite(admin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = root.JoinWorkspace(member, token); err != nil {
		t.Fatal(err)
	}
	return root, scoped, admin, member
}

func TestWorkspaceAccess(t *testing.T) {
	root, s, admin, member := accessFixture(t)
	ctx := context.Background()
	if _, err := root.ListWorkspaces(ctx); !errors.Is(err, ErrIdentityRequired) {
		t.Fatal(err)
	}
	outsider := WithIdentity(ctx, Identity{Subject: "outsider"})
	if _, err := root.InWorkspace(outsider, "alpha"); !errors.Is(err, ErrAccessDenied) {
		t.Fatal(err)
	}
	if _, err := root.CreateWorkspace(outsider, Workspace{Slug: "beta", Name: "Beta"}); err != nil {
		t.Fatal(err)
	}
	list, err := root.ListWorkspaces(member)
	if err != nil || len(list) != 1 || list[0].Slug != "alpha" {
		t.Fatal(list, err)
	}
	if _, err := root.RenameWorkspace(member, "alpha", "New"); !errors.Is(err, ErrAdminRequired) {
		t.Fatal(err)
	}
	if _, err := root.RenameWorkspace(admin, "alpha", "New"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTeam(member, CreateTeam{Key: "AL", Name: "Alpha"}); err != nil {
		t.Fatal(err)
	}
	issue, err := s.Create(member, CreateIssue{Team: "AL", Title: "Shared"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(admin, issue.Identifier); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ListMembers(member); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateMember(member, UpdateMember{Subject: "admin", Role: "member", Status: "active"}); !errors.Is(err, ErrAdminRequired) {
		t.Fatal(err)
	}
	if err := s.UpdateMember(admin, UpdateMember{Subject: "member", Role: "member", Status: "suspended"}); err != nil {
		t.Fatal(err)
	}
	// A previously scoped service is not a cached grant: all exposed operations recheck.
	checks := map[string]func() error{
		"list":          func() error { _, e := s.List(member, ListIssues{}); return e },
		"get":           func() error { _, e := s.Get(member, issue.Identifier); return e },
		"details":       func() error { _, e := s.Details(member, issue.Identifier); return e },
		"create":        func() error { _, e := s.Create(member, CreateIssue{}); return e },
		"update":        func() error { _, e := s.Update(member, UpdateIssue{}); return e },
		"comment":       func() error { _, e := s.AddComment(member, issue.Identifier, "bad"); return e },
		"link":          func() error { _, e := s.AddLink(member, issue.Identifier, "bad", "https://example.test"); return e },
		"relation":      func() error { return s.Relation(member, "AL-1", "related", "AL-2", false) },
		"teams":         func() error { _, e := s.ListTeams(member); return e },
		"createTeam":    func() error { _, e := s.CreateTeam(member, CreateTeam{}); return e },
		"updateTeam":    func() error { _, e := s.UpdateTeam(member, UpdateTeam{}); return e },
		"projects":      func() error { _, e := s.ListProjects(member, false); return e },
		"project":       func() error { _, e := s.GetProject(member, ProjectRef{}); return e },
		"createProject": func() error { _, e := s.CreateProject(member, "bad", ""); return e },
		"updateProject": func() error { _, e := s.UpdateProject(member, UpdateProject{}); return e },
		"labels":        func() error { _, e := s.ListLabels(member, false); return e },
		"groups":        func() error { _, e := s.ListLabelGroups(member); return e },
		"saveGroup":     func() error { _, e := s.SaveLabelGroup(member, SaveLabelGroup{}); return e },
		"createLabel":   func() error { _, e := s.CreateLabel(member, CreateLabel{}); return e },
		"updateLabel":   func() error { _, e := s.UpdateLabel(member, UpdateLabel{}); return e },
		"changeLabels":  func() error { return s.ChangeIssueLabels(member, ChangeIssueLabels{}) },
		"mergeLabels":   func() error { return s.MergeLabels(member, MergeLabels{}) },
	}
	for name, check := range checks {
		t.Run(name, func(t *testing.T) {
			if err := check(); !errors.Is(err, ErrAccessDenied) {
				t.Fatal(err)
			}
		})
	}
	list, err = root.ListWorkspaces(member)
	if err != nil || len(list) != 0 {
		t.Fatal(list, err)
	}
	if err := s.UpdateMember(admin, UpdateMember{Subject: "member", Role: "member", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(member, issue.Identifier); err != nil {
		t.Fatal(err)
	}
}

func TestWorkspaceMembership(t *testing.T) {
	_, s, admin, member := accessFixture(t)
	for _, input := range []UpdateMember{{Subject: "admin", Role: "member", Status: "active"}, {Subject: "admin", Role: "admin", Status: "suspended"}} {
		if err := s.UpdateMember(admin, input); !errors.Is(err, ErrLastAdmin) {
			t.Fatal(err)
		}
	}
	if err := s.UpdateMember(admin, UpdateMember{Subject: "member", Role: "admin", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	// Concurrent self-demotions cannot leave a workspace without an administrator.
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for _, ctx := range []context.Context{admin, member} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- s.UpdateMember(ctx, UpdateMember{Subject: requestIdentity(ctx).Subject, Role: "member", Status: "active"})
		}()
	}
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, ErrLastAdmin) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatal("successful demotions", success)
	}
}

func TestWorkspaceInvite(t *testing.T) {
	root, s, admin, member := accessFixture(t)
	if _, err := s.ResetInvite(member); !errors.Is(err, ErrAdminRequired) {
		t.Fatal(err)
	}
	old, err := s.ResetInvite(admin)
	if err != nil {
		t.Fatal(err)
	}
	token, err := s.ResetInvite(admin)
	if err != nil {
		t.Fatal(err)
	}
	third := WithIdentity(context.Background(), Identity{Subject: "third"})
	if _, err := root.JoinWorkspace(third, old); !errors.Is(err, ErrInvalidInvite) {
		t.Fatal(err)
	}
	if _, err := root.JoinWorkspace(third, token); err != nil {
		t.Fatal(err)
	}
	if _, err := root.JoinWorkspace(admin, token); err != nil {
		t.Fatal(err)
	}
	access, err := s.Access(admin)
	if err != nil || access.Role != "admin" {
		t.Fatal(access, err)
	}
	if err := s.UpdateMember(admin, UpdateMember{Subject: "member", Role: "member", Status: "suspended"}); err != nil {
		t.Fatal(err)
	}
	if _, err := root.JoinWorkspace(member, token); !errors.Is(err, ErrAccessDenied) {
		t.Fatal(err)
	}
	var digest []byte
	if err := s.db.QueryRow(`SELECT digest FROM workspace_invites WHERE workspace_id=?`, s.workspace).Scan(&digest); err != nil || string(digest) == token || len(digest) != 32 {
		t.Fatal("invalid invite storage", err)
	}
}

func TestWorkspaceAccessUpgrade(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "upgrade.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	scoped := inWorkspace(t, s, "default")
	alTeam(t, scoped)
	issue, err := scoped.Create(context.Background(), CreateIssue{Team: "AL", Title: "Preserve"})
	if err != nil {
		t.Fatal(err)
	}
	removeAssignmentSchema(t, s.db)
	// Recreate the previous shape, preserving real issue data.
	removeTimelineSchema(t, s.db)
	if _, err = s.db.Exec(fmt.Sprintf(`DROP TRIGGER team_members_workspace; DROP TABLE team_members; ALTER TABLE teams DROP COLUMN private; DROP TABLE workspace_invites; DROP TABLE workspace_members; DROP TABLE users; PRAGMA user_version=%d`, 8)); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	fresh, err := Open(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	if !reflect.DeepEqual(structure(t, s.db), structure(t, fresh.db)) {
		t.Fatal("fresh and upgraded schemas differ")
	}
	if _, err = os.Stat(filepath.Join(dir, "backups", "before-migration-9.db")); err != nil {
		t.Fatal(err)
	}
	admin := WithIdentity(context.Background(), Identity{Subject: "initial"})
	if _, err = s.InWorkspace(admin, "default"); !errors.Is(err, ErrAccessDenied) {
		t.Fatal(err)
	}
	if err = s.BootstrapAccess(context.Background(), "default", "initial"); err != nil {
		t.Fatal(err)
	}
	if err = s.BootstrapAccess(context.Background(), "default", "intruder"); !errors.Is(err, ErrAlreadyBootstrapped) {
		t.Fatal(err)
	}
	scoped, err = s.ForRequests(true).InWorkspace(admin, "default")
	if err != nil {
		t.Fatal(err)
	}
	got, err := scoped.Get(admin, issue.Identifier)
	if err != nil || got.Title != issue.Title {
		t.Fatal(got, err)
	}
	if _, err = s.InWorkspace(context.Background(), "default"); err != nil {
		t.Fatal("internal service", err)
	}
	local, _ := s.InWorkspace(context.Background(), "default")
	if _, err = local.ResetInvite(context.Background()); !errors.Is(err, ErrCloudRequired) {
		t.Fatal(err)
	}
	if err = s.ForRequests(true).BootstrapAccess(context.Background(), "default", "bad"); !errors.Is(err, ErrAccessDenied) {
		t.Fatal(err)
	}
}
