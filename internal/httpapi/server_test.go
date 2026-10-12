package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"reflect"
	"slices"

	"github.com/DengNaichen/inftask/internal/app"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// openService opens a new database with workspace default and its team AL,
// the team most tests number their issues in. It returns the database's
// service, which the handler takes, and the service limited to default.
func openService(t *testing.T) (*app.Service, *app.Service) {
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
	return service, def
}

func TestRejectsCrossSiteBrowserRequests(t *testing.T) {
	reached := false
	mcp := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { reached = true })
	service, _ := openService(t)
	handler := New(service, http.NotFoundHandler(), mcp)

	for _, tc := range []struct {
		site string
		want int
	}{
		{site: "cross-site", want: http.StatusForbidden},
		{site: "", want: http.StatusOK},
	} {
		reached = false
		req := httptest.NewRequest(http.MethodPost, "/w/default/mcp", strings.NewReader("{}"))
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
	service, def := openService(t)
	issue, err := def.Create(context.Background(), app.CreateIssue{Team: "AL", Title: "Original", Description: "Old context", Type: "bug"})
	if err != nil {
		t.Fatal(err)
	}
	handler := New(service, http.NotFoundHandler(), http.NotFoundHandler())
	req := httptest.NewRequest(http.MethodPatch, "/api/w/default/issues/"+issue.Identifier, strings.NewReader(`{"title":"Edited","description":"","status":"in_review"}`))
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
	stored, err := def.Get(context.Background(), issue.Identifier)
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
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPatch, "/api/w/default/issues/"+tc.identifier, strings.NewReader(tc.body)))
		if rec.Code != tc.want {
			t.Fatalf("status %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
		}
	}
}

