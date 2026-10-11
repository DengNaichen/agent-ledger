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
	s, _ := openService(t)
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
	json.Unmarshal(request("POST", "/api/w/default/label-groups", `{"name":"Risk"}`, 200), &g)
	var l app.Label
	json.Unmarshal(request("POST", "/api/w/default/labels", fmt.Sprintf(`{"name":"Low","color":"#123456","group_id":%d}`, g.ID), 201), &l)
	request("POST", "/api/w/default/labels", `{"name":"bad","color":"red"}`, 400)
	request("POST", "/api/w/default/labels", `{"name":"bad","color":"#123456","unknown":true}`, 400)
	request("POST", "/api/w/default/labels", fmt.Sprintf(`{"name":"low","color":"#123456","group_id":%d}`, g.ID), 400)
	var i app.Issue
	json.Unmarshal(request("POST", "/api/w/default/issues", `{"team":"AL","title":"A"}`, 201), &i)
	change := fmt.Sprintf(`{"identifiers":[%q],"add":[%d]}`, i.Identifier, l.ID)
	request("POST", "/api/w/default/issue-labels", change, 200)
	var detail app.IssueDetails
	json.Unmarshal(request("GET", "/api/w/default/issues/"+i.Identifier, "", 200), &detail)
	if len(detail.Labels) != 1 {
		t.Fatal(detail)
	}
	var listed struct{ Issues []app.Issue }
	json.Unmarshal(request("GET", fmt.Sprintf("/api/w/default/issues?label_id=%d&label_mode=all", l.ID), "", 200), &listed)
	if len(listed.Issues) != 1 || len(listed.Issues[0].Labels) != 1 {
		t.Fatal(listed)
	}
	request("GET", "/api/w/default/issues?label_id=bad", "", 400)
	request("GET", "/api/w/default/issues?label_mode=bad", "", 400)
	request("GET", "/api/w/default/issues?label_group_id=999", "", 404)
	request("PATCH", fmt.Sprintf("/api/w/default/labels/%d", l.ID), `{"archived":true}`, 200)
	request("GET", "/api/w/default/labels?include_archived=true", "", 200)
	request("GET", "/api/w/default/label-groups", "", 200)
	request("PATCH", "/api/w/default/labels/999", `{"name":"missing"}`, 404)
	request("POST", "/api/w/default/issue-labels", `{"identifiers":[]}`, 400)
	r := httptest.NewRequest("POST", "/api/w/default/issue-labels", strings.NewReader(change))
	r.Header.Set("Sec-Fetch-Site", "cross-site")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != 403 {
		t.Fatal(w.Code)
	}
}
