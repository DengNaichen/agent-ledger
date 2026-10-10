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
	"time"
)

const (
	gitHubSyncInterval = time.Minute
	gitHubPageSize     = 100
)

// GitHubSync polls pull requests, because webhooks cannot reach a server that
// listens on localhost. A pull request whose branch or title names an issue is
// recorded on it and moves it forward.
type GitHubSync struct {
	service  *Service
	repos    []string
	token    string
	linkback bool
	baseURL  string
	pattern  *regexp.Regexp
}

func NewGitHubSync(service *Service, repos []string, token string, linkback bool) *GitHubSync {
	return &GitHubSync{
		service: service, repos: repos, token: token, linkback: linkback,
		baseURL: "https://api.github.com",
		pattern: regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(service.prefix) + `-(\d+)`),
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
		}
	}
}

func (g *GitHubSync) syncAll(ctx context.Context) {
	for _, repo := range g.repos {
		if err := g.syncRepo(ctx, repo); err != nil {
			log.Printf("github sync %s failed: %v", repo, err)
		}
	}
}

// syncRepo reads pull requests by most recent update and stops at the first
// one updated before the previous sync. The cursor moves only after the whole
// read succeeds.
func (g *GitHubSync) syncRepo(ctx context.Context, repo string) error {
	db := g.service.db
	var since string
	if err := db.QueryRowContext(ctx, `SELECT COALESCE((SELECT last_updated_at FROM github_sync WHERE repo = ?), '')`, repo).Scan(&since); err != nil {
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
			for _, match := range g.pattern.FindAllStringSubmatch(pull.Head.Ref+" "+pull.Title, -1) {
				if err := g.link(ctx, repo, pull, g.service.prefix+"-"+match[1]); err != nil {
					return err
				}
			}
		}
		if len(pulls) < gitHubPageSize {
			break
		}
	}
	_, err := db.ExecContext(ctx, `INSERT INTO github_sync(repo, last_updated_at) VALUES (?, ?)
		ON CONFLICT(repo) DO UPDATE SET last_updated_at = excluded.last_updated_at`, repo, newest)
	return err
}

// link records that a pull request names an issue. When the issue exists, it
// moves the issue forward and, with linkback on, comments once on the pull
// request while it is open.
func (g *GitHubSync) link(ctx context.Context, repo string, pull gitHubPull, identifier string) error {
	s := g.service
	state := pull.state()
	var linkbackPostedAt *string
	if err := s.db.QueryRowContext(ctx, `
		INSERT INTO pull_requests(repo, number, title, url, head_ref, state, issue_identifier, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(repo, number, issue_identifier) DO UPDATE SET
			title = excluded.title, url = excluded.url, head_ref = excluded.head_ref, state = excluded.state, updated_at = excluded.updated_at
		RETURNING linkback_posted_at`,
		repo, pull.Number, pull.Title, pull.URL, pull.Head.Ref, state, identifier, pull.UpdatedAt).Scan(&linkbackPostedAt); err != nil {
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
	_, err = s.db.ExecContext(ctx, `UPDATE pull_requests SET linkback_posted_at = ? WHERE repo = ? AND number = ? AND issue_identifier = ?`,
		s.now().UTC().Format(time.RFC3339Nano), repo, pull.Number, identifier)
	return err
}

// advance moves an issue forward to match its pull request and never back: an
// open pull request moves work before review into review, and a merged one
// finishes anything not done or canceled.
func (s *Service) advance(ctx context.Context, issue Issue, state, source string) error {
	var target Status
	switch {
	case state == "open" && slices.Contains([]Status{StatusTriage, StatusTodo, StatusAgentReady, StatusInProgress}, issue.Status):
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
	if _, err := tx.ExecContext(ctx, `INSERT INTO events(issue_id, kind, detail, created_at) SELECT id, 'status_changed', ?, ? FROM issues WHERE identifier = ?`,
		fmt.Sprintf("%s -> %s via %s", issue.Status, target, source), now, issue.Identifier); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE issues SET status = ?, updated_at = ? WHERE identifier = ?`, target, now, issue.Identifier); err != nil {
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
