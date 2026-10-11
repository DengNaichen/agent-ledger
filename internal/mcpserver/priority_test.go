package mcpserver

import (
	"context"
	"testing"

	"github.com/DengNaichen/inftask/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestIssuePriorityOverMCP(t *testing.T) {
	session := connect(t)
	var created app.Issue
	for _, priority := range []int{1, 4, 0} {
		call(t, session, "create_issue", map[string]any{"team": "AL", "title": "Work", "status": "agent_ready", "priority": priority}, &created)
		if created.Priority != priority {
			t.Fatalf("create: %#v", created)
		}
	}
	var listed listOutput
	for _, tc := range []struct {
		args map[string]any
		want []int
	}{
		{map[string]any{"status": "agent_ready"}, []int{1, 4, 0}},
		{map[string]any{"status": "agent_ready", "sort": "updated"}, []int{0, 4, 1}},
		{map[string]any{"sort": "priority", "limit": 1}, []int{1}},
		{map[string]any{"priority": 0}, []int{0}},
	} {
		call(t, session, "list_issues", tc.args, &listed)
		if len(listed.Issues) != len(tc.want) {
			t.Fatalf("list: %#v", listed)
		}
		for i, issue := range listed.Issues {
			if issue.Priority != tc.want[i] {
				t.Fatalf("list: %#v", listed)
			}
		}
	}
	for _, tc := range []struct {
		args map[string]any
		want int
	}{
		{map[string]any{"identifier": "AL-1", "title": "Keep"}, 1},
		{map[string]any{"identifier": "AL-1", "priority": 2}, 2},
		{map[string]any{"identifier": "AL-1", "priority": 0}, 0},
	} {
		call(t, session, "update_issue", tc.args, &created)
		if created.Priority != tc.want {
			t.Fatalf("update: %#v", created)
		}
	}
	var detail app.IssueDetails
	call(t, session, "get_issue", map[string]any{"identifier": "AL-2"}, &detail)
	if detail.Priority != 4 {
		t.Fatalf("detail: %#v", detail)
	}
	for _, tc := range []struct {
		name string
		args map[string]any
	}{
		{"create_issue", map[string]any{"team": "AL", "title": "Bad", "priority": 5}},
		{"update_issue", map[string]any{"identifier": "AL-1", "priority": -1}},
		{"update_issue", map[string]any{"identifier": "AL-1", "priority": 1.5}},
		{"list_issues", map[string]any{"priority": 5}},
		{"list_issues", map[string]any{"sort": "title"}},
	} {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
		if err == nil && !result.IsError {
			t.Fatalf("accepted invalid input: %#v", tc)
		}
	}
}
