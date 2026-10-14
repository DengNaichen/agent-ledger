package mcpserver

import (
	"context"
	"github.com/DengNaichen/inftask/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func cycleTools(server *mcp.Server, s *app.Service) {
	addTool(server, &mcp.Tool{Name: "list_cycles", Description: "List a team's cycles and schedule settings. Completed statistics are immutable snapshots; issue lists may change afterward."}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		Team string `json:"team"`
	}) (*mcp.CallToolResult, app.CyclePage, error) { v, e := s.ListCycles(ctx, in.Team); return nil, v, e })
	addTool(server, &mcp.Tool{Name: "configure_cycles", Description: "Enable or disable a team's repeating cycles. Team owner/admin required. Supply IANA timezone, YYYY-MM-DD start_date (today or later), duration_weeks 1–8 and upcoming_count 1–15. Disable before changing cadence. Disabling closes current cycle, clears upcoming assignments and removes upcoming plans, preserving history. Does not change issue statuses."}, func(ctx context.Context, _ *mcp.CallToolRequest, in app.ConfigureCycles) (*mcp.CallToolResult, app.CyclePage, error) {
		v, e := s.ConfigureCycles(ctx, in)
		return nil, v, e
	})
	addTool(server, &mcp.Tool{Name: "get_cycle", Description: "Get an accessible cycle with live counts or its completion snapshot."}, func(ctx context.Context, _ *mcp.CallToolRequest, in struct {
		ID int64 `json:"id"`
	}) (*mcp.CallToolResult, app.Cycle, error) { v, e := s.GetCycle(ctx, in.ID); return nil, v, e })
	addTool(server, &mcp.Tool{Name: "update_cycle", Description: "Rename a cycle or edit its description. Dates and historical statistics are preserved."}, func(ctx context.Context, _ *mcp.CallToolRequest, in app.UpdateCycle) (*mcp.CallToolResult, app.Cycle, error) {
		v, e := s.UpdateCycle(ctx, in)
		return nil, v, e
	})
}
