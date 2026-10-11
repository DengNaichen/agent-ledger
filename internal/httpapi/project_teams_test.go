package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/DengNaichen/inftask/internal/app"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProjectTeamsHTTP(t *testing.T) {
	root, s := openService(t)
	ctx := context.Background()
	if _, err := s.CreateTeam(ctx, app.CreateTeam{Key: "BB", Name: "B"}); err != nil {
		t.Fatal(err)
	}
	if _, err := root.CreateWorkspace(ctx, app.Workspace{Slug: "other", Name: "Other"}); err != nil {
		t.Fatal(err)
	}
	h := New(root, http.NotFoundHandler(), http.NotFoundHandler())
	request := func(method, path, body string, status int) []byte {
		t.Helper()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/api/w/"+path, strings.NewReader(body)))
		if rec.Code != status {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	request("POST", "default/projects", `{"name":"Missing"}`, 400)
	var p app.Project
	json.Unmarshal(request("POST", "default/projects", `{"name":"Empty","teams":["AL"]}`, 201), &p)
	if p.ID == 0 || len(p.Teams) != 1 {
		t.Fatalf("project: %#v", p)
	}
	path := fmt.Sprintf("default/projects/by-id/%d", p.ID)
	request("GET", path, "", 200)
	request("GET", "default/projects/Empty", "", 200)
	request("GET", fmt.Sprintf("other/projects/by-id/%d", p.ID), "", 404)
	request("PATCH", path, `{"name":"Wrong","description":"bad"}`, 400)
	request("PATCH", path, `{"teams":[]}`, 400)
	for _, tc := range []struct {
		team  string
		count int
	}{{"AL", 1}, {"BB", 0}} {
		var out struct {
			Projects []app.Project `json:"projects"`
		}
		json.Unmarshal(request("GET", "default/projects?team="+tc.team, "", 200), &out)
		if len(out.Projects) != tc.count {
			t.Fatal(out)
		}
	}
	request("PATCH", "default/projects/Empty", `{"teams":["AL","BB"],"description":"Shared"}`, 200)
	var issue app.Issue
	json.Unmarshal(request("POST", "default/issues", fmt.Sprintf(`{"team":"BB","title":"shared","project_id":%d}`, p.ID), 201), &issue)
	if issue.ProjectID == nil || *issue.ProjectID != p.ID {
		t.Fatal(issue)
	}
	request("PATCH", path, `{"teams":["AL"]}`, 409)
	request("POST", "default/issues", fmt.Sprintf(`{"team":"AL","title":"bad","project":"","project_id":%d}`, p.ID), 400)
	request("POST", "default/issues", `{"team":"AL","title":"bad","project":"Empty","project_id":0}`, 400)
	request("PATCH", "default/issues/"+issue.Identifier, fmt.Sprintf(`{"project":"","project_id":%d}`, p.ID), 400)
	request("PATCH", path, `{"archived":true}`, 200)
	request("POST", "default/issues", fmt.Sprintf(`{"team":"AL","title":"archive","project_id":%d}`, p.ID), 409)
	request("PATCH", "default/issues/"+issue.Identifier, fmt.Sprintf(`{"title":"retained","project_id":%d}`, p.ID), 200)
	request("GET", fmt.Sprintf("default/issues?project_id=%d", p.ID), "", 200)
	request("GET", fmt.Sprintf("default/issues?project_id=%d&project=Wrong", p.ID), "", 400)
	request("PATCH", "default/issues/"+issue.Identifier, `{"project_id":0}`, 200)
	request("PATCH", path, `{"teams":["AL"]}`, 200)
	var out struct {
		Projects []app.Project `json:"projects"`
	}
	json.Unmarshal(request("GET", "default/projects?include_archived=true", "", 200), &out)
	if len(out.Projects) != 1 {
		t.Fatal(out)
	}
}
