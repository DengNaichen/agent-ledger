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

// newTestSync binds each repository to workspace default, which has team AL,
// and returns the service limited to default.
func newTestSync(t *testing.T, fake *fakeGitHub, repos ...string) (*Service, *GitHubSync) {
	t.Helper()
	root, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { root.Close() })
	service := inWorkspace(t, root, "default")
	alTeam(t, service)
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
	bindings := []GitHubRepo{}
	for _, repo := range repos {
		bindings = append(bindings, GitHubRepo{Workspace: "default", Repo: repo})
	}
	sync := NewGitHubSync(service, bindings, "test-token", true)
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
		{Team: "AL", Title: "A", Description: "Context for A", Status: new(StatusInProgress)},
		{Team: "AL", Title: "B", Status: new(StatusAgentReady)},
		{Team: "AL", Title: "C", Description: "Context for C", Status: new(StatusInProgress)},
		{Team: "AL", Title: "D", Status: new(StatusCanceled)},
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
	if _, err := service.Create(ctx, CreateIssue{Team: "AL", Title: "A", Status: new(StatusInProgress)}); err != nil {
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

func TestPrivateTeamGitHubLinkback(t *testing.T) {
	fake := &fakeGitHub{pulls: map[string][]map[string]any{"owner/app": {pull(11, "al-1-work", "Work", "open", "2026-10-01T12:00:00Z")}}}
	service, sync := newTestSync(t, fake, "owner/app")
	ctx := context.Background()
	if _, err := service.UpdateTeam(ctx, UpdateTeam{Key: "AL", Private: new(true)}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(ctx, CreateIssue{Team: "AL", Title: "Confidential title", Description: "Confidential body", Status: new(StatusInProgress)}); err != nil {
		t.Fatal(err)
	}
	sync.syncAll(ctx)
	if len(fake.comments) != 1 || fake.comments[0] != "owner/app#11 **AL-1**" {
		t.Fatal(fake.comments)
	}
	issue, err := service.Get(ctx, "AL-1")
	if err != nil || issue.Status != StatusInReview {
		t.Fatal(issue, err)
	}
}

func TestGitHubNotificationsCoalesce(t *testing.T) {
	g := NewGitHubSync(&Service{}, []GitHubRepo{{Workspace: "default", Repo: "team/repo"}}, "", false)
	if g.Notify("other/repo") || len(g.wake) != 0 {
		t.Fatal("unconfigured repository scheduled")
	}
	for range 10 {
		if !g.Notify("team/repo") {
			t.Fatal("configured repository rejected")
		}
	}
	if len(g.wake) != 1 {
		t.Fatal("notifications not coalesced")
	}
	<-g.wake
	if !g.Notify("team/repo") || len(g.wake) != 1 {
		t.Fatal("notification lost after draining")
	}
}

func TestGitHubSyncMatchesEveryTeamKey(t *testing.T) {
	const updated = "2026-10-01T12:00:00Z"
	fake := &fakeGitHub{pulls: map[string][]map[string]any{"owner/app": {
		pull(21, "al-1-build", "Build", "open", updated),
		pull(22, "import", "tok-1 Import tickets", "open", updated),
		pull(23, "tok-2-and-al-2", "Both teams", "merged", updated),
	}}}
	service, sync := newTestSync(t, fake, "owner/app")
	ctx := context.Background()
	stored := func(want string) {
		t.Helper()
		var got string
		if err := service.db.QueryRowContext(ctx, `SELECT COALESCE(group_concat(number || ' ' || issue_identifier, ', '), '') FROM (SELECT * FROM pull_requests ORDER BY number, issue_identifier)`).Scan(&got); err != nil || got != want {
			t.Fatalf("pull requests = %q, err = %v; want %q", got, err, want)
		}
	}
	for _, issue := range []CreateIssue{{Team: "AL", Title: "Build", Status: new(StatusInProgress)}, {Team: "AL", Title: "Both", Status: new(StatusTodo)}} {
		if _, err := service.Create(ctx, issue); err != nil {
			t.Fatal(err)
		}
	}

	// TOK is not a team yet, so the first round does not recognize its identifiers.
	sync.syncAll(ctx)
	stored("21 AL-1, 23 AL-2")

	if _, err := service.CreateTeam(ctx, CreateTeam{Key: "TOK", Name: "HarnessKit"}); err != nil {
		t.Fatal(err)
	}
	for _, issue := range []CreateIssue{{Team: "TOK", Title: "Import", Status: new(StatusInProgress)}, {Team: "TOK", Title: "Both", Status: new(StatusTodo)}} {
		if _, err := service.Create(ctx, issue); err != nil {
			t.Fatal(err)
		}
	}
	sync.syncAll(ctx)
	stored("21 AL-1, 22 TOK-1, 23 AL-2, 23 TOK-2")
	for identifier, want := range map[string]Status{"AL-1": StatusInReview, "AL-2": StatusDone, "TOK-1": StatusInReview, "TOK-2": StatusDone} {
		if issue, err := service.Get(ctx, identifier); err != nil || issue.Status != want {
			t.Fatalf("%s: status = %q, err = %v; want %q", identifier, issue.Status, err, want)
		}
	}
}

// The webhook only calls Notify. The round Notify wakes reads the current
// teams, so a team created while the server runs is recognized right away.
func TestGitHubSyncNotifyReadsCurrentTeams(t *testing.T) {
	fake := &fakeGitHub{pulls: map[string][]map[string]any{"owner/app": {
		pull(31, "tok-1-import", "Import tickets", "open", "2026-10-01T12:00:00Z"),
	}}}
	service, sync := newTestSync(t, fake, "owner/app")
	ctx, cancel := context.WithCancel(context.Background())
	stopped := make(chan struct{})
	go func() {
		sync.Run(ctx)
		close(stopped)
	}()
	defer func() {
		cancel()
		<-stopped
	}()
	waitFor := func(what string, done func() bool) {
		t.Helper()
		for deadline := time.Now().Add(5 * time.Second); !done(); time.Sleep(10 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
		}
	}
	waitFor("the first round to store its cursor", func() bool {
		var rounds int
		return service.db.QueryRow(`SELECT count(*) FROM github_sync`).Scan(&rounds) == nil && rounds == 1
	})

	if _, err := service.CreateTeam(context.Background(), CreateTeam{Key: "TOK", Name: "HarnessKit"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Create(context.Background(), CreateIssue{Team: "TOK", Title: "Import", Status: new(StatusInProgress)}); err != nil {
		t.Fatal(err)
	}
	if !sync.Notify("owner/app") {
		t.Fatal("configured repository rejected")
	}
	// Without the notification the next round would start a minute later.
	waitFor("TOK-1 to move to in_review", func() bool {
		issue, err := service.Get(context.Background(), "TOK-1")
		return err == nil && issue.Status == StatusInReview
	})
}

// A repository is bound to one workspace and matches only that workspace's
// team keys, even a key another workspace also uses.
func TestGitHubSyncMatchesWithinWorkspace(t *testing.T) {
	const updated = "2026-10-01T12:00:00Z"
	fake := &fakeGitHub{pulls: map[string][]map[string]any{"owner/app": {
		pull(41, "al-1-build", "Build", "open", updated),
		pull(42, "ops-1-deploy", "Deploy", "merged", updated),
	}}}
	def, sync := newTestSync(t, fake, "owner/app")
	ctx := context.Background()
	acme := inWorkspace(t, def, "acme")
	if _, err := acme.CreateTeam(ctx, CreateTeam{Key: "OPS", Name: "Ops"}); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		service *Service
		input   CreateIssue
	}{
		{def, CreateIssue{Team: "AL", Title: "Build", Status: new(StatusInProgress)}},
		{acme, CreateIssue{Team: "OPS", Title: "Deploy", Status: new(StatusInProgress)}},
	} {
		if _, err := step.service.Create(ctx, step.input); err != nil {
			t.Fatal(err)
		}
	}
	check := func(service *Service, identifier string, status Status, pulls ...int) {
		t.Helper()
		detail, err := service.Details(ctx, identifier)
		if err != nil {
			t.Fatal(err)
		}
		numbers := []int{}
		for _, pull := range detail.PullRequests {
			numbers = append(numbers, pull.Number)
		}
		if detail.Status != status || !slices.Equal(numbers, append([]int{}, pulls...)) {
			t.Fatalf("%s: status %q, pull requests %v; want %q, %v", identifier, detail.Status, numbers, status, pulls)
		}
	}
	stored := func(want string) {
		t.Helper()
		var got string
		if err := def.db.QueryRowContext(ctx, `SELECT COALESCE(group_concat(number || ' ' || issue_identifier || ' ' || slug, ', '), '')
			FROM (SELECT number, issue_identifier, slug FROM pull_requests JOIN workspaces ON workspaces.id = workspace_id ORDER BY number)`).Scan(&got); err != nil || got != want {
			t.Fatalf("pull requests = %q, err = %v; want %q", got, err, want)
		}
	}

	// OPS is a team only in acme, so the default repository ignores ops-1.
	sync.syncAll(ctx)
	check(def, "AL-1", StatusInReview, 41)
	check(acme, "OPS-1", StatusInProgress)
	stored("41 AL-1 default")

	// acme's AL-1 is not default's AL-1.
	if _, err := acme.CreateTeam(ctx, CreateTeam{Key: "AL", Name: "Acme"}); err != nil {
		t.Fatal(err)
	}
	if _, err := acme.Create(ctx, CreateIssue{Team: "AL", Title: "Acme build", Status: new(StatusInProgress)}); err != nil {
		t.Fatal(err)
	}
	sync.syncAll(ctx)
	check(def, "AL-1", StatusInReview, 41)
	check(acme, "AL-1", StatusInProgress)
	check(acme, "OPS-1", StatusInProgress)
	stored("41 AL-1 default")
	var cursors string
	if err := def.db.QueryRowContext(ctx, `SELECT group_concat(slug || ' ' || repo || ' ' || last_updated_at, ', ') FROM github_sync JOIN workspaces ON workspaces.id = workspace_id`).Scan(&cursors); err != nil || cursors != "default owner/app "+updated {
		t.Fatalf("cursors = %q, err = %v", cursors, err)
	}
}
