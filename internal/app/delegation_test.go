package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func delegationContext() *ExecutionContext {
	return &ExecutionContext{Agent: "codex", SessionID: "parent-session", Hostname: "test-host"}
}

func TestIssueDelegationRequiresAssignee(t *testing.T) {
	_, s, admin, member := accessFixture(t)
	if _, err := s.CreateTeam(admin, CreateTeam{Key: "AL", Name: "Alpha"}); err != nil {
		t.Fatal(err)
	}
	issue, err := s.Create(admin, CreateIssue{Team: "AL", Title: "Unassigned"})
	if err != nil {
		t.Fatal(err)
	}
	inputs := []DelegateIssue{
		{Identifier: issue.Identifier, ClientContext: delegationContext()},
		{Identifier: issue.Identifier, ClientContext: delegationContext()},
	}
	inputs[1].ClientContext.AgentID = "child"
	assertRejected := func() {
		t.Helper()
		before, err := s.Details(admin, issue.Identifier)
		if err != nil {
			t.Fatal(err)
		}
		for _, input := range inputs {
			t.Run("unassigned/"+input.ClientContext.AgentID+"/"+input.ClientContext.SessionID, func(t *testing.T) {
				_, err := s.Delegate(member, input)
				if !errors.Is(err, ErrDelegationRequiresAssignee) {
					t.Fatalf("expected assignee prerequisite, got %v", err)
				}
			})
		}
		after, err := s.Details(admin, issue.Identifier)
		if err != nil || !reflect.DeepEqual(before, after) {
			t.Fatal("rejected delegation changed issue or history", after, err)
		}
	}
	assertRejected()
	assignee := "admin"
	if _, err = s.Update(admin, UpdateIssue{Identifier: issue.Identifier, Assignee: &assignee}); err != nil {
		t.Fatal(err)
	}
	first := make([]Delegation, len(inputs))
	for i, input := range inputs {
		first[i], err = s.Delegate(member, input)
		if err != nil {
			t.Fatal(err)
		}
	}
	assignee = ""
	if _, err = s.Update(admin, UpdateIssue{Identifier: issue.Identifier, Assignee: &assignee}); err != nil {
		t.Fatal(err)
	}
	assertRejected() // Even existing execution identities must have an assignee.
	for _, input := range inputs {
		input.ClientContext.SessionID = "new-session"
	}
	assertRejected()
	for _, input := range inputs {
		input.ClientContext.SessionID = "parent-session"
	}
	assignee = "member"
	if _, err = s.Update(admin, UpdateIssue{Identifier: issue.Identifier, Assignee: &assignee}); err != nil {
		t.Fatal(err)
	}
	for i, input := range inputs {
		duplicate, err := s.Delegate(admin, input)
		if err != nil || !reflect.DeepEqual(first[i], duplicate) {
			t.Fatal("reassignment changed original delegation", duplicate, err)
		}
	}
	detail, err := s.Details(admin, issue.Identifier)
	if err != nil || len(detail.Delegations) != len(first) {
		t.Fatal("delegation history was lost", detail, err)
	}
}

func TestIssueDelegationLifecycle(t *testing.T) {
	_, s, admin, member := accessFixture(t)
	if _, err := s.CreateTeam(admin, CreateTeam{Key: "AL", Name: "Alpha"}); err != nil {
		t.Fatal(err)
	}
	issue, err := s.Create(admin, CreateIssue{Team: "AL", Title: "Work", Assignee: "member"})
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.Details(admin, issue.Identifier)
	if err != nil {
		t.Fatal(err)
	}
	s.now = func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }
	input := DelegateIssue{Identifier: issue.Identifier, ClientContext: delegationContext()}
	first, err := s.Delegate(member, input)
	if err != nil || first.Actor == nil || first.Actor.Subject != "member" || first.CreatedAt != "2026-01-01T00:00:00Z" {
		t.Fatal(first, err)
	}
	s.now = func() time.Time { return time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC) }
	duplicate, err := s.Delegate(admin, input)
	if err != nil || !reflect.DeepEqual(first, duplicate) {
		t.Fatal(duplicate, err)
	}
	for _, child := range []string{"child-a", "child-b"} {
		input.ClientContext.AgentID = child
		input.ClientContext.AgentType = "implementer"
		if _, err = s.Delegate(member, input); err != nil {
			t.Fatal(err)
		}
	}
	after, err := s.Details(admin, issue.Identifier)
	if err != nil || len(after.Delegations) != 3 {
		t.Fatal(after, err)
	}
	if after.Delegations[1].AgentID != "child-a" || after.Delegations[2].AgentID != "child-b" {
		t.Fatal(after.Delegations)
	}
	after.Delegations = before.Delegations
	if !reflect.DeepEqual(before, after) {
		t.Fatal("delegate changed business fields", after)
	}
	// Status is not a delegation trigger or restriction.
	for _, status := range Statuses {
		if _, err = s.Update(admin, UpdateIssue{Identifier: issue.Identifier, Status: &status}); err != nil {
			t.Fatal(err)
		}
		input.ClientContext.SessionID = string(status)
		if _, err = s.Delegate(member, input); err != nil {
			t.Fatal(status, err)
		}
	}
}

