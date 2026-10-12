// Package ucauth connects the shared board to InfOne UC. The confidential
// client stays on the server; browsers receive only an opaque local session.
package ucauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/DengNaichen/inftask/internal/app"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const sessionCookie = "inftask_session"
const flowCookie = "inftask_oauth_flow"
const flowTTL = 10 * time.Minute
const sessionTTL = 30 * 24 * time.Hour
const pendingLimit = 256

// Config accepts the existing HarnessKit UC client credentials without baking
// either the client identity or the deployment address into the application.
type Config struct {
	BaseURL       string
	ClientID      string
	ClientSecret  string
	PublicURL     string
	SessionFile   string
	PublicKeyPath string
	Issuer        string
}

type Auth struct {
	cfg       Config
	verifier  *InfOneVerifier
	client    *http.Client
	secure    bool
	mu        sync.Mutex
	refreshMu sync.Mutex
	flows     map[string]flow
	sessions  map[string]session
	grants    map[string]grant
	now       func() time.Time
}
type flow struct {
	ReturnTo    string
	Redirect    string
	ClientState string
	Challenge   string
	Resource    string
	Expires     time.Time
}
type session struct {
	Identity      Identity
	Expires       time.Time
	AccessExpires time.Time
	RefreshToken  string
}
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in,omitempty"`
}

func New(cfg Config) (*Auth, error) {
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	cfg.PublicURL = strings.TrimRight(cfg.PublicURL, "/")
	for _, raw := range []string{cfg.BaseURL, cfg.PublicURL} {
		u, err := url.Parse(raw)
		if err != nil || u.Host == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" || u.Hostname() == "::1"))) {
			return nil, errors.New("UC and public URLs must be HTTPS origins (HTTP is allowed on loopback only)")
		}
	}
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, errors.New("UC client ID and secret are required")
	}
	verifier, err := NewInfOneVerifier(InfOneConfig{PublicKeyPath: cfg.PublicKeyPath, Issuer: cfg.Issuer, ClientID: cfg.ClientID})
	if err != nil {
		return nil, err
	}
	a := &Auth{
		cfg: cfg, verifier: verifier, secure: strings.HasPrefix(cfg.PublicURL, "https:"),
		client: &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		flows:  map[string]flow{}, sessions: map[string]session{}, grants: map[string]grant{}, now: time.Now,
	}
	if err := a.loadSessions(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *Auth) Handler(next http.Handler) http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("GET /auth/signed-out", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, `<!doctype html><html lang="en"><meta name="viewport" content="width=device-width,initial-scale=1"><title>InfTask — Signed out</title><body style="background:#17181c;color:#eee;font:16px system-ui;padding:12vw"><h1>Signed out</h1><p>Your InfTask session has ended.</p><a style="color:#b49bdd" href="/auth/login">Sign in with InfOne</a></body></html>`)
	})
	routes.HandleFunc("GET /auth/login", a.login)
	routes.HandleFunc("GET /auth/callback", a.callback)
	routes.HandleFunc("POST /auth/logout", a.logout)
	routes.HandleFunc("GET /auth/session", a.session)
	routes.HandleFunc("GET /.well-known/oauth-authorization-server", a.metadata)
	routes.HandleFunc("GET /.well-known/oauth-protected-resource/w/{workspace}/mcp", a.resourceMetadata)
	routes.HandleFunc("GET /mcp/oauth/authorize", a.authorizeMCP)
	routes.HandleFunc("POST /mcp/oauth/token", a.tokenMCP)
	routes.Handle("/", a.protect(next))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		routes.ServeHTTP(w, r)
	})
}

