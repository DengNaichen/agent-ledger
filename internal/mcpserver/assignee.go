package mcpserver

import (
	"context"
	"github.com/DengNaichen/inftask/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type assigneesOutput struct {
	Assignees []app.Identity `json:"assignees"`
}

func addAssigneeTools(server *mcp.Server, service *app.Service) {
	addTool(server, &mcp.Tool{Name: "list_assignees", Description: "List active human assignee candidates for a visible team. Public teams allow all workspace members; private teams allow only their members. Use the returned subject in create_issue or update_issue; an empty assignee clears it. Assignment does not lock editing."}, func(ctx context.Context, _ *mcp.CallToolRequest, input app.ListAssignees) (*mcp.CallToolResult, assigneesOutput, error) {
		members, err := service.ListAssignees(ctx, input)
		return nil, assigneesOutput{members}, err
	})
}
