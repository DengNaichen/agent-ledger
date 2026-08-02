package app

import (
	"context"
	"path/filepath"
	"testing"
)

func TestIssueLifecycle(t *testing.T) {
	service, err := Open(filepath.Join(t.TempDir(), "test.db"))
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

	issue, err = service.Move(ctx, issue.Identifier, StatusReady)
	if err != nil {
		t.Fatal(err)
	}
	if issue.Status != StatusReady {
		t.Fatalf("status = %q, want %q", issue.Status, StatusReady)
	}

	issues, err := service.List(ctx, StatusReady)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || issues[0].Identifier != issue.Identifier {
		t.Fatalf("unexpected list: %#v", issues)
	}
}
