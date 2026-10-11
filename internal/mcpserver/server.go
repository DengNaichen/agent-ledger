package mcpserver

import (
	"context"
	"net/http"

	"github.com/DengNaichen/inftask/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type identifierInput struct {
	Identifier string `json:"identifier" jsonschema:"issue identifier <team key>-<number>, such as AL-1"`
}

type listOutput struct {
	Issues []app.IssueSummary `json:"issues" jsonschema:"issue summaries without descriptions, including priority and ordered by the requested sort"`
}

type commentInput struct {
	Identifier string `json:"identifier" jsonschema:"issue identifier, such as AL-1"`
	Body       string `json:"body" jsonschema:"comment body; comments are append-only"`
}

type relationInput struct {
	Identifier string `json:"identifier" jsonschema:"issue identifier, such as AL-1"`
	Kind       string `json:"kind" jsonschema:"relation kind: blocks (this issue blocks other), blocked_by (other blocks this issue), or related (undirected)"`
	Other      string `json:"other" jsonschema:"other issue identifier; must differ from identifier"`
}

type linkInput struct {
	Identifier string `json:"identifier" jsonschema:"issue identifier, such as AL-1"`
	Title      string `json:"title" jsonschema:"link title"`
	URL        string `json:"url" jsonschema:"link URL"`
}

type listProjectsInput struct {
	IncludeArchived bool `json:"include_archived,omitempty" jsonschema:"include archived Projects; defaults to false"`
}

type projectsOutput struct {
	Projects []app.Project `json:"projects"`
}

type teamsOutput struct {
	Teams []app.Team `json:"teams"`
}

type createProjectInput struct {
	Name        string `json:"name" jsonschema:"unique Project name"`
	Description string `json:"description,omitempty" jsonschema:"the outcome this Project should deliver"`
}

// Handler serves the tools over streamable HTTP. Each request is
// self-contained, so clients survive server restarts without re-initializing.
func Handler(service *app.Service) http.Handler {
	server := New(service)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server },
		&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
}

