package app

import (
	"context"
	"path/filepath"
	"testing"
)

func TestIssueLifecycle(t *testing.T) {
	service, err := Open(filepath.Join(t.TempDir(), "test.db"), "AL")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()

	ctx := context.Background()
	issue, err := service.Create(ctx, CreateIssue{Title: "Ship the first board", Description: "One vertical slice."})
	if err != nil {
		t.Fatal(err)
	}
	if issue.Identifier != "AL-1" || issue.Status != StatusTriage {
		t.Fatalf("unexpected issue: %#v", issue)
	}

	issue, err = service.Update(ctx, UpdateIssue{Identifier: issue.Identifier, Status: new(StatusAgentReady)})
	if err != nil {
		t.Fatal(err)
	}
	if issue.Status != StatusAgentReady {
		t.Fatalf("status = %q, want %q", issue.Status, StatusAgentReady)
	}

	issues, err := service.List(ctx, ListIssues{Status: new(StatusAgentReady)})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || issues[0].Identifier != issue.Identifier {
		t.Fatalf("unexpected list: %#v", issues)
	}

	// Repeating a status and editing other fields must not add status events.
	for _, update := range []UpdateIssue{
		{Identifier: issue.Identifier, Status: new(StatusAgentReady)},
		{Identifier: issue.Identifier, Description: new("Updated context")},
	} {
		if _, err := service.Update(ctx, update); err != nil {
			t.Fatal(err)
		}
	}
	var history string
	if err := service.db.QueryRowContext(ctx, `SELECT group_concat(kind || ':' || detail, ',') FROM (SELECT kind, detail FROM events ORDER BY id)`).Scan(&history); err != nil {
		t.Fatal(err)
	}
	if history != "created:triage,status_changed:triage -> agent_ready" {
		t.Fatalf("unexpected event history: %s", history)
	}
}