func TestIssueDelegationInvalidContext(t *testing.T) {
	root, s, admin, _ := accessFixture(t)
	s = s.ForRequests(false)
	alTeam(t, s)
	issue, err := s.Create(admin, CreateIssue{Team: "AL", Title: "Work", Assignee: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	bad := []*ExecutionContext{nil, {}, {Agent: "other", SessionID: "s", Hostname: "h"}}
	for _, mutate := range []func(*ExecutionContext){
		func(c *ExecutionContext) { c.SessionID = "" }, func(c *ExecutionContext) { c.Hostname = "" },
		func(c *ExecutionContext) { c.SessionID = " spaced " }, func(c *ExecutionContext) { c.SessionID = "line\nbreak" },
		func(c *ExecutionContext) { c.SessionID = strings.Repeat("x", 257) }, func(c *ExecutionContext) { c.AgentID = strings.Repeat("x", 257) },
		func(c *ExecutionContext) { c.AgentType = strings.Repeat("x", 129) }, func(c *ExecutionContext) { c.Hostname = strings.Repeat("x", 254) },
	} {
		c := delegationContext()
		mutate(c)
		bad = append(bad, c)
	}
	for _, c := range bad {
		if _, err = s.Delegate(context.Background(), DelegateIssue{Identifier: issue.Identifier, ClientContext: c}); !errors.Is(err, ErrInvalidDelegation) {
			t.Fatal(c, err)
		}
	}
	for _, id := range []string{"", "AL-999"} {
		if _, err = s.Delegate(context.Background(), DelegateIssue{Identifier: id, ClientContext: delegationContext()}); !errors.Is(err, ErrNotFound) {
			t.Fatal(id, err)
		}
	}
	// A rejected storage write leaves no partial record or issue mutation.
	if _, err = root.db.Exec(`CREATE TRIGGER fail_delegation BEFORE INSERT ON issue_delegations BEGIN SELECT RAISE(ABORT,'storage unavailable'); END;`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Delegate(context.Background(), DelegateIssue{Identifier: issue.Identifier, ClientContext: delegationContext()}); err == nil {
		t.Fatal("accepted failed insert")
	}
	detail, err := s.Details(context.Background(), issue.Identifier)
	if err != nil || len(detail.Delegations) != 0 || !reflect.DeepEqual(detail.Issue, issue) {
		t.Fatal(detail, err)
	}
	if _, err = root.db.Exec("DROP TRIGGER fail_delegation"); err != nil {
		t.Fatal(err)
	}
	d, err := s.Delegate(context.Background(), DelegateIssue{Identifier: issue.Identifier, ClientContext: delegationContext()})
	if err != nil || d.Actor != nil {
		t.Fatal(d, err)
	}
}

func TestIssueDelegationAuthorization(t *testing.T) {
	root, s, admin, member := accessFixture(t)
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
	input := DelegateIssue{Identifier: secret.Identifier, ClientContext: delegationContext()}
	if _, err = s.Delegate(member, input); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err = s.Delegate(admin, input); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Details(member, secret.Identifier); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if _, err = s.Delegate(context.Background(), input); !errors.Is(err, ErrIdentityRequired) {
		t.Fatal(err)
	}
	if _, err = root.CreateWorkspace(admin, Workspace{Slug: "other", Name: "Other"}); err != nil {
		t.Fatal(err)
	}
	other, err := root.InWorkspace(admin, "other")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = other.Delegate(admin, input); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err = s.ChangeTeamMember(admin, ChangeTeamMember{Key: "SEC", Subject: "member", Role: "member"}); err != nil {
		t.Fatal(err)
	}
	input.ClientContext.AgentID = "child"
	if _, err = s.Delegate(member, input); err != nil {
		t.Fatal(err)
	}
	if err = s.UpdateMember(admin, UpdateMember{Subject: "member", Role: "member", Status: "suspended"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Delegate(member, input); !errors.Is(err, ErrAccessDenied) {
		t.Fatal(err)
	}
	detail, err := s.Details(admin, secret.Identifier)
	if err != nil || len(detail.Delegations) != 2 || detail.Delegations[1].Actor.Subject != "member" {
		t.Fatal(detail, err)
	}
}

func TestIssueDelegationConcurrent(t *testing.T) {
	_, s, admin, _ := accessFixture(t)
	s = s.ForRequests(false)
	alTeam(t, s)
	issue, err := s.Create(admin, CreateIssue{Team: "AL", Title: "Work", Assignee: "admin"})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			if _, err := s.Delegate(context.Background(), DelegateIssue{Identifier: issue.Identifier, ClientContext: delegationContext()}); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	detail, err := s.Details(context.Background(), issue.Identifier)
	if err != nil || len(detail.Delegations) != 1 {
		t.Fatal(detail, err)
	}
}

func TestDelegationUpgrade(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "upgrade.db")
	root, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s := inWorkspace(t, root, "default")
	alTeam(t, s)
	before, err := s.Create(context.Background(), CreateIssue{Team: "AL", Title: "Preserve", Description: "Body"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = root.db.Exec("DROP TABLE issue_delegations"); err != nil {
		t.Fatal(err)
	}
	if _, err = root.db.Exec("PRAGMA user_version = 11"); err != nil {
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
	if _, err = os.Stat(filepath.Join(dir, "backups", "before-migration-12.db")); err != nil {
		t.Fatal(err)
	}
	s, err = root.InWorkspace(context.Background(), "default")
	if err != nil {
		t.Fatal(err)
	}
	after, err := s.Details(context.Background(), before.Identifier)
	if err != nil || !reflect.DeepEqual(before, after.Issue) || len(after.Delegations) != 0 {
		t.Fatal(after, err)
	}
}
