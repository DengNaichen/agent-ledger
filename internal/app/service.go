package app

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Status string

const (
	StatusTriage     Status = "triage"
	StatusTodo       Status = "todo"
	StatusAgentReady Status = "agent_ready"
	StatusInProgress Status = "in_progress"
	StatusInReview   Status = "in_review"
	StatusDone       Status = "done"
	StatusCanceled   Status = "canceled"
)

var (
	ErrNotFound        = errors.New("issue not found")
	ErrProjectNotFound = errors.New("project not found")
	ErrInvalidTitle    = errors.New("title is required")
	ErrInvalidStatus   = errors.New("invalid status: use triage, todo, agent_ready, in_progress, in_review, done, or canceled")
	ErrInvalidType     = errors.New("invalid type: use spec, feature, bug, improvement, chore, or an empty string")
	ErrInvalidKind     = errors.New("invalid relation kind: use blocks, blocked_by, or related")
	ErrTeamRequired    = errors.New("team is required")
	ErrTeamNotFound    = errors.New("team not found")
	ErrInvalidTeam     = errors.New("invalid team: key must be 2 to 6 uppercase letters or digits starting with a letter, name is required, and start_number must be at least 1")
	ErrTeamExists      = errors.New("a team with this key already exists")
)

var teamKey = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,5}$`)

var Statuses = []Status{StatusTriage, StatusTodo, StatusAgentReady, StatusInProgress, StatusInReview, StatusDone, StatusCanceled}

//go:embed schema.sql
var schema string

type IssueSummary struct {
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
	LabelIDs      []int64 `json:"label_ids,omitempty" jsonschema:"selected label IDs, including archived labels"`
	LabelGroupIDs []int64 `json:"label_group_ids,omitempty" jsonschema:"selected groups; each matches any child label"`
	LabelMode     string  `json:"label_mode,omitempty" jsonschema:"any (default), all, none, or unlabeled; unlabeled requires empty selections"`
	Status        *Status `json:"status,omitempty" jsonschema:"filter by status: triage, todo, agent_ready, in_progress, in_review, done, or canceled"`
	Type          *string `json:"type,omitempty" jsonschema:"filter by type: spec, feature, bug, improvement, chore; empty string matches untyped issues"`
	Team          string  `json:"team,omitempty" jsonschema:"filter by exact team key, such as AL"`
	Project       string  `json:"project,omitempty" jsonschema:"filter by exact Project name"`
	Parent        string  `json:"parent,omitempty" jsonschema:"filter by parent issue identifier, such as AL-1"`
	Query         string  `json:"query,omitempty" jsonschema:"LIKE match against title or description; percent and underscore are SQL wildcards"`
	Limit         *int    `json:"limit,omitempty" jsonschema:"maximum results, defaults to 50; zero returns none and negative means unlimited"`
}

type CreateIssue struct {
	Team        string  `json:"team" jsonschema:"required key of an existing team, such as AL; the issue is numbered <key>-<n> in that team's sequence"`
	Title       string  `json:"title" jsonschema:"required nonempty issue title"`
	Description string  `json:"description,omitempty" jsonschema:"self-contained issue description"`
	Status      *Status `json:"status,omitempty" jsonschema:"initial status: triage (default), todo, agent_ready, in_progress, in_review, done, or canceled"`
	Type        string  `json:"type,omitempty" jsonschema:"optional type: spec, feature, bug, improvement, or chore; empty string means no type"`
	Project     string  `json:"project,omitempty" jsonschema:"existing Project name; empty string means no Project"`
	Parent      string  `json:"parent,omitempty" jsonschema:"existing parent issue identifier; empty string means no parent"`
}

type UpdateIssue struct {
	Identifier  string  `json:"identifier" jsonschema:"issue identifier, such as AL-1"`
	Title       *string `json:"title,omitempty" jsonschema:"new nonempty title; omitted leaves unchanged"`
	Description *string `json:"description,omitempty" jsonschema:"new description; omitted leaves unchanged"`
	Status      *Status `json:"status,omitempty" jsonschema:"new status: triage, todo, agent_ready, in_progress, in_review, done, or canceled; omitted leaves unchanged"`
	Type        *string `json:"type,omitempty" jsonschema:"new type: spec, feature, bug, improvement, or chore; empty string clears; omitted leaves unchanged"`
	Project     *string `json:"project,omitempty" jsonschema:"existing Project name; empty string clears; omitted leaves unchanged"`
	Parent      *string `json:"parent,omitempty" jsonschema:"existing parent issue identifier; empty string clears; omitted leaves unchanged; cannot be self"`
}

type Project struct {
	Name        string `json:"name" jsonschema:"unique Project name"`
	Description string `json:"description" jsonschema:"the Project outcome"`
	Archived    bool   `json:"archived"`
}

type UpdateProject struct {
	Name        string  `json:"name" jsonschema:"existing Project name; cannot be renamed"`
	Description *string `json:"description,omitempty" jsonschema:"new Project outcome; omitted leaves unchanged"`
	Archived    *bool   `json:"archived,omitempty" jsonschema:"archive or unarchive the Project; omitted leaves unchanged"`
}

type Team struct {
	Key        string `json:"key" jsonschema:"identifier prefix; fixed once created"`
	Name       string `json:"name"`
	NextNumber int    `json:"next_number" jsonschema:"number the team's next issue gets"`
}

type CreateTeam struct {
	Key         string `json:"key" jsonschema:"2 to 6 uppercase letters or digits starting with a letter, such as AL; cannot be changed later"`
	Name        string `json:"name" jsonschema:"required nonempty team name"`
	StartNumber *int   `json:"start_number,omitempty" jsonschema:"number the team's first issue gets, at least 1; omitted starts at 1"`
}

type Comment struct {
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
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

type Service struct {
	db        *sql.DB
	backupDir string
	now       func() time.Time
}

func Open(path string) (*Service, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("database path is required")
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, err
		}
	}
	backupDir := filepath.Join(filepath.Dir(path), "backups")
	if err := migrate(db, backupDir, migrations); err != nil {
		db.Close()
		return nil, err
	}

	return &Service{db: db, backupDir: backupDir, now: time.Now}, nil
}

func (s *Service) Close() error { return s.db.Close() }

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
		switch *issueType {
		case "", "spec", "feature", "bug", "improvement", "chore":
		default:
			return ErrInvalidType
		}
	}
	return nil
}

const issueColumns = `identifier, title, status, type, project, parent, updated_at, description, created_at`

func (s *Service) list(ctx context.Context, where string, args ...any) ([]Issue, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+issueColumns+` FROM issues WHERE `+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	issues := []Issue{}
	for rows.Next() {
		var issue Issue
		if err := rows.Scan(&issue.Identifier, &issue.Title, &issue.Status, &issue.Type, &issue.Project, &issue.Parent, &issue.UpdatedAt, &issue.Description, &issue.CreatedAt); err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	for i := range issues {
		labels, err := s.issueLabels(ctx, issues[i].Identifier)
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
			Identifier: issue.Identifier, Title: issue.Title, Status: issue.Status, Type: issue.Type,
			Project: issue.Project, Parent: issue.Parent, UpdatedAt: issue.UpdatedAt, Labels: issue.Labels,
		})
	}
	return summaries
}

func (s *Service) List(ctx context.Context, input ListIssues) ([]Issue, error) {
	if err := validateFields(nil, input.Status, input.Type); err != nil {
		return nil, err
	}
	if err := s.checkReferences(ctx, &input.Project, &input.Parent); err != nil {
		return nil, err
	}
	if input.Team != "" {
		if err := s.checkTeam(ctx, input.Team); err != nil {
			return nil, err
		}
	}
	where, args := `1 = 1`, []any{}
	if input.Status != nil {
		where += ` AND status = ?`
		args = append(args, *input.Status)
	}
	if input.Type != nil {
		where += ` AND type IS NULLIF(?, '')`
		args = append(args, *input.Type)
	}
	for _, filter := range []struct{ column, value string }{{"team", input.Team}, {"project", input.Project}, {"parent", input.Parent}} {
		if filter.value != "" {
			where += ` AND ` + filter.column + ` = ?`
			args = append(args, filter.value)
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
	return s.list(ctx, where+` ORDER BY updated_at DESC, id DESC LIMIT ?`, append(args, limit)...)
}

func (s *Service) Create(ctx context.Context, input CreateIssue) (Issue, error) {
	if input.Team == "" {
		return Issue{}, ErrTeamRequired
	}
	if err := validateFields(&input.Title, input.Status, &input.Type); err != nil {
		return Issue{}, err
	}
	if err := s.checkReferences(ctx, &input.Project, &input.Parent); err != nil {
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
	var number int
	err = tx.QueryRowContext(ctx, `UPDATE teams SET next_number = next_number + 1 WHERE key = ? RETURNING next_number - 1`, input.Team).Scan(&number)
	if errors.Is(err, sql.ErrNoRows) {
		return Issue{}, ErrTeamNotFound
	}
	if err != nil {
		return Issue{}, err
	}
	identifier := fmt.Sprintf("%s-%d", input.Team, number)
	now := s.now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `
		INSERT INTO issues(identifier, team, number, title, description, status, type, project, parent, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), NULLIF(?, ''), ?, ?)`,
		identifier, input.Team, number, strings.TrimSpace(input.Title), input.Description, status, input.Type, input.Project, input.Parent, now, now)
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
	var issue Issue
	err := s.db.QueryRowContext(ctx, `SELECT `+issueColumns+` FROM issues WHERE identifier = ?`, identifier).Scan(
		&issue.Identifier, &issue.Title, &issue.Status, &issue.Type, &issue.Project, &issue.Parent, &issue.UpdatedAt, &issue.Description, &issue.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Issue{}, ErrNotFound
	}
	if err != nil {
		return Issue{}, err
	}
	issue.Labels, err = s.issueLabels(ctx, identifier)
	return issue, err
}

func (s *Service) Update(ctx context.Context, input UpdateIssue) (Issue, error) {
	if err := validateFields(input.Title, input.Status, input.Type); err != nil {
		return Issue{}, err
	}
	if err := s.checkReferences(ctx, input.Project, input.Parent); err != nil {
		return Issue{}, err
	}
	sets, args := []string{}, []any{}
	for _, field := range []struct {
		column string
		value  *string
	}{
		{"title", input.Title}, {"description", input.Description}, {"type", input.Type}, {"project", input.Project}, {"parent", input.Parent},
	} {
		if field.value != nil {
			expression := `?`
			switch field.column {
			case "type", "project", "parent":
				expression = `NULLIF(?, '')`
			}
			sets = append(sets, field.column+` = `+expression)
			args = append(args, *field.value)
		}
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
	args = append(args, now, input.Identifier)
	query := `UPDATE issues SET ` + strings.Join(sets, ", ") + ` WHERE identifier = ?`
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
			SELECT id, 'status_changed', status || ' -> ' || ?, ? FROM issues WHERE identifier = ? AND status <> ?`,
			*input.Status, now, input.Identifier, *input.Status); err != nil {
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
	issue, err := s.Get(ctx, identifier)
	if err != nil {
		return IssueDetails{}, err
	}
	detail := IssueDetails{Issue: issue, Comments: []Comment{}, Links: []Link{}, PullRequests: []PullRequest{}}
	for _, group := range []struct {
		target *[]IssueSummary
		where  string
	}{
		{&detail.Children, `parent = ?`},
		{&detail.Blocks, `identifier IN (SELECT other FROM relations WHERE issue = ? AND kind = 'blocks')`},
		{&detail.BlockedBy, `identifier IN (SELECT issue FROM relations WHERE other = ? AND kind = 'blocks')`},
		{&detail.Related, `identifier IN (SELECT other FROM relations WHERE issue = ?1 AND kind = 'related' UNION SELECT issue FROM relations WHERE other = ?1 AND kind = 'related')`},
	} {
		issues, err := s.list(ctx, group.where+` ORDER BY updated_at DESC, id DESC`, identifier)
		if err != nil {
			return IssueDetails{}, err
		}
		*group.target = Summaries(issues)
	}
	rows, err := s.db.QueryContext(ctx, `SELECT body, created_at FROM comments WHERE issue = ? ORDER BY id`, identifier)
	if err != nil {
		return IssueDetails{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var comment Comment
		if err := rows.Scan(&comment.Body, &comment.CreatedAt); err != nil {
			return IssueDetails{}, err
		}
		detail.Comments = append(detail.Comments, comment)
	}
	if err := rows.Err(); err != nil {
		return IssueDetails{}, err
	}
	rows, err = s.db.QueryContext(ctx, `SELECT title, url FROM links WHERE issue = ? ORDER BY id`, identifier)
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
		WHERE issue_identifier = ? ORDER BY updated_at DESC`, identifier)
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
	if _, err := s.Get(ctx, identifier); err != nil {
		return Comment{}, err
	}
	comment := Comment{Body: body, CreatedAt: s.now().UTC().Format(time.RFC3339Nano)}
	_, err := s.db.ExecContext(ctx, `INSERT INTO comments(issue, body, created_at) VALUES (?, ?, ?)`, identifier, body, comment.CreatedAt)
	return comment, err
}

func (s *Service) AddLink(ctx context.Context, identifier, title, url string) (Link, error) {
	if _, err := s.Get(ctx, identifier); err != nil {
		return Link{}, err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO links(issue, title, url) VALUES (?, ?, ?)`, identifier, title, url)
	return Link{Title: title, URL: url}, err
}