func TestReadIssueDetails(t *testing.T) {
	service, def := openService(t)
	ctx := context.Background()
	if _, err := def.CreateProject(ctx, "Delivery", "Ship the board", "AL"); err != nil {
		t.Fatal(err)
	}
	parent, err := def.Create(ctx, app.CreateIssue{Team: "AL", Title: "Board spec", Type: "spec"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := def.Create(ctx, app.CreateIssue{Team: "AL", Title: "Build the board", Project: new("Delivery"), Parent: parent.Identifier})
	if err != nil {
		t.Fatal(err)
	}
	if err := def.Relation(ctx, child.Identifier, "blocked_by", parent.Identifier, false); err != nil {
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
	request(http.MethodPost, "/api/w/default/issues/"+parent.Identifier+"/comments", `{"body":"Agreed scope"}`, http.StatusCreated, &comment)
	var detail app.IssueDetails
	request(http.MethodGet, "/api/w/default/issues/"+parent.Identifier, "", http.StatusOK, &detail)
	if len(detail.Comments) != 1 || detail.Comments[0] != comment || comment.Body != "Agreed scope" {
		t.Fatalf("unexpected comments: %#v", detail.Comments)
	}
	if len(detail.Children) != 1 || detail.Children[0].Identifier != child.Identifier || len(detail.Blocks) != 1 || detail.Blocks[0].Identifier != child.Identifier {
		t.Fatalf("unexpected children or relations: %#v", detail)
	}
	var listed struct {
		Issues []app.Issue `json:"issues"`
	}
	request(http.MethodGet, "/api/w/default/issues?project=Delivery", "", http.StatusOK, &listed)
	if len(listed.Issues) != 1 || listed.Issues[0].Identifier != child.Identifier {
		t.Fatalf("unexpected project list: %#v", listed)
	}
	var projects struct {
		Projects []app.Project `json:"projects"`
	}
	request(http.MethodGet, "/api/w/default/projects", "", http.StatusOK, &projects)
	if len(projects.Projects) != 1 || projects.Projects[0].Name != "Delivery" {
		t.Fatalf("unexpected projects: %#v", projects)
	}
}

func TestIssueJSONUsesSnakeCase(t *testing.T) {
	service, def := openService(t)
	issue, err := def.Create(context.Background(), app.CreateIssue{Team: "AL", Title: "Timestamps"})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	New(service, http.NotFoundHandler(), http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/w/default/issues/"+issue.Identifier, nil))
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["updated_at"] == nil || body["created_at"] == nil || body["updatedAt"] != nil || body["createdAt"] != nil {
		t.Fatalf("timestamp keys are not snake_case: %s", rec.Body)
	}
}

func TestListIssuesFiltersByQueryAndType(t *testing.T) {
	service, def := openService(t)
	for _, input := range []app.CreateIssue{
		{Team: "AL", Title: "Crash on login", Type: "bug"},
		{Team: "AL", Title: "Search box", Description: "Find issues by keyword", Type: "feature"},
		{Team: "AL", Title: "Release notes"},
	} {
		if _, err := def.Create(context.Background(), input); err != nil {
			t.Fatal(err)
		}
	}
	handler := New(service, http.NotFoundHandler(), http.NotFoundHandler())
	for _, tc := range []struct{ query, want string }{
		{"query=keyword", "Search box"},
		{"type=bug", "Crash on login"},
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/w/default/issues?"+tc.query, nil))
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

func TestListIssuesReturnsSummaries(t *testing.T) {
	service, def := openService(t)
	ctx := context.Background()
	if _, err := def.CreateProject(ctx, "Delivery", "Ship the board", "AL"); err != nil {
		t.Fatal(err)
	}
	if _, err := def.UpdateProject(ctx, app.UpdateProject{Name: "Delivery", Emoji: new("☁️")}); err != nil {
		t.Fatal(err)
	}
	parent, err := def.Create(ctx, app.CreateIssue{Team: "AL", Title: "Board spec", Description: "Long context", Type: "spec"})
	if err != nil {
		t.Fatal(err)
	}
	child, err := def.Create(ctx, app.CreateIssue{Team: "AL", Title: "Build the board", Description: "More context", Project: new("Delivery"), Parent: parent.Identifier, Priority: 2})
	if err != nil {
		t.Fatal(err)
	}
	label, err := def.CreateLabel(ctx, app.CreateLabel{Name: "Risk", Color: "#123456"})
	if err != nil {
		t.Fatal(err)
	}
	if err := def.ChangeIssueLabels(ctx, app.ChangeIssueLabels{Identifiers: []string{child.Identifier}, Add: []int64{label.ID}}); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(app.IssueSummary{})
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	want := slices.Sorted(maps.Keys(fields))

	rec := httptest.NewRecorder()
	New(service, http.NotFoundHandler(), http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/w/default/issues", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
	}
	var raw struct {
		Issues []map[string]json.RawMessage `json:"issues"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.Issues) != 2 {
		t.Fatalf("listed %d issues: %s", len(raw.Issues), rec.Body.String())
	}
	for _, issue := range raw.Issues {
		var createdAt string
		if err := json.Unmarshal(issue["created_at"], &createdAt); err != nil || createdAt == "" {
			t.Fatalf("summary has no creation date: %s", issue["created_at"])
		}
		var identifier, emoji string
		if err := json.Unmarshal(issue["identifier"], &identifier); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(issue["project_emoji"], &emoji); err != nil {
			t.Fatal(err)
		}
		if identifier == child.Identifier && (createdAt != child.CreatedAt || emoji != "☁️") {
			t.Fatalf("child metadata: created %q, emoji %q", createdAt, emoji)
		}
		if identifier == parent.Identifier && (createdAt != parent.CreatedAt || emoji != "") {
			t.Fatalf("parent metadata: created %q, emoji %q", createdAt, emoji)
		}
		if got := slices.Sorted(maps.Keys(issue)); !slices.Equal(got, want) {
			t.Fatalf("issue fields %v, want the list_issues summary fields %v", got, want)
		}
	}
	var listed struct {
		Issues []app.IssueSummary `json:"issues"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	limit := -1
	issues, err := def.List(ctx, app.ListIssues{Limit: &limit})
	if err != nil {
		t.Fatal(err)
	}
	if summaries := app.Summaries(issues); !reflect.DeepEqual(listed.Issues, summaries) {
		t.Fatalf("listed %#v, want %#v", listed.Issues, summaries)
	}
}

func TestTeamsOverREST(t *testing.T) {
	service, def := openService(t)
	if _, err := def.CreateTeam(context.Background(), app.CreateTeam{Key: "TOK", Name: "HarnessKit"}); err != nil {
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
	request(http.MethodGet, "/api/w/default/teams", "", http.StatusOK, &teams)
	if len(teams.Teams) != 2 || teams.Teams[0] != (app.Team{Key: "AL", Name: "InfTask", NextNumber: 1}) || teams.Teams[1].Key != "TOK" {
		t.Fatalf("unexpected teams: %#v", teams)
	}
	var failure struct {
		Error string `json:"error"`
	}
	request(http.MethodPost, "/api/w/default/issues", `{"title":"No team"}`, http.StatusBadRequest, &failure)
	if failure.Error != app.ErrTeamRequired.Error() {
		t.Fatalf("error = %q", failure.Error)
	}
	request(http.MethodPost, "/api/w/default/issues", `{"team":"NOPE","title":"Unknown team"}`, http.StatusNotFound, &failure)
	var created app.Issue
	for _, tc := range []struct{ body, want string }{
		{`{"team":"TOK","title":"Imported"}`, "TOK-1"},
		{`{"team":"AL","title":"Local"}`, "AL-1"},
	} {
		request(http.MethodPost, "/api/w/default/issues", tc.body, http.StatusCreated, &created)
		if created.Identifier != tc.want {
			t.Fatalf("created %s, want %s", created.Identifier, tc.want)
		}
	}
	var listed struct {
		Issues []app.Issue `json:"issues"`
	}
	request(http.MethodGet, "/api/w/default/issues?team=TOK", "", http.StatusOK, &listed)
	if len(listed.Issues) != 1 || listed.Issues[0].Identifier != "TOK-1" {
		t.Fatalf("unexpected TOK issues: %#v", listed)
	}
}

func TestCreateTeamOverREST(t *testing.T) {
	service, _ := openService(t)
	handler := New(service, http.NotFoundHandler(), http.NotFoundHandler())
	for _, tc := range []struct {
		body  string
		want  int
		check string
	}{
		{`{"key":"TOK","name":"TokfinityFde","start_number":1227}`, http.StatusCreated, `{"emoji":"","key":"TOK","name":"TokfinityFde","next_number":1227}`},
		{`{"key":"OPS","name":"Ops"}`, http.StatusCreated, `{"emoji":"","key":"OPS","name":"Ops","next_number":1}`},
		{`{"key":"TOK","name":"Again"}`, http.StatusConflict, `{"error":"` + app.ErrTeamExists.Error() + `"}`},
		{`{"key":"tok","name":"Lowercase"}`, http.StatusBadRequest, `{"error":"` + app.ErrInvalidTeam.Error() + `"}`},
		{`{"key":"ZERO","name":"Zero","start_number":0}`, http.StatusBadRequest, `{"error":"` + app.ErrInvalidTeam.Error() + `"}`},
		{`{"key":"BAD","name":"Bad","start":5}`, http.StatusBadRequest, `{"error":"json: unknown field \"start\""}`},
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/w/default/teams", strings.NewReader(tc.body)))
		if rec.Code != tc.want || strings.TrimSpace(rec.Body.String()) != tc.check {
			t.Fatalf("POST %s: status %d %s, want %d %s", tc.body, rec.Code, rec.Body.String(), tc.want, tc.check)
		}
	}
}

func TestWriteErrorUsesErrorKinds(t *testing.T) {
	for _, tc := range []struct {
		err     error
		status  int
		message string
	}{
		{app.ErrInvalidKind, http.StatusBadRequest, app.ErrInvalidKind.Error()},
		{app.ErrNotFound, http.StatusNotFound, app.ErrNotFound.Error()},
		{app.ErrTeamExists, http.StatusConflict, app.ErrTeamExists.Error()},
		{fmt.Errorf("%w: x", app.ErrInvalidLabel), http.StatusBadRequest, app.ErrInvalidLabel.Error() + ": x"},
		{errors.New("boom"), http.StatusInternalServerError, "internal server error"},
	} {
		rec := httptest.NewRecorder()
		writeError(rec, tc.err)
		var body struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		if rec.Code != tc.status || body.Error != tc.message {
			t.Errorf("%q: status %d, error %q; want %d, %q", tc.err, rec.Code, body.Error, tc.status, tc.message)
		}
	}
}
