package httpapi

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/DengNaichen/inftask/internal/app"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// openService opens a new database with team AL, the team most tests number
// their issues in.
func openService(t *testing.T) *app.Service {
	t.Helper()
	service, err := app.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { service.Close() })
	if _, err := service.CreateTeam(context.Background(), app.CreateTeam{Key: "AL", Name: "InfTask"}); err != nil {
		t.Fatal(err)
	}
	return service
}

func TestRejectsCrossSiteBrowserRequests(t *testing.T) {
	reached := false
	mcp := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { reached = true })
	handler := New(nil, http.NotFoundHandler(), mcp)

	for _, tc := range []struct {
		site string
		want int
	}{
		{site: "cross-site", want: http.StatusForbidden},
		{site: "", want: http.StatusOK},
	} {
		reached = false
		req := httptest.NewRequest(http.MethodPost, "/mcp", strings.NewReader("{}"))
		if tc.site != "" {
			req.Header.Set("Sec-Fetch-Site", tc.site)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != tc.want || reached != (tc.want == http.StatusOK) {
			t.Fatalf("Sec-Fetch-Site %q: status %d, reached %v", tc.site, rec.Code, reached)
		}
	}
}

func TestEditIssue(t *testing.T) {
	service := openService(t)
	issue, err := service.Create(context.Background(), app.CreateIssue{Team: "AL", Title: "Original", Description: "Old context", Type: "bug"})
	if err != nil {
		t.Fatal(err)
	}
	handler := New(service, http.NotFoundHandler(), http.NotFoundHandler())
	req := httptest.NewRequest(http.MethodPatch, "/api/issues/"+issue.Identifier, strings.NewReader(`{"title":"Edited","description":"","status":"in_review"}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var updated app.Issue
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Title != "Edited" || updated.Description != "" || updated.Status != app.StatusInReview || *updated.Type != "bug" {
		t.Fatalf("unexpected edit: %#v", updated)
	}
	stored, err := service.Get(context.Background(), issue.Identifier)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Title != updated.Title || stored.Description != updated.Description || stored.Status != updated.Status {
		t.Fatalf("edit was not persisted: %#v", stored)
	}
	for _, tc := range []struct {
		identifier, body string
		want             int
	}{
		{issue.Identifier, `{"title":" "}`, http.StatusBadRequest},
		{"AL-999", `{"title":"Missing"}`, http.StatusNotFound},
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/api/issues/"+tc.identifier, strings.NewReader(tc.body)))
		if rec.Code != tc.want {
			t.Fatalf("status %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
		}
	}
}

func TestReadIssueDetails(t *testing.T) {
	service := openService(t)
	ctx := context.Background()
	if _, err := service.CreateProject(ctx, "Delivery", "Ship the board"); err != nil {
		t.Fatal(err)
	}
	parent, err := service.Create(ctx, app.CreateIssue{Team: "AL", Title: "Board spec", Type: "spec"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := service.Create(ctx, app.CreateIssue{Team: "AL", Title: "Build the board", Project: "Delivery", Parent: parent.Identifier})
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Relation(ctx, child.Identifier, "blocked_by", parent.Identifier, false); err != nil {
		t.Fatal(err)
	}
	handler := New(service, http.NotFoundHandler(), http.NotFoundHandler())
	request := func(method, path, body string, want int, output any) {
		t.Helper()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		if rec.Code != want {
			t.Fatalf("%s %s: status %d, want %d: %s", method, path, rec.Code, want, rec.Body.String())
		}
		if err := json.Unmarshal(rec.Body.Bytes(), output); err != nil {
			t.Fatal(err)
		}
	}

	var comment app.Comment
	request(http.MethodPost, "/api/issues/"+parent.Identifier+"/comments", `{"body":"Agreed scope"}`, http.StatusCreated, &comment)
	var detail app.IssueDetails
	request(http.MethodGet, "/api/issues/"+parent.Identifier, "", http.StatusOK, &detail)
	if len(detail.Comments) != 1 || detail.Comments[0] != comment || comment.Body != "Agreed scope" {
		t.Fatalf("unexpected comments: %#v", detail.Comments)
	}
	if len(detail.Children) != 1 || detail.Children[0].Identifier != child.Identifier || len(detail.Blocks) != 1 || detail.Blocks[0].Identifier != child.Identifier {
		t.Fatalf("unexpected children or relations: %#v", detail)
	}
	var listed struct {
		Issues []app.Issue `json:"issues"`
	}
	request(http.MethodGet, "/api/issues?project=Delivery", "", http.StatusOK, &listed)
	if len(listed.Issues) != 1 || listed.Issues[0].Identifier != child.Identifier {
		t.Fatalf("unexpected project list: %#v", listed)
	}
	var projects struct {
		Projects []app.Project `json:"projects"`
	}
	request(http.MethodGet, "/api/projects", "", http.StatusOK, &projects)
	if len(projects.Projects) != 1 || projects.Projects[0].Name != "Delivery" {
		t.Fatalf("unexpected projects: %#v", projects)
	}
}

func TestIssueJSONUsesSnakeCase(t *testing.T) {
	service := openService(t)
	issue, err := service.Create(context.Background(), app.CreateIssue{Team: "AL", Title: "Timestamps"})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	New(service, http.NotFoundHandler(), http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/issues/"+issue.Identifier, nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["updated_at"] == nil || body["created_at"] == nil || body["updatedAt"] != nil || body["createdAt"] != nil {
		t.Fatalf("timestamp keys are not snake_case: %s", rec.Body)
	}
}

func TestListIssuesFiltersByQueryAndType(t *testing.T) {
	service := openService(t)
	for _, input := range []app.CreateIssue{
		{Team: "AL", Title: "Crash on login", Type: "bug"},
		{Team: "AL", Title: "Search box", Description: "Find issues by keyword", Type: "feature"},
		{Team: "AL", Title: "Release notes"},
	} {
		if _, err := service.Create(context.Background(), input); err != nil {
			t.Fatal(err)
		}
	}
	handler := New(service, http.NotFoundHandler(), http.NotFoundHandler())
	for _, tc := range []struct{ query, want string }{
		{"query=keyword", "Search box"},
		{"type=bug", "Crash on login"},
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/issues?"+tc.query, nil))
		var listed struct {
			Issues []app.Issue `json:"issues"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
			t.Fatal(err)
		}
		if len(listed.Issues) != 1 || listed.Issues[0].Title != tc.want {
			t.Fatalf("%s: status %d, issues %#v", tc.query, rec.Code, listed.Issues)
		}
	}
}

