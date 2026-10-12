package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPrivateTeamAccess(t *testing.T) {
	_, s, admin, member := accessFixture(t)
	var input CreateTeam
	if err := json.Unmarshal([]byte(`{"key":"SEC","name":"Sensitive","private":true}`), &input); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTeam(admin, input); err != nil {
		t.Fatal(err)
	}
	issue, err := s.Create(admin, CreateIssue{Team: "SEC", Title: "Secret"})
	if err != nil {
		t.Fatal(err)
	}
	items, err := s.List(member, ListIssues{})
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("private issues leaked: %v", items)
	}
	if _, err := s.Get(member, issue.Identifier); !errors.Is(err, ErrNotFound) {
		t.Errorf("Get private issue: %v", err)
	}
	if _, err := s.Details(member, issue.Identifier); !errors.Is(err, ErrNotFound) {
		t.Errorf("Details private issue: %v", err)
	}
	// The internal background service retains access independently of human membership.
	if _, err := s.ForRequests(false).Get(context.Background(), issue.Identifier); err != nil {
		t.Fatal(err)
	}
}

func TestTeamMembership(t *testing.T) {
	root, s, admin, member := accessFixture(t)
	team, err := s.CreateTeam(member, CreateTeam{Key: "SEC", Name: "Sensitive", Private: true})
	if err != nil || !team.CanManage || !team.Joined {
		t.Fatal(team, err)
	}
	issue, err := s.Create(member, CreateIssue{Team: "SEC", Title: "Secret"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(admin, issue.Identifier); !errors.Is(err, ErrNotFound) {
		t.Fatal("admin read before join", err)
	}
	visible, err := s.ListTeams(admin)
	if err != nil || len(visible) != 0 {
		t.Fatal(visible, err)
	}
	directory, err := s.TeamDirectory(admin)
	if err != nil || len(directory) != 1 || directory[0].Accessible || directory[0].NextNumber != 0 {
		t.Fatal(directory, err)
	}
	if _, err = s.JoinTeam(admin, JoinTeam{Key: "SEC"}); !errors.Is(err, ErrPrivateJoinConfirmation) {
		t.Fatal(err)
	}
	if _, err = s.JoinTeam(admin, JoinTeam{Key: "SEC", ConfirmPrivate: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(admin, issue.Identifier); err != nil {
		t.Fatal(err)
	}
	token, err := s.ResetInvite(admin)
	if err != nil {
		t.Fatal(err)
	}
	outsider := WithIdentity(context.Background(), Identity{Subject: "outsider' OR 1=1 --"})
	if _, err = root.JoinWorkspace(outsider, token); err != nil {
		t.Fatal(err)
	}
	if _, err = s.JoinTeam(outsider, JoinTeam{Key: "SEC", ConfirmPrivate: true}); !errors.Is(err, ErrTeamNotFound) {
		t.Fatal(err)
	}
	if err = s.ChangeTeamMember(member, ChangeTeamMember{Key: "SEC", Subject: requestIdentity(outsider).Subject, Role: "member"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.UpdateTeam(outsider, UpdateTeam{Key: "SEC", Private: new(false)}); !errors.Is(err, ErrTeamOwnerRequired) {
		t.Fatal(err)
	}
	if err = s.ChangeTeamMember(outsider, ChangeTeamMember{Key: "SEC", Subject: "member", Remove: true}); !errors.Is(err, ErrTeamOwnerRequired) {
		t.Fatal(err)
	}
	if _, err = s.Get(outsider, issue.Identifier); err != nil {
		t.Fatal(err)
	}
	if _, err = s.JoinTeam(outsider, JoinTeam{Key: "SEC", Leave: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Get(outsider, issue.Identifier); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err = s.JoinTeam(outsider, JoinTeam{Key: "SEC"}); !errors.Is(err, ErrTeamNotFound) {
		t.Fatal(err)
	}
	// Linear permits zero team owners; the workspace administrator can recover access.
	if _, err = s.JoinTeam(admin, JoinTeam{Key: "SEC", Leave: true}); err != nil {
		t.Fatal(err)
	}
	if err = s.ChangeTeamMember(member, ChangeTeamMember{Key: "SEC", Subject: "member", Role: "member"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.JoinTeam(member, JoinTeam{Key: "SEC", Leave: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.JoinTeam(admin, JoinTeam{Key: "SEC", ConfirmPrivate: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateTeam(member, CreateTeam{Key: "PUB", Name: "Public"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Create(outsider, CreateIssue{Team: "PUB", Title: "Public collaboration"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.JoinTeam(outsider, JoinTeam{Key: "PUB"}); err != nil {
		t.Fatal(err)
	}
	if err = s.UpdateMember(admin, UpdateMember{Subject: requestIdentity(outsider).Subject, Role: "member", Status: "suspended"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.ListTeamMembers(outsider, "PUB"); !errors.Is(err, ErrAccessDenied) {
		t.Fatal(err)
	}
}

func TestPrivateProjectVisibility(t *testing.T) {
	_, s, admin, member := accessFixture(t)
	for _, team := range []CreateTeam{{Key: "SEC", Name: "Secret", Private: true}, {Key: "PUB", Name: "Public"}} {
		if _, err := s.CreateTeam(admin, team); err != nil {
			t.Fatal(err)
		}
	}
	private, err := s.CreateProject(admin, "Private project", "Secret", "SEC")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.GetProject(member, ProjectRef{ID: private.ID}); !errors.Is(err, ErrProjectNotFound) {
		t.Fatal(err)
	}
	if _, err = s.GetProject(member, ProjectRef{Name: private.Name}); !errors.Is(err, ErrProjectNotFound) {
		t.Fatal(err)
	}
	if ps, err := s.ListProjects(member, true); err != nil || len(ps) != 0 {
		t.Fatal(ps, err)
	}
	shared, err := s.CreateProject(admin, "Shared", "", "PUB", "SEC")
	if err != nil {
		t.Fatal(err)
	}
	public, err := s.Create(admin, CreateIssue{Team: "PUB", Title: "Public", ProjectID: new(shared.ID)})
	if err != nil {
		t.Fatal(err)
	}
	secret, err := s.Create(admin, CreateIssue{Team: "SEC", Title: "Secret", ProjectID: new(shared.ID), Parent: public.Identifier})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Relation(admin, public.Identifier, "related", secret.Identifier, false); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Update(admin, UpdateIssue{Identifier: public.Identifier, Parent: new(secret.Identifier)}); err != nil {
		t.Fatal(err)
	}
	detail, err := s.Details(member, public.Identifier)
	if err != nil || detail.Parent != nil || len(detail.Children) != 0 || len(detail.Related) != 0 {
		t.Fatal(detail, err)
	}
	got, err := s.GetProject(member, ProjectRef{ID: shared.ID})
	if err != nil || !reflect.DeepEqual(got.Teams, []string{"PUB"}) {
		t.Fatal(got, err)
	}
	if _, err = s.UpdateProject(member, UpdateProject{ID: shared.ID, Teams: new([]string{"PUB"})}); err != nil {
		t.Fatal(err)
	}
	got, err = s.GetProject(admin, ProjectRef{ID: shared.ID})
	if err != nil || len(got.Teams) != 2 {
		t.Fatal("hidden association lost", got, err)
	}
	if _, err = s.CreateProject(member, "Denied", "", "SEC"); !errors.Is(err, ErrTeamNotFound) {
		t.Fatal(err)
	}
	if _, err = s.UpdateProject(member, UpdateProject{ID: private.ID, NewName: new("Denied")}); !errors.Is(err, ErrProjectNotFound) {
		t.Fatal(err)
	}
	items, err := s.List(member, ListIssues{Limit: new(1), Sort: "updated"})
	if err != nil || len(items) != 1 || items[0].Identifier != public.Identifier {
		t.Fatal(items, err)
	}
	if _, err = s.Update(member, UpdateIssue{Identifier: secret.Identifier, Title: new("Hacked")}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	stored, err := s.Get(admin, secret.Identifier)
	if err != nil || stored.Title != "Secret" {
		t.Fatal(stored, err)
	}
	if _, err = s.AddComment(member, secret.Identifier, "bad"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err = s.AddLink(member, secret.Identifier, "bad", "https://example.test"); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err = s.Relation(member, public.Identifier, "blocks", secret.Identifier, false); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	label, err := s.CreateLabel(admin, CreateLabel{Name: "Shared label", Color: "#112233"})
	if err != nil {
		t.Fatal(err)
	}
	if err = s.ChangeIssueLabels(member, ChangeIssueLabels{Identifiers: []string{public.Identifier, secret.Identifier}, Add: []int64{label.ID}}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	stored, err = s.Get(admin, public.Identifier)
	if err != nil || len(stored.Labels) != 0 {
		t.Fatal("partial batch", stored, err)
	}
	if _, err := s.Create(member, CreateIssue{Team: "SEC", Title: "Denied"}); !errors.Is(err, ErrTeamNotFound) {
		t.Fatal(err)
	}
	target, err := s.CreateLabel(admin, CreateLabel{Name: "Target", Color: "#334455"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ChangeIssueLabels(admin, ChangeIssueLabels{Identifiers: []string{secret.Identifier}, Add: []int64{label.ID}}); err != nil {
		t.Fatal(err)
	}
	if err := s.MergeLabels(member, MergeLabels{SourceID: label.ID, TargetID: target.ID}); !errors.Is(err, ErrAccessDenied) {
		t.Fatal(err)
	}
	stored, err = s.Get(admin, secret.Identifier)
	if err != nil || len(stored.Labels) != 1 || stored.Labels[0].ID != label.ID {
		t.Fatal("hidden labels changed", stored, err)
	}
	if err := s.MergeLabels(admin, MergeLabels{SourceID: label.ID, TargetID: target.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.UpdateTeam(admin, UpdateTeam{Key: "PUB", Private: new(true)}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.GetProject(member, ProjectRef{ID: shared.ID}); !errors.Is(err, ErrProjectNotFound) {
		t.Fatal(err)
	}
}

func TestTeamAccessUpgrade(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "upgrade.db")
	root, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s := inWorkspace(t, root, "default")
	alTeam(t, s)
	if _, err = s.Create(context.Background(), CreateIssue{Team: "AL", Title: "Keep"}); err != nil {
		t.Fatal(err)
	}
	if err = root.BootstrapAccess(context.Background(), "default", "admin"); err != nil {
		t.Fatal(err)
	}
	if _, err = root.db.Exec(`DROP TRIGGER team_members_workspace;DROP TABLE team_members;ALTER TABLE teams DROP COLUMN private;PRAGMA user_version=9`); err != nil {
		t.Fatal(err)
	}
	root.Close()
	root, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if _, err = os.Stat(filepath.Join(dir, "backups", "before-migration-10.db")); err != nil {
		t.Fatal(err)
	}
	fresh, err := Open(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	if !reflect.DeepEqual(structure(t, root.db), structure(t, fresh.db)) {
		t.Fatal("fresh and upgraded schemas differ")
	}
	ctx := WithIdentity(context.Background(), Identity{Subject: "admin"})
	s, err = root.ForRequests(true).InWorkspace(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	teams, err := s.ListTeams(ctx)
	if err != nil || len(teams) != 1 || teams[0].Private || teams[0].Joined {
		t.Fatal(teams, err)
	}
	if issue, err := s.Get(ctx, "AL-1"); err != nil || issue.Title != "Keep" {
		t.Fatal(issue, err)
	}
}
