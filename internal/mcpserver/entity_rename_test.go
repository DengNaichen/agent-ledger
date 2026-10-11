package mcpserver

import (
	"context"
	"github.com/DengNaichen/inftask/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"testing"
)

func TestEntityRenameOverHTTP(t *testing.T) {
	session := connect(t)
	var p app.Project
	var team app.Team
	var issue app.Issue
	call(t, session, "create_project", map[string]any{"name": "Old", "teams": []string{"AL"}}, &p)
	id := p.ID
	call(t, session, "create_issue", map[string]any{"team": "AL", "title": "Keep", "project_id": id}, &issue)
	call(t, session, "update_team", map[string]any{"key": "AL", "name": " 产品 A "}, &team)
	if team.Name != "产品 A" || team.Key != "AL" || team.NextNumber != 2 {
		t.Fatal(team)
	}
	call(t, session, "update_project", map[string]any{"id": id, "new_name": " New "}, &p)
	if p.ID != id || p.Name != "New" || len(p.Teams) != 1 {
		t.Fatal(p)
	}
	call(t, session, "get_project", map[string]any{"name": "New"}, &p)
	if p.ID != id {
		t.Fatal(p)
	}
	call(t, session, "create_project", map[string]any{"name": "Taken", "teams": []string{"AL"}}, &app.Project{})
	for _, tc := range []struct {
		tool string
		args map[string]any
	}{
		{"update_team", map[string]any{"key": "AL", "name": " "}},
		{"update_project", map[string]any{"id": id, "new_name": " "}},
		{"update_project", map[string]any{"id": id, "new_name": "Taken", "description": "bad"}},
		{"update_project", map[string]any{"name": "Old", "new_name": "Stale"}},
	} {
		r, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: tc.args})
		if err == nil && !r.IsError {
			t.Fatalf("accepted %#v", tc)
		}
	}
	call(t, session, "get_project", map[string]any{"id": id}, &p)
	if p.Name != "New" || p.Description != "" {
		t.Fatal(p)
	}
	call(t, session, "get_issue", map[string]any{"identifier": "AL-1"}, &issue)
	if issue.ProjectID == nil || *issue.ProjectID != id || *issue.Project != "New" {
		t.Fatal(issue)
	}
}
