package mcpserver

import (
	"testing"

	"github.com/DengNaichen/inftask/internal/app"
)

func TestBacklogOverMCP(t *testing.T) {
	session := connect(t)
	var issue app.Issue
	call(t, session, "create_issue", map[string]any{"team": "AL", "title": "Keep for later", "status": "backlog"}, &issue)
	if issue.Status != "backlog" {
		t.Fatalf("created status = %q", issue.Status)
	}
	var listed struct {
		Issues []app.Issue `json:"issues"`
	}
	call(t, session, "list_issues", map[string]any{"status": "backlog"}, &listed)
	if len(listed.Issues) != 1 || listed.Issues[0].Identifier != issue.Identifier {
		t.Fatalf("backlog filter = %#v", listed)
	}
	identifier := issue.Identifier
	call(t, session, "update_issue", map[string]any{"identifier": identifier, "status": "todo"}, &issue)
	call(t, session, "update_issue", map[string]any{"identifier": identifier, "status": "backlog"}, &issue)
	if issue.Status != "backlog" {
		t.Fatalf("updated status = %q", issue.Status)
	}
	call(t, session, "create_issue", map[string]any{"team": "AL", "title": "Undecided"}, &issue)
	if issue.Status != app.StatusTriage {
		t.Fatalf("default status = %q", issue.Status)
	}
}
