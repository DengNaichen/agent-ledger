package mcpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/DengNaichen/inftask/internal/app"
	"github.com/DengNaichen/inftask/internal/httpapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestIssueDelegationIdentityMCP(t *testing.T) {
	root, err := app.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	root = root.ForRequests(true)
	actor := app.Identity{Subject: "member", Name: "Member"}
	ctx := app.WithIdentity(context.Background(), actor)
	if _, err = root.CreateWorkspace(ctx, app.Workspace{Slug: "default", Name: "Default"}); err != nil {
		t.Fatal(err)
	}
	scoped, err := root.InWorkspace(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = scoped.CreateTeam(ctx, app.CreateTeam{Key: "AL", Name: "InfTask"}); err != nil {
		t.Fatal(err)
	}
	issue, err := scoped.Create(ctx, app.CreateIssue{Team: "AL", Title: "Identity"})
	if err != nil {
		t.Fatal(err)
	}
	handler := httpapi.New(root, http.NotFoundHandler(), Handler(root))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.WithContext(app.WithIdentity(r.Context(), actor)))
	}))
	defer server.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "identity-test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: server.URL + "/w/default/mcp"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	var delegation app.Delegation
	call(t, session, "delegate_issue", map[string]any{"identifier": issue.Identifier, "client_context": map[string]any{"agent": "claude", "session_id": "session", "agent_id": "child", "hostname": "host"}}, &delegation)
	if delegation.Actor == nil || delegation.Actor.Subject != actor.Subject || delegation.AgentID != "child" {
		t.Fatal(delegation)
	}
}

func TestIssueDelegationLifecycle(t *testing.T) {
	session := connect(t)
	var issue app.Issue
	call(t, session, "create_issue", map[string]any{"team": "AL", "title": "Delegate", "status": "todo"}, &issue)
	metadata := map[string]any{"agent": "codex", "session_id": "parent-session", "hostname": "test-host"}
	args := map[string]any{"identifier": issue.Identifier, "client_context": metadata}
	var first map[string]any
	call(t, session, "delegate_issue", args, &first)
	if first["session_id"] != "parent-session" || first["agent_id"] != nil || first["actor"] != nil {
		t.Fatal(first)
	}
	var duplicate map[string]any
	call(t, session, "delegate_issue", args, &duplicate)
	if duplicate["id"] != first["id"] || duplicate["created_at"] != first["created_at"] {
		t.Fatal(duplicate)
	}
	metadata["agent_id"] = "child-a"
	metadata["agent_type"] = "implementer"
	call(t, session, "delegate_issue", args, &duplicate)
	metadata["agent_id"] = "child-b"
	call(t, session, "delegate_issue", args, &duplicate)
	var detail struct {
		app.IssueDetails
		Delegations []map[string]any `json:"delegations"`
	}
	call(t, session, "get_issue", map[string]any{"identifier": issue.Identifier}, &detail)
	if len(detail.Delegations) != 3 || detail.Status != app.StatusTodo || detail.Assignee != nil || detail.UpdatedAt != issue.UpdatedAt {
		t.Fatal(detail)
	}
	for _, invalid := range []map[string]any{
		{"identifier": issue.Identifier},
		{"identifier": issue.Identifier, "client_context": nil},
		{"identifier": issue.Identifier, "client_context": map[string]any{"agent": "codex", "session_id": "s", "hostname": "h", "actor": "spoof"}},
		{"identifier": issue.Identifier, "client_context": map[string]any{"agent": "other", "session_id": "s", "hostname": "h"}},
		{"identifier": issue.Identifier, "client_context": map[string]any{"agent": "codex", "session_id": "", "hostname": "h"}},
		{"identifier": issue.Identifier, "client_context": metadata, "actor": "spoof"},
		{"identifier": "AL-999", "client_context": metadata},
	} {
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "delegate_issue", Arguments: invalid})
		if err == nil && !result.IsError {
			t.Fatalf("accepted invalid input: %#v", invalid)
		}
	}
}
