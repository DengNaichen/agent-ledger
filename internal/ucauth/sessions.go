package ucauth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

var errUCUnavailable = errors.New("UC temporarily unavailable")
var errSessionUnavailable = errors.New("session temporarily unavailable")

// Session encryption is purpose- and deployment-bound. The browser holds only
// a random identifier; UC credentials stay encrypted in the server data volume.
func (a *Auth) sessionCipher() cipher.AEAD {
	key := hmac.New(sha256.New, []byte(a.cfg.ClientSecret))
	key.Write([]byte("inftask web sessions v1\x00" + a.cfg.PublicURL + "\x00" + a.cfg.ClientID))
	block, _ := aes.NewCipher(key.Sum(nil))
	sealed, _ := cipher.NewGCM(block)
	return sealed
}

func (a *Auth) loadSessions() error {
	if a.cfg.SessionFile == "" {
		return nil
	}
	data, err := os.ReadFile(a.cfg.SessionFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return errors.New("cannot read web sessions")
	}
	sealed := a.sessionCipher()
	if len(data) < sealed.NonceSize() {
		return errors.New("invalid web session store")
	}
	plain, err := sealed.Open(nil, data[:sealed.NonceSize()], data[sealed.NonceSize():], nil)
	if err != nil {
		return errors.New("cannot decrypt web sessions")
	}
	if err := json.Unmarshal(plain, &a.sessions); err != nil || a.sessions == nil {
		return errors.New("invalid web session store")
	}
	a.prune()
	return nil
}

// saveSessions runs with mu held. Rename publishes the complete new snapshot;
// callers return an error rather than claiming a login/logout was persisted.
func (a *Auth) saveSessions() error {
	if a.cfg.SessionFile == "" {
		return nil
	}
	plain, err := json.Marshal(a.sessions)
	if err != nil {
		return err
	}
	sealed := a.sessionCipher()
	nonce := make([]byte, sealed.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	data := sealed.Seal(nonce, nonce, plain, nil)
	dir := filepath.Dir(a.cfg.SessionFile)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(dir, ".sessions-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err := file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), a.cfg.SessionFile); err != nil {
		return err
	}
	directory, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}

func (a *Auth) current(r *http.Request) (Identity, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return Identity{}, ErrInvalidToken
	}
	a.mu.Lock()
	s, ok := a.sessions[cookie.Value]
	a.mu.Unlock()
	if !ok || !a.now().Before(s.Expires) {
		return Identity{}, ErrInvalidToken
	}
	if a.now().Before(s.AccessExpires) {
		return s.Identity, nil
	}
	return a.refreshSession(r, cookie.Value)
}

func (a *Auth) refreshSession(r *http.Request, id string) (Identity, error) {
	// Serialize rotation without blocking requests using still-valid sessions or
	// holding the flow/session map lock during an upstream request.
	a.refreshMu.Lock()
	defer a.refreshMu.Unlock()
	a.mu.Lock()
	s, ok := a.sessions[id]
	a.mu.Unlock()
	if !ok || !a.now().Before(s.Expires) {
		return Identity{}, ErrInvalidToken
	}
	if a.now().Before(s.AccessExpires) {
		return s.Identity, nil
	}
	if s.RefreshToken == "" {
		return Identity{}, ErrInvalidToken
	}
	token, err := a.exchange(r.Context(), url.Values{"grant_type": {"refresh_token"}, "refresh_token": {s.RefreshToken}})
	if errors.Is(err, errUCUnavailable) {
		return Identity{}, errSessionUnavailable
	}
	var claims jwtClaims
	if err == nil {
		claims, err = a.verifier.verifiedClaims(token.AccessToken)
		if err == nil && claims.Subject != s.Identity.Subject {
			err = ErrInvalidToken
		}
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	// A concurrent logout must not be undone by an in-flight refresh.
	if _, ok := a.sessions[id]; !ok {
		return Identity{}, ErrInvalidToken
	}
	if err != nil {
		delete(a.sessions, id)
		if a.saveSessions() != nil {
			return Identity{}, errSessionUnavailable
		}
		return Identity{}, ErrInvalidToken
	}
	if token.RefreshToken != "" {
		s.RefreshToken = token.RefreshToken
	}
	s.AccessExpires = time.Unix(claims.ExpiresAt, 0)
	a.sessions[id] = s
	if a.saveSessions() != nil {
		return Identity{}, errSessionUnavailable
	}
	return s.Identity, nil
}
