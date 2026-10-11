package ucauth

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

type refreshTransport func(*http.Request) (*http.Response, error)

func (f refreshTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMCPRefreshTemporaryFailure(t *testing.T) {
	for _, mode := range []string{"network", "upstream-503"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			f.auth.client.Transport = refreshTransport(func(r *http.Request) (*http.Response, error) {
				if mode == "network" {
					return nil, errors.New("private connection details")
				}
				return &http.Response{StatusCode: 503, Body: io.NopCloser(strings.NewReader("private upstream details")), Header: make(http.Header)}, nil
			})
			form := url.Values{"grant_type": {"refresh_token"}, "client_id": {"inftask"}, "resource": {"https://board.example.test/w/default/mcp"}, "refresh_token": {"still-valid"}}
			w := request(f.handler, "POST", "/mcp/oauth/token", nil, form)
			if w.Code != 503 || !strings.Contains(w.Body.String(), "temporarily_unavailable") {
				t.Fatalf("got %d %s; want retryable 503", w.Code, w.Body)
			}
			if strings.Contains(w.Body.String(), "private") {
				t.Fatal("upstream details leaked")
			}
		})
	}
}

func TestWebSessionSurvivesRestart(t *testing.T) {
	f := newFixture(t)
	cookie, state := begin(t, f, "/auth/login")
	w := request(f.handler, "GET", "/auth/callback?state="+state+"&code=good", cookie, nil)
	session := getCookie(t, w, sessionCookie)
	restarted, err := New(f.auth.cfg)
	if err != nil {
		t.Fatal(err)
	}
	handler := restarted.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) }))
	if got := request(handler, "GET", "/api/workspaces", session, nil).Code; got != 200 {
		t.Fatalf("after restart: %d", got)
	}
	if got := request(handler, "POST", "/auth/logout", session, nil).Code; got != 204 {
		t.Fatal(got)
	}
	restarted, err = New(f.auth.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := request(restarted.Handler(http.NotFoundHandler()), "GET", "/api/workspaces", session, nil).Code; got != 401 {
		t.Fatalf("logout not persisted: %d", got)
	}
}

func TestWebSessionRefreshesAfterAccessExpiry(t *testing.T) {
	f := newFixture(t)
	cookie, state := begin(t, f, "/auth/login")
	w := request(f.handler, "GET", "/auth/callback?state="+state+"&code=good", cookie, nil)
	session := getCookie(t, w, sessionCookie)
	now := time.Now().Add(2 * time.Hour)
	f.auth.now = func() time.Time { return now }
	f.auth.verifier.now = f.auth.now
	f.token = f.sign(map[string]any{"sub": "user-1", "client_id": "harnesskit", "type": "access", "exp": now.Add(time.Hour).Unix()})
	before := f.upstreamCalls.Load()
	if got := request(f.handler, "GET", "/api/workspaces", session, nil).Code; got != 200 {
		t.Fatal(got)
	}
	if f.upstreamCalls.Load() != before+1 {
		t.Fatal("expired access token did not trigger exactly one refresh")
	}
}

func TestExpiredBearerChallenge(t *testing.T) {
	f := newFixture(t)
	r := httptest.NewRequest("POST", "/w/default/mcp", nil)
	r.Header.Set("Authorization", "Bearer expired")
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if !strings.Contains(w.Header().Get("WWW-Authenticate"), `error="invalid_token"`) {
		t.Fatal("invalid bearer challenge lacks invalid_token")
	}
}

func TestWebSessionRotationAndFailure(t *testing.T) {
	f := newFixture(t)
	cookie, state := begin(t, f, "/auth/login")
	w := request(f.handler, "GET", "/auth/callback?state="+state+"&code=good", cookie, nil)
	browser := getCookie(t, w, sessionCookie)
	now := time.Now()
	f.auth.now = func() time.Time { return now }
	f.auth.verifier.now = f.auth.now
	wantRefresh := "rotated-refresh"
	count := 0
	status := 200
	subject := "user-1"
	f.auth.client.Transport = refreshTransport(func(r *http.Request) (*http.Response, error) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if got := r.PostForm.Get("refresh_token"); got != wantRefresh {
			t.Errorf("refresh = %q, want %q", got, wantRefresh)
		}
		count++
		token := f.sign(map[string]any{"sub": subject, "client_id": "harnesskit", "type": "access", "exp": now.Add(time.Hour).Unix()})
		next := "rotation-" + strconv.Itoa(count)
		body := `{"error":"invalid_grant"}`
		if status == 200 {
			raw, _ := json.Marshal(tokenResponse{AccessToken: token, RefreshToken: next, TokenType: "Bearer", ExpiresIn: 3600})
			body = string(raw)
			wantRefresh = next
		}
		return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
	})
	for range 2 {
		now = now.Add(2 * time.Hour)
		if got := request(f.handler, "GET", "/auth/session", browser, nil).Code; got != 200 {
			t.Fatalf("refresh: %d", got)
		}
	}
	if count != 2 {
		t.Fatalf("refresh count %d", count)
	}
	now = now.Add(2 * time.Hour)
	status = 503
	if got := request(f.handler, "GET", "/api/workspaces", browser, nil).Code; got != 503 {
		t.Fatalf("transient: %d", got)
	}
	status = 200
	if got := request(f.handler, "GET", "/api/workspaces", browser, nil).Code; got != 200 {
		t.Fatalf("recovery: %d", got)
	}
	now = now.Add(2 * time.Hour)
	status = 400
	if got := request(f.handler, "GET", "/api/workspaces", browser, nil).Code; got != 401 {
		t.Fatalf("revoked: %d", got)
	}
	restarted, err := New(f.auth.cfg)
	if err != nil {
		t.Fatal(err)
	}
	if got := request(restarted.Handler(http.NotFoundHandler()), "GET", "/api/workspaces", browser, nil).Code; got != 401 {
		t.Fatalf("revoked session restored: %d", got)
	}
}

