package mcpserver

import (
	"context"
	"github.com/DengNaichen/inftask/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"testing"
	"time"
)

func TestCycleMCP(t *testing.T) {
	session := connect(t)
	var page app.CyclePage
	call(t, session, "configure_cycles", map[string]any{"team": "AL", "enabled": true, "timezone": "UTC", "start_date": time.Now().UTC().Format(time.DateOnly), "duration_weeks": 2, "upcoming_count": 2}, &page)
	if len(page.Cycles) != 3 {
		t.Fatal(page)
	}
	id := page.Cycles[0].ID
	var issue app.Issue
	call(t, session, "create_issue", map[string]any{"team": "AL", "title": "Cycle issue", "cycle_id": id}, &issue)
	if issue.CycleID == nil || *issue.CycleID != id {
		t.Fatal(issue)
	}
	var list listOutput
	call(t, session, "list_issues", map[string]any{"cycle_id": id}, &list)
	if len(list.Issues) != 1 {
		t.Fatal(list)
	}
	var cycle app.Cycle
	call(t, session, "update_cycle", map[string]any{"id": id, "name": "Focus"}, &cycle)
	call(t, session, "get_cycle", map[string]any{"id": id}, &cycle)
	if cycle.Name != "Focus" {
		t.Fatal(cycle)
	}
	call(t, session, "update_issue", map[string]any{"identifier": issue.Identifier, "cycle_id": 0}, &issue)
	if issue.CycleID != nil {
		t.Fatal(issue)
	}
	for _, args := range []map[string]any{{"id": id, "actor": "spoof"}, {"id": int64(999999)}} {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_cycle", Arguments: args})
		if err == nil && !result.IsError {
			t.Fatal("invalid request accepted", result)
		}
	}
}
