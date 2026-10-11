package app

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	_ "modernc.org/sqlite"
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

var (
	ErrNotFound        = errors.New("issue not found")
	ErrProjectNotFound = errors.New("project not found")
	ErrInvalidTitle    = errors.New("title is required")
	ErrInvalidStatus   = errors.New("invalid status: use triage, backlog, todo, agent_ready, in_progress, in_review, done, or canceled")
	ErrInvalidPriority = errors.New("invalid priority: use 0 (no priority), 1 (urgent), 2 (high), 3 (medium), or 4 (low)")
	ErrInvalidSort     = errors.New("invalid sort: use priority or updated")
	ErrInvalidType     = errors.New("invalid type: use spec, feature, bug, improvement, chore, or an empty string")
	ErrInvalidKind     = errors.New("invalid relation kind: use blocks, blocked_by, or related")
	ErrTeamRequired    = errors.New("team is required")
	ErrTeamNotFound    = errors.New("team not found")
	ErrInvalidTeam     = errors.New("invalid team: key must be 2 to 6 uppercase letters or digits starting with a letter, name is required, and start_number must be at least 1")
	ErrTeamExists      = errors.New("a team with this key already exists")

	ErrWorkspaceNotFound = errors.New("workspace not found")
	ErrInvalidWorkspace  = errors.New("invalid workspace: slug must be 2 to 32 lowercase letters, digits, or hyphens starting with a letter and cannot be api, assets, auth, mcp, w, or webhooks; name is required")
	ErrWorkspaceExists   = errors.New("a workspace with this slug already exists")
)

var teamKey = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,5}$`)

// workspaceSlug matches a workspace slug. Pages live under /<slug>/, so the
// reserved slugs are the server's own top-level paths.
var (
	workspaceSlug = regexp.MustCompile(`^[a-z][a-z0-9-]{1,31}$`)
	reservedSlugs = []string{"api", "assets", "auth", "mcp", "w", "webhooks"}
)

var Statuses = []Status{StatusTriage, StatusBacklog, StatusTodo, StatusAgentReady, StatusInProgress, StatusInReview, StatusDone, StatusCanceled}

//go:embed schema.sql
var schema string

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
	Status        *Status `json:"status,omitempty" jsonschema:"filter by status: triage, todo, agent_ready, in_progress, in_review, done, or canceled"`
	Type          *string `json:"type,omitempty" jsonschema:"filter by type: spec, feature, bug, improvement, chore; empty string matches untyped issues"`
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
	Status      *Status `json:"status,omitempty" jsonschema:"initial status: triage (default), todo, agent_ready, in_progress, in_review, done, or canceled"`
	Type        string  `json:"type,omitempty" jsonschema:"optional type: spec, feature, bug, improvement, or chore; empty string means no type"`
	Project     string  `json:"project,omitempty" jsonschema:"existing Project name; empty string means no Project"`
	Parent      string  `json:"parent,omitempty" jsonschema:"existing parent issue identifier; empty string means no parent"`
}

type UpdateIssue struct {
	Priority    *int    `json:"priority,omitempty" jsonschema:"0 clears priority, 1 urgent, 2 high, 3 medium, 4 low; omitted leaves unchanged"`
	Identifier  string  `json:"identifier" jsonschema:"issue identifier, such as AL-1"`
	Title       *string `json:"title,omitempty" jsonschema:"new nonempty title; omitted leaves unchanged"`
	Description *string `json:"description,omitempty" jsonschema:"new description; omitted leaves unchanged"`
	Status      *Status `json:"status,omitempty" jsonschema:"new status: triage, todo, agent_ready, in_progress, in_review, done, or canceled; omitted leaves unchanged"`
	Type        *string `json:"type,omitempty" jsonschema:"new type: spec, feature, bug, improvement, or chore; empty string clears; omitted leaves unchanged"`
	Project     *string `json:"project,omitempty" jsonschema:"existing Project name; empty string clears; omitted leaves unchanged"`
	Parent      *string `json:"parent,omitempty" jsonschema:"existing parent issue identifier; empty string clears; omitted leaves unchanged; cannot be self"`
}

type Project struct {
	Emoji       string `json:"emoji" jsonschema:"emoji icon; empty uses the default icon"`
	Name        string `json:"name" jsonschema:"unique Project name"`
	Description string `json:"description" jsonschema:"the Project outcome"`
	Archived    bool   `json:"archived"`
}

type UpdateProject struct {
	Emoji       *string `json:"emoji,omitempty" jsonschema:"Unicode emoji icon; omitted leaves unchanged, empty clears"`
	Name        string  `json:"name" jsonschema:"existing Project name; cannot be renamed"`
	Description *string `json:"description,omitempty" jsonschema:"new Project outcome; omitted leaves unchanged"`
	Archived    *bool   `json:"archived,omitempty" jsonschema:"archive or unarchive the Project; omitted leaves unchanged"`
}

type Team struct {
	Emoji      string `json:"emoji" jsonschema:"emoji icon; empty uses the default icon"`
	Key        string `json:"key" jsonschema:"identifier prefix; fixed once created"`
	Name       string `json:"name"`
	NextNumber int    `json:"next_number" jsonschema:"number the team's next issue gets"`
}

type CreateTeam struct {
	Emoji       string `json:"emoji,omitempty" jsonschema:"optional Unicode emoji icon; empty uses the default icon"`
	Key         string `json:"key" jsonschema:"2 to 6 uppercase letters or digits starting with a letter, such as AL; cannot be changed later"`
	Name        string `json:"name" jsonschema:"required nonempty team name"`
	StartNumber *int   `json:"start_number,omitempty" jsonschema:"number the team's first issue gets, at least 1; omitted starts at 1"`
}

// Workspace groups teams, Projects, and labels. Its slug names it in page,
// REST, and MCP paths and cannot change.
type Workspace struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
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

// Service reads and writes the database. A Service returned by InWorkspace
// reads and writes one workspace; Open returns one that manages workspaces,
// backups, and GitHub sync.
type Service struct {
	db        *sql.DB
	backupDir string
	now       func() time.Time
	workspace int64
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

// InWorkspace returns a Service that reads and writes only the workspace with
// this slug; other workspaces' records are not found through it.
func (s *Service) InWorkspace(ctx context.Context, slug string) (*Service, error) {
	scoped := *s
	err := s.db.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE slug = ?`, slug).Scan(&scoped.workspace)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrWorkspaceNotFound
	}
	if err != nil {
		return nil, err
	}
	return &scoped, nil
}

