package mcpserver

import (
	"context"

	"github.com/DengNaichen/inftask/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type labelsOutput struct {
	Labels []app.Label `json:"labels"`
}
type groupsOutput struct {
	Groups []app.LabelGroup `json:"groups"`
}
type listLabelsInput struct {
	IncludeArchived bool `json:"include_archived,omitempty" jsonschema:"include archived labels; defaults to false"`
}

func addLabelTools(server *mcp.Server, service *app.Service) {
	mcp.AddTool(server, &mcp.Tool{Name: "list_labels", Description: "List the workspace's issue labels by name. Reuse existing labels; include_archived includes historical labels usable in filters."}, func(ctx context.Context, _ *mcp.CallToolRequest, input listLabelsInput) (*mcp.CallToolResult, labelsOutput, error) {
		labels, err := service.ListLabels(ctx, input.IncludeArchived)
		return nil, labelsOutput{labels}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "list_label_groups", Description: "List single-select label groups. Ungrouped labels are multi-select."}, func(ctx context.Context, _ *mcp.CallToolRequest, input struct{}) (*mcp.CallToolResult, groupsOutput, error) {
		groups, err := service.ListLabelGroups(ctx)
		return nil, groupsOutput{groups}, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "save_label_group", Description: "Create a group, or rename by stable ID. Names are trimmed and case-insensitively unique. Groups cannot nest."}, func(ctx context.Context, _ *mcp.CallToolRequest, input app.SaveLabelGroup) (*mcp.CallToolResult, app.LabelGroup, error) {
		group, err := service.SaveLabelGroup(ctx, input)
		return nil, group, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "create_label", Description: "Explicitly create an issue label in the workspace. Search existing labels first. Names are trimmed and unique ignoring case within a group. Group membership is fixed."}, func(ctx context.Context, _ *mcp.CallToolRequest, input app.CreateLabel) (*mcp.CallToolResult, app.Label, error) {
		label, err := service.CreateLabel(ctx, input)
		return nil, label, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "update_label", Description: "Edit supplied properties by stable ID. Archive preserves existing associations and filters, but prevents new associations. Set archived=false to restore."}, func(ctx context.Context, _ *mcp.CallToolRequest, input app.UpdateLabel) (*mcp.CallToolResult, app.Label, error) {
		label, err := service.UpdateLabel(ctx, input)
		return nil, label, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "change_issue_labels", Description: "Atomically add/remove labels across one or more issues. Incremental and idempotent; an added grouped label replaces the old value. Multiple additions in one group or overlapping add/remove fail. Archived labels cannot be newly applied."}, func(ctx context.Context, _ *mcp.CallToolRequest, input app.ChangeIssueLabels) (*mcp.CallToolResult, struct{}, error) {
		return nil, struct{}{}, service.ChangeIssueLabels(ctx, input)
	})
	mcp.AddTool(server, &mcp.Tool{Name: "merge_labels", Description: "Transfer source associations to an active target in the same group, then delete the source. Atomic; source ID stops existing. Update any external filters using that ID."}, func(ctx context.Context, _ *mcp.CallToolRequest, input app.MergeLabels) (*mcp.CallToolResult, struct{}, error) {
		return nil, struct{}{}, service.MergeLabels(ctx, input)
	})
}
