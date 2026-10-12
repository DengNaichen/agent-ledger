package ucauth

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/DengNaichen/inftask/internal/app"
	"github.com/DengNaichen/inftask/internal/httpapi"
	"github.com/DengNaichen/inftask/internal/mcpserver"
	webui "github.com/DengNaichen/inftask/internal/web"
)

// This opt-in fixture serves only a temporary database on a random loopback port.
// The browser script uses real UC cookies produced by the fake provider.
func TestWorkspaceAuthorizationBrowser(t *testing.T) {
	destination := os.Getenv("INFTASK_ACCESS_BROWSER")
	if destination == "" {
		t.Skip("set INFTASK_ACCESS_BROWSER to a temporary fixture JSON path")
	}
	f := newFixture(t)
	service, err := app.Open(filepath.Join(t.TempDir(), "browser.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	root := service.ForRequests(true)
	admin := app.WithIdentity(context.Background(), app.Identity{Subject: "user-1", Name: "Admin"})
	if _, err = root.CreateWorkspace(admin, app.Workspace{Slug: "alpha", Name: "Access test"}); err != nil {
		t.Fatal(err)
	}
	scoped, err := root.InWorkspace(admin, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = scoped.CreateTeam(admin, app.CreateTeam{Key: "AL", Name: "InfTask"}); err != nil {
		t.Fatal(err)
	}
	h := f.auth.Handler(httpapi.New(root, webui.Handler(), mcpserver.Handler(root)))
	server := httptest.NewServer(h)
	defer server.Close()
	f.auth.cfg.PublicURL = server.URL
	cookies := map[string]string{}
	for _, user := range []string{"user-1", "user-2"} {
		f.token = f.sign(map[string]any{"sub": user, "name": user, "client_id": "harnesskit", "type": "access", "exp": time.Now().Add(time.Hour).Unix()})
		f.identity = Identity{Subject: user, Name: user, Email: user + "@example.test"}
		c, state := begin(t, f, "/auth/login?return_to=%2F")
		w := request(f.handler, "GET", "/auth/callback?code=good&state="+state, c, nil)
		if w.Code != 302 {
			t.Fatal(w.Code, w.Body)
		}
		cookies[user] = getCookie(t, w, sessionCookie).Value
	}
	data, err := json.Marshal(map[string]any{"url": server.URL, "cookie_name": sessionCookie, "cookies": cookies})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(destination, data, 0600); err != nil {
		t.Fatal(err)
	}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(10 * time.Minute)
	defer deadline.Stop()
	for {
		select {
		case <-ticker.C:
			if _, err := os.Stat(destination + ".done"); err == nil {
				return
			}
		case <-deadline.C:
			t.Fatal("timed out waiting for browser script completion")
		}
	}
}
