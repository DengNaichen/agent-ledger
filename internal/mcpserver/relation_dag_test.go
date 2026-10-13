package mcpserver

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/DengNaichen/inftask/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestRelationDAG(t *testing.T) {
	session := connect(t)
	var issue app.Issue
	for range 3 {
		call(t, session, "create_issue", map[string]any{"team": "AL", "title": "Dependency"}, &issue)
	}
	var ignored struct{}
	for _, edge := range [][2]string{{"AL-1", "AL-2"}, {"AL-2", "AL-3"}} {
		call(t, session, "add_relation", map[string]any{"identifier": edge[0], "kind": "blocks", "other": edge[1]}, &ignored)
	}
	for _, input := range []map[string]any{
		{"identifier": "AL-3", "kind": "blocks", "other": "AL-1"},
		{"identifier": "AL-1", "kind": "blocked_by", "other": "AL-3"},
		{"identifier": "AL-1", "kind": "blocks", "other": "AL-1"},
	} {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "add_relation", Arguments: input})
		if err != nil {
			t.Fatal(err)
		}
		content, err := json.Marshal(result.Content)
		if err != nil {
			t.Fatal(err)
		}
		if !result.IsError || !strings.Contains(string(content), app.ErrRelationCycle.Error()) {
			t.Fatalf("expected cycle error, got %s", content)
		}
	}
	var detail app.IssueDetails
	call(t, session, "get_issue", map[string]any{"identifier": "AL-3"}, &detail)
	if len(detail.Blocks) != 0 || len(detail.BlockedBy) != 1 {
		t.Fatal("rejected writes changed graph", detail)
	}
	call(t, session, "remove_relation", map[string]any{"identifier": "AL-2", "kind": "blocks", "other": "AL-3"}, &ignored)
	call(t, session, "add_relation", map[string]any{"identifier": "AL-3", "kind": "blocks", "other": "AL-1"}, &ignored)
}
