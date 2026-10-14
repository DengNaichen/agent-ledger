package mcpserver

import (
	"context"
	"github.com/DengNaichen/inftask/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"testing"
)

func TestInitiativeMCP(t *testing.T) {
	session := connect(t)
	fail := func(tool string, args map[string]any) {
		t.Helper()
		r, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
		if err == nil && !r.IsError {
			t.Fatalf("accepted %s %#v", tool, args)
		}
	}
	var p app.Project
	call(t, session, "create_project", map[string]any{"name": "Timeline", "teams": []string{"AL"}}, &p)
	call(t, session, "update_project", map[string]any{"id": p.ID, "start_date": "2026-10-01", "target_date": "2026-10-31"}, &p)
	if p.StartDate == nil || *p.StartDate != "2026-10-01" {
		t.Fatal(p)
	}
	fail("update_project", map[string]any{"id": p.ID, "target_date": "2026-09-01"})
	fail("update_project", map[string]any{"id": p.ID, "start_date": "2026-02-30"})
	var i app.Initiative
	call(t, session, "create_initiative", map[string]any{"name": "Objective", "project_ids": []int64{p.ID}}, &i)
	fail("create_initiative", map[string]any{"name": "Bad", "actor": "forged"})
	fail("update_initiative", map[string]any{"id": i.ID, "project_ids": []int64{99999}})
	call(t, session, "get_initiative", map[string]any{"id": i.ID}, &i)
	if len(i.ProjectIDs) != 1 {
		t.Fatal(i)
	}
	call(t, session, "update_initiative", map[string]any{"id": i.ID, "project_ids": []int64{}, "archived": true}, &i)
	fail("update_initiative", map[string]any{"id": i.ID, "project_ids": []int64{p.ID}})
	var out struct {
		Initiatives []app.Initiative `json:"initiatives"`
	}
	call(t, session, "list_initiatives", map[string]any{}, &out)
	if len(out.Initiatives) != 0 {
		t.Fatal(out)
	}
	call(t, session, "update_initiative", map[string]any{"id": i.ID, "archived": false, "project_ids": []int64{p.ID}}, &i)
	call(t, session, "list_initiatives", map[string]any{}, &out)
	if len(out.Initiatives) != 1 {
		t.Fatal(out)
	}
}
