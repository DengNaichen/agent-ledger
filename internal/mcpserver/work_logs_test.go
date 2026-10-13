package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/DengNaichen/inftask/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestIssueWorkLogs(t *testing.T) {
	session := connectDelegation(t, true)
	var issue app.Issue
	call(t, session, "create_issue", map[string]any{"team": "AL", "title": "Work logs"}, &issue)
	var record app.WorkLog
	call(t, session, "add_work_log", map[string]any{"identifier": issue.Identifier, "kind": "decision", "summary": "Keep comments", "body": "PRIVATE_DETAIL_BODY"}, &record)
	if record.Actor == nil || record.Actor.Subject != "member" || record.Body != "PRIVATE_DETAIL_BODY" {
		t.Fatal(record)
	}
	var detail map[string]any
	call(t, session, "get_issue", map[string]any{"identifier": issue.Identifier}, &detail)
	body, _ := json.Marshal(detail)
	if strings.Contains(string(body), "PRIVATE_DETAIL_BODY") || detail["work_log_count"] != float64(1) || len(detail["decisions"].([]any)) != 1 || len(detail["comments"].([]any)) != 0 {
		t.Fatal(detail)
	}
	var page app.WorkLogPage
	call(t, session, "list_work_logs", map[string]any{"identifier": issue.Identifier, "kind": "decision", "limit": 1}, &page)
	if len(page.WorkLogs) != 1 || page.WorkLogs[0].ID != record.ID {
		t.Fatal(page)
	}
	var got app.WorkLog
	call(t, session, "get_work_log", map[string]any{"identifier": issue.Identifier, "id": record.ID}, &got)
	if got.Body != record.Body {
		t.Fatal(got)
	}
	for _, test := range []struct {
		name string
		args map[string]any
	}{
		{"add_work_log", map[string]any{"identifier": issue.Identifier, "summary": "S", "body": "B", "actor": "spoof"}},
		{"add_work_log", map[string]any{"identifier": issue.Identifier, "summary": "S", "body": "B", "kind": "invalid"}},
		{"add_work_log", map[string]any{"identifier": issue.Identifier, "body": "B"}},
		{"list_work_logs", map[string]any{"identifier": issue.Identifier, "limit": 101}},
		{"list_work_logs", map[string]any{"identifier": issue.Identifier, "unexpected": true}},
		{"get_work_log", map[string]any{"identifier": "AL-999", "id": record.ID}},
	} {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: test.name, Arguments: test.args})
		if err == nil && !result.IsError {
			t.Fatal("accepted invalid input", test)
		}
	}
}
