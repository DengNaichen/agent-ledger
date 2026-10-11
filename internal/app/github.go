package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
)

const (
	gitHubSyncInterval = time.Minute
	gitHubPageSize     = 100
)

// GitHubRepo binds a GitHub repository to a workspace: its pull requests link
// only to that workspace's issues.
type GitHubRepo struct {
	Workspace string
	Repo      string
}

// GitHubSync polls pull requests and coalesces webhook-triggered refreshes.
// All reads and linkbacks run in the same worker.
type GitHubSync struct {
	service  *Service
	repos    []GitHubRepo
	token    string
	linkback bool
	baseURL  string
	wake     chan struct{}
}

func NewGitHubSync(service *Service, repos []GitHubRepo, token string, linkback bool) *GitHubSync {
	return &GitHubSync{
		service: service, repos: repos, token: token, linkback: linkback,
		baseURL: "https://api.github.com",
		wake:    make(chan struct{}, 1),
	}
}

type gitHubPull struct {
	Number    int     `json:"number"`
	Title     string  `json:"title"`
	URL       string  `json:"html_url"`
	State     string  `json:"state"`
	Draft     bool    `json:"draft"`
	MergedAt  *string `json:"merged_at"`
	UpdatedAt string  `json:"updated_at"`
	Head      struct {
		Ref string `json:"ref"`
	} `json:"head"`
}

func (pull gitHubPull) state() string {
	switch {
	case pull.MergedAt != nil:
		return "merged"
	case pull.State == "closed":
		return "closed"
	case pull.Draft:
		return "draft"
	}
	return "open"
}

// Run syncs the repositories one after another once per interval until ctx is
// done. A repository that fails is logged and read again in the next round.
func (g *GitHubSync) Run(ctx context.Context) {
	for {
		g.syncAll(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(gitHubSyncInterval):
		case <-g.wake:
		}
	}
}

func (g *GitHubSync) syncAll(ctx context.Context) {
	for _, binding := range g.repos {
		if err := g.syncRepo(ctx, binding); err != nil {
			log.Printf("github sync %s=%s failed: %v", binding.Workspace, binding.Repo, err)
		}
	}
}

// identifierFinder returns a function that finds the given teams' issue
// identifiers in text. Keys match in any case; each identifier found is
// spelled with the team's own key.
func identifierFinder(teams []Team) func(text string) []string {
	if len(teams) == 0 {
		return func(string) []string { return nil }
	}
	keys := map[string]string{}
	quoted := make([]string, 0, len(teams))
	for _, team := range teams {
		keys[strings.ToUpper(team.Key)] = team.Key
		quoted = append(quoted, regexp.QuoteMeta(team.Key))
	}
	pattern := regexp.MustCompile(`(?i)\b(` + strings.Join(quoted, "|") + `)-(\d+)`)
	return func(text string) []string {
		var identifiers []string
		for _, match := range pattern.FindAllStringSubmatch(text, -1) {
			identifiers = append(identifiers, keys[strings.ToUpper(match[1])]+"-"+match[2])
		}
		return identifiers
	}
}

// syncRepo reads pull requests by most recent update and stops at the first
// one updated before the previous sync. The cursor moves only after the whole
// read succeeds. It reads the workspace's teams first, so a team created while
// the server runs is recognized from the next round on.
func (g *GitHubSync) syncRepo(ctx context.Context, binding GitHubRepo) error {
	s, err := g.service.InWorkspace(ctx, binding.Workspace)
	if err != nil {
		return err
	}
	teams, err := s.ListTeams(ctx)
	if err != nil {
		return err
	}
	identifiers := identifierFinder(teams)
	repo := binding.Repo
	var since string
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE((SELECT last_updated_at FROM github_sync WHERE workspace_id = ? AND repo = ?), '')`, s.workspace, repo).Scan(&since); err != nil {
		return err
	}
	newest := since
pages:
	for page := 1; ; page++ {
		var pulls []gitHubPull
		path := fmt.Sprintf("/repos/%s/pulls?state=all&sort=updated&direction=desc&per_page=%d&page=%d", repo, gitHubPageSize, page)
		if err := g.request(ctx, http.MethodGet, path, nil, &pulls); err != nil {
			return err
		}
		for _, pull := range pulls {
			if pull.UpdatedAt < since {
				break pages
			}
			newest = max(newest, pull.UpdatedAt)
			// When the branch and the title name the same issue, the second link changes nothing.
			for _, identifier := range identifiers(pull.Head.Ref + " " + pull.Title) {
				if err := g.link(ctx, s, repo, pull, identifier); err != nil {
					return err
				}
			}
		}
		if len(pulls) < gitHubPageSize {
			break
		}
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO github_sync(workspace_id, repo, last_updated_at) VALUES (?, ?, ?)
		ON CONFLICT(workspace_id, repo) DO UPDATE SET last_updated_at = excluded.last_updated_at`, s.workspace, repo, newest)
	return err
}

