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
)

func TestWorkspaceAuthorizationHTTP(t *testing.T) {
	f := newFixture(t)
	s, err := app.Open(filepath.Join(t.TempDir(), "access.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.CreateWorkspace(context.Background(), app.Workspace{Slug: "default", Name: "Private workspace"}); err != nil {
		t.Fatal(err)
	}
	h := f.auth.Handler(httpapi.New(s, http.NotFoundHandler(), mcpserver.Handler(s)))
	for _, sub := range []string{"user-1", "user-2"} {
		token := f.sign(map[string]any{"sub": sub, "client_id": "harnesskit", "type": "access", "exp": time.Now().Add(time.Hour).Unix()})
		r := httptest.NewRequest("GET", "/api/workspaces", nil)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != 200 || !strings.Contains(w.Body.String(), `"workspaces":[]`) {
			t.Errorf("unaffiliated %s sees workspaces: %d %s", sub, w.Code, w.Body)
		}
		for _, path := range []string{"/api/w/default/issues", "/w/default/mcp"} {
			r = httptest.NewRequest("GET", path, nil)
			r.Header.Set("Authorization", "Bearer "+token)
			w = httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != http.StatusForbidden {
				t.Errorf("unaffiliated %s accesses %s: %d", sub, path, w.Code)
			}
		}
	}
}

func TestWorkspaceAuthorizationRevocation(t *testing.T) {
	f := newFixture(t)
	s, err := app.Open(filepath.Join(t.TempDir(), "access.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	admin := app.WithIdentity(context.Background(), app.Identity{Subject: "user-1"})
	member := app.WithIdentity(context.Background(), app.Identity{Subject: "user-2"})
	requests := s.ForRequests(true)
	if _, err := requests.CreateWorkspace(admin, app.Workspace{Slug: "alpha", Name: "Alpha"}); err != nil {
		t.Fatal(err)
	}
	scoped, err := requests.InWorkspace(admin, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := scoped.CreateTeam(admin, app.CreateTeam{Key: "AL", Name: "Alpha"}); err != nil {
		t.Fatal(err)
	}
	token, err := scoped.ResetInvite(admin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := requests.JoinWorkspace(member, token); err != nil {
		t.Fatal(err)
	}
	h := f.auth.Handler(httpapi.New(requests, http.NotFoundHandler(), mcpserver.Handler(requests)))
	bearer := f.sign(map[string]any{"sub": "user-2", "name": "Member", "client_id": "harnesskit", "type": "access", "exp": time.Now().Add(time.Hour).Unix()})
	call := func(method, path, body string, want int) {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+bearer)
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Accept", "application/json, text/event-stream")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body)
		}
	}
	call("POST", "/api/w/alpha/issues", `{"team":"AL","title":"Member write"}`, 201)
	call("PATCH", "/api/w/alpha/members", `{"subject":"user-2","role":"admin","status":"active"}`, 403)
	call("POST", "/api/w/alpha/invite", `{}`, 403)
	call("POST", "/api/w/alpha/issues", `{"team":"AL","title":"Spoof","subject":"user-1"}`, 400)
	call("POST", "/w/alpha/mcp", `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"list_issues","arguments":{}}}`, 200)
	if err := scoped.UpdateMember(admin, app.UpdateMember{Subject: "user-2", Role: "member", Status: "suspended"}); err != nil {
		t.Fatal(err)
	}
	call("GET", "/api/w/alpha/issues", ``, 403)
	call("POST", "/api/w/alpha/issues", `{"team":"AL","title":"Denied"}`, 403)
	call("POST", "/w/alpha/mcp", `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"list_issues","arguments":{}}}`, 403)
	body, _ := json.Marshal(map[string]string{"token": token})
	call("POST", "/api/join", string(body), 403)
	// Even a request service accidentally mounted without UC middleware fails closed.
	unwrapped := httpapi.New(requests, http.NotFoundHandler(), mcpserver.Handler(requests))
	w := request(unwrapped, "GET", "/api/workspaces", nil, nil)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
}

func TestIdentityCookie(t *testing.T) {
	f := newFixture(t)
	c, state := begin(t, f, "/auth/login?return_to=%2F")
	w := request(f.handler, "GET", "/auth/callback?code=good&state="+state, c, nil)
	cookie := getCookie(t, w, sessionCookie)
	s, err := app.Open(filepath.Join(t.TempDir(), "cookie.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	h := f.auth.Handler(httpapi.New(s.ForRequests(true), http.NotFoundHandler(), http.NotFoundHandler()))
	r := httptest.NewRequest("POST", "/api/workspaces", strings.NewReader(`{"slug":"cookie","name":"Cookie"}`))
	r.AddCookie(cookie)
	r.Header.Set("Origin", "https://board.example.test")
	r.Header.Set("Sec-Fetch-Site", "same-origin")
	result := httptest.NewRecorder()
	h.ServeHTTP(result, r)
	if result.Code != 201 {
		t.Fatal(result.Code, result.Body)
	}
	w = request(h, "GET", "/api/w/cookie/members", cookie, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"subject":"user-1"`) || !strings.Contains(w.Body.String(), `"email":"member@example.test"`) {
		t.Fatal(w.Code, w.Body)
	}
}
