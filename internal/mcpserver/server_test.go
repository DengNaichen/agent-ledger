package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DengNaichen/inftask/internal/app"
	"github.com/DengNaichen/inftask/internal/httpapi"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// connect opens a session on the MCP endpoint of workspace default, which has
// team AL.
func connect(t *testing.T) *mcp.ClientSession {
	t.Helper()
	service, err := app.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { service.Close() })
	ctx := context.Background()
	if _, err := service.CreateWorkspace(ctx, app.Workspace{Slug: "default", Name: "Default"}); err != nil {
		t.Fatal(err)
	}
	def, err := service.InWorkspace(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := def.CreateTeam(ctx, app.CreateTeam{Key: "AL", Name: "InfTask"}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(httpapi.New(service, http.NotFoundHandler(), Handler(service)))
	t.Cleanup(server.Close)
	client := mcp.NewClient(&mcp.Implementation{Name: "inftask-test", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/w/default/mcp"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func call(t *testing.T, session *mcp.ClientSession, name string, args map[string]any, output any) {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("%s: %v", name, result.Content)
	}
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, output); err != nil {
		t.Fatal(err)
	}
}

func TestIssueWorkflowOverHTTP(t *testing.T) {
	session := connect(t)
	var project app.Project
	call(t, session, "create_project", map[string]any{"name": "Delivery", "description": "Ship the issue model"}, &project)
	if project.Name != "Delivery" || project.Description != "Ship the issue model" || project.Archived {
		t.Fatalf("unexpected Project: %#v", project)
	}
	var created app.Issue
	call(t, session, "create_issue", map[string]any{"team": "AL", "title": "Issue specification", "type": "spec"}, &created)
	parent := created.Identifier
	if parent != "AL-1" || created.Status != app.StatusTriage {
		t.Fatalf("unexpected spec: %#v", created)
	}
	call(t, session, "create_issue", map[string]any{
		"team": "AL", "title": "Implement model", "description": "Keep all context", "status": "agent_ready", "type": "feature", "project": "Delivery", "parent": parent,
	}, &created)
	child := created.Identifier
	var ignored struct{}
	for range 2 {
		call(t, session, "add_relation", map[string]any{"identifier": child, "kind": "blocked_by", "other": parent}, &ignored)
	}
	call(t, session, "add_comment", map[string]any{"identifier": child, "body": "Implementation notes"}, &ignored)
	call(t, session, "add_link", map[string]any{"identifier": child, "title": "Design", "url": "https://example.com/design"}, &ignored)
	var listed struct {
		Issues []map[string]any `json:"issues"`
	}
	call(t, session, "list_issues", map[string]any{"parent": parent}, &listed)
	if len(listed.Issues) != 1 || listed.Issues[0]["identifier"] != child || listed.Issues[0]["updated_at"] == "" {
		t.Fatalf("unexpected children: %#v", listed)
	}
	if _, exists := listed.Issues[0]["description"]; exists {
		t.Fatal("list_issues included description")
	}
	var detail app.IssueDetails
	call(t, session, "get_issue", map[string]any{"identifier": child}, &detail)
	if detail.Description != "Keep all context" || detail.Status != app.StatusAgentReady || *detail.Type != "feature" || *detail.Project != "Delivery" || *detail.Parent != parent {
		t.Fatalf("unexpected issue: %#v", detail)
	}
	if len(detail.BlockedBy) != 1 || detail.BlockedBy[0].Identifier != parent || len(detail.Blocks) != 0 {
		t.Fatalf("unexpected relations: %#v", detail)
	}
	if len(detail.Comments) != 1 || detail.Comments[0].Body != "Implementation notes" || detail.Comments[0].CreatedAt == "" || len(detail.Links) != 1 || detail.Links[0].Title != "Design" || detail.Links[0].URL != "https://example.com/design" {
		t.Fatalf("unexpected comments or links: %#v", detail)
	}
	call(t, session, "get_issue", map[string]any{"identifier": parent}, &detail)
	if *detail.Type != "spec" || len(detail.Children) != 1 || detail.Children[0].Identifier != child || len(detail.Blocks) != 1 || detail.Blocks[0].Identifier != child {
		t.Fatalf("unexpected parent: %#v", detail)
	}
	call(t, session, "update_issue", map[string]any{"identifier": child, "parent": ""}, &created)
	if created.Parent != nil || *created.Type != "feature" || *created.Project != "Delivery" || created.Description != "Keep all context" {
		t.Fatalf("clearing parent changed other fields: %#v", created)
	}
	call(t, session, "list_issues", map[string]any{"parent": parent}, &listed)
	if len(listed.Issues) != 0 {
		t.Fatalf("parent was not cleared: %#v", listed)
	}

	// Exercise the other filters together, including a match in the description.
	filters := map[string]any{"status": "agent_ready", "type": "feature", "project": "Delivery", "query": "context", "limit": 1}
	call(t, session, "list_issues", filters, &listed)
	if len(listed.Issues) != 1 || listed.Issues[0]["identifier"] != child {
		t.Fatalf("unexpected filtered list: %#v", listed)
	}
	filters["type"] = "bug"
	call(t, session, "list_issues", filters, &listed)
	if len(listed.Issues) != 0 {
		t.Fatal("filters did not combine with AND")
	}
	call(t, session, "update_issue", map[string]any{"identifier": child, "title": "Shipped", "description": "", "status": "done", "type": "", "project": ""}, &created)
	if created.Title != "Shipped" || created.Description != "" || created.Status != app.StatusDone || created.Type != nil || created.Project != nil {
		t.Fatalf("unexpected updated issue: %#v", created)
	}
	for _, pair := range [][2]string{{parent, child}, {child, parent}} {
		call(t, session, "add_relation", map[string]any{"identifier": pair[0], "kind": "related", "other": pair[1]}, &ignored)
	}
	for _, pair := range [][2]string{{parent, child}, {child, parent}} {
		call(t, session, "get_issue", map[string]any{"identifier": pair[0]}, &detail)
		if len(detail.Related) != 1 || detail.Related[0].Identifier != pair[1] {
			t.Fatalf("unexpected undirected relation: %#v", detail)
		}
	}
	call(t, session, "remove_relation", map[string]any{"identifier": child, "kind": "related", "other": parent}, &ignored)
	call(t, session, "remove_relation", map[string]any{"identifier": child, "kind": "blocked_by", "other": parent}, &ignored)
	call(t, session, "get_issue", map[string]any{"identifier": parent}, &detail)
	if len(detail.Related) != 0 || len(detail.Blocks) != 0 {
		t.Fatalf("relations were not removed: %#v", detail)
	}
	var projects projectsOutput
	call(t, session, "list_projects", map[string]any{}, &projects)
	if len(projects.Projects) != 1 {
		t.Fatalf("unexpected Projects: %#v", projects)
	}
	call(t, session, "update_project", map[string]any{"name": "Delivery", "description": "Delivered", "archived": true}, &project)
	if project.Description != "Delivered" || !project.Archived {
		t.Fatalf("unexpected archived Project: %#v", project)
	}
	call(t, session, "list_projects", map[string]any{}, &projects)
	if len(projects.Projects) != 0 {
		t.Fatal("archived Project included by default")
	}
	call(t, session, "list_projects", map[string]any{"include_archived": true}, &projects)
	if len(projects.Projects) != 1 || !projects.Projects[0].Archived {
		t.Fatalf("archived Project missing: %#v", projects)
	}
	call(t, session, "update_project", map[string]any{"name": "Delivery", "archived": false}, &project)
	if project.Description != "Delivered" || project.Archived {
		t.Fatalf("unexpected restored Project: %#v", project)
	}
}

func TestSingleObjectToolsReturnObjectAtTopLevel(t *testing.T) {
	session := connect(t)
	for _, step := range []struct {
		tool string
		args map[string]any
	}{
		{"create_issue", map[string]any{"team": "AL", "title": "Top-level output"}},
		{"update_issue", map[string]any{"identifier": "AL-1", "status": "todo"}},
	} {
		var output map[string]any
		call(t, session, step.tool, step.args, &output)
		if _, wrapped := output["issue"]; wrapped || output["identifier"] != "AL-1" {
			t.Errorf("%s: want the issue at the top level, got %#v", step.tool, output)
		}
	}
}

func TestValidationOverHTTP(t *testing.T) {
	session := connect(t)
	var created app.Issue
	call(t, session, "create_issue", map[string]any{"team": "AL", "title": "Existing issue"}, &created)
	var project app.Project
	call(t, session, "create_project", map[string]any{"name": "Existing"}, &project)
	for _, tc := range []struct {
		name string
		tool string
		args map[string]any
		want string
	}{
		{"title", "create_issue", map[string]any{"team": "AL", "title": "  "}, app.ErrInvalidTitle.Error()},
		{"status", "create_issue", map[string]any{"team": "AL", "title": "Invalid status", "status": "ready"}, app.ErrInvalidStatus.Error()},
		{"type", "update_issue", map[string]any{"identifier": "AL-1", "type": "decision"}, app.ErrInvalidType.Error()},
		{"kind", "add_relation", map[string]any{"identifier": "AL-1", "kind": "duplicate", "other": "AL-2"}, app.ErrInvalidKind.Error()},
		{"self parent", "update_issue", map[string]any{"identifier": "AL-1", "parent": "AL-1"}, "CHECK constraint failed"},
		{"self relation", "add_relation", map[string]any{"identifier": "AL-1", "kind": "related", "other": "AL-1"}, "CHECK constraint failed"},
		{"duplicate project", "create_project", map[string]any{"name": "Existing"}, "UNIQUE constraint failed"},
		{"missing project", "create_issue", map[string]any{"team": "AL", "title": "New issue", "project": "Missing"}, app.ErrProjectNotFound.Error()},
		{"missing parent", "create_issue", map[string]any{"team": "AL", "title": "New issue", "parent": "AL-99"}, app.ErrNotFound.Error()},
		{"missing team", "create_issue", map[string]any{"title": "No team"}, `missing properties: [\"team\"]`},
		{"empty team", "create_issue", map[string]any{"team": "", "title": "Empty team"}, app.ErrTeamRequired.Error()},
		{"unknown team", "create_issue", map[string]any{"team": "NOPE", "title": "Unknown team"}, app.ErrTeamNotFound.Error()},
		{"unknown team filter", "list_issues", map[string]any{"team": "NOPE"}, app.ErrTeamNotFound.Error()},
		{"lowercase team key", "create_team", map[string]any{"key": "tok", "name": "Lowercase"}, app.ErrInvalidTeam.Error()},
		{"short team key", "create_team", map[string]any{"key": "T", "name": "Too short"}, app.ErrInvalidTeam.Error()},
		{"long team key", "create_team", map[string]any{"key": "TOOLONG1", "name": "Too long"}, app.ErrInvalidTeam.Error()},
		{"duplicate team", "create_team", map[string]any{"key": "AL", "name": "Again"}, app.ErrTeamExists.Error()},
		{"zero start number", "create_team", map[string]any{"key": "OPS", "name": "Ops", "start_number": 0}, app.ErrInvalidTeam.Error()},
		{"missing updated project", "update_issue", map[string]any{"identifier": "AL-1", "project": "Missing"}, app.ErrProjectNotFound.Error()},
		{"missing updated parent", "update_issue", map[string]any{"identifier": "AL-1", "parent": "AL-99"}, app.ErrNotFound.Error()},
		{"missing relation target", "add_relation", map[string]any{"identifier": "AL-1", "kind": "blocks", "other": "AL-99"}, app.ErrNotFound.Error()},
		{"missing removal target", "remove_relation", map[string]any{"identifier": "AL-1", "kind": "blocks", "other": "AL-99"}, app.ErrNotFound.Error()},
		{"missing issue", "get_issue", map[string]any{"identifier": "AL-99"}, app.ErrNotFound.Error()},
		{"missing updated issue", "update_issue", map[string]any{"identifier": "AL-99", "title": "Updated"}, app.ErrNotFound.Error()},
		{"missing comment issue", "add_comment", map[string]any{"identifier": "AL-99", "body": "Notes"}, app.ErrNotFound.Error()},
		{"missing link issue", "add_link", map[string]any{"identifier": "AL-99", "title": "Design", "url": "design"}, app.ErrNotFound.Error()},
		{"missing updated project name", "update_project", map[string]any{"name": "Missing", "archived": true}, app.ErrProjectNotFound.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: tc.args})
			if err != nil {
				t.Fatal(err)
			}
			content, err := json.Marshal(result.Content)
			if err != nil {
				t.Fatal(err)
			}
			if !result.IsError || !strings.Contains(string(content), tc.want) {
				t.Fatalf("expected %q, got %s", tc.want, content)
			}
		})
	}
}

