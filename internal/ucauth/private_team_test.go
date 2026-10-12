package ucauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DengNaichen/inftask/internal/app"
	"github.com/DengNaichen/inftask/internal/httpapi"
	"github.com/DengNaichen/inftask/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestPrivateTeamAuthorization(t *testing.T) {
	f := newFixture(t)
	s, err := app.Open(filepath.Join(t.TempDir(), "private.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	root := s.ForRequests(true)
	admin := app.WithIdentity(context.Background(), app.Identity{Subject: "user-1"})
	owner := app.WithIdentity(context.Background(), app.Identity{Subject: "user-2"})
	if _, err = root.CreateWorkspace(admin, app.Workspace{Slug: "alpha", Name: "Alpha"}); err != nil {
		t.Fatal(err)
	}
	scoped, err := root.InWorkspace(admin, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	invite, err := scoped.ResetInvite(admin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = root.JoinWorkspace(owner, invite); err != nil {
		t.Fatal(err)
	}
	if _, err = scoped.CreateTeam(owner, app.CreateTeam{Key: "SEC", Name: "Secret", Private: true}); err != nil {
		t.Fatal(err)
	}
	if _, err = scoped.Create(owner, app.CreateIssue{Team: "SEC", Title: "Private task"}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(f.auth.Handler(httpapi.New(root, http.NotFoundHandler(), mcpserver.Handler(root))))
	defer server.Close()
	token := f.sign(map[string]any{"sub": "user-1", "client_id": "harnesskit", "type": "access", "exp": time.Now().Add(time.Hour).Unix()})
	client := mcp.NewClient(&mcp.Implementation{Name: "private-test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: server.URL + "/w/alpha/mcp", HTTPClient: &http.Client{Transport: bearerTransport{token}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	call := func(name string, args map[string]any, wantError bool) string {
		t.Helper()
		result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || result.IsError != wantError {
			t.Fatal(name, result, err)
		}
		data, _ := json.Marshal(result.StructuredContent)
		return string(data)
	}
	call("get_issue", map[string]any{"identifier": "SEC-1"}, true)
	if result := call("list_issues", map[string]any{}, false); strings.Contains(result, "Private task") {
		t.Fatal(result)
	}
	httpClient := &http.Client{Transport: bearerTransport{token}}
	response, err := httpClient.Get(server.URL + "/api/w/alpha/issues/SEC-1")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 404 {
		t.Fatal(response.StatusCode)
	}
	call("join_team", map[string]any{"key": "SEC"}, true)
	call("join_team", map[string]any{"key": "SEC", "confirm_private": true}, false)
	call("get_issue", map[string]any{"identifier": "SEC-1"}, false)
	response, err = httpClient.Get(server.URL + "/api/w/alpha/issues/SEC-1")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal(response.StatusCode)
	}
	call("join_team", map[string]any{"key": "SEC", "leave": true}, false)
	call("update_issue", map[string]any{"identifier": "SEC-1", "title": "Denied"}, true)
	call("get_issue", map[string]any{"identifier": "SEC-1"}, true)
}
