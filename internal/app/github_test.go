package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"
)

// fakeGitHub serves each repository's pull requests in the order given and
// records the list queries and posted comments. Unknown repositories fail.
type fakeGitHub struct {
	pulls    map[string][]map[string]any
	reads    []string
	comments []string
}

func newTestSync(t *testing.T, fake *fakeGitHub, repos ...string) (*Service, *GitHubSync) {
	t.Helper()
	service, err := Open(filepath.Join(t.TempDir(), "test.db"), "AL")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { service.Close() })
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/{owner}/{name}/pulls", func(w http.ResponseWriter, r *http.Request) {
		repo := r.PathValue("owner") + "/" + r.PathValue("name")
		fake.reads = append(fake.reads, repo+"?"+r.URL.RawQuery)
		pulls, ok := fake.pulls[repo]
		if !ok {
			http.Error(w, "unavailable", http.StatusInternalServerError)
			return
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		json.NewEncoder(w).Encode(pulls[min((page-1)*100, len(pulls)):min(page*100, len(pulls))])
	})
	mux.HandleFunc("POST /repos/{owner}/{name}/issues/{number}/comments", func(w http.ResponseWriter, r *http.Request) {
		var comment struct {
			Body string `json:"body"`
		}
		json.NewDecoder(r.Body).Decode(&comment)
		fake.comments = append(fake.comments, r.PathValue("owner")+"/"+r.PathValue("name")+"#"+r.PathValue("number")+" "+comment.Body)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{}`))
	})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			http.Error(w, "bad credentials", http.StatusUnauthorized)
			return
		}
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	sync := NewGitHubSync(service, repos, "test-token", true)
	sync.baseURL = server.URL
	return service, sync
}

// pull returns a pull request shaped like GitHub's response. Every body names
// AL-1, which must not link anything.
func pull(number int, branch, title, state, updatedAt string) map[string]any {
	response := map[string]any{
		"number": number, "title": title, "html_url": fmt.Sprintf("https://github.com/owner/app/pull/%d", number),
		"body": "Follows AL-1", "state": "open", "draft": state == "draft", "merged_at": nil,
		"updated_at": updatedAt, "head": map[string]any{"ref": branch},
	}
	switch state {
	case "merged":
		response["state"], response["merged_at"] = "closed", updatedAt
	case "closed":
		response["state"] = "closed"
	}
	return response
}

func TestGitHubSyncLinksAndAdvancesIssues(t *testing.T) {
	// Every pull request shares updated_at, so the second sync reads them all again.
	const updated = "2026-10-01T12:00:00Z"
	fake := &fakeGitHub{pulls: map[string][]map[string]any{"owner/app": {
		pull(11, "al-1-build-a", "AL-1 Build A", "open", updated),
		pull(12, "build-b", "al-2 Build B", "merged", updated),
		pull(13, "al-3-build-c", "Build C", "draft", updated),
		pull(14, "AL-4-build-d", "Build D", "merged", updated),
		pull(15, "chore", "Bump dependencies", "open", updated),
	}}}
	service, sync := newTestSync(t, fake, "owner/app")
	ctx := context.Background()
	for _, issue := range []CreateIssue{
		{Title: "A", Description: "Context for A", Status: new(StatusInProgress)},
		{Title: "B", Status: new(StatusAgentReady)},
		{Title: "C", Description: "Context for C", Status: new(StatusInProgress)},
		{Title: "D", Status: new(StatusCanceled)},
	} {
		if _, err := service.Create(ctx, issue); err != nil {
			t.Fatal(err)
		}
	}

	for range 2 {
		sync.syncAll(ctx)
		for identifier, want := range map[string]Status{"AL-1": StatusInReview, "AL-2": StatusDone, "AL-3": StatusInProgress, "AL-4": StatusCanceled} {
			if issue, err := service.Get(ctx, identifier); err != nil || issue.Status != want {
				t.Fatalf("%s: status = %q, err = %v; want %q", identifier, issue.Status, err, want)
			}
		}
		wantComments := []string{
			"owner/app#11 **AL-1 · A**\n\n<details>\n\nContext for A\n\n</details>",
			"owner/app#13 **AL-3 · C**\n\n<details>\n\nContext for C\n\n</details>",
		}
		if !slices.Equal(fake.comments, wantComments) {
			t.Fatalf("comments = %q", fake.comments)
		}
		var events, stored string
		if err := service.db.QueryRowContext(ctx, `SELECT group_concat(detail, ', ') FROM (SELECT detail FROM events WHERE kind = 'status_changed' ORDER BY id)`).Scan(&events); err != nil {
			t.Fatal(err)
		}
		if events != "in_progress -> in_review via owner/app#11, agent_ready -> done via owner/app#12" {
			t.Fatalf("status events = %q", events)
		}
		if err := service.db.QueryRowContext(ctx, `SELECT group_concat(number || ' ' || issue_identifier || ' ' || state, ', ') FROM (SELECT * FROM pull_requests ORDER BY number)`).Scan(&stored); err != nil {
			t.Fatal(err)
		}
		if stored != "11 AL-1 open, 12 AL-2 merged, 13 AL-3 draft, 14 AL-4 merged" {
			t.Fatalf("pull requests = %q", stored)
		}
	}

	detail, err := service.Details(ctx, "AL-1")
	if err != nil {
		t.Fatal(err)
	}
	want := []PullRequest{{Repo: "owner/app", Number: 11, Title: "AL-1 Build A", URL: "https://github.com/owner/app/pull/11", State: "open", UpdatedAt: updated}}
	if !slices.Equal(detail.PullRequests, want) {
		t.Fatalf("pull requests = %#v", detail.PullRequests)
	}
}

func TestGitHubSyncStopsPagingAtCursor(t *testing.T) {
	pulls := func(first, last int) []map[string]any {
		var list []map[string]any
		for number := last; number >= first; number-- {
			updated := time.Date(2026, 10, 1, 0, number, 0, 0, time.UTC).Format(time.RFC3339)
			list = append(list, pull(number, fmt.Sprintf("al-%d", number), "Work", "open", updated))
		}
		return list
	}
	fake := &fakeGitHub{pulls: map[string][]map[string]any{"owner/app": pulls(1, 150)}}
	service, sync := newTestSync(t, fake, "owner/app")
	ctx := context.Background()

	sync.syncAll(ctx)
	fake.pulls["owner/app"] = append(pulls(151, 270), fake.pulls["owner/app"]...)
	sync.syncAll(ctx)

	// Without a cursor the first sync reads every page. The second stops on
	// page 2, where it reaches pull requests from before the first sync.
	page := func(n int) string {
		return fmt.Sprintf("owner/app?state=all&sort=updated&direction=desc&per_page=100&page=%d", n)
	}
	if want := []string{page(1), page(2), page(1), page(2)}; !slices.Equal(fake.reads, want) {
		t.Fatalf("reads = %q", fake.reads)
	}
	var stored int
	if err := service.db.QueryRowContext(ctx, `SELECT count(*) FROM pull_requests`).Scan(&stored); err != nil || stored != 270 {
		t.Fatalf("stored = %d, err = %v", stored, err)
	}
}

func TestGitHubSyncContinuesPastFailingRepository(t *testing.T) {
	fake := &fakeGitHub{pulls: map[string][]map[string]any{"owner/app": {
		pull(11, "al-1-build-a", "Build A", "open", "2026-10-01T12:00:00Z"),
	}}}
	service, sync := newTestSync(t, fake, "owner/broken", "owner/app")
	ctx := context.Background()
	if _, err := service.Create(ctx, CreateIssue{Title: "A", Status: new(StatusInProgress)}); err != nil {
		t.Fatal(err)
	}

	sync.syncAll(ctx)

	if len(fake.reads) != 2 {
		t.Fatalf("reads = %q", fake.reads)
	}
	if issue, err := service.Get(ctx, "AL-1"); err != nil || issue.Status != StatusInReview {
		t.Fatalf("status = %q, err = %v", issue.Status, err)
	}
}
