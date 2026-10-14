package httpapi

import (
	"encoding/json"
	"fmt"
	"github.com/DengNaichen/inftask/internal/app"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInitiativeHTTP(t *testing.T) {
	root, _ := openService(t)
	h := New(root, http.NotFoundHandler(), http.NotFoundHandler())
	request := func(method, path, body string, status int) []byte {
		t.Helper()
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(method, "/api/w/default"+path, strings.NewReader(body)))
		if rec.Code != status {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	var p app.Project
	json.Unmarshal(request("POST", "/projects", `{"name":"Scheduled","teams":["AL"]}`, 201), &p)
	path := fmt.Sprintf("/projects/by-id/%d", p.ID)
	json.Unmarshal(request("PATCH", path, `{"start_date":"2026-10-01","target_date":"2026-10-31"}`, 200), &p)
	if p.StartDate == nil || *p.StartDate != "2026-10-01" {
		t.Fatal(p)
	}
	request("PATCH", path, `{"start_date":"2027-01-01"}`, 400)
	request("PATCH", path, `{"start_date":"2026-02-30"}`, 400)
	request("PATCH", path, `{"unknown":true}`, 400)
	request("PATCH", path, `{"start_date":""}`, 200)
	var i app.Initiative
	json.Unmarshal(request("POST", "/initiatives", fmt.Sprintf(`{"name":"Objective","project_ids":[%d]}`, p.ID), 201), &i)
	if len(i.ProjectIDs) != 1 {
		t.Fatal(i)
	}
	path = fmt.Sprintf("/initiatives/%d", i.ID)
	request("POST", "/initiatives", `{"name":"Bad","actor":"forged"}`, 400)
	request("PATCH", path, `{"project_ids":[99999],"name":"Must rollback"}`, 404)
	json.Unmarshal(request("GET", path, "", 200), &i)
	if i.Name != "Objective" {
		t.Fatal(i)
	}
	request("PATCH", path, `{"archived":true}`, 200)
	var items struct {
		Initiatives []app.Initiative `json:"initiatives"`
	}
	json.Unmarshal(request("GET", "/initiatives", "", 200), &items)
	if len(items.Initiatives) != 0 {
		t.Fatal(items)
	}
	request("PATCH", path, `{"project_ids":[]}`, 200)
	request("PATCH", path, fmt.Sprintf(`{"project_ids":[%d]}`, p.ID), 409)
	request("PATCH", path, fmt.Sprintf(`{"archived":false,"project_ids":[%d]}`, p.ID), 200)
	request("GET", "/initiatives/invalid", "", 404)
}
