package mcpserver

import (
	"context"
	"github.com/DengNaichen/inftask/internal/app"
	"github.com/DengNaichen/inftask/internal/httpapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestIssueAssigneeMCP(t *testing.T) {
	root, err := app.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	root = root.ForRequests(true)
	ctx := context.Background()
	actor := app.Identity{Subject: "member", Name: "Member"}
	admin := app.WithIdentity(ctx, app.Identity{Subject: "admin", Name: "Admin"})
	if _, err = root.CreateWorkspace(admin, app.Workspace{Slug: "default", Name: "Default"}); err != nil {
		t.Fatal(err)
	}
	s, err := root.InWorkspace(admin, "default")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateTeam(admin, app.CreateTeam{Key: "AL", Name: "Public"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateTeam(admin, app.CreateTeam{Key: "SEC", Name: "Secret", Private: true}); err != nil {
		t.Fatal(err)
	}
	token, err := s.ResetInvite(admin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = root.JoinWorkspace(app.WithIdentity(ctx, actor), token); err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(root, http.NotFoundHandler(), Handler(root))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.WithContext(app.WithIdentity(r.Context(), actor)))
	}))
	defer server.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "assignment-test", Version: "1"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/w/default/mcp"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	var people assigneesOutput
	call(t, session, "list_assignees", map[string]any{"team": "AL"}, &people)
	if len(people.Assignees) != 2 {
		t.Fatal(people)
	}
	var issue app.Issue
	call(t, session, "create_issue", map[string]any{"team": "AL", "title": "Assigned", "assignee": "admin"}, &issue)
	if issue.Assignee == nil || issue.Assignee.Subject != "admin" {
		t.Fatal(issue)
	}
	call(t, session, "update_issue", map[string]any{"identifier": issue.Identifier, "assignee": "member"}, &issue)
	var list listOutput
	call(t, session, "list_issues", map[string]any{"assigned_to_me": true}, &list)
	if len(list.Issues) != 1 || list.Issues[0].Assignee.Subject != "member" {
		t.Fatal(list)
	}
	call(t, session, "update_issue", map[string]any{"identifier": issue.Identifier, "title": "Retain"}, &issue)
	if issue.Assignee.Subject != "member" {
		t.Fatal(issue)
	}
	call(t, session, "update_issue", map[string]any{"identifier": issue.Identifier, "assignee": ""}, &issue)
	if issue.Assignee != nil {
		t.Fatal(issue)
	}
	call(t, session, "list_issues", map[string]any{"assignee": ""}, &list)
	if len(list.Issues) != 1 {
		t.Fatal(list)
	}
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"list_assignees", map[string]any{"team": "SEC"}},
		{"update_issue", map[string]any{"identifier": issue.Identifier, "assignee": 7}},
		{"update_issue", map[string]any{"identifier": issue.Identifier, "assignee": "missing"}},
		{"update_issue", map[string]any{"identifier": issue.Identifier, "assignee": "admin", "actor": "admin"}},
		{"create_issue", map[string]any{"team": "AL", "title": "Spoof", "actor_subject": "admin"}},
		{"list_issues", map[string]any{"assigned_to_me": true, "assignee": ""}},
	} {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
		if err == nil && !result.IsError {
			t.Fatalf("accepted invalid input: %#v", tc)
		}
	}
	var detail app.IssueDetails
	call(t, session, "get_issue", map[string]any{"identifier": issue.Identifier}, &detail)
	if len(detail.Assignments) != 3 || detail.Assignments[0].Actor.Subject != "member" || detail.Assignments[0].To.Subject != "admin" || detail.Assignments[2].From.Subject != "member" || detail.Assignments[2].To != nil {
		t.Fatal(detail)
	}
}
