package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DengNaichen/inftask/internal/app"
)

func TestBacklogOverREST(t *testing.T) {
	handler := New(openService(t), http.NotFoundHandler(), http.NotFoundHandler())
	request := func(method, path, body string, want int, output any) {
		t.Helper()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		if rec.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
		if err := json.Unmarshal(rec.Body.Bytes(), output); err != nil {
			t.Fatal(err)
		}
	}
	var issue app.Issue
	request("POST", "/api/issues", `{"team":"AL","title":"Keep for later","status":"backlog"}`, http.StatusCreated, &issue)
	if issue.Status != "backlog" {
		t.Fatalf("created status = %q", issue.Status)
	}
	var listed struct {
		Issues []app.Issue `json:"issues"`
	}
	request("GET", "/api/issues?status=backlog", "", http.StatusOK, &listed)
	if len(listed.Issues) != 1 || listed.Issues[0].Identifier != issue.Identifier {
		t.Fatalf("backlog filter = %#v", listed)
	}
	path := "/api/issues/" + issue.Identifier + "/status"
	request("PATCH", path, `{"status":"todo"}`, http.StatusOK, &issue)
	request("PATCH", path, `{"status":"backlog"}`, http.StatusOK, &issue)
	if issue.Status != "backlog" {
		t.Fatalf("updated status = %q", issue.Status)
	}
}
