package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DengNaichen/inftask/internal/app"
)

func TestIssuePriorityOverREST(t *testing.T) {
	service, _ := openService(t)
	handler := New(service, http.NotFoundHandler(), http.NotFoundHandler())
	request := func(method, path, body string, want int) []byte {
		t.Helper()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
		if rec.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	for _, body := range []string{
		`{"team":"AL","title":"Urgent","status":"agent_ready","priority":1}`,
		`{"team":"AL","title":"Low","status":"agent_ready","priority":4}`,
		`{"team":"AL","title":"None","status":"agent_ready"}`,
	} {
		request("POST", "/api/w/default/issues", body, 201)
	}
	for _, tc := range []struct {
		query string
		want  []int
	}{
		{"?status=agent_ready", []int{1, 4, 0}},
		{"?status=agent_ready&sort=updated", []int{0, 4, 1}},
		{"?sort=priority", []int{1, 4, 0}},
		{"?priority=0&status=agent_ready", []int{0}},
	} {
		var result struct{ Issues []app.Issue }
		if err := json.Unmarshal(request("GET", "/api/w/default/issues"+tc.query, "", 200), &result); err != nil {
			t.Fatal(err)
		}
		if len(result.Issues) != len(tc.want) {
			t.Fatalf("list: %#v", result)
		}
		for i, issue := range result.Issues {
			if issue.Priority != tc.want[i] {
				t.Fatalf("%s: %#v", tc.query, result)
			}
		}
	}
	for _, tc := range []struct {
		body string
		want int
	}{
		{`{"title":"Keep priority"}`, 1}, {`{"priority":2}`, 2}, {`{"priority":0}`, 0},
	} {
		var issue app.Issue
		if err := json.Unmarshal(request("PATCH", "/api/w/default/issues/AL-1", tc.body, 200), &issue); err != nil {
			t.Fatal(err)
		}
		if issue.Priority != tc.want {
			t.Fatalf("patch: %#v", issue)
		}
	}
	for _, body := range []string{`{"priority":5}`, `{"priority":-1}`, `{"priority":1.5}`, `{"priority":"high"}`} {
		request("PATCH", "/api/w/default/issues/AL-1", body, 400)
		request("POST", "/api/w/default/issues", `{"team":"AL","title":"Bad",`+body[1:], 400)
	}
	for _, query := range []string{"priority=high", "priority=1.5", "priority=5", "priority=", "sort=title"} {
		request("GET", "/api/w/default/issues?"+query, "", 400)
	}
	request("PATCH", "/api/w/default/issues/AL-999", `{"priority":1}`, 404)
}