func (s *Service) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT slug, name FROM workspaces ORDER BY slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	workspaces := []Workspace{}
	for rows.Next() {
		var workspace Workspace
		if err := rows.Scan(&workspace.Slug, &workspace.Name); err != nil {
			return nil, err
		}
		workspaces = append(workspaces, workspace)
	}
	return workspaces, rows.Err()
}

// CreateWorkspace adds an empty workspace. Its slug cannot be changed later.
func (s *Service) CreateWorkspace(ctx context.Context, input Workspace) (Workspace, error) {
	workspace := Workspace{Slug: input.Slug, Name: strings.TrimSpace(input.Name)}
	if !workspaceSlug.MatchString(workspace.Slug) || slices.Contains(reservedSlugs, workspace.Slug) || workspace.Name == "" {
		return Workspace{}, ErrInvalidWorkspace
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO workspaces(slug, name) VALUES (?, ?) ON CONFLICT(slug) DO NOTHING`, workspace.Slug, workspace.Name)
	if err != nil {
		return Workspace{}, err
	}
	if inserted, _ := result.RowsAffected(); inserted == 0 {
		return Workspace{}, ErrWorkspaceExists
	}
	return workspace, nil
}

// RenameWorkspace changes the name of the workspace with this slug.
func (s *Service) RenameWorkspace(ctx context.Context, slug, name string) (Workspace, error) {
	workspace := Workspace{Slug: slug, Name: strings.TrimSpace(name)}
	if workspace.Name == "" {
		return Workspace{}, ErrInvalidWorkspace
	}
	result, err := s.db.ExecContext(ctx, `UPDATE workspaces SET name = ? WHERE slug = ?`, workspace.Name, slug)
	if err != nil {
		return Workspace{}, err
	}
	if updated, _ := result.RowsAffected(); updated == 0 {
		return Workspace{}, ErrWorkspaceNotFound
	}
	return workspace, nil
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
		switch *issueType {
		case "", "spec", "feature", "bug", "improvement", "chore":
		default:
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
		var issue Issue
		var id int64
		if err := rows.Scan(&id, &issue.Identifier, &issue.Title, &issue.Status, &issue.Type, &issue.Project, &issue.Parent, &issue.UpdatedAt, &issue.Description, &issue.CreatedAt, &issue.Priority); err != nil {
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
	var issue Issue
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT `+issueColumns+` FROM issues WHERE workspace_id = ? AND identifier = ?`, s.workspace, identifier).Scan(
		&id, &issue.Identifier, &issue.Title, &issue.Status, &issue.Type, &issue.Project, &issue.Parent, &issue.UpdatedAt, &issue.Description, &issue.CreatedAt, &issue.Priority)
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

func (s *Service) ListProjects(ctx context.Context, includeArchived bool) ([]Project, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT name, description, archived, emoji FROM projects WHERE workspace_id = ? AND (? OR NOT archived) ORDER BY name`, s.workspace, includeArchived)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := []Project{}
	for rows.Next() {
		var project Project
		if err := rows.Scan(&project.Name, &project.Description, &project.Archived, &project.Emoji); err != nil {
			return nil, err
		}
		projects = append(projects, project)
	}
	return projects, rows.Err()
}

func (s *Service) CreateProject(ctx context.Context, name, description string) (Project, error) {
	_, err := s.db.ExecContext(ctx, `INSERT INTO projects(workspace_id, name, description) VALUES (?, ?, ?)`, s.workspace, name, description)
	return Project{Name: name, Description: description}, err
}

func (s *Service) UpdateProject(ctx context.Context, input UpdateProject) (Project, error) {
	if input.Emoji != nil && !validEmoji(*input.Emoji) {
		return Project{}, ErrInvalidEmoji
	}
	var project Project
	err := s.db.QueryRowContext(ctx, `UPDATE projects SET description = COALESCE(?, description), archived = COALESCE(?, archived), emoji = COALESCE(?, emoji)
		WHERE workspace_id = ? AND name = ? RETURNING name, description, archived, emoji`, input.Description, input.Archived, input.Emoji, s.workspace, input.Name).Scan(&project.Name, &project.Description, &project.Archived, &project.Emoji)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrProjectNotFound
	}
	return project, err
}

func (s *Service) ListTeams(ctx context.Context) ([]Team, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, name, next_number, emoji FROM teams WHERE workspace_id = ? ORDER BY key`, s.workspace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	teams := []Team{}
	for rows.Next() {
		var team Team
		if err := rows.Scan(&team.Key, &team.Name, &team.NextNumber, &team.Emoji); err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}
	return teams, rows.Err()
}

// CreateTeam adds a team whose numbering starts at StartNumber, or 1 when it
// is omitted. A team's key, name, and numbering cannot be changed.
func (s *Service) CreateTeam(ctx context.Context, input CreateTeam) (Team, error) {
	team := Team{Key: input.Key, Name: strings.TrimSpace(input.Name), NextNumber: 1, Emoji: input.Emoji}
	if !validEmoji(input.Emoji) {
		return Team{}, ErrInvalidEmoji
	}
	if input.StartNumber != nil {
		team.NextNumber = *input.StartNumber
	}
	if !teamKey.MatchString(team.Key) || team.Name == "" || team.NextNumber < 1 {
		return Team{}, ErrInvalidTeam
	}
	result, err := s.db.ExecContext(ctx, `INSERT INTO teams(workspace_id, key, name, next_number, emoji) VALUES (?, ?, ?, ?, ?) ON CONFLICT(workspace_id, key) DO NOTHING`,
		s.workspace, team.Key, team.Name, team.NextNumber, team.Emoji)
	if err != nil {
		return Team{}, err
	}
	if inserted, _ := result.RowsAffected(); inserted == 0 {
		return Team{}, ErrTeamExists
	}
	return team, nil
}
