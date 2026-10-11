package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

type Status string

const (
	StatusTriage     Status = "triage"
	StatusBacklog    Status = "backlog"
	StatusTodo       Status = "todo"
	StatusAgentReady Status = "agent_ready"
	StatusInProgress Status = "in_progress"
	StatusInReview   Status = "in_review"
	StatusDone       Status = "done"
	StatusCanceled   Status = "canceled"
)

var Statuses = []Status{StatusTriage, StatusBacklog, StatusTodo, StatusAgentReady, StatusInProgress, StatusInReview, StatusDone, StatusCanceled}

// Types lists the issue types; an empty string means no type.
var Types = []string{"spec", "feature", "bug", "improvement", "chore"}

func statusNames() []string {
	names := make([]string, len(Statuses))
	for i, status := range Statuses {
		names[i] = string(status)
	}
	return names
}

// joinOr renders "a, b, or c".
func joinOr(items []string) string {
	last := len(items) - 1
	return strings.Join(items[:last], ", ") + ", or " + items[last]
}

type IssueSummary struct {
	Priority   int     `json:"priority"`
	Labels     []Label `json:"labels"`
	Identifier string  `json:"identifier"`
	Title      string  `json:"title"`
	Status     Status  `json:"status"`
	Type       *string `json:"type"`
	Project    *string `json:"project"`
	Parent     *string `json:"parent"`
	UpdatedAt  string  `json:"updated_at"`
}

type Issue struct {
	Priority    int     `json:"priority"`
	Labels      []Label `json:"labels"`
	Identifier  string  `json:"identifier"`
	Title       string  `json:"title"`
	Status      Status  `json:"status"`
	Type        *string `json:"type"`
	Project     *string `json:"project"`
	Parent      *string `json:"parent"`
	UpdatedAt   string  `json:"updated_at"`
	Description string  `json:"description"`
	CreatedAt   string  `json:"created_at"`
}

type IssueDetails struct {
	Issue
	Comments     []Comment      `json:"comments"`
	Links        []Link         `json:"links"`
	PullRequests []PullRequest  `json:"pull_requests"`
	Children     []IssueSummary `json:"children"`
	Blocks       []IssueSummary `json:"blocks"`
	BlockedBy    []IssueSummary `json:"blocked_by"`
	Related      []IssueSummary `json:"related"`
}

type ListIssues struct {
	Priority      *int    `json:"priority,omitempty" jsonschema:"filter by priority: 0 no priority, 1 urgent, 2 high, 3 medium, 4 low"`
	Sort          string  `json:"sort,omitempty" jsonschema:"priority or updated; omitted uses priority for an agent_ready status filter, otherwise updated; priority puts no priority last; ties use newest updated then newest id"`
	LabelIDs      []int64 `json:"label_ids,omitempty" jsonschema:"selected label IDs, including archived labels"`
	LabelGroupIDs []int64 `json:"label_group_ids,omitempty" jsonschema:"selected groups; each matches any child label"`
	LabelMode     string  `json:"label_mode,omitempty" jsonschema:"any (default), all, none, or unlabeled; unlabeled requires empty selections"`
	Status        *Status `json:"status,omitempty" jsonschema:"filter by status"`
	Type          *string `json:"type,omitempty" jsonschema:"filter by type; empty string matches untyped issues"`
	Team          string  `json:"team,omitempty" jsonschema:"filter by exact team key, such as AL"`
	Project       string  `json:"project,omitempty" jsonschema:"filter by exact Project name"`
	Parent        string  `json:"parent,omitempty" jsonschema:"filter by parent issue identifier, such as AL-1"`
	Query         string  `json:"query,omitempty" jsonschema:"LIKE match against title or description; percent and underscore are SQL wildcards"`
	Limit         *int    `json:"limit,omitempty" jsonschema:"maximum results, defaults to 50; zero returns none and negative means unlimited"`
}

type CreateIssue struct {
	Priority    int     `json:"priority,omitempty" jsonschema:"0 no priority (default), 1 urgent, 2 high, 3 medium, 4 low"`
	Team        string  `json:"team" jsonschema:"required key of an existing team, such as AL; the issue is numbered <key>-<n> in that team's sequence"`
	Title       string  `json:"title" jsonschema:"required nonempty issue title"`
	Description string  `json:"description,omitempty" jsonschema:"self-contained issue description"`
	Status      *Status `json:"status,omitempty" jsonschema:"initial status; triage (default)"`
	Type        string  `json:"type,omitempty" jsonschema:"optional type; empty string means no type"`
	Project     string  `json:"project,omitempty" jsonschema:"existing Project name; empty string means no Project"`
	Parent      string  `json:"parent,omitempty" jsonschema:"existing parent issue identifier; empty string means no parent"`
}

