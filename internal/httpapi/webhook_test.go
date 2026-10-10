package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"github.com/DengNaichen/inftask/internal/app"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestGitHubWebhook(t *testing.T) {
	service, err := app.Open(filepath.Join(t.TempDir(), "test.db"), "AL")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	sync := app.NewGitHubSync(service, []string{"team/repo"}, "", false)
	handler := GitHubWebhook(sync, "webhook-secret")
	for _, tc := range []struct {
		name, body, event, secret string
		want                      int
	}{
		{"valid", `{"repository":{"full_name":"team/repo"}}`, "pull_request", "webhook-secret", 202},
		{"duplicate", `{"repository":{"full_name":"team/repo"}}`, "pull_request", "webhook-secret", 202},
		{"foreign", `{"repository":{"full_name":"other/repo"}}`, "pull_request", "webhook-secret", 204},
		{"bad signature", `{"repository":{"full_name":"team/repo"}}`, "pull_request", "wrong", 401},
		{"ping", `{}`, "ping", "webhook-secret", 204},
		{"bad json", `{`, "pull_request", "webhook-secret", 400},
		{"missing repository", `{}`, "pull_request", "webhook-secret", 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(tc.body))
			r.Header.Set("X-GitHub-Event", tc.event)
			mac := hmac.New(sha256.New, []byte(tc.secret))
			_, _ = mac.Write([]byte(tc.body))
			r.Header.Set("X-Hub-Signature-256", "sha256="+hex.EncodeToString(mac.Sum(nil)))
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != tc.want {
				t.Fatalf("got %d want %d", w.Code, tc.want)
			}
		})
	}
	r := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(strings.Repeat("x", (1<<20)+1)))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if w.Code != 400 {
		t.Fatal("oversized body accepted")
	}
}
