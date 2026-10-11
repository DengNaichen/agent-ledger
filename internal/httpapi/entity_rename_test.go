package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEntityRenameOverHTTP(t *testing.T) {
	root, s := openService(t)
	p, err := s.CreateProject(context.Background(), "Old", "Outcome", "AL")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateProject(context.Background(), "Taken", "", "AL"); err != nil {
		t.Fatal(err)
	}
	h := New(root, http.NotFoundHandler(), http.NotFoundHandler())
	for _, tc := range []struct {
		path, body string
		status     int
		contains   string
	}{
		{"teams/AL", `{"name":" 产品 A "}`, 200, `"name":"产品 A"`},
		{"teams/AL", `{"name":" "}`, 400, "name is required"},
		{fmt.Sprintf("projects/by-id/%d", p.ID), `{"new_name":" New "}`, 200, `"name":"New"`},
		{"projects/New", `{"description":"Updated"}`, 200, "Updated"},
		{"projects/Old", `{"new_name":"Stale"}`, 404, "project not found"},
		{fmt.Sprintf("projects/by-id/%d", p.ID), `{"new_name":"Taken","description":"bad"}`, 409, "already exists"},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("PATCH", "/api/w/default/"+tc.path, strings.NewReader(tc.body)))
		if rec.Code != tc.status || !strings.Contains(rec.Body.String(), tc.contains) {
			t.Fatalf("%s: %d %s", tc.path, rec.Code, rec.Body.String())
		}
	}
}
