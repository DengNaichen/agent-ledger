package httpapi

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/DengNaichen/inftask/internal/app"
)

func TestWorkspacesOverREST(t *testing.T) {
	service, err := app.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	handler := New(service, http.NotFoundHandler(), http.NotFoundHandler())
	request := func(method, path, body string, want int) string {
		t.Helper()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		if rec.Code != want || rec.Header().Get("Content-Type") != "application/json" {
			t.Fatalf("%s %s %s: status %d %q %s, want %d application/json", method, path, body, rec.Code, rec.Header().Get("Content-Type"), rec.Body.String(), want)
		}
		return strings.TrimSpace(rec.Body.String())
	}
	expect := func(got, want string) {
		t.Helper()
		if got != want {
			t.Fatalf("body = %s, want %s", got, want)
		}
	}

	// A new database has no workspace.
	expect(request(http.MethodGet, "/api/workspaces", "", http.StatusOK), `{"workspaces":[]}`)
	expect(request(http.MethodPost, "/api/workspaces", `{"slug":"default","name":"Default"}`, http.StatusCreated), `{"slug":"default","name":"Default"}`)
	expect(request(http.MethodPost, "/api/workspaces", `{"slug":"acme-2","name":" Acme "}`, http.StatusCreated), `{"slug":"acme-2","name":"Acme"}`)
	expect(request(http.MethodGet, "/api/workspaces", "", http.StatusOK), `{"workspaces":[{"slug":"acme-2","name":"Acme"},{"slug":"default","name":"Default"}]}`)
	expect(request(http.MethodPatch, "/api/workspaces/acme-2", `{"name":"Acme Corp"}`, http.StatusOK), `{"slug":"acme-2","name":"Acme Corp"}`)
	expect(request(http.MethodGet, "/api/workspaces", "", http.StatusOK), `{"workspaces":[{"slug":"acme-2","name":"Acme Corp"},{"slug":"default","name":"Default"}]}`)

	for _, body := range []string{
		`{"slug":"Acme","name":"Uppercase"}`, `{"slug":"a","name":"Too short"}`, `{"slug":"` + strings.Repeat("a", 33) + `","name":"Too long"}`,
		`{"slug":"1acme","name":"Starts with a digit"}`, `{"slug":"-acme","name":"Starts with a hyphen"}`, `{"slug":"acme_co","name":"Underscore"}`,
		`{"slug":"ops","name":" "}`, `{"slug":"ops"}`,
		`{"slug":"api","name":"Reserved"}`, `{"slug":"assets","name":"Reserved"}`, `{"slug":"auth","name":"Reserved"}`,
		`{"slug":"mcp","name":"Reserved"}`, `{"slug":"w","name":"Reserved"}`, `{"slug":"webhooks","name":"Reserved"}`,
		`{"slug":"ops","name":"Ops","id":3}`,
	} {
		request(http.MethodPost, "/api/workspaces", body, http.StatusBadRequest)
	}
	request(http.MethodPost, "/api/workspaces", `{"slug":"default","name":"Again"}`, http.StatusConflict)
	request(http.MethodPost, "/api/workspaces", `{"slug":"`+strings.Repeat("a", 32)+`","name":"Longest"}`, http.StatusCreated)
	// Only the name can change.
	request(http.MethodPatch, "/api/workspaces/default", `{"slug":"other"}`, http.StatusBadRequest)
	request(http.MethodPatch, "/api/workspaces/default", `{"name":" "}`, http.StatusBadRequest)
	request(http.MethodPatch, "/api/workspaces/missing", `{"name":"Missing"}`, http.StatusNotFound)

	// Every issue, Project, team, and label route is under /api/w/<slug>/ and
	// unknown slugs are not found.
	for _, route := range []struct{ method, path, body string }{
		{http.MethodGet, "/api/w/missing/issues", ""},
		{http.MethodPost, "/api/w/missing/issues", `{"team":"AL","title":"Lost"}`},
		{http.MethodGet, "/api/w/missing/issues/AL-1", ""},
		{http.MethodPatch, "/api/w/missing/issues/AL-1", `{"title":"Lost"}`},
		{http.MethodPatch, "/api/w/missing/issues/AL-1/status", `{"status":"todo"}`},
		{http.MethodPost, "/api/w/missing/issues/AL-1/comments", `{"body":"Lost"}`},
		{http.MethodGet, "/api/w/missing/projects", ""},
		{http.MethodGet, "/api/w/missing/teams", ""},
		{http.MethodPost, "/api/w/missing/teams", `{"key":"AL","name":"Lost"}`},
		{http.MethodGet, "/api/w/missing/labels", ""},
		{http.MethodPost, "/api/w/missing/labels", `{"name":"Lost","color":"#123456"}`},
		{http.MethodPatch, "/api/w/missing/labels/1", `{"name":"Lost"}`},
		{http.MethodPost, "/api/w/missing/labels/merge", `{"source_id":1,"target_id":2}`},
		{http.MethodGet, "/api/w/missing/label-groups", ""},
		{http.MethodPost, "/api/w/missing/label-groups", `{"name":"Lost"}`},
		{http.MethodPost, "/api/w/missing/issue-labels", `{"identifiers":["AL-1"]}`},
	} {
		request(route.method, route.path, route.body, http.StatusNotFound)
	}

	// Each workspace has its own teams and numbering.
	request(http.MethodPost, "/api/w/acme-2/teams", `{"key":"AL","name":"Acme"}`, http.StatusCreated)
	expect(request(http.MethodGet, "/api/w/default/teams", "", http.StatusOK), `{"teams":[]}`)
	request(http.MethodPost, "/api/w/default/issues", `{"team":"AL","title":"No team here"}`, http.StatusNotFound)
	request(http.MethodPost, "/api/w/default/teams", `{"key":"AL","name":"InfTask"}`, http.StatusCreated)
	for _, slug := range []string{"acme-2", "default"} {
		if got := request(http.MethodPost, "/api/w/"+slug+"/issues", `{"team":"AL","title":"First in `+slug+`"}`, http.StatusCreated); !strings.Contains(got, `"identifier":"AL-1"`) {
			t.Fatalf("%s: %s", slug, got)
		}
	}
	for _, slug := range []string{"acme-2", "default"} {
		if got := request(http.MethodGet, "/api/w/"+slug+"/issues/AL-1", "", http.StatusOK); !strings.Contains(got, `"title":"First in `+slug+`"`) {
			t.Fatalf("%s: %s", slug, got)
		}
		if got := request(http.MethodGet, "/api/w/"+slug+"/issues", "", http.StatusOK); strings.Count(got, `"identifier"`) != 1 {
			t.Fatalf("%s lists other workspaces' issues: %s", slug, got)
		}
	}
}