func (a *Auth) protect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/health" {
			next.ServeHTTP(w, r)
			return
		}
		// An explicitly supplied bearer must validate; it never falls back to cookies.
		if header := r.Header.Get("Authorization"); header != "" {
			fields := strings.Fields(header)
			if len(fields) == 2 && strings.EqualFold(fields[0], "Bearer") {
				if user, err := a.verifier.VerifyToken(fields[1]); err == nil {
					next.ServeHTTP(w, r.WithContext(app.WithIdentity(r.Context(), app.Identity{Subject: user.Subject, Name: user.Name, Email: user.Email})))
					return
				}
			}
		} else if !mcpPath.MatchString(r.URL.Path) {
			if user, err := a.current(r); err == nil {
				if unsafe(r.Method) && r.Header.Get("Origin") != a.cfg.PublicURL {
					fail(w, 403, "origin_forbidden")
					return
				}
				next.ServeHTTP(w, r.WithContext(app.WithIdentity(r.Context(), app.Identity{Subject: user.Subject, Name: user.Name, Email: user.Email})))
				return
			} else if errors.Is(err, errSessionUnavailable) {
				fail(w, 503, "temporarily_unavailable")
				return
			}
		}
		if strings.HasPrefix(r.URL.Path, "/api/") || mcpPath.MatchString(r.URL.Path) {
			if mcpPath.MatchString(r.URL.Path) {
				challenge := `Bearer resource_metadata="` + a.cfg.PublicURL + `/.well-known/oauth-protected-resource` + r.URL.Path + `"`
				if r.Header.Get("Authorization") != "" {
					challenge += `, error="invalid_token"`
				}
				w.Header().Set("WWW-Authenticate", challenge)
			}
			fail(w, 401, "unauthorized")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			fail(w, 401, "unauthorized")
			return
		}
		http.Redirect(w, r, "/auth/login?return_to="+url.QueryEscape(r.URL.RequestURI()), http.StatusFound)
	})
}

func unsafe(method string) bool { return method != "GET" && method != "HEAD" && method != "OPTIONS" }
func (a *Auth) session(w http.ResponseWriter, r *http.Request) {
	user, err := a.current(r)
	if errors.Is(err, errSessionUnavailable) {
		fail(w, 503, "temporarily_unavailable")
		return
	}
	if err != nil {
		fail(w, 401, "unauthorized")
		return
	}
	jsonResponse(w, 200, map[string]any{"enabled": true, "user": user})
}
func (a *Auth) logout(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != a.cfg.PublicURL {
		fail(w, 403, "origin_forbidden")
		return
	}
	if c, err := r.Cookie(sessionCookie); err == nil {
		a.mu.Lock()
		delete(a.sessions, c.Value)
		err := a.saveSessions()
		a.mu.Unlock()
		if err != nil {
			fail(w, 503, "temporarily_unavailable")
			return
		}
	}
	a.cookie(w, sessionCookie, "", "/", -1)
	w.WriteHeader(http.StatusNoContent)
}
func (a *Auth) cookie(w http.ResponseWriter, name, value, path string, seconds int) {
	http.SetCookie(w, &http.Cookie{Name: name, Value: value, Path: path, MaxAge: seconds, HttpOnly: true, Secure: a.secure, SameSite: http.SameSiteLaxMode})
}
func randomValue() string {
	var value [32]byte
	_, _ = rand.Read(value[:])
	return base64.RawURLEncoding.EncodeToString(value[:])
}
func safeReturn(raw string) bool {
	u, err := url.Parse(raw)
	return err == nil && u.Scheme == "" && u.Host == "" && u.User == nil && u.Fragment == "" && strings.HasPrefix(u.Path, "/") && !strings.HasPrefix(u.Path, "//") && !strings.ContainsAny(u.Path, "\\\r\n\x00")
}
func (a *Auth) login(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("return_to")
	if target == "" {
		target = "/"
	}
	if !safeReturn(target) {
		fail(w, 400, "invalid_return_to")
		return
	}
	a.begin(w, r, flow{ReturnTo: target})
}
func (a *Auth) begin(w http.ResponseWriter, r *http.Request, f flow) {
	state := randomValue()
	f.Expires = a.now().Add(flowTTL)
	a.mu.Lock()
	a.prune()
	if len(a.flows) >= pendingLimit {
		a.mu.Unlock()
		fail(w, 503, "login_busy")
		return
	}
	a.flows[state] = f
	a.mu.Unlock()
	a.cookie(w, flowCookie, state, "/auth/callback", int(flowTTL.Seconds()))
	query := url.Values{"response_type": {"code"}, "client_id": {a.cfg.ClientID}, "redirect_uri": {a.cfg.PublicURL + "/auth/callback"}, "state": {state}, "scope": {"profile email"}}
	http.Redirect(w, r, a.cfg.BaseURL+"/user_center/v1/oauth2/authorize?"+query.Encode(), http.StatusFound)
}
func (a *Auth) callback(w http.ResponseWriter, r *http.Request) {
	a.cookie(w, flowCookie, "", "/auth/callback", -1)
	state := r.URL.Query().Get("state")
	c, err := r.Cookie(flowCookie)
	if err != nil || state == "" || c.Value != state {
		fail(w, 400, "invalid_oauth_state")
		return
	}
	a.mu.Lock()
	f, ok := a.flows[state]
	delete(a.flows, state)
	a.mu.Unlock()
	if !ok || !a.now().Before(f.Expires) {
		fail(w, 400, "expired_oauth_state")
		return
	}
	if r.URL.Query().Get("error") != "" || r.URL.Query().Get("code") == "" {
		fail(w, 400, "login_denied")
		return
	}
	token, err := a.exchange(r.Context(), url.Values{"grant_type": {"authorization_code"}, "code": {r.URL.Query().Get("code")}, "redirect_uri": {a.cfg.PublicURL + "/auth/callback"}})
	if err != nil {
		fail(w, 502, "uc_login_failed")
		return
	}
	claims, err := a.verifier.verifiedClaims(token.AccessToken)
	if err != nil {
		fail(w, 401, "invalid_uc_token")
		return
	}
	profile, err := a.userinfo(r.Context(), token.AccessToken)
	if err != nil || profile.Subject != claims.Subject {
		fail(w, 502, "uc_profile_failed")
		return
	}
	if f.Redirect != "" {
		a.completeMCP(w, r, f, token)
		return
	}
	id := randomValue()
	a.mu.Lock()
	a.prune()
	if len(a.sessions) >= 4096 {
		a.mu.Unlock()
		fail(w, 503, "session_limit")
		return
	}
	if old, err := r.Cookie(sessionCookie); err == nil {
		delete(a.sessions, old.Value)
	}
	a.sessions[id] = session{Identity: profile, Expires: a.now().Add(sessionTTL), AccessExpires: time.Unix(claims.ExpiresAt, 0), RefreshToken: token.RefreshToken}
	err = a.saveSessions()
	if err != nil {
		delete(a.sessions, id)
	}
	a.mu.Unlock()
	if err != nil {
		fail(w, 503, "temporarily_unavailable")
		return
	}
	a.cookie(w, sessionCookie, id, "/", int(sessionTTL.Seconds()))
	http.Redirect(w, r, f.ReturnTo, http.StatusFound)
}

