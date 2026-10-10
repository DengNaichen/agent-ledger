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

func TestLabelAPI(t *testing.T) {
	s := openService(t)
	h := New(s, http.NotFoundHandler(), http.NotFoundHandler())
	request := func(method, path, body string, status int) []byte {
		t.Helper()
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != status {
			t.Fatalf("%s %s: %d %s", method, path, w.Code, w.Body.String())
		}
		return w.Body.Bytes()
	}
	var g app.LabelGroup
	json.Unmarshal(request("POST", "/api/label-groups", `{"name":"Risk"}`, 200), &g)
	var l app.Label
	json.Unmarshal(request("POST", "/api/labels", fmt.Sprintf(`{"name":"Low","color":"#123456","group_id":%d}`, g.ID), 201), &l)
	request("POST", "/api/labels", `{"name":"bad","color":"red"}`, 400)
	request("POST", "/api/labels", `{"name":"bad","color":"#123456","unknown":true}`, 400)
	request("POST", "/api/labels", fmt.Sprintf(`{"name":"low","color":"#123456","group_id":%d}`, g.ID), 400)
	var i app.Issue
	json.Unmarshal(request("POST", "/api/issues", `{"team":"AL","title":"A"}`, 201), &i)
	change := fmt.Sprintf(`{"identifiers":[%q],"add":[%d]}`, i.Identifier, l.ID)
	request("POST", "/api/issue-labels", change, 200)
	var detail app.IssueDetails
	json.Unmarshal(request("GET", "/api/issues/"+i.Identifier, "", 200), &detail)
	if len(detail.Labels) != 1 {
		t.Fatal(detail)
	}
	var listed struct{ Issues []app.Issue }
	json.Unmarshal(request("GET", fmt.Sprintf("/api/issues?label_id=%d&label_mode=all", l.ID), "", 200), &listed)
	if len(listed.Issues) != 1 || len(listed.Issues[0].Labels) != 1 {
		t.Fatal(listed)
	}
	request("GET", "/api/issues?label_id=bad", "", 400)
	request("GET", "/api/issues?label_mode=bad", "", 400)
	request("GET", "/api/issues?label_group_id=999", "", 404)
	request("PATCH", fmt.Sprintf("/api/labels/%d", l.ID), `{"archived":true}`, 200)
	request("GET", "/api/labels?include_archived=true", "", 200)
	request("GET", "/api/label-groups", "", 200)
	request("PATCH", "/api/labels/999", `{"name":"missing"}`, 404)
	request("POST", "/api/issue-labels", `{"identifiers":[]}`, 400)
	r := httptest.NewRequest("POST", "/api/issue-labels", strings.NewReader(change))
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
}
