package mcpserver

import (
	"context"
	"net/http"

	"github.com/DengNaichen/agent-ledger/internal/app"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type identifierInput struct {
	Identifier string `json:"identifier" jsonschema:"issue identifier using the configured prefix, such as AL-1"`
}

type listOutput struct {
	Issues []app.IssueSummary `json:"issues" jsonschema:"issue summaries without descriptions, ordered by updated_at descending"`
}

type issueOutput struct {
	Issue app.Issue `json:"issue"`
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
	server := mcp.NewServer(&mcp.Implementation{Name: "agent-ledger", Version: "0.1.0"}, nil)
	const statuses = "Statuses: triage, todo, agent_ready, in_progress, in_review, done, canceled. "
	const types = "Types: spec, feature, bug, improvement, chore; empty string means no type. "
	mcp.AddTool(server, &mcp.Tool{Name: "list_issues", Description: "List summaries without descriptions, newest updated first. All filters combine with AND; limit defaults to 50. " + statuses + types},
		func(ctx context.Context, _ *mcp.CallToolRequest, input app.ListIssues) (*mcp.CallToolResult, listOutput, error) {
			issues, err := service.List(ctx, input)
			return nil, listOutput{Issues: app.Summaries(issues)}, err
		})
	mcp.AddTool(server, &mcp.Tool{Name: "get_issue", Description: "Get a complete issue with comments, links, linked GitHub pull requests, parent identifier, children summaries, and blocks, blocked_by, related summaries. " + statuses + types},
		func(ctx context.Context, _ *mcp.CallToolRequest, input identifierInput) (*mcp.CallToolResult, app.IssueDetails, error) {
			issue, err := service.Details(ctx, input.Identifier)
			return nil, issue, err
		})
	mcp.AddTool(server, &mcp.Tool{Name: "create_issue", Description: "Create an issue; title is required, status defaults to triage. Project and parent must already exist. " + statuses + types},
		func(ctx context.Context, _ *mcp.CallToolRequest, input app.CreateIssue) (*mcp.CallToolResult, issueOutput, error) {
			issue, err := service.Create(ctx, input)
			return nil, issueOutput{Issue: issue}, err
		})
	mcp.AddTool(server, &mcp.Tool{Name: "update_issue", Description: "Update only supplied fields. Empty strings clear type, project, or parent. Parent cannot be self. " + statuses + types},
		func(ctx context.Context, _ *mcp.CallToolRequest, input app.UpdateIssue) (*mcp.CallToolResult, issueOutput, error) {
			issue, err := service.Update(ctx, input)
			return nil, issueOutput{Issue: issue}, err
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
	return server
}