type UpdateIssue struct {
	Priority    *int    `json:"priority,omitempty" jsonschema:"0 clears priority, 1 urgent, 2 high, 3 medium, 4 low; omitted leaves unchanged"`
	Identifier  string  `json:"identifier" jsonschema:"issue identifier, such as AL-1"`
	Title       *string `json:"title,omitempty" jsonschema:"new nonempty title; omitted leaves unchanged"`
	Description *string `json:"description,omitempty" jsonschema:"new description; omitted leaves unchanged"`
	Status      *Status `json:"status,omitempty" jsonschema:"new status; omitted leaves unchanged"`
	Type        *string `json:"type,omitempty" jsonschema:"new type; empty string clears; omitted leaves unchanged"`
	Project     *string `json:"project,omitempty" jsonschema:"existing Project name; empty string clears; omitted leaves unchanged"`
	Parent      *string `json:"parent,omitempty" jsonschema:"existing parent issue identifier; empty string clears; omitted leaves unchanged; cannot be self"`
}

// Comment is append-only. Author names who wrote a comment imported from
// Linear and is empty for comments added here.
type Comment struct {
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
	Author    string `json:"author"`
}

type Link struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}

type PullRequest struct {
	Repo      string `json:"repo"`
	Number    int    `json:"number"`
	Title     string `json:"title"`
	URL       string `json:"url"`
	State     string `json:"state"`
	UpdatedAt string `json:"updated_at"`
}

func ValidStatus(status Status) bool {
	for _, candidate := range Statuses {
		if candidate == status {
			return true
		}
	}
	return false
}

func validateFields(title *string, status *Status, issueType *string) error {
	if title != nil && strings.TrimSpace(*title) == "" {
		return ErrInvalidTitle
	}
	if status != nil && !ValidStatus(*status) {
		return ErrInvalidStatus
	}
	if issueType != nil {
		if *issueType != "" && !slices.Contains(Types, *issueType) {
			return ErrInvalidType
		}
	}
	return nil
}

func validatePriority(priority *int) error {
	if priority != nil && (*priority < 0 || *priority > 4) {
		return ErrInvalidPriority
	}
	return nil
}

// issueColumns starts with the internal id, which other tables reference.
const issueColumns = `id, identifier, title, status, type,
	(SELECT name FROM projects WHERE projects.id = issues.project_id),
	(SELECT p.identifier FROM issues p WHERE p.id = issues.parent_id),
	updated_at, description, created_at, priority`

// scanIssue reads a row of issueColumns into an issue without its labels and
// returns the issue's id with it.
func scanIssue(row interface{ Scan(...any) error }) (Issue, int64, error) {
	var issue Issue
	var id int64
	err := row.Scan(&id, &issue.Identifier, &issue.Title, &issue.Status, &issue.Type, &issue.Project, &issue.Parent, &issue.UpdatedAt, &issue.Description, &issue.CreatedAt, &issue.Priority)
	return issue, id, err
}

// teamID, projectID, and issueID look up the id of a team key, a Project name,
// and an issue identifier in the workspace; an empty string gives NULL.
func (s *Service) teamID(ctx context.Context, key string) (sql.NullInt64, error) {
	var id sql.NullInt64
	if key == "" {
		return id, nil
	}
	err := s.db.QueryRowContext(ctx, `SELECT id FROM teams WHERE workspace_id = ? AND key = ?`, s.workspace, key).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return id, ErrTeamNotFound
	}
	return id, err
}

func (s *Service) projectID(ctx context.Context, name string) (sql.NullInt64, error) {
	var id sql.NullInt64
	if name == "" {
		return id, nil
	}
	err := s.db.QueryRowContext(ctx, `SELECT id FROM projects WHERE workspace_id = ? AND name = ?`, s.workspace, name).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return id, ErrProjectNotFound
	}
	return id, err
}

