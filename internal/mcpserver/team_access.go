package mcpserver

import (
	"context"
	"github.com/DengNaichen/inftask/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func addTeamAccessTools(server *mcp.Server, service *app.Service) {
	addTool(server, &mcp.Tool{Name: "list_team_directory", Description: "Discover accessible teams; administrators also see private team metadata. Discovery does not grant issue access. Use join_team with confirm_private to explicitly enter a private team."}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, teamsOutput, error) {
		v, e := service.TeamDirectory(ctx)
		return nil, teamsOutput{Teams: v}, e
	})
	addTool(server, &mcp.Tool{Name: "join_team", Description: "Join or leave a team as the authenticated user. Public teams are open. Administrators must explicitly confirm before entering private teams; other users must be added by an owner."}, func(ctx context.Context, _ *mcp.CallToolRequest, input app.JoinTeam) (*mcp.CallToolResult, app.Team, error) {
		v, e := service.JoinTeam(ctx, input)
		return nil, v, e
	})
	addTool(server, &mcp.Tool{Name: "list_team_members", Description: "List team members and effective owner roles."}, func(ctx context.Context, _ *mcp.CallToolRequest, input struct {
		Key string `json:"key"`
	}) (*mcp.CallToolResult, struct {
		Members []app.TeamMember `json:"members"`
	}, error) { v, e := service.ListTeamMembers(ctx, input.Key); return nil, struct {
		Members []app.TeamMember `json:"members"`
	}{v}, e })
	addTool(server, &mcp.Tool{Name: "change_team_member", Description: "As team owner or workspace administrator, add, promote, demote or remove a workspace member from a team. Teams may have no owners; workspace administrators retain management access."}, func(ctx context.Context, _ *mcp.CallToolRequest, input app.ChangeTeamMember) (*mcp.CallToolResult, struct{}, error) {
		return nil, struct{}{}, service.ChangeTeamMember(ctx, input)
	})
}