func (s *Service) Relation(ctx context.Context, identifier, kind, other string, remove bool) error {
	switch kind {
	case "blocks", "related", "blocked_by":
	default:
		return ErrInvalidKind
	}
	for _, reference := range []string{identifier, other} {
		if _, err := s.Get(ctx, reference); err != nil {
			return err
		}
	}
	if kind == "blocked_by" {
		identifier, other, kind = other, identifier, "blocks"
	}
	if kind == "related" && identifier > other {
		identifier, other = other, identifier
	}
	query := `INSERT INTO relations(issue, kind, other) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`
	if remove {
		query = `DELETE FROM relations WHERE issue = ? AND kind = ? AND other = ?`
	}
	_, err := s.db.ExecContext(ctx, query, identifier, kind, other)
	return err
}

func (s *Service) ListProjects(ctx context.Context, includeArchived bool) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, description, archived FROM projects WHERE ? OR NOT archived ORDER BY name`, includeArchived)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := []Project{}
	for rows.Next() {
		var project Project
		if err := rows.Scan(&project.Name, &project.Description, &project.Archived); err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (s *Service) CreateProject(ctx context.Context, name, description string) (Project, error) {
	_, err := s.db.ExecContext(ctx, `INSERT INTO projects(name, description) VALUES (?, ?)`, name, description)
	return Project{Name: name, Description: description}, err
}

func (s *Service) UpdateProject(ctx context.Context, input UpdateProject) (Project, error) {
	var project Project
	err := s.db.QueryRowContext(ctx, `UPDATE projects SET description = COALESCE(?, description), archived = COALESCE(?, archived)
		WHERE name = ? RETURNING name, description, archived`, input.Description, input.Archived, input.Name).Scan(&project.Name, &project.Description, &project.Archived)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrProjectNotFound
	}
	return project, err
}

func (s *Service) ListTeams(ctx context.Context) ([]Team, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, name, next_number FROM teams ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	teams := []Team{}
	for rows.Next() {
		var team Team
		if err := rows.Scan(&team.Key, &team.Name, &team.NextNumber); err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}
	return teams, rows.Err()
}

// CreateTeam adds a team whose numbering starts at StartNumber, or 1 when it
// is omitted. Teams cannot be changed or deleted.
func (s *Service) CreateTeam(ctx context.Context, input CreateTeam) (Team, error) {
	team := Team{Key: input.Key, Name: strings.TrimSpace(input.Name), NextNumber: 1}
	if input.StartNumber != nil {
		team.NextNumber = *input.StartNumber
	}
	if !teamKey.MatchString(team.Key) || team.Name == "" || team.NextNumber < 1 {
		return Team{}, ErrInvalidTeam
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO teams(key, name, next_number) VALUES (?, ?, ?) ON CONFLICT(key) DO NOTHING`, team.Key, team.Name, team.NextNumber)
	if err != nil {
		return Team{}, err
	}
	if inserted, _ := result.RowsAffected(); inserted == 0 {
		return Team{}, ErrTeamExists
	}
	return team, nil
}

func (s *Service) checkTeam(ctx context.Context, key string) error {
	var found string
	err := s.db.QueryRowContext(ctx, `SELECT key FROM teams WHERE key = ?`, key).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTeamNotFound
	}
	return err
}

func (s *Service) checkReferences(ctx context.Context, project, parent *string) error {
	if project != nil && *project != "" {
		var name string
		err := s.db.QueryRowContext(ctx, `SELECT name FROM projects WHERE name = ?`, *project).Scan(&name)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrProjectNotFound
		}
		if err != nil {
			return err
		}
	}
	if parent != nil && *parent != "" {
		_, err := s.Get(ctx, *parent)
		return err
	}
	return nil
}
