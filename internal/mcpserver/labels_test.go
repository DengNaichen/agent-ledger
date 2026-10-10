package mcpserver

import (
	"context"
	"github.com/DengNaichen/inftask/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"testing"
)

func TestLabelsOverMCP(t *testing.T) {
	session := connect(t)
	var g app.LabelGroup
	var l, other app.Label
	var issue app.Issue
	var ignored struct{}
	call(t, session, "save_label_group", map[string]any{"name": "Risk"}, &g)
	call(t, session, "create_label", map[string]any{"name": "Low", "color": "#123456", "group_id": g.ID}, &l)
	call(t, session, "create_label", map[string]any{"name": "High", "color": "#abcdef", "group_id": g.ID}, &other)
	call(t, session, "create_issue", map[string]any{"title": "A"}, &issue)
	call(t, session, "change_issue_labels", map[string]any{"identifiers": []string{issue.Identifier}, "add": []int64{l.ID}}, &ignored)
	var detail app.IssueDetails
	call(t, session, "get_issue", map[string]any{"identifier": issue.Identifier}, &detail)
	if len(detail.Labels) != 1 || detail.Labels[0].ID != l.ID {
		t.Fatal(detail)
	}
	var listed listOutput
	call(t, session, "list_issues", map[string]any{"label_ids": []int64{l.ID}, "label_mode": "all"}, &listed)
	if len(listed.Issues) != 1 || len(listed.Issues[0].Labels) != 1 {
		t.Fatal(listed)
	}
	call(t, session, "update_label", map[string]any{"id": l.ID, "archived": true}, &l)
	var labels labelsOutput
	call(t, session, "list_labels", map[string]any{}, &labels)
	if len(labels.Labels) != 1 {
		t.Fatal(labels)
	}
	call(t, session, "list_labels", map[string]any{"include_archived": true}, &labels)
	if len(labels.Labels) != 2 {
		t.Fatal(labels)
	}
	var groups groupsOutput
	call(t, session, "list_label_groups", map[string]any{}, &groups)
	if len(groups.Groups) != 1 {
		t.Fatal(groups)
	}
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "change_issue_labels", Arguments: map[string]any{"identifiers": []string{issue.Identifier}, "add": []int64{l.ID, other.ID}}})
	if err != nil || !result.IsError {
		t.Fatalf("invalid group additions: %v %v", result, err)
	}
	call(t, session, "merge_labels", map[string]any{"source_id": l.ID, "target_id": other.ID}, &ignored)
	call(t, session, "get_issue", map[string]any{"identifier": issue.Identifier}, &detail)
	if len(detail.Labels) != 1 || detail.Labels[0].ID != other.ID {
		t.Fatal(detail)
	}
	call(t, session, "change_issue_labels", map[string]any{"identifiers": []string{issue.Identifier}, "remove": []int64{other.ID}}, &ignored)
	call(t, session, "get_issue", map[string]any{"identifier": issue.Identifier}, &detail)
	if len(detail.Labels) != 0 {
		t.Fatal(detail)
	}
}