func TestTeamsOverHTTP(t *testing.T) {
	session := connect(t)
	var team map[string]any
	call(t, session, "create_team", map[string]any{"key": "TOK", "name": "HarnessKit"}, &team)
	if team["key"] != "TOK" || team["name"] != "HarnessKit" || team["next_number"] != float64(1) {
		t.Fatalf("unexpected team: %#v", team)
	}
	var identifiers []string
	for _, key := range []string{"TOK", "AL", "TOK"} {
		var created map[string]any
		call(t, session, "create_issue", map[string]any{"team": key, "title": "Work for " + key}, &created)
		identifiers = append(identifiers, created["identifier"].(string))
	}
	if strings.Join(identifiers, " ") != "TOK-1 AL-1 TOK-2" {
		t.Fatalf("identifiers = %q", identifiers)
	}
	var teams struct {
		Teams []map[string]any `json:"teams"`
	}
	call(t, session, "list_teams", map[string]any{}, &teams)
	if len(teams.Teams) != 2 || teams.Teams[0]["key"] != "AL" || teams.Teams[0]["next_number"] != float64(2) ||
		teams.Teams[1]["key"] != "TOK" || teams.Teams[1]["next_number"] != float64(3) {
		t.Fatalf("unexpected teams: %#v", teams)
	}
	var listed struct {
		Issues []map[string]any `json:"issues"`
	}
	call(t, session, "list_issues", map[string]any{"team": "TOK"}, &listed)
	if len(listed.Issues) != 2 || listed.Issues[0]["identifier"] != "TOK-2" || listed.Issues[1]["identifier"] != "TOK-1" {
		t.Fatalf("unexpected TOK issues: %#v", listed)
	}

	call(t, session, "create_team", map[string]any{"key": "OPS", "name": "Ops", "start_number": 1227}, &team)
	if team["next_number"] != float64(1227) {
		t.Fatalf("unexpected team: %#v", team)
	}
	var created map[string]any
	call(t, session, "create_issue", map[string]any{"team": "OPS", "title": "First after import"}, &created)
	if created["identifier"] != "OPS-1227" {
		t.Fatalf("identifier = %v, want OPS-1227", created["identifier"])
	}
}

// TestIssueFieldsListEveryValue guards the status and type field descriptions
// against drifting from the value lists in package app.
func TestIssueFieldsListEveryValue(t *testing.T) {
	session := connect(t)
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, tool := range tools.Tools {
		raw, err := json.Marshal(tool.InputSchema)
		if err != nil {
			t.Fatal(err)
		}
		var schema struct {
			Properties map[string]struct {
				Description string `json:"description"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(raw, &schema); err != nil {
			t.Fatal(err)
		}
		values := map[string][]string{"status": nil, "type": app.Types}
		for _, status := range app.Statuses {
			values["status"] = append(values["status"], string(status))
		}
		for name, want := range values {
			property, ok := schema.Properties[name]
			if !ok {
				continue
			}
			for _, value := range want {
				if !strings.Contains(property.Description, value) {
					t.Errorf("%s: %s description %q misses %q", tool.Name, name, property.Description, value)
				}
			}
		}
	}
}