func TestWebSessionStoreEncryptedAndBound(t *testing.T) {
	f := newFixture(t)
	cookie, state := begin(t, f, "/auth/login")
	w := request(f.handler, "GET", "/auth/callback?state="+state+"&code=good", cookie, nil)
	browser := getCookie(t, w, sessionCookie)
	data, err := os.ReadFile(f.auth.cfg.SessionFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{browser.Value, "rotated-refresh", "user-1"} {
		if strings.Contains(string(data), secret) {
			t.Fatal("session secret written in plaintext")
		}
	}
	info, err := os.Stat(f.auth.cfg.SessionFile)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("store permissions: %v %v", info, err)
	}
	cfg := f.auth.cfg
	cfg.ClientSecret = "changed-secret"
	if _, err := New(cfg); err == nil {
		t.Fatal("store accepted under wrong encryption key")
	}
	data[len(data)-1] ^= 1
	if err := os.WriteFile(f.auth.cfg.SessionFile, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(f.auth.cfg); err == nil {
		t.Fatal("tampered store accepted")
	}
}

func TestWebSessionLogoutDuringRefresh(t *testing.T) {
	f := newFixture(t)
	cookie, state := begin(t, f, "/auth/login")
	w := request(f.handler, "GET", "/auth/callback?state="+state+"&code=good", cookie, nil)
	browser := getCookie(t, w, sessionCookie)
	now := time.Now().Add(2 * time.Hour)
	f.auth.now = func() time.Time { return now }
	f.auth.verifier.now = f.auth.now
	started := make(chan struct{})
	release := make(chan struct{})
	f.auth.client.Transport = refreshTransport(func(r *http.Request) (*http.Response, error) {
		close(started)
		<-release
		token := f.sign(map[string]any{"sub": "user-1", "client_id": "harnesskit", "type": "access", "exp": now.Add(time.Hour).Unix()})
		body, _ := json.Marshal(tokenResponse{AccessToken: token, RefreshToken: "next", TokenType: "Bearer", ExpiresIn: 3600})
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
	})
	result := make(chan int, 1)
	go func() { result <- request(f.handler, "GET", "/api/workspaces", browser, nil).Code }()
	<-started
	if got := request(f.handler, "POST", "/auth/logout", browser, nil).Code; got != 204 {
		t.Fatal(got)
	}
	close(release)
	if got := <-result; got != 401 {
		t.Fatalf("refresh revived logged-out session: %d", got)
	}
}

func TestMCPReportsRemainingSignedLifetime(t *testing.T) {
	f := newFixture(t)
	now := time.Now().Truncate(time.Second)
	f.auth.verifier.now = func() time.Time { return now }
	f.token = f.sign(map[string]any{"sub": "user-1", "client_id": "harnesskit", "type": "access", "exp": now.Add(90 * time.Second).Unix()})
	form := url.Values{"grant_type": {"refresh_token"}, "client_id": {"inftask"}, "resource": {"https://board.example.test/w/default/mcp"}, "refresh_token": {"refresh"}}
	w := request(f.handler, "POST", "/mcp/oauth/token", nil, form)
	var token tokenResponse
	if err := json.Unmarshal(w.Body.Bytes(), &token); err != nil {
		t.Fatal(err)
	}
	if w.Code != 200 || token.ExpiresIn != 90 {
		t.Fatalf("status %d lifetime %d; want 90, not upstream 3600", w.Code, token.ExpiresIn)
	}
}

func TestWebSessionRejectsIdentityChangeAndHardExpiry(t *testing.T) {
	for _, mode := range []string{"subject-change", "hard-expiry"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			cookie, state := begin(t, f, "/auth/login")
			w := request(f.handler, "GET", "/auth/callback?state="+state+"&code=good", cookie, nil)
			browser := getCookie(t, w, sessionCookie)
			now := time.Now().Add(2 * time.Hour)
			if mode == "hard-expiry" {
				now = time.Now().Add(sessionTTL + time.Hour)
			}
			f.auth.now = func() time.Time { return now }
			f.auth.verifier.now = f.auth.now
			f.token = f.sign(map[string]any{"sub": "other-user", "client_id": "harnesskit", "type": "access", "exp": now.Add(time.Hour).Unix()})
			before := f.upstreamCalls.Load()
			if got := request(f.handler, "GET", "/api/workspaces", browser, nil).Code; got != 401 {
				t.Fatalf("%s: %d", mode, got)
			}
			if mode == "hard-expiry" && f.upstreamCalls.Load() != before {
				t.Fatal("expired session renewed past cap")
			}
		})
	}
}

func TestWebSessionStorageFailure(t *testing.T) {
	f := newFixture(t)
	cookie, state := begin(t, f, "/auth/login")
	// A non-directory parent makes persistence fail without relying on Unix uid.
	path := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(path, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	f.auth.cfg.SessionFile = filepath.Join(path, "sessions")
	w := request(f.handler, "GET", "/auth/callback?state="+state+"&code=good", cookie, nil)
	if w.Code != 503 {
		t.Fatalf("login claims success without storage: %d", w.Code)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie {
			t.Fatal("failed login issued cookie")
		}
	}
}
