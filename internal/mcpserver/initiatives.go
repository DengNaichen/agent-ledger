package mcpserver

import (
	"context"
	"github.com/DengNaichen/inftask/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func addInitiativeTools(server *mcp.Server, service *app.Service) {
	type listInput struct {
		IncludeArchived bool `json:"include_archived,omitempty"`
	}
	type listOutput struct {
		Initiatives []app.Initiative `json:"initiatives"`
	}
	type getInput struct {
		ID int64 `json:"id"`
	}
	addTool(server, &mcp.Tool{Name: "list_initiatives", Description: "List workspace objectives; project_ids include only visible projects."}, func(ctx context.Context, _ *mcp.CallToolRequest, input listInput) (*mcp.CallToolResult, listOutput, error) {
		items, err := service.ListInitiatives(ctx, input.IncludeArchived)
		return nil, listOutput{items}, err
	})
	addTool(server, &mcp.Tool{Name: "get_initiative", Description: "Read an initiative by stable id and its visible project associations."}, func(ctx context.Context, _ *mcp.CallToolRequest, input getInput) (*mcp.CallToolResult, app.Initiative, error) {
		item, err := service.GetInitiative(ctx, input.ID)
		return nil, item, err
	})
	addTool(server, &mcp.Tool{Name: "create_initiative", Description: "Create a workspace objective with optional existing project associations."}, func(ctx context.Context, _ *mcp.CallToolRequest, input app.CreateInitiative) (*mcp.CallToolResult, app.Initiative, error) {
		item, err := service.CreateInitiative(ctx, input)
		return nil, item, err
	})
	addTool(server, &mcp.Tool{Name: "update_initiative", Description: "Edit or archive an initiative. Omitted fields stay unchanged. project_ids replaces visible associations, preserving hidden ones; empty clears visible associations. Archived initiatives/projects cannot receive new associations."}, func(ctx context.Context, _ *mcp.CallToolRequest, input app.UpdateInitiative) (*mcp.CallToolResult, app.Initiative, error) {
		item, err := service.UpdateInitiative(ctx, input)
		return nil, item, err
	})
}
