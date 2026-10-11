package app

import (
	"context"
	"testing"
)

func TestGitHubSyncAdvancesBacklog(t *testing.T) {
	const updated = "2026-10-01T12:00:00Z"
	fake := &fakeGitHub{pulls: map[string][]map[string]any{"owner/app": {
		pull(11, "al-1-later", "Later", "open", updated),
		pull(12, "al-2-later", "Later", "draft", updated),
		pull(13, "al-3-later", "Later", "closed", updated),
		pull(14, "al-4-later", "Later", "merged", updated),
	}}}
	service, sync := newTestSync(t, fake, "owner/app")
	ctx := context.Background()
	for range 4 {
		if _, err := service.Create(ctx, CreateIssue{Team: "AL", Title: "Keep for later", Status: new(StatusBacklog)}); err != nil {
			t.Fatal(err)
		}
	}
	for range 2 {
		sync.syncAll(ctx)
		for identifier, want := range map[string]Status{"AL-1": StatusInReview, "AL-2": "backlog", "AL-3": "backlog", "AL-4": StatusDone} {
			issue, err := service.Get(ctx, identifier)
			if err != nil || issue.Status != want {
				t.Fatalf("%s: status = %q, err = %v; want %q", identifier, issue.Status, err, want)
			}
		}
		var history string
		if err := service.db.QueryRow(`SELECT group_concat(detail, ', ') FROM (SELECT detail FROM events WHERE kind = 'status_changed' ORDER BY id)`).Scan(&history); err != nil {
			t.Fatal(err)
		}
		if history != "backlog -> in_review via owner/app#11, backlog -> done via owner/app#14" {
			t.Fatalf("status events = %q", history)
		}
	}
}
