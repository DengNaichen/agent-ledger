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

func TestIssueAssigneeREST(t *testing.T) {
	root, _ := openService(t)
	admin := app.WithIdentity(context.Background(), app.Identity{Subject: "admin", Name: "Admin"})
	member := app.WithIdentity(context.Background(), app.Identity{Subject: "member", Name: "Member"})
	if err := root.BootstrapAccess(context.Background(), "default", "admin"); err != nil {
		t.Fatal(err)
	}
	root = root.ForRequests(true)
	s, err := root.InWorkspace(admin, "default")
	if err != nil {
		t.Fatal(err)
	}
	token, err := s.ResetInvite(admin)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = root.JoinWorkspace(member, token); err != nil {
		t.Fatal(err)
	}
	if _, err = s.CreateTeam(admin, app.CreateTeam{Key: "SEC", Name: "Secret", Private: true}); err != nil {
		t.Fatal(err)
	}
	handler := New(root, http.NotFoundHandler(), http.NotFoundHandler())
	request := func(ctx context.Context, method, path, body string, want int) []byte {
		t.Helper()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, "/api/w/default"+path, strings.NewReader(body)).WithContext(ctx))
		if rec.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	request(member, "GET", "/teams/SEC/assignees", "", 404)
	var candidates struct{ Assignees []app.Identity }
	if err = json.Unmarshal(request(member, "GET", "/teams/AL/assignees", "", 200), &candidates); err != nil || len(candidates.Assignees) != 2 {
		t.Fatal(candidates, err)
	}
	request(admin, "POST", "/issues", `{"team":"AL","title":"Assigned","assignee":"member"}`, 201)
	request(member, "PATCH", "/issues/AL-1", `{"assignee":"admin"}`, 200)
	request(member, "PATCH", "/issues/AL-1", `{"assignee":""}`, 200)
	request(member, "PATCH", "/issues/AL-1", `{"assignee":"member"}`, 200)
	for _, body := range []string{`{"assignee":7}`, `{"assignee":"missing"}`, `{"assignee":"admin","actor":"admin"}`} {
		request(member, "PATCH", "/issues/AL-1", body, 400)
	}
	request(member, "POST", "/issues", `{"team":"AL","title":"Spoof","assignee":"admin","actor_subject":"admin"}`, 400)
	request(member, "GET", "/issues?assignee=&assigned_to_me=true", "", 400)
	request(member, "GET", "/issues?assigned_to_me=bad", "", 400)
	for _, query := range []string{"?assignee=member", "?assigned_to_me=true"} {
		var list struct{ Issues []app.IssueSummary }
		if err = json.Unmarshal(request(member, "GET", "/issues"+query, "", 200), &list); err != nil || len(list.Issues) != 1 || list.Issues[0].Assignee.Subject != "member" {
			t.Fatal(list, err)
		}
	}
	var unassigned struct{ Issues []app.IssueSummary }
	json.Unmarshal(request(member, "GET", "/issues?assignee=", "", 200), &unassigned)
	if len(unassigned.Issues) != 0 {
		t.Fatal(unassigned)
	}
	var detail app.IssueDetails
	if err = json.Unmarshal(request(member, "GET", "/issues/AL-1", "", 200), &detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Assignments) != 4 || detail.Assignments[1].Actor.Subject != "member" || detail.Assignments[1].From.Subject != "member" || detail.Assignments[1].To.Subject != "admin" {
		t.Fatal(detail)
	}
	request(context.Background(), "GET", "/issues/AL-1", "", 403)
}