func (s *Service) issueID(ctx context.Context, identifier string) (sql.NullInt64, error) {
	var id sql.NullInt64
	if identifier == "" {
		return id, nil
	}
	err := s.db.QueryRowContext(ctx, `SELECT id FROM issues WHERE workspace_id = ? AND identifier = ?`, s.workspace, identifier).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return id, ErrNotFound
	}
	return id, err
}

func (s *Service) list(ctx context.Context, where string, args ...any) ([]Issue, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+issueColumns+` FROM issues WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	issues := []Issue{}
	ids := []int64{}
	for rows.Next() {
		issue, id, err := scanIssue(rows)
		if err != nil {
			return nil, err
		}
		issues = append(issues, issue)
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	for i := range issues {
		labels, err := s.issueLabels(ctx, ids[i])
		if err != nil {
			return nil, err
		}
		issues[i].Labels = labels
	}
	return issues, nil
}

func Summaries(issues []Issue) []IssueSummary {
	summaries := make([]IssueSummary, 0, len(issues))
	for _, issue := range issues {
		summaries = append(summaries, IssueSummary{
			Identifier: issue.Identifier, Title: issue.Title, Status: issue.Status, Type: issue.Type, Priority: issue.Priority,
			Project: issue.Project, Parent: issue.Parent, UpdatedAt: issue.UpdatedAt, Labels: issue.Labels,
		})
	}
	return summaries
}

func (s *Service) List(ctx context.Context, input ListIssues) ([]Issue, error) {
	if err := validatePriority(input.Priority); err != nil {
		return nil, err
	}
	if input.Sort != "" && input.Sort != "priority" && input.Sort != "updated" {
		return nil, ErrInvalidSort
	}
	if err := validateFields(nil, input.Status, input.Type); err != nil {
		return nil, err
	}
	projectID, err := s.projectID(ctx, input.Project)
	if err != nil {
		return nil, err
	}
	parentID, err := s.issueID(ctx, input.Parent)
	if err != nil {
		return nil, err
	}
	teamID, err := s.teamID(ctx, input.Team)
	if err != nil {
		return nil, err
	}
	where, args := `workspace_id = ?`, []any{s.workspace}
	if input.Priority != nil {
		where += ` AND priority = ?`
		args = append(args, *input.Priority)
	}
	if input.Status != nil {
		where += ` AND status = ?`
		args = append(args, *input.Status)
	}
	if input.Type != nil {
		where += ` AND type IS NULLIF(?, '')`
		args = append(args, *input.Type)
	}
	for _, filter := range []struct {
		column string
		id     sql.NullInt64
	}{{"team_id", teamID}, {"project_id", projectID}, {"parent_id", parentID}} {
		if filter.id.Valid {
			where += ` AND ` + filter.column + ` = ?`
			args = append(args, filter.id)
		}
	}
	if input.Query != "" {
		where += ` AND (title LIKE ? OR description LIKE ?)`
		args = append(args, "%"+input.Query+"%", "%"+input.Query+"%")
	}
	labelWhere, labelArgs, err := s.labelFilter(ctx, input)
	if err != nil {
		return nil, err
	}
	where += labelWhere
	args = append(args, labelArgs...)
	limit := 50
	if input.Limit != nil {
		limit = *input.Limit
	}
	order := `updated_at DESC, id DESC`
	if input.Sort == "priority" || (input.Sort == "" && input.Status != nil && *input.Status == StatusAgentReady) {
		order = `CASE priority WHEN 0 THEN 5 ELSE priority END, ` + order
	}
	return s.list(ctx, where+` ORDER BY `+order+` LIMIT ?`, append(args, limit)...)
}

func (s *Service) Create(ctx context.Context, input CreateIssue) (Issue, error) {
	if err := validatePriority(&input.Priority); err != nil {
		return Issue{}, err
	}
	if input.Team == "" {
		return Issue{}, ErrTeamRequired
	}
	if err := validateFields(&input.Title, input.Status, &input.Type); err != nil {
		return Issue{}, err
	}
	projectID, err := s.projectID(ctx, input.Project)
	if err != nil {
		return Issue{}, err
	}
	parentID, err := s.issueID(ctx, input.Parent)
	if err != nil {
		return Issue{}, err
	}
	status := StatusTriage
	if input.Status != nil {
		status = *input.Status
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Issue{}, err
	}
	defer tx.Rollback()
	// Taking the number in the same transaction as the insert means a failed
	// insert gives the number back.
	var teamID int64
	var number int
	err = tx.QueryRowContext(ctx, `UPDATE teams SET next_number = next_number + 1 WHERE workspace_id = ? AND key = ? RETURNING id, next_number - 1`, s.workspace, input.Team).Scan(&teamID, &number)
	if errors.Is(err, sql.ErrNoRows) {
		return Issue{}, ErrTeamNotFound
	}
	if err != nil {
		return Issue{}, err
	}
	identifier := fmt.Sprintf("%s-%d", input.Team, number)
	now := s.now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `
		INSERT INTO issues(workspace_id, identifier, team_id, number, title, description, status, type, project_id, parent_id, created_at, updated_at, priority)
		VALUES (?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?, ?)`,
		s.workspace, identifier, teamID, number, strings.TrimSpace(input.Title), input.Description, status, input.Type, projectID, parentID, now, now, input.Priority)
	if err != nil {
		return Issue{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Issue{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO events(issue_id, kind, detail, created_at) VALUES (?, 'created', ?, ?)`, id, string(status), now); err != nil {
		return Issue{}, err
	}
	if err := tx.Commit(); err != nil {
		return Issue{}, err
	}
	return s.Get(ctx, identifier)
}

