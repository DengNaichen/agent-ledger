package ucauth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"github.com/DengNaichen/inftask/internal/app"
	"github.com/DengNaichen/inftask/internal/httpapi"
	"github.com/DengNaichen/inftask/internal/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type fixture struct {
	auth          *Auth
	handler       http.Handler
	key           *ecdsa.PrivateKey
	token         string
	upstreamCalls atomic.Int64
}

func (f *fixture) sign(claims map[string]any) string {
	h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"ES256"}`))
	payload, _ := json.Marshal(claims)
	body := h + "." + base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(body))
	r, s, _ := ecdsa.Sign(rand.Reader, f.key, digest[:])
	sig := append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
	return body + "." + base64.RawURLEncoding.EncodeToString(sig)
}
func newFixture(t *testing.T) *fixture {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{key: key}
	f.token = f.sign(map[string]any{"sub": "user-1", "name": "Team member", "client_id": "harnesskit", "type": "access", "exp": time.Now().Add(time.Hour).Unix()})
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.upstreamCalls.Add(1)
		switch r.URL.Path {
		case "/user_center/v1/oauth2/token":
			_ = r.ParseForm()
			if r.Form.Get("client_secret") != "test-only-secret" || r.Form.Get("client_id") != "harnesskit" {
				http.Error(w, "no", 401)
				return
			}
			if r.Form.Get("code") == "bad" || r.Form.Get("refresh_token") == "bad" {
				http.Error(w, "DO NOT EXPOSE SECRET", 400)
				return
			}
			jsonResponse(w, 200, tokenResponse{AccessToken: f.token, RefreshToken: "rotated-refresh", TokenType: "Bearer", ExpiresIn: 3600})
		case "/user_center/v1/oauth2/userinfo":
			if r.Header.Get("Authorization") != "Bearer "+f.token {
				http.Error(w, "no", 401)
				return
			}
			jsonResponse(w, 200, Identity{Subject: "user-1", Name: "Team member", Email: "member@example.test"})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)
	pub, _ := x509.MarshalPKIXPublicKey(&key.PublicKey)
	path := filepath.Join(t.TempDir(), "infone.pub")
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pub}), 0600); err != nil {
		t.Fatal(err)
	}
	f.auth, err = New(Config{BaseURL: upstream.URL, ClientID: "harnesskit", ClientSecret: "test-only-secret", PublicURL: "https://board.example.test", PublicKeyPath: path})
	if err != nil {
		t.Fatal(err)
	}
	f.handler = f.auth.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		_, _ = w.Write([]byte("protected board"))
	}))
	return f
}
func request(h http.Handler, method, target string, cookie *http.Cookie, form url.Values) *httptest.ResponseRecorder {
	var body string
	if form != nil {
		body = form.Encode()
	}
	r := httptest.NewRequest(method, target, strings.NewReader(body))
	if cookie != nil {
		r.AddCookie(cookie)
	}
	if form != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if unsafe(method) && cookie != nil {
		r.Header.Set("Origin", "https://board.example.test")
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}
func getCookie(t *testing.T, w *httptest.ResponseRecorder, name string) *http.Cookie {
	t.Helper()
	for _, c := range w.Result().Cookies() {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("missing cookie %s", name)
	return nil
}
func begin(t *testing.T, f *fixture, target string) (*http.Cookie, string) {
	t.Helper()
	w := request(f.handler, "GET", target, nil, nil)
	if w.Code != 302 {
		t.Fatalf("begin: %d %s", w.Code, w.Body)
	}
	location, err := url.Parse(w.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(location.String(), "secret") || location.Query().Get("client_id") != "harnesskit" {
		t.Fatal("invalid upstream authorization URL")
	}
	return getCookie(t, w, flowCookie), location.Query().Get("state")
}
func TestWebLoginAndLogout(t *testing.T) {
	f := newFixture(t)
	for _, path := range []string{"/api/issues", "/mcp"} {
		if got := request(f.handler, "GET", path, nil, nil).Code; got != 401 {
			t.Fatalf("%s = %d", path, got)
		}
	}
	if got := request(f.handler, "GET", "/issue/AL-1", nil, nil).Code; got != 302 {
		t.Fatal(got)
	}
	c, state := begin(t, f, "/auth/login?return_to=%2Fissue%2FAL-1")
	if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteLaxMode {
		t.Fatal("unsafe flow cookie")
	}
	callback := "/auth/callback?code=good&state=" + url.QueryEscape(state)
	if w := request(f.handler, "GET", callback, nil, nil); w.Code != 400 {
		t.Fatal("missing state cookie accepted")
	}
	w := request(f.handler, "GET", callback, c, nil)
	if w.Code != 302 || w.Header().Get("Location") != "/issue/AL-1" {
		t.Fatalf("callback: %d %s", w.Code, w.Body)
	}
	session := getCookie(t, w, sessionCookie)
	if strings.Contains(session.Value, "user-1") || !session.HttpOnly || !session.Secure {
		t.Fatal("session must be opaque and secure")
	}
	if got := request(f.handler, "GET", callback, c, nil).Code; got != 400 {
		t.Fatal("state replay accepted")
	}
	if got := request(f.handler, "GET", "/api/issues", session, nil).Code; got != 200 {
		t.Fatal(got)
	}
	if got := request(f.handler, "GET", "/mcp", session, nil).Code; got != 401 {
		t.Fatal("MCP accepted cookie")
	}
	r := httptest.NewRequest("POST", "/api/issues", nil)
	r.AddCookie(session)
	r.Header.Set("Origin", "https://evil.test")
	rec := httptest.NewRecorder()
	f.handler.ServeHTTP(rec, r)
	if rec.Code != 403 {
		t.Fatal("cross-origin cookie write accepted")
	}
	if got := request(f.handler, "POST", "/auth/logout", session, nil).Code; got != 204 {
		t.Fatal(got)
	}
	if got := request(f.handler, "GET", "/api/issues", session, nil).Code; got != 401 {
		t.Fatal("logged-out session still valid")
	}
}
func TestOAuthFailureAndExpiry(t *testing.T) {
	for _, target := range []string{"https://evil.test", "//evil.test", "/%2f%2fevil.test", "/\\evil.test"} {
		f := newFixture(t)
		if got := request(f.handler, "GET", "/auth/login?return_to="+url.QueryEscape(target), nil, nil).Code; got != 400 {
			t.Fatalf("return %q accepted", target)
		}
	}
	for _, mode := range []string{"expired", "mismatch", "denied", "upstream"} {
		t.Run(mode, func(t *testing.T) {
			f := newFixture(t)
			c, state := begin(t, f, "/auth/login")
			code := "good"
			want := 400
			switch mode {
			case "expired":
				f.auth.now = func() time.Time { return time.Now().Add(time.Hour) }
			case "mismatch":
				state = "different"
			case "denied":
				code = ""
			case "upstream":
				code = "bad"
				want = 502
			}
			w := request(f.handler, "GET", "/auth/callback?state="+state+"&code="+code, c, nil)
			if w.Code != want || strings.Contains(w.Body.String(), "SECRET") {
				t.Fatalf("failure: %d %s", w.Code, w.Body)
			}
			for _, cookie := range w.Result().Cookies() {
				if cookie.Name == sessionCookie {
					t.Fatal("failure issued session")
				}
			}
		})
	}
}
func TestUCTokenValidation(t *testing.T) {
	f := newFixture(t)
	good := map[string]any{"sub": "user-1", "client_id": "harnesskit", "type": "access", "exp": time.Now().Add(time.Hour).Unix()}
	for _, tc := range []struct {
		key   string
		value any
	}{{"sub", ""}, {"client_id", "other"}, {"type", "refresh"}, {"exp", time.Now().Add(-time.Second).Unix()}, {"nbf", time.Now().Add(time.Hour).Unix()}} {
		claims := map[string]any{}
		for k, v := range good {
			claims[k] = v
		}
		claims[tc.key] = tc.value
		if _, err := f.auth.verifier.VerifyToken(f.sign(claims)); err == nil {
			t.Fatalf("accepted invalid %s", tc.key)
		}
	}
	if _, err := f.auth.verifier.VerifyToken(f.token + "x"); err == nil {
		t.Fatal("invalid signature accepted")
	}
	f.auth.verifier.issuer = "expected"
	if _, err := f.auth.verifier.VerifyToken(f.token); err == nil {
		t.Fatal("wrong issuer accepted")
	}
	f.auth.verifier.issuer = ""
	r := httptest.NewRequest("POST", "/mcp", nil)
	r.Header.Set("Authorization", "Bearer "+f.token)
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	if w.Code != 200 {
		t.Fatal(w.Code)
	}
}
func mcpQuery(challenge string) url.Values {
	return url.Values{"response_type": {"code"}, "client_id": {"inftask"}, "redirect_uri": {"http://127.0.0.1:54321/callback"}, "state": {"client-state"}, "code_challenge": {challenge}, "code_challenge_method": {"S256"}, "resource": {"https://board.example.test/mcp"}}
}
func TestMCPOAuthPKCEAndRefresh(t *testing.T) {
	f := newFixture(t)
	verifier := strings.Repeat("v", 43)
	sum := sha256.Sum256([]byte(verifier))
	q := mcpQuery(base64.RawURLEncoding.EncodeToString(sum[:]))
	c, state := begin(t, f, "/mcp/oauth/authorize?"+q.Encode())
	w := request(f.handler, "GET", "/auth/callback?code=good&state="+state, c, nil)
	if w.Code != 302 {
		t.Fatal(w.Code, w.Body)
	}
	callback, _ := url.Parse(w.Header().Get("Location"))
	if callback.Query().Get("state") != "client-state" {
		t.Fatal("lost MCP state")
	}
	form := url.Values{"grant_type": {"authorization_code"}, "client_id": {"inftask"}, "resource": q["resource"], "redirect_uri": q["redirect_uri"], "code": {callback.Query().Get("code")}, "code_verifier": {verifier}}
	w = request(f.handler, "POST", "/mcp/oauth/token", nil, form)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "rotated-refresh") || strings.Contains(w.Body.String(), "test-only-secret") {
		t.Fatal(w.Code, w.Body)
	}
	if got := request(f.handler, "POST", "/mcp/oauth/token", nil, form).Code; got != 400 {
		t.Fatal("code replay accepted")
	}
	refresh := url.Values{"grant_type": {"refresh_token"}, "client_id": {"inftask"}, "resource": q["resource"], "refresh_token": {"refresh"}}
	if got := request(f.handler, "POST", "/mcp/oauth/token", nil, refresh).Code; got != 200 {
		t.Fatal(got)
	}
	refresh.Set("resource", "https://other.test/mcp")
	before := f.upstreamCalls.Load()
	if got := request(f.handler, "POST", "/mcp/oauth/token", nil, refresh).Code; got != 400 || f.upstreamCalls.Load() != before {
		t.Fatal("foreign resource reached UC")
	}

}
func TestMCPRejectsBadBinding(t *testing.T) {
	for _, change := range []string{"verifier", "redirect", "resource", "expired"} {
		t.Run(change, func(t *testing.T) {
			f := newFixture(t)
			v := strings.Repeat("v", 43)
			hash := sha256.Sum256([]byte(v))
			q := mcpQuery(base64.RawURLEncoding.EncodeToString(hash[:]))
			c, state := begin(t, f, "/mcp/oauth/authorize?"+q.Encode())
			callback, _ := url.Parse(request(f.handler, "GET", "/auth/callback?state="+state+"&code=good", c, nil).Header().Get("Location"))
			form := url.Values{"grant_type": {"authorization_code"}, "client_id": {"inftask"}, "resource": q["resource"], "redirect_uri": q["redirect_uri"], "code": {callback.Query().Get("code")}, "code_verifier": {v}}
			switch change {
			case "verifier":
				form.Set("code_verifier", strings.Repeat("z", 43))
			case "redirect":
				form.Set("redirect_uri", "http://127.0.0.1:1234/callback")
			case "resource":
				form.Set("resource", "https://other.test/mcp")
			case "expired":
				f.auth.now = func() time.Time { return time.Now().Add(time.Hour) }
			}
			if got := request(f.handler, "POST", "/mcp/oauth/token", nil, form).Code; got != 400 {
				t.Fatal("bad binding accepted")
			}
		})
	}
	for _, redirect := range []string{"https://evil.test/callback", "http://evil.test:123/callback", "http://127.0.0.1:123/callback?next=evil"} {
		if loopbackRedirect(redirect) {
			t.Fatal("invalid redirect accepted")
		}
	}
}

type bearerTransport struct{ token string }

func (b bearerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.Header.Set("Authorization", "Bearer "+b.token)
	return http.DefaultTransport.RoundTrip(r)
}
func TestAuthenticatedMCPWritesSharedBoard(t *testing.T) {
	f := newFixture(t)
	service, err := app.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	if _, err := service.CreateTeam(context.Background(), app.CreateTeam{Key: "AL", Name: "InfTask"}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(f.auth.Handler(httpapi.New(service, http.NotFoundHandler(), mcpserver.Handler(service))))
	defer server.Close()
	client := mcp.NewClient(&mcp.Implementation{Name: "uc-test", Version: "1"}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{Endpoint: server.URL + "/mcp", HTTPClient: &http.Client{Transport: bearerTransport{f.token}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{Name: "create_issue", Arguments: map[string]any{"team": "AL", "title": "Shared UC issue"}})
	if err != nil || result.IsError {
		t.Fatal(err, result)
	}
	stored, err := service.Get(context.Background(), "AL-1")
	if err != nil || stored.Title != "Shared UC issue" {
		t.Fatal(err, stored)
	}
}