func TestTeamsOverREST(t *testing.T) {
	service := openService(t)
	if _, err := service.CreateTeam(context.Background(), app.CreateTeam{Key: "TOK", Name: "HarnessKit"}); err != nil {
		t.Fatal(err)
	}
	handler := New(service, http.NotFoundHandler(), http.NotFoundHandler())
	request := func(method, path, body string, want int, output any) {
		t.Helper()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		if rec.Code != want {
			t.Fatalf("%s %s: status %d, want %d: %s", method, path, rec.Code, want, rec.Body.String())
		}
		if err := json.Unmarshal(rec.Body.Bytes(), output); err != nil {
			t.Fatal(err)
		}
	}

	var teams struct {
		Teams []app.Team `json:"teams"`
	}
	request(http.MethodGet, "/api/teams", "", http.StatusOK, &teams)
	if len(teams.Teams) != 2 || teams.Teams[0] != (app.Team{Key: "AL", Name: "InfTask", NextNumber: 1}) || teams.Teams[1].Key != "TOK" {
		t.Fatalf("unexpected teams: %#v", teams)
	}
	var failure struct {
		Error string `json:"error"`
	}
	request(http.MethodPost, "/api/issues", `{"title":"No team"}`, http.StatusBadRequest, &failure)
	if failure.Error != app.ErrTeamRequired.Error() {
		t.Fatalf("error = %q", failure.Error)
	}
	request(http.MethodPost, "/api/issues", `{"team":"NOPE","title":"Unknown team"}`, http.StatusNotFound, &failure)
	var created app.Issue
	for _, tc := range []struct{ body, want string }{
		{`{"team":"TOK","title":"Imported"}`, "TOK-1"},
		{`{"team":"AL","title":"Local"}`, "AL-1"},
	} {
		request(http.MethodPost, "/api/issues", tc.body, http.StatusCreated, &created)
		if created.Identifier != tc.want {
			t.Fatalf("created %s, want %s", created.Identifier, tc.want)
		}
	}
	var listed struct {
		Issues []app.Issue `json:"issues"`
	}
	request(http.MethodGet, "/api/issues?team=TOK", "", http.StatusOK, &listed)
	if len(listed.Issues) != 1 || listed.Issues[0].Identifier != "TOK-1" {
		t.Fatalf("unexpected TOK issues: %#v", listed)
	}
}

func TestCreateTeamOverREST(t *testing.T) {
	handler := New(openService(t), http.NotFoundHandler(), http.NotFoundHandler())
	for _, tc := range []struct {
		body  string
		want  int
		check string
	}{
		{`{"key":"TOK","name":"TokfinityFde","start_number":1227}`, http.StatusCreated, `{"key":"TOK","name":"TokfinityFde","next_number":1227}`},
		{`{"key":"OPS","name":"Ops"}`, http.StatusCreated, `{"key":"OPS","name":"Ops","next_number":1}`},
		{`{"key":"TOK","name":"Again"}`, http.StatusConflict, `{"error":"` + app.ErrTeamExists.Error() + `"}`},
		{`{"key":"tok","name":"Lowercase"}`, http.StatusBadRequest, `{"error":"` + app.ErrInvalidTeam.Error() + `"}`},
		{`{"key":"ZERO","name":"Zero","start_number":0}`, http.StatusBadRequest, `{"error":"` + app.ErrInvalidTeam.Error() + `"}`},
		{`{"key":"BAD","name":"Bad","start":5}`, http.StatusBadRequest, `{"error":"json: unknown field \"start\""}`},
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/teams", strings.NewReader(tc.body)))
		if rec.Code != tc.want || strings.TrimSpace(rec.Body.String()) != tc.check {
			t.Fatalf("POST %s: status %d %s, want %d %s", tc.body, rec.Code, rec.Body.String(), tc.want, tc.check)
		}
	}
}