func New(service *app.Service) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{Name: "inftask", Version: "0.1.0"}, nil)
	const statuses = "Statuses: triage, todo, agent_ready, in_progress, in_review, done, canceled. "
	const types = "Types: spec, feature, bug, improvement, chore; empty string means no type. "
	mcp.AddTool(server, &mcp.Tool{Name: "list_issues", Description: "List summaries without descriptions, including priority. Default sort: priority for status=agent_ready, otherwise newest updated first. Explicit sort=priority or updated overrides; no priority sorts last; ties use newest updated then newest id. All filters combine with AND; team filters by team key; limit defaults to 50. " + statuses + types},
		func(ctx context.Context, _ *mcp.CallToolRequest, input app.ListIssues) (*mcp.CallToolResult, listOutput, error) {
			issues, err := service.List(ctx, input)
			return nil, listOutput{Issues: app.Summaries(issues)}, err
		})
	mcp.AddTool(server, &mcp.Tool{Name: "get_issue", Description: "Get a complete issue with comments, links, linked GitHub pull requests, parent identifier, children summaries, and blocks, blocked_by, related summaries. " + statuses + types},
		func(ctx context.Context, _ *mcp.CallToolRequest, input identifierInput) (*mcp.CallToolResult, app.IssueDetails, error) {
			issue, err := service.Details(ctx, input.Identifier)
			return nil, issue, err
		})
	mcp.AddTool(server, &mcp.Tool{Name: "create_issue", Description: "Create an issue in a team; team and title are required, status defaults to triage. There is no default team: use the team key the repository names, or list_teams. The identifier is <team key>-<n>, where n is the team's next number. Project and parent must already exist. " + statuses + types},
		func(ctx context.Context, _ *mcp.CallToolRequest, input app.CreateIssue) (*mcp.CallToolResult, app.Issue, error) {
			issue, err := service.Create(ctx, input)
			return nil, issue, err
		})
	mcp.AddTool(server, &mcp.Tool{Name: "update_issue", Description: "Update only supplied fields; omitted priority is unchanged and priority=0 clears it. Empty strings clear type, project, or parent. Parent cannot be self. " + statuses + types},
		func(ctx context.Context, _ *mcp.CallToolRequest, input app.UpdateIssue) (*mcp.CallToolResult, app.Issue, error) {
			issue, err := service.Update(ctx, input)
			return nil, issue, err
		})
	mcp.AddTool(server, &mcp.Tool{Name: "add_comment", Description: "Append a comment to an issue. Comments cannot be edited or deleted."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input commentInput) (*mcp.CallToolResult, app.Comment, error) {
			comment, err := service.AddComment(ctx, input.Identifier, input.Body)
			return nil, comment, err
		})
	for _, operation := range []struct {
		name, description string
		remove            bool
	}{
		{"add_relation", "Add a relation; duplicate additions succeed without changing it.", false},
		{"remove_relation", "Remove a relation; removing an absent relation succeeds.", true},
	} {
		mcp.AddTool(server, &mcp.Tool{Name: operation.name, Description: operation.description + " Kinds: blocks (this issue blocks other), blocked_by (other blocks this issue), related (undirected). Self-relations are forbidden."},
			func(ctx context.Context, _ *mcp.CallToolRequest, input relationInput) (*mcp.CallToolResult, struct{}, error) {
				err := service.Relation(ctx, input.Identifier, input.Kind, input.Other, operation.remove)
				return nil, struct{}{}, err
			})
	}
	mcp.AddTool(server, &mcp.Tool{Name: "add_link", Description: "Attach a titled URL to an issue."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input linkInput) (*mcp.CallToolResult, app.Link, error) {
			link, err := service.AddLink(ctx, input.Identifier, input.Title, input.URL)
			return nil, link, err
		})
	mcp.AddTool(server, &mcp.Tool{Name: "list_teams", Description: "List teams by key. Each team numbers its own issues <key>-<n>; next_number is the number its next issue gets."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, teamsOutput, error) {
			teams, err := service.ListTeams(ctx)
			return nil, teamsOutput{Teams: teams}, err
		})
	mcp.AddTool(server, &mcp.Tool{Name: "create_team", Description: "Create a team whose issues are numbered <key>-1, <key>-2, and so on. key is 2 to 6 uppercase letters or digits starting with a letter; name is required. start_number makes numbering begin above numbers the team already used elsewhere, such as a team imported from another tracker. A key cannot be changed, and teams cannot be renamed or deleted."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input app.CreateTeam) (*mcp.CallToolResult, app.Team, error) {
			team, err := service.CreateTeam(ctx, input)
			return nil, team, err
		})
	mcp.AddTool(server, &mcp.Tool{Name: "list_projects", Description: "List Projects alphabetically; archived Projects are excluded unless include_archived is true."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input listProjectsInput) (*mcp.CallToolResult, projectsOutput, error) {
			projects, err := service.ListProjects(ctx, input.IncludeArchived)
			return nil, projectsOutput{Projects: projects}, err
		})
	mcp.AddTool(server, &mcp.Tool{Name: "create_project", Description: "Create a Project with a unique name and an optional outcome description. New Projects are unarchived."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input createProjectInput) (*mcp.CallToolResult, app.Project, error) {
			project, err := service.CreateProject(ctx, input.Name, input.Description)
			return nil, project, err
		})
	mcp.AddTool(server, &mcp.Tool{Name: "update_project", Description: "Find a Project by name and update only supplied description or archived fields. Renaming is not supported."},
		func(ctx context.Context, _ *mcp.CallToolRequest, input app.UpdateProject) (*mcp.CallToolResult, app.Project, error) {
			project, err := service.UpdateProject(ctx, input)
			return nil, project, err
		})
	addLabelTools(server, service)
	return server
}