func (s *Service) Get(ctx context.Context, identifier string) (Issue, error) {
	issue, _, err := s.get(ctx, identifier)
	return issue, err
}

// get also returns the issue's id, which other tables reference.
func (s *Service) get(ctx context.Context, identifier string) (Issue, int64, error) {
	issue, id, err := scanIssue(s.db.QueryRowContext(ctx, `SELECT `+issueColumns+` FROM issues WHERE workspace_id = ? AND identifier = ?`, s.workspace, identifier))
	if errors.Is(err, sql.ErrNoRows) {
		return Issue{}, 0, ErrNotFound
	}
	if err != nil {
		return Issue{}, 0, err
	}
	issue.Labels, err = s.issueLabels(ctx, id)
	return issue, id, err
}

func (s *Service) Update(ctx context.Context, input UpdateIssue) (Issue, error) {
	if err := validatePriority(input.Priority); err != nil {
		return Issue{}, err
	}
	if err := validateFields(input.Title, input.Status, input.Type); err != nil {
		return Issue{}, err
	}
	sets, args := []string{}, []any{}
	for _, field := range []struct {
		column string
		value  *string
	}{
		{"title", input.Title}, {"description", input.Description}, {"type", input.Type},
	} {
		if field.value != nil {
			expression := `?`
			if field.column == "type" {
				expression = `NULLIF(?, '')`
			}
			sets = append(sets, field.column+` = `+expression)
			args = append(args, *field.value)
		}
	}
	for _, reference := range []struct {
		column string
		value  *string
		id     func(context.Context, string) (sql.NullInt64, error)
	}{
		{"project_id", input.Project, s.projectID}, {"parent_id", input.Parent, s.issueID},
	} {
		if reference.value != nil {
			id, err := reference.id(ctx, *reference.value)
			if err != nil {
				return Issue{}, err
			}
			sets = append(sets, reference.column+` = ?`)
			args = append(args, id)
		}
	}
	if input.Priority != nil {
		sets = append(sets, `priority = ?`)
		args = append(args, *input.Priority)
	}
	if input.Status != nil {
		sets = append(sets, `status = ?`)
		args = append(args, *input.Status)
	}
	if len(sets) == 0 {
		return s.Get(ctx, input.Identifier)
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	sets = append(sets, `updated_at = ?`)
	args = append(args, now, s.workspace, input.Identifier)
	query := `UPDATE issues SET ` + strings.Join(sets, ", ") + ` WHERE workspace_id = ? AND identifier = ?`
	if input.Status == nil {
		if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
			return Issue{}, err
		}
	} else {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return Issue{}, err
		}
		defer tx.Rollback()
		if _, err := tx.ExecContext(ctx, `INSERT INTO events(issue_id, kind, detail, created_at)
			SELECT id, 'status_changed', status || ' -> ' || ?, ? FROM issues WHERE workspace_id = ? AND identifier = ? AND status <> ?`,
			*input.Status, now, s.workspace, input.Identifier, *input.Status); err != nil {
			return Issue{}, err
		}
		if _, err := tx.ExecContext(ctx, query, args...); err != nil {
			return Issue{}, err
		}
		if err := tx.Commit(); err != nil {
			return Issue{}, err
		}
	}
	return s.Get(ctx, input.Identifier)
}

