package httpapi

import (
	"encoding/json"
	"fmt"
	"github.com/DengNaichen/inftask/internal/app"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCycleHTTP(t *testing.T) {
	root, _ := openService(t)
	handler := New(root, http.NotFoundHandler(), http.NotFoundHandler())
	request := func(method, path, body string, want int) []byte {
		t.Helper()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(method, "/api/w/default"+path, strings.NewReader(body)))
		if rec.Code != want {
			t.Fatalf("%s %s: %d %s", method, path, rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	config := fmt.Sprintf(`{"enabled":true,"timezone":"UTC","start_date":%q,"duration_weeks":2,"upcoming_count":2}`, time.Now().UTC().Format(time.DateOnly))
	var page app.CyclePage
	if err := json.Unmarshal(request("PUT", "/teams/AL/cycles", config, 200), &page); err != nil {
		t.Fatal(err)
	}
	id := page.Cycles[0].ID
	request("PUT", "/teams/AL/cycles", `{"enabled":true,"actor":"forged"}`, 400)
	request("PUT", "/teams/AL/cycles", `{"enabled":true,"timezone":"invalid"}`, 400)
	var issue app.Issue
	if err := json.Unmarshal(request("POST", "/issues", fmt.Sprintf(`{"team":"AL","title":"Cycle issue","cycle_id":%d}`, id), 201), &issue); err != nil {
		t.Fatal(err)
	}
	if issue.CycleID == nil || *issue.CycleID != id {
		t.Fatal(issue)
	}
	var listed struct {
		Issues []app.IssueSummary `json:"issues"`
	}
	if err := json.Unmarshal(request("GET", fmt.Sprintf("/issues?cycle_id=%d", id), "", 200), &listed); err != nil || len(listed.Issues) != 1 {
		t.Fatal(listed, err)
	}
	request("PATCH", fmt.Sprintf("/cycles/%d", id), `{"name":"Focus","description":"Deliver"}`, 200)
	request("GET", fmt.Sprintf("/cycles/%d", id), "", 200)
	request("GET", "/issues?cycle_id=bad", "", 400)
	request("GET", "/issues?cycle_id=-1", "", 400)
	request("GET", "/cycles/999999", "", 404)
	request("PATCH", "/issues/"+issue.Identifier, `{"cycle_id":0}`, 200)
	request("PATCH", "/issues/"+issue.Identifier, `{"cycle_id":999999}`, 404)
}
