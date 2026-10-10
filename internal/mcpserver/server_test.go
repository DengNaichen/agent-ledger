package mcpserver

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/DengNaichen/agent-ledger/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCreateIssueTool(t *testing.T) {
	service, err := app.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	ctx := context.Background()
	server := httptest.NewServer(Handler(service))
	defer server.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "agent-ledger-test", Version: "0.1.0"}, nil)
	clientSession, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer clientSession.Close()

	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name:      "create_issue",
		Arguments: map[string]any{"title": "Created through MCP", "description": "One shared service."},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("tool returned an error: %#v", result.Content)
	}

	issues, err := service.List(ctx, app.StatusTriage)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || issues[0].Title != "Created through MCP" {
		t.Fatalf("unexpected issues: %#v", issues)
	}
}
