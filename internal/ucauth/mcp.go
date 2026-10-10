package ucauth

import (
	"crypto/sha256"
	"encoding/base64"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const mcpClientID = "inftask"

type grant struct {
	Flow    flow
	Token   tokenResponse
	Expires time.Time
}

func (a *Auth) metadata(w http.ResponseWriter, _ *http.Request) {
	jsonResponse(w, 200, map[string]any{
		"issuer": a.cfg.PublicURL, "authorization_endpoint": a.cfg.PublicURL + "/mcp/oauth/authorize", "token_endpoint": a.cfg.PublicURL + "/mcp/oauth/token",
		"response_types_supported": []string{"code"}, "grant_types_supported": []string{"authorization_code", "refresh_token"}, "code_challenge_methods_supported": []string{"S256"}, "token_endpoint_auth_methods_supported": []string{"none"},
	})
}
func (a *Auth) resourceMetadata(w http.ResponseWriter, _ *http.Request) {
	jsonResponse(w, 200, map[string]any{"resource": a.cfg.PublicURL + "/mcp", "authorization_servers": []string{a.cfg.PublicURL}, "bearer_methods_supported": []string{"header"}})
}
func loopbackRedirect(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Fragment != "" || u.RawQuery != "" || u.ForceQuery || u.RawPath != "" || strings.ContainsAny(raw, "\\\r\n") {
		return false
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil {
		return false
	}
	n, err := strconv.Atoi(port)
	return err == nil && n > 0 && n <= 65535 && (host == "localhost" || host == "127.0.0.1" || host == "::1")
}
func validChallenge(s string) bool {
	b, err := base64.RawURLEncoding.Strict().DecodeString(s)
	return err == nil && len(b) == 32 && len(s) == 43
}
func validVerifier(s string) bool {
	if len(s) < 43 || len(s) > 128 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-._~", c)) {
			return false
		}
	}
	return true
}
func exact(values url.Values, keys ...string) bool {
	if len(values) != len(keys) {
		return false
	}
	for _, key := range keys {
		if len(values[key]) != 1 || values.Get(key) == "" || len(values.Get(key)) > 8192 {
			return false
		}
	}
	return true
}
func (a *Auth) authorizeMCP(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	// scope is optional; UC scopes are owned by the confidential integration.
	if len(q["scope"]) > 1 {
		fail(w, 400, "invalid_request")
		return
	}
	q.Del("scope")
	if !exact(q, "response_type", "client_id", "redirect_uri", "state", "code_challenge", "code_challenge_method", "resource") || q.Get("response_type") != "code" || q.Get("client_id") != mcpClientID || q.Get("resource") != a.cfg.PublicURL+"/mcp" || q.Get("code_challenge_method") != "S256" || !validChallenge(q.Get("code_challenge")) || !loopbackRedirect(q.Get("redirect_uri")) || len(q.Get("state")) > 1024 {
		fail(w, 400, "invalid_request")
		return
	}
	a.begin(w, r, flow{Redirect: q.Get("redirect_uri"), ClientState: q.Get("state"), Challenge: q.Get("code_challenge")})
}
func (a *Auth) completeMCP(w http.ResponseWriter, r *http.Request, f flow, token tokenResponse) {
	code := randomValue()
	a.mu.Lock()
	a.prune()
	if len(a.grants) >= pendingLimit {
		a.mu.Unlock()
		fail(w, 503, "oauth_busy")
		return
	}
	a.grants[code] = grant{Flow: f, Token: token, Expires: a.now().Add(2 * time.Minute)}
	a.mu.Unlock()
	target, _ := url.Parse(f.Redirect)
	q := target.Query()
	q.Set("code", code)
	q.Set("state", f.ClientState)
	target.RawQuery = q.Encode()
	http.Redirect(w, r, target.String(), http.StatusFound)
}
func (a *Auth) tokenMCP(w http.ResponseWriter, r *http.Request) {
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/x-www-form-urlencoded" || r.URL.RawQuery != "" || r.URL.ForceQuery || r.Header.Get("Authorization") != "" {
		fail(w, 400, "invalid_request")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		fail(w, 400, "invalid_request")
		return
	}
	f := r.PostForm
	if f.Get("client_id") != mcpClientID || f.Get("resource") != a.cfg.PublicURL+"/mcp" {
		fail(w, 400, "invalid_request")
		return
	}
	var token tokenResponse
	switch f.Get("grant_type") {
	case "authorization_code":
		if !exact(f, "grant_type", "client_id", "resource", "code", "redirect_uri", "code_verifier") || !validVerifier(f.Get("code_verifier")) {
			fail(w, 400, "invalid_request")
			return
		}
		a.mu.Lock()
		g, ok := a.grants[f.Get("code")]
		delete(a.grants, f.Get("code"))
		a.mu.Unlock()
		sum := sha256.Sum256([]byte(f.Get("code_verifier")))
		if !ok || !a.now().Before(g.Expires) || g.Flow.Redirect != f.Get("redirect_uri") || g.Flow.Challenge != base64.RawURLEncoding.EncodeToString(sum[:]) {
			fail(w, 400, "invalid_grant")
			return
		}
		token = g.Token
	case "refresh_token":
		if !exact(f, "grant_type", "client_id", "resource", "refresh_token") {
			fail(w, 400, "invalid_request")
			return
		}
		token, err = a.exchange(r.Context(), url.Values{"grant_type": {"refresh_token"}, "refresh_token": {f.Get("refresh_token")}})
		if err != nil {
			fail(w, 400, "invalid_grant")
			return
		}
	default:
		fail(w, 400, "unsupported_grant_type")
		return
	}
	if _, err := a.verifier.VerifyToken(token.AccessToken); err != nil {
		fail(w, 400, "invalid_grant")
		return
	}
	w.Header().Set("Pragma", "no-cache")
	jsonResponse(w, 200, token)
}
