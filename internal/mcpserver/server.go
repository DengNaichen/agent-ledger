package mcpserver

import (
	"context"
	"net/http"

	"github.com/DengNaichen/agent-ledger/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type listInput struct {
	Status string `json:"status,omitempty" jsonschema:"optional status filter: triage, ready, in_progress, in_review, or done"`
}

type listOutput struct {
	Issues []app.Issue `json:"issues" jsonschema:"issues ordered newest first"`
}

type createInput struct {
	Title       string `json:"title" jsonschema:"short outcome-oriented issue title"`
	Description string `json:"description,omitempty" jsonschema:"self-contained issue description"`
}

type issueOutput struct {
	Issue app.Issue `json:"issue"`
}

type moveInput struct {
	Identifier string `json:"identifier" jsonschema:"issue identifier such as AL-1"`
	Status     string `json:"status" jsonschema:"target status: triage, ready, in_progress, in_review, or done"`
}

// Handler serves the tools over streamable HTTP. Each request is
// self-contained, so clients survive server restarts without re-initializing.
func Handler(service *app.Service) http.Handler {
	server := New(service)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
}

func New(service *app.Service) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "agent-ledger", Version: "0.1.0"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "list_issues", Description: "List Agent Ledger issues, optionally by status."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input listInput) (*mcp.CallToolResult, listOutput, error) {
			issues, err := service.List(ctx, app.Status(input.Status))
			return nil, listOutput{Issues: issues}, err
		})
	mcp.AddTool(server, &mcp.Tool{Name: "create_issue", Description: "Create a self-contained issue in Triage."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input createInput) (*mcp.CallToolResult, issueOutput, error) {
			issue, err := service.Create(ctx, app.CreateIssue{Title: input.Title, Description: input.Description})
			return nil, issueOutput{Issue: issue}, err
		})
	mcp.AddTool(server, &mcp.Tool{Name: "move_issue", Description: "Move an issue to another workflow status."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input moveInput) (*mcp.CallToolResult, issueOutput, error) {
			issue, err := service.Move(ctx, input.Identifier, app.Status(input.Status))
			return nil, issueOutput{Issue: issue}, err
		})
	return server
}
