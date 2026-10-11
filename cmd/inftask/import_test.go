package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/DengNaichen/inftask/internal/app"
	"github.com/DengNaichen/inftask/internal/httpapi"
	"github.com/DengNaichen/inftask/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestImportLinear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "inftask.db")
	service, err := app.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := service.CreateWorkspace(ctx, app.Workspace{Slug: "default", Name: "Default"}); err != nil {
		t.Fatal(err)
	}
	def, err := service.InWorkspace(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := def.CreateTeam(ctx, app.CreateTeam{Key: "LIN", Name: "Linear", StartNumber: new(20)}); err != nil {
		t.Fatal(err)
	}
	service.Close()

	run := func(args ...string) (int, string, string) {
		var stdout, stderr bytes.Buffer
		code := importLinear(args, &stdout, &stderr)
		return code, stdout.String(), stderr.String()
	}
	args := []string{"-data", path, "-workspace", "default", "-team", "LIN", "-from", filepath.Join("..", "..", "internal", "app", "testdata", "linear")}
	if code, _, stderr := run(args[:6]...); code != 2 || !strings.Contains(stderr, "usage: inftask import-linear") {
		t.Fatalf("missing -from: exit %d, stderr %q", code, stderr)
	}
	if code, stdout, stderr := run(args...); code != 0 || stdout != "imported 8 issues, 4 comments, 3 relations, 2 links into LIN; created 5 Projects\n" {
		t.Fatalf("import: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}
	if code, stdout, stderr := run(args...); code != 1 || stdout != "" || !strings.Contains(stderr, "LIN-1, LIN-2, LIN-3, LIN-4, LIN-5, LIN-6, LIN-7, LIN-9") {
		t.Fatalf("second import: exit %d, stdout %q, stderr %q", code, stdout, stderr)
	}

	// REST and get_issue return every comment with its author; a comment
	// added locally has none.
	service, err = app.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	server := httptest.NewServer(httpapi.New(service, http.NotFoundHandler(), mcpserver.Handler(service)))
	defer server.Close()
	response, err := http.Post(server.URL+"/api/w/default/issues/LIN-1/comments", "application/json", strings.NewReader(`{"body":"Local"}`))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	want := []map[string]any{
		{"body": "First, from a person.", "created_at": "2026-01-02T09:00:00.000Z", "author": "Ada Example"},
		{"body": "A reply from a bot.", "created_at": "2026-01-03T09:00:00.000Z", "author": "Robot"},
		{"body": "Last, edited later.", "created_at": "2026-01-04T09:00:00.000Z", "author": "Bo Example"},
	}
	comments := func(issue map[string]any) []map[string]any {
		t.Helper()
		var result []map[string]any
		for _, item := range issue["comments"].([]any) {
			comment := item.(map[string]any)
			if comment["body"] == "Local" {
				if comment["author"] != "" {
					t.Fatalf("local comment = %#v", comment)
				}
				continue
			}
			result = append(result, comment)
		}
		return result
	}
	response, err = http.Get(server.URL + "/api/w/default/issues/LIN-1")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var issue map[string]any
	if err := json.NewDecoder(response.Body).Decode(&issue); err != nil {
		t.Fatal(err)
	}
	if got := comments(issue); len(issue["comments"].([]any)) != 4 || !reflect.DeepEqual(got, want) {
		t.Fatalf("REST comments = %#v", issue["comments"])
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "inftask-test", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: server.URL + "/w/default/mcp"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_issue", Arguments: map[string]any{"identifier": "LIN-1"}})
	if err != nil || result.IsError {
		t.Fatalf("get_issue: %#v, %v", result, err)
	}
	issue = result.StructuredContent.(map[string]any)
	if got := comments(issue); len(issue["comments"].([]any)) != 4 || !reflect.DeepEqual(got, want) {
		t.Fatalf("get_issue comments = %#v", issue["comments"])
	}
}
