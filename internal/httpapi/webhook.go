package httpapi

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"

	"github.com/DengNaichen/inftask/internal/app"
)

// GitHubWebhook authenticates deliveries before interpreting their body. The
// payload only schedules a read of current GitHub state; it cannot write tickets.
func GitHubWebhook(sync *app.GitHubSync, secret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, 400, map[string]string{"error": "invalid payload"})
			return
		}
		mac := hmac.New(sha256.New, []byte(secret))
		_, _ = mac.Write(body)
		expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if secret == "" || !hmac.Equal([]byte(expected), []byte(r.Header.Get("X-Hub-Signature-256"))) {
			writeJSON(w, 401, map[string]string{"error": "invalid signature"})
			return
		}
		if r.Header.Get("X-GitHub-Event") != "pull_request" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		var event struct {
			Repository struct {
				FullName string `json:"full_name"`
			} `json:"repository"`
		}
		if json.Unmarshal(body, &event) != nil || event.Repository.FullName == "" {
			writeJSON(w, 400, map[string]string{"error": "invalid payload"})
			return
		}
		if !sync.Notify(event.Repository.FullName) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	})
}