// link records that a pull request names an issue of the workspace s reads.
// When the issue exists, it moves the issue forward and, with linkback on,
// comments once on the pull request while it is open.
func (g *GitHubSync) link(ctx context.Context, s *Service, repo string, pull gitHubPull, identifier string) error {
	state := pull.state()
	var linkbackPostedAt *string
	if err := s.db.QueryRowContext(ctx, `
		INSERT INTO pull_requests(workspace_id, repo, number, title, url, head_ref, state, issue_identifier, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(workspace_id, repo, number, issue_identifier) DO UPDATE SET
			title = excluded.title, url = excluded.url, head_ref = excluded.head_ref, state = excluded.state, updated_at = excluded.updated_at
		RETURNING linkback_posted_at`,
		s.workspace, repo, pull.Number, pull.Title, pull.URL, pull.Head.Ref, state, identifier, pull.UpdatedAt).Scan(&linkbackPostedAt); err != nil {
		return err
	}
	issue, err := s.Get(ctx, identifier)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := s.advance(ctx, issue, state, fmt.Sprintf("%s#%d", repo, pull.Number)); err != nil {
		return err
	}
	// GitHub reports drafts as open. Skipping merged and closed pull requests
	// keeps the first sync from commenting on old ones.
	if !g.linkback || linkbackPostedAt != nil || pull.State != "open" {
		return nil
	}
	body, _ := json.Marshal(map[string]string{"body": fmt.Sprintf("**%s · %s**\n\n<details>\n\n%s\n\n</details>", issue.Identifier, issue.Title, issue.Description)})
	if err := g.request(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/issues/%d/comments", repo, pull.Number), bytes.NewReader(body), &struct{}{}); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `UPDATE pull_requests SET linkback_posted_at = ? WHERE workspace_id = ? AND repo = ? AND number = ? AND issue_identifier = ?`,
		s.now().UTC().Format(time.RFC3339Nano), s.workspace, repo, pull.Number, identifier)
	return err
}

// advance moves an issue forward to match its pull request and never back: an
// open pull request moves work before review into review, and a merged one
// finishes anything not done or canceled.
func (s *Service) advance(ctx context.Context, issue Issue, state, source string) error {
	var target Status
	switch {
	case state == "open" && slices.Contains([]Status{StatusTriage, StatusBacklog, StatusTodo, StatusAgentReady, StatusInProgress}, issue.Status):
		target = StatusInReview
	case state == "merged" && issue.Status != StatusDone && issue.Status != StatusCanceled:
		target = StatusDone
	default:
		return nil
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := s.changeStatus(ctx, tx, issue.Identifier, target, " via "+source, now); err != nil {
		return err
	}
	return tx.Commit()
}

func (g *GitHubSync) request(ctx context.Context, method, path string, body io.Reader, out any) error {
	req, err := http.NewRequestWithContext(ctx, method, g.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+g.token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %s", method, path, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Notify schedules a refresh only for a configured repository. Multiple pending
// deliveries coalesce; polling recovers notifications lost during a restart.
func (g *GitHubSync) Notify(repo string) bool {
	if !slices.ContainsFunc(g.repos, func(binding GitHubRepo) bool { return binding.Repo == repo }) {
		return false
	}
	select {
	case g.wake <- struct{}{}:
	default:
	}
	return true
}