func (s *Service) Details(ctx context.Context, identifier string) (IssueDetails, error) {
	issue, id, err := s.get(ctx, identifier)
	if err != nil {
		return IssueDetails{}, err
	}
	detail := IssueDetails{Issue: issue, Comments: []Comment{}, Links: []Link{}, PullRequests: []PullRequest{}}
	for _, group := range []struct {
		target *[]IssueSummary
		where  string
	}{
		{&detail.Children, `parent_id = ?`},
		{&detail.Blocks, `id IN (SELECT other_id FROM relations WHERE issue_id = ? AND kind = 'blocks')`},
		{&detail.BlockedBy, `id IN (SELECT issue_id FROM relations WHERE other_id = ? AND kind = 'blocks')`},
		{&detail.Related, `id IN (SELECT other_id FROM relations WHERE issue_id = ?1 AND kind = 'related' UNION SELECT issue_id FROM relations WHERE other_id = ?1 AND kind = 'related')`},
	} {
		issues, err := s.list(ctx, group.where+` ORDER BY updated_at DESC, id DESC`, id)
		if err != nil {
			return IssueDetails{}, err
		}
		*group.target = Summaries(issues)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT body, created_at, author FROM comments WHERE issue_id = ? ORDER BY id`, id)
	if err != nil {
		return IssueDetails{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var comment Comment
		if err := rows.Scan(&comment.Body, &comment.CreatedAt, &comment.Author); err != nil {
			return IssueDetails{}, err
		}
		detail.Comments = append(detail.Comments, comment)
	}
	if err := rows.Err(); err != nil {
		return IssueDetails{}, err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT title, url FROM links WHERE issue_id = ? ORDER BY id`, id)
	if err != nil {
		return IssueDetails{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var link Link
		if err := rows.Scan(&link.Title, &link.URL); err != nil {
			return IssueDetails{}, err
		}
		detail.Links = append(detail.Links, link)
	}
	if err := rows.Err(); err != nil {
		return IssueDetails{}, err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT repo, number, title, url, state, updated_at FROM pull_requests
		WHERE workspace_id = ? AND issue_identifier = ? ORDER BY updated_at DESC`, s.workspace, identifier)
	if err != nil {
		return IssueDetails{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var pull PullRequest
		if err := rows.Scan(&pull.Repo, &pull.Number, &pull.Title, &pull.URL, &pull.State, &pull.UpdatedAt); err != nil {
			return IssueDetails{}, err
		}
		detail.PullRequests = append(detail.PullRequests, pull)
	}
	return detail, rows.Err()
}

func (s *Service) AddComment(ctx context.Context, identifier, body string) (Comment, error) {
	_, id, err := s.get(ctx, identifier)
	if err != nil {
		return Comment{}, err
	}
	comment := Comment{Body: body, CreatedAt: s.now().UTC().Format(time.RFC3339Nano)}
	_, err = s.db.ExecContext(ctx, `INSERT INTO comments(issue_id, body, created_at) VALUES (?, ?, ?)`, id, body, comment.CreatedAt)
	return comment, err
}

func (s *Service) AddLink(ctx context.Context, identifier, title, url string) (Link, error) {
	_, id, err := s.get(ctx, identifier)
	if err != nil {
		return Link{}, err
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO links(issue_id, title, url) VALUES (?, ?, ?)`, id, title, url)
	return Link{Title: title, URL: url}, err
}

func (s *Service) Relation(ctx context.Context, identifier, kind, other string, remove bool) error {
	switch kind {
	case "blocks", "related", "blocked_by":
	default:
		return ErrInvalidKind
	}
	ids := make([]int64, 2)
	for i, reference := range []string{identifier, other} {
		_, id, err := s.get(ctx, reference)
		if err != nil {
			return err
		}
		ids[i] = id
	}
	from, to := ids[0], ids[1]
	if kind == "blocked_by" {
		from, to, kind = to, from, "blocks"
	}
	if kind == "related" && from > to {
		from, to = to, from
	}
	query := `INSERT INTO relations(issue_id, kind, other_id) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`
	if remove {
		query = `DELETE FROM relations WHERE issue_id = ? AND kind = ? AND other_id = ?`
	}
	_, err := s.db.ExecContext(ctx, query, from, kind, to)
	return err
}
