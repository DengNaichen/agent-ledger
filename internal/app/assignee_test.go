package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestIssueAssigneeLifecycle(t *testing.T) {
	_, s, admin, member := accessFixture(t)
	if _, err := s.CreateTeam(admin, CreateTeam{Key: "AL", Name: "Alpha"}); err != nil {
		t.Fatal(err)
	}
	var input CreateIssue
	if err := json.Unmarshal([]byte(`{"team":"AL","title":"Assigned","assignee":"member"}`), &input); err != nil {
		t.Fatal(err)
	}
	issue, err := s.Create(admin, input)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(issue)
	var got struct {
		Assignee *Identity `json:"assignee"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got.Assignee == nil || got.Assignee.Subject != "member" {
		t.Fatalf("created assignee missing: %s", raw)
	}
	for _, subject := range []string{"admin", "admin", ""} {
		if _, err = s.Update(member, UpdateIssue{Identifier: issue.Identifier, Assignee: &subject}); err != nil {
			t.Fatal(err)
		}
	}
	detail, err := s.Details(admin, issue.Identifier)
	if err != nil || detail.Assignee != nil || len(detail.Assignments) != 3 {
		t.Fatal(detail, err)
	}
	changes := detail.Assignments
	if changes[0].Actor.Subject != "admin" || changes[0].From != nil || changes[0].To.Subject != "member" || changes[1].Actor.Subject != "member" || changes[1].From.Subject != "member" || changes[1].To.Subject != "admin" || changes[2].To != nil || changes[2].From.Subject != "admin" {
		t.Fatal(changes)
	}
	if _, err = s.Update(admin, UpdateIssue{Identifier: issue.Identifier, Assignee: new("member")}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Create(admin, CreateIssue{Team: "AL", Title: "Unassigned"}); err != nil {
		t.Fatal(err)
	}
	for _, input := range []ListIssues{{Assignee: new("member")}, {AssignedToMe: true}, {Assignee: new("")}} {
		found, err := s.List(member, input)
		if err != nil || len(found) != 1 {
			t.Fatal(found, err)
		}
	}

}

// Downgrade only test databases to their pre-assignment shape.
func removeAssignmentSchema(t *testing.T, db *sql.DB) {
	removeCycleSchema(t, db)
	t.Helper()
	if _, err := db.Exec(`DROP TABLE issue_assignments; DROP INDEX issues_assignee_idx; ALTER TABLE issues DROP COLUMN assignee_subject`); err != nil {
		t.Fatal(err)
	}
}

func TestAssigneeUpgrade(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "upgrade.db")
	root, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s := inWorkspace(t, root, "default")
	alTeam(t, s)
	if _, err = s.Create(context.Background(), CreateIssue{Team: "AL", Title: "Preserve", Description: "Body"}); err != nil {
		t.Fatal(err)
	}
	removeAssignmentSchema(t, root.db)
	if _, err = root.db.Exec(`PRAGMA user_version=10`); err != nil {
		t.Fatal(err)
	}
	root.Close()
	root, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	fresh, err := Open(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	if !reflect.DeepEqual(structure(t, root.db), structure(t, fresh.db)) {
		t.Fatal("fresh/upgrade schema mismatch")
	}
	if _, err = os.Stat(filepath.Join(dir, "backups", "before-migration-11.db")); err != nil {
		t.Fatal(err)
	}
	s, err = root.InWorkspace(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	issue, err := s.Details(context.Background(), "AL-1")
	if err != nil || issue.Title != "Preserve" || issue.Description != "Body" || issue.Assignee != nil || len(issue.Assignments) != 0 {
		t.Fatal(issue, err)
	}
}

func TestIssueAssigneeAuthorization(t *testing.T) {
	root, s, admin, member := accessFixture(t)
	outsider := WithIdentity(context.Background(), Identity{Subject: "outsider"})
	if _, err := root.CreateWorkspace(outsider, Workspace{Slug: "other", Name: "Other"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTeam(admin, CreateTeam{Key: "AL", Name: "Public"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTeam(admin, CreateTeam{Key: "SEC", Name: "Secret", Private: true}); err != nil {
		t.Fatal(err)
	}
	secret, err := s.Create(admin, CreateIssue{Team: "SEC", Title: "Secret", Assignee: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	for _, subject := range []string{"member", "outsider", "missing"} {
		if _, err = s.Update(admin, UpdateIssue{Identifier: secret.Identifier, Assignee: &subject}); !errors.Is(err, ErrInvalidAssignee) {
			t.Fatal(subject, err)
		}
	}
	if _, err = s.Update(member, UpdateIssue{Identifier: secret.Identifier, Assignee: new("member")}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err = s.ListAssignees(member, ListAssignees{Team: "SEC"}); !errors.Is(err, ErrTeamNotFound) {
		t.Fatal(err)
	}
	candidates, err := s.ListAssignees(admin, ListAssignees{Team: "SEC"})
	if err != nil || len(candidates) != 1 || candidates[0].Subject != "admin" {
		t.Fatal(candidates, err)
	}
	if _, err = s.Create(admin, CreateIssue{Team: "AL", Title: "Bad", Assignee: "outsider"}); !errors.Is(err, ErrInvalidAssignee) {
		t.Fatal(err)
	}
	public, err := s.Create(member, CreateIssue{Team: "AL", Title: "Public", Assignee: "admin"})
	if err != nil || public.Identifier != "AL-1" {
		t.Fatal(public, err)
	}
	// Any writer can reassign an issue without assignee consent or an edit lock.
	if _, err = s.Update(member, UpdateIssue{Identifier: public.Identifier, Assignee: new("member")}); err != nil {
		t.Fatal(err)
	}
	if err = s.UpdateMember(admin, UpdateMember{Subject: "member", Role: "member", Status: "suspended"}); err != nil {
		t.Fatal(err)
	}
	retained, err := s.Get(admin, public.Identifier)
	if err != nil || retained.Assignee == nil || retained.Assignee.Subject != "member" {
		t.Fatal(retained, err)
	}
	if _, err = s.Update(admin, UpdateIssue{Identifier: public.Identifier, Title: new("Keep history")}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Create(admin, CreateIssue{Team: "AL", Title: "Bad", Assignee: "member"}); !errors.Is(err, ErrInvalidAssignee) {
		t.Fatal(err)
	}
	candidates, err = s.ListAssignees(admin, ListAssignees{Team: "AL"})
	if err != nil || len(candidates) != 1 {
		t.Fatal(candidates, err)
	}
	if _, err = s.ListAssignees(member, ListAssignees{Team: "AL"}); !errors.Is(err, ErrAccessDenied) {
		t.Fatal(err)
	}
	before, err := s.Details(admin, public.Identifier)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Update(admin, UpdateIssue{Identifier: public.Identifier, Title: new("Rollback"), Status: new(StatusDone), Assignee: new("missing")}); !errors.Is(err, ErrInvalidAssignee) {
		t.Fatal(err)
	}
	after, err := s.Details(admin, public.Identifier)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("partial update", after, err)
	}
	if _, err = s.List(admin, ListIssues{Assignee: new("admin"), AssignedToMe: true}); !errors.Is(err, ErrInvalidAssigneeFilter) {
		t.Fatal(err)
	}
	// A history write failure must also roll back valid assignment and field changes.
	if _, err = s.db.Exec(`CREATE TRIGGER fail_assignment BEFORE INSERT ON issue_assignments BEGIN SELECT RAISE(ABORT,'audit unavailable'); END;`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Update(admin, UpdateIssue{Identifier: public.Identifier, Title: new("Rollback audit"), Assignee: new("admin")}); err == nil {
		t.Fatal("accepted assignment without audit")
	}
	after, err = s.Details(admin, public.Identifier)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("audit failure left partial update", after, err)
	}
	if _, err = s.db.Exec(`DROP TRIGGER fail_assignment`); err != nil {
		t.Fatal(err)
	}
	if err = s.UpdateMember(admin, UpdateMember{Subject: "member", Role: "member", Status: "active"}); err != nil {
		t.Fatal(err)
	}
	if err = s.ChangeTeamMember(admin, ChangeTeamMember{Key: "SEC", Subject: "member", Role: "member"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Update(member, UpdateIssue{Identifier: secret.Identifier, Assignee: new("member")}); err != nil {
		t.Fatal(err)
	}
	if err = s.ChangeTeamMember(admin, ChangeTeamMember{Key: "SEC", Subject: "member", Remove: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Update(admin, UpdateIssue{Identifier: secret.Identifier, Assignee: new("member")}); !errors.Is(err, ErrInvalidAssignee) {
		t.Fatal(err)
	}
}
