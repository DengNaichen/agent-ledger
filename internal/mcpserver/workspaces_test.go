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

func TestWorkspaceScopedMCP(t *testing.T) {
	service, err := app.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	server := httptest.NewServer(httpapi.New(service, http.NotFoundHandler(), Handler(service)))
	defer server.Close()
	for _, body := range []string{`{"slug":"default","name":"Default"}`, `{"slug":"acme","name":"Acme"}`} {
		response, err := http.Post(server.URL+"/api/workspaces", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusCreated {
			t.Fatalf("create workspace %s: status %d", body, response.StatusCode)
		}
	}
	sessions := map[string]*mcp.ClientSession{}
	for _, slug := range []string{"default", "acme"} {
		client := mcp.NewClient(&mcp.Implementation{Name: "inftask-test", Version: "0.1.0"}, nil)
		session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: server.URL + "/w/" + slug + "/mcp"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer session.Close()
		sessions[slug] = session
	}
	def, acme := sessions["default"], sessions["acme"]

	// The same team key and numbering exist in both workspaces.
	var team app.Team
	var created app.Issue
	var ignored struct{}
	for slug, session := range sessions {
		call(t, session, "create_team", map[string]any{"key": "AL", "name": "Team in " + slug}, &team)
		call(t, session, "create_issue", map[string]any{"team": "AL", "title": "First in " + slug}, &created)
		if created.Identifier != "AL-1" {
			t.Fatalf("%s: identifier = %s, want AL-1", slug, created.Identifier)
		}
	}
	call(t, acme, "create_issue", map[string]any{"team": "AL", "title": "Second in acme"}, &created)
	call(t, acme, "create_project", map[string]any{"name": "Acme only"}, &app.Project{})
	var label app.Label
	call(t, acme, "create_label", map[string]any{"name": "Acme label", "color": "#123456"}, &label)
	var group app.LabelGroup
	call(t, acme, "save_label_group", map[string]any{"name": "Acme group"}, &group)
	call(t, def, "add_comment", map[string]any{"identifier": "AL-1", "body": "Default comment"}, &ignored)

	for slug, session := range sessions {
		var detail app.IssueDetails
		call(t, session, "get_issue", map[string]any{"identifier": "AL-1"}, &detail)
		if detail.Title != "First in "+slug {
			t.Fatalf("%s: get_issue AL-1 = %q", slug, detail.Title)
		}
		if (slug == "default") != (len(detail.Comments) == 1) {
			t.Fatalf("%s: comments = %#v", slug, detail.Comments)
		}
		var teams teamsOutput
		call(t, session, "list_teams", map[string]any{}, &teams)
		if len(teams.Teams) != 1 || teams.Teams[0].Name != "Team in "+slug {
			t.Fatalf("%s: teams = %#v", slug, teams)
		}
	}
	listed := func(session *mcp.ClientSession) string {
		var output listOutput
		call(t, session, "list_issues", map[string]any{}, &output)
		var titles []string
		for _, issue := range output.Issues {
			titles = append(titles, issue.Identifier+" "+issue.Title)
		}
		return strings.Join(titles, ", ")
	}
	if got := listed(def); got != "AL-1 First in default" {
		t.Fatalf("default lists %q", got)
	}
	if got := listed(acme); got != "AL-2 Second in acme, AL-1 First in acme" {
		t.Fatalf("acme lists %q", got)
	}
	var projects projectsOutput
	call(t, def, "list_projects", map[string]any{"include_archived": true}, &projects)
	var labels labelsOutput
	call(t, def, "list_labels", map[string]any{"include_archived": true}, &labels)
	var groups groupsOutput
	call(t, def, "list_label_groups", map[string]any{}, &groups)
	if len(projects.Projects) != 0 || len(labels.Labels) != 0 || len(groups.Groups) != 0 {
		t.Fatalf("default sees acme's records: %#v %#v %#v", projects, labels, groups)
	}

	// References to another workspace's records are not found.
	for _, tc := range []struct {
		name string
		tool string
		args map[string]any
		want string
	}{
		{"issue", "get_issue", map[string]any{"identifier": "AL-2"}, app.ErrNotFound.Error()},
		{"parent", "create_issue", map[string]any{"team": "AL", "title": "Child", "parent": "AL-2"}, app.ErrNotFound.Error()},
		{"updated parent", "update_issue", map[string]any{"identifier": "AL-1", "parent": "AL-2"}, app.ErrNotFound.Error()},
		{"parent filter", "list_issues", map[string]any{"parent": "AL-2"}, app.ErrNotFound.Error()},
		{"relation", "add_relation", map[string]any{"identifier": "AL-1", "kind": "blocks", "other": "AL-2"}, app.ErrNotFound.Error()},
		{"project", "create_issue", map[string]any{"team": "AL", "title": "In Project", "project": "Acme only"}, app.ErrProjectNotFound.Error()},
		{"updated project", "update_issue", map[string]any{"identifier": "AL-1", "project": "Acme only"}, app.ErrProjectNotFound.Error()},
		{"project filter", "list_issues", map[string]any{"project": "Acme only"}, app.ErrProjectNotFound.Error()},
		{"archive project", "update_project", map[string]any{"name": "Acme only", "archived": true}, app.ErrProjectNotFound.Error()},
		{"apply label", "change_issue_labels", map[string]any{"identifiers": []string{"AL-1"}, "add": []int64{label.ID}}, app.ErrLabelNotFound.Error()},
		{"update label", "update_label", map[string]any{"id": label.ID, "name": "Taken"}, app.ErrLabelNotFound.Error()},
		{"label filter", "list_issues", map[string]any{"label_ids": []int64{label.ID}}, app.ErrLabelNotFound.Error()},
		{"group filter", "list_issues", map[string]any{"label_group_ids": []int64{group.ID}}, app.ErrLabelNotFound.Error()},
		{"rename group", "save_label_group", map[string]any{"id": group.ID, "name": "Taken"}, app.ErrLabelNotFound.Error()},
		{"label in group", "create_label", map[string]any{"name": "Grouped", "color": "#123456", "group_id": group.ID}, app.ErrInvalidLabel.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := def.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: tc.args})
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
	var detail app.IssueDetails
	call(t, acme, "get_issue", map[string]any{"identifier": "AL-2"}, &detail)
	if detail.Title != "Second in acme" || len(detail.Labels) != 0 {
		t.Fatalf("acme AL-2 changed: %#v", detail)
	}

	// Unknown workspaces are not found, and /mcp no longer serves MCP.
	for path, want := range map[string]int{"/w/missing/mcp": http.StatusNotFound, "/mcp": http.StatusMethodNotAllowed} {
		request, err := http.NewRequest(http.MethodPost, server.URL+path, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_teams","arguments":{}}}`))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Accept", "application/json, text/event-stream")
		request.Header.Set("MCP-Protocol-Version", "2025-06-18")
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != want {
			t.Fatalf("POST %s: status %d, want %d", path, response.StatusCode, want)
		}
	}
}
