package httpapi

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/DengNaichen/agent-ledger/internal/app"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
	service, err := app.Open(filepath.Join(t.TempDir(), "test.db"), "AL")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	issue, err := service.Create(context.Background(), app.CreateIssue{Title: "Original", Description: "Old context", Type: "bug"})
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