// prune runs with mu held and removes expired in-memory entries.
// Web sessions are persisted separately; flows and grants remain transient.
func (a *Auth) prune() {
	now := a.now()
	for k, v := range a.flows {
		if !now.Before(v.Expires) {
			delete(a.flows, k)
		}
	}
	for k, v := range a.sessions {
		if !now.Before(v.Expires) {
			delete(a.sessions, k)
		}
	}
	for k, v := range a.grants {
		if !now.Before(v.Expires) {
			delete(a.grants, k)
		}
	}
}
func (a *Auth) exchange(ctx context.Context, form url.Values) (tokenResponse, error) {
	form.Set("client_id", a.cfg.ClientID)
	form.Set("client_secret", a.cfg.ClientSecret)
	req, err := http.NewRequestWithContext(ctx, "POST", a.cfg.BaseURL+"/user_center/v1/oauth2/token", strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var token tokenResponse
	if err = a.requestJSON(req, &token); err != nil {
		return tokenResponse{}, err
	}
	if token.AccessToken == "" || !strings.EqualFold(token.TokenType, "Bearer") || token.ExpiresIn < 0 {
		return tokenResponse{}, errors.New("invalid UC token response")
	}
	return token, nil
}
func (a *Auth) userinfo(ctx context.Context, token string) (Identity, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", a.cfg.BaseURL+"/user_center/v1/oauth2/userinfo", nil)
	if err != nil {
		return Identity{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	var user Identity
	err = a.requestJSON(req, &user)
	if err == nil && user.Subject == "" {
		err = errors.New("empty UC subject")
	}
	return user, err
}
func (a *Auth) requestJSON(req *http.Request, out any) error {
	req.Header.Set("Accept", "application/json")
	resp, err := a.client.Do(req)
	if err != nil {
		return errUCUnavailable
	}
	defer resp.Body.Close()
	if resp.StatusCode == 429 || resp.StatusCode >= 500 {
		return errUCUnavailable
	}
	if resp.StatusCode != http.StatusOK {
		return errors.New("UC rejected request")
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil || len(data) > 1<<20 {
		return errors.New("invalid UC response")
	}
	return json.Unmarshal(data, out)
}
func jsonResponse(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
func fail(w http.ResponseWriter, status int, code string) {
	jsonResponse(w, status, map[string]string{"error": code})
}
