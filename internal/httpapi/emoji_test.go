package httpapi

import (
	"context"
	"encoding/json"
	"github.com/DengNaichen/inftask/internal/app"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEntityEmojiHTTP(t *testing.T) {
	service, def := openService(t)
	if _, err := def.CreateProject(context.Background(), "Core", "Keep outcome"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateWorkspace(context.Background(), app.Workspace{Slug: "other", Name: "Other"}); err != nil {
		t.Fatal(err)
	}
	handler := New(service, http.NotFoundHandler(), http.NotFoundHandler())
	for _, path := range []string{"/teams/AL", "/projects/Core"} {
		for _, value := range []string{"🚀", "👩🏽‍💻", ""} {
			rec := httptest.NewRecorder()
			body, _ := json.Marshal(map[string]string{"emoji": value})
			handler.ServeHTTP(rec, httptest.NewRequest("PATCH", "/api/w/default"+path, strings.NewReader(string(body))))
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: status %d: %s", path, rec.Code, rec.Body.String())
			}
			var result map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			list := httptest.NewRecorder()
			handler.ServeHTTP(list, httptest.NewRequest("GET", "/api/w/default/"+strings.Split(path, "/")[1], nil))
			var persisted map[string][]map[string]any
			if err := json.Unmarshal(list.Body.Bytes(), &persisted); err != nil {
				t.Fatal(err)
			}
			entities := persisted[strings.Split(path, "/")[1]]
			if list.Code != http.StatusOK || len(entities) != 1 || entities[0]["emoji"] != value {
				t.Fatalf("GET after PATCH: %d %s", list.Code, list.Body.String())
			}
			if result["emoji"] != value {
				t.Fatalf("emoji = %#v, want %q", result["emoji"], value)
			}
		}
		for _, tc := range []struct {
			workspace, body string
			status          int
		}{
			{"default", `{"emoji":"hello"}`, 400},
			{"default", `{"emoji":"🚀🚀"}`, 400},
			{"default", `{"emoji":"🚀","name":"Renamed"}`, 400},
			{"other", `{"emoji":"🚀"}`, 404},
		} {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest("PATCH", "/api/w/"+tc.workspace+path, strings.NewReader(tc.body)))
			if rec.Code != tc.status {
				t.Fatalf("%s %s: status %d, want %d: %s", path, tc.body, rec.Code, tc.status, rec.Body.String())
			}
		}
		rec := httptest.NewRecorder()
		req := httptest.NewRequest("PATCH", "/api/w/default"+path, strings.NewReader(`{"emoji":"🚀"}`))
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("cross-site status %d", rec.Code)
		}
	}
}
