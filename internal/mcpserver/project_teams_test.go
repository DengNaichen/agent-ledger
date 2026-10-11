package mcpserver

import (
	"context"
	"github.com/DengNaichen/inftask/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"testing"
)

func TestProjectTeamsMCP(t *testing.T) {
	session := connect(t)
	var p app.Project
	var team app.Team
	var i app.Issue
	fail := func(tool string, args map[string]any) {
		t.Helper()
		r, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
		if err == nil && !r.IsError {
			t.Fatalf("accepted %s %#v", tool, args)
		}
	}
	fail("create_project", map[string]any{"name": "Missing"})
	call(t, session, "create_team", map[string]any{"key": "BB", "name": "B"}, &team)
	call(t, session, "create_project", map[string]any{"name": "Empty", "teams": []string{"AL"}}, &p)
	if p.ID == 0 || len(p.Teams) != 1 {
		t.Fatal(p)
	}
	id := p.ID
	call(t, session, "get_project", map[string]any{"id": id}, &p)
	call(t, session, "get_project", map[string]any{"name": "Empty"}, &p)
	fail("get_project", map[string]any{"id": id, "name": "Wrong"})
	fail("update_project", map[string]any{"id": id, "teams": []string{}})
	var out projectsOutput
	call(t, session, "list_projects", map[string]any{"team": "BB"}, &out)
	if len(out.Projects) != 0 {
		t.Fatal(out)
	}
	fail("create_issue", map[string]any{"team": "BB", "title": "bad", "project_id": id})
	call(t, session, "update_project", map[string]any{"id": id, "teams": []string{"BB", "AL"}}, &p)
	call(t, session, "create_issue", map[string]any{"team": "BB", "title": "shared", "project_id": id, "project": "Empty"}, &i)
	if i.Identifier != "BB-1" || i.ProjectID == nil || *i.ProjectID != id {
		t.Fatal(i)
	}
	fail("update_project", map[string]any{"id": id, "teams": []string{"AL"}})
	fail("create_issue", map[string]any{"team": "AL", "title": "bad", "project_id": id, "project": ""})
	fail("update_issue", map[string]any{"identifier": i.Identifier, "project_id": 0, "project": "Empty"})
	call(t, session, "update_project", map[string]any{"name": "Empty", "archived": true}, &p)
	fail("create_issue", map[string]any{"team": "AL", "title": "archive", "project_id": id})
	call(t, session, "update_issue", map[string]any{"identifier": i.Identifier, "project_id": id, "title": "retained"}, &i)
	call(t, session, "update_issue", map[string]any{"identifier": i.Identifier, "project_id": 0}, &i)
	call(t, session, "update_project", map[string]any{"id": id, "teams": []string{"AL"}}, &p)
	call(t, session, "list_projects", map[string]any{"include_archived": true, "team": "AL"}, &out)
	if len(out.Projects) != 1 {
		t.Fatal(out)
	}
}
