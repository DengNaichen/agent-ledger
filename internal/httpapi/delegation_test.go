package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/DengNaichen/inftask/internal/app"
)

func TestIssueDelegationREST(t *testing.T) {
	root, s := openService(t)
	ctx := context.Background()
	if err := root.BootstrapAccess(ctx, "default", "owner"); err != nil {
		t.Fatal(err)
	}
	issue, err := s.Create(ctx, app.CreateIssue{Team: "AL", Title: "Delegation", Assignee: "owner"})
	if err != nil {
		t.Fatal(err)
	}
	first, err := s.Delegate(ctx, app.DelegateIssue{Identifier: issue.Identifier, ClientContext: &app.ExecutionContext{Agent: "claude", SessionID: "session", AgentID: "child", AgentType: "reviewer", Hostname: "test-host"}})
	if err != nil {
		t.Fatal(err)
	}
	assignee := ""
	if _, err = s.Update(ctx, app.UpdateIssue{Identifier: issue.Identifier, Assignee: &assignee}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	New(root, http.NotFoundHandler(), http.NotFoundHandler()).ServeHTTP(rec, httptest.NewRequest("GET", "/api/w/default/issues/"+issue.Identifier, nil))
	if rec.Code != 200 {
		t.Fatal(rec.Code, rec.Body.String())
	}
	var detail app.IssueDetails
	if err = json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if len(detail.Delegations) != 1 || detail.Delegations[0].ID != first.ID || detail.Delegations[0].AgentID != "child" || detail.Status != issue.Status || detail.Assignee != nil {
		t.Fatal(detail)
	}
}
