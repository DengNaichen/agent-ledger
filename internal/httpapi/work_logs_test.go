package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DengNaichen/inftask/internal/app"
)

func TestIssueWorkLogs(t *testing.T) {
	root, s := openService(t)
	ctx := context.Background()
	issue, err := s.Create(ctx, app.CreateIssue{Team: "AL", Title: "Separate records"})
	if err != nil {
		t.Fatal(err)
	}
	handler := New(root, http.NotFoundHandler(), http.NotFoundHandler())
	base := "/api/w/default/issues/" + issue.Identifier
	get := func(path string, status int) []byte {
		t.Helper()
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != status {
			t.Fatalf("GET %s: %d %s", path, rec.Code, rec.Body.String())
		}
		return rec.Body.Bytes()
	}
	get(base+"/work-logs", 200)
	record, err := s.AddWorkLog(ctx, app.AddWorkLog{Identifier: issue.Identifier, Kind: "decision", Summary: "Summary", Body: "FULL_LOG_BODY"})
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{base, base + "/work-logs?kind=decision&limit=1"} {
		body := get(path, 200)
		if strings.Contains(string(body), record.Body) || !strings.Contains(string(body), record.Summary) {
			t.Fatal(string(body))
		}
	}
	var got app.WorkLog
	if err = json.Unmarshal(get(fmt.Sprintf("%s/work-logs/%d", base, record.ID), 200), &got); err != nil || got.Body != record.Body {
		t.Fatal(got, err)
	}
	for _, query := range []string{"limit=101", "limit=-1", "limit=no", "before=-1", "before=1&before=2", "kind=invalid", "actor=spoof"} {
		get(base+"/work-logs?"+query, 400)
	}
	get(base+"/work-logs/not-an-id", 404)
	get(base+"/work-logs/999", 404)
	other, err := s.Create(ctx, app.CreateIssue{Team: "AL", Title: "Other"})
	if err != nil {
		t.Fatal(err)
	}
	get(fmt.Sprintf("/api/w/default/issues/%s/work-logs/%d", other.Identifier, record.ID), 404)
}
