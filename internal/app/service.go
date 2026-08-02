package app

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Status string

const (
	StatusTriage     Status = "triage"
	StatusReady      Status = "ready"
	StatusInProgress Status = "in_progress"
	StatusInReview   Status = "in_review"
	StatusDone       Status = "done"
)

var (
	ErrNotFound      = errors.New("issue not found")
	ErrInvalidTitle  = errors.New("title is required")
	ErrInvalidStatus = errors.New("invalid status")
)

var Statuses = []Status{StatusTriage, StatusReady, StatusInProgress, StatusInReview, StatusDone}

//go:embed schema.sql
var schema string

type Issue struct {
	Identifier  string `json:"identifier"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Status      Status `json:"status"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

type CreateIssue struct {
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Service struct {
	db  *sql.DB
	now func() time.Time
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
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, err
	}

	return &Service{db: db, now: time.Now}, nil
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

func (s *Service) List(ctx context.Context, status Status) ([]Issue, error) {
	query := `SELECT identifier, title, description, status, created_at, updated_at FROM issues`
	args := []any{}
	if status != "" {
		if !ValidStatus(status) {
			return nil, ErrInvalidStatus
		}
		query += ` WHERE status = ?`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC, id DESC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	issues := []Issue{}
	for rows.Next() {
		var issue Issue
		if err := rows.Scan(&issue.Identifier, &issue.Title, &issue.Description, &issue.Status, &issue.CreatedAt, &issue.UpdatedAt); err != nil {
			return nil, err
		}
		issues = append(issues, issue)
	}
	return issues, rows.Err()
}

func (s *Service) Create(ctx context.Context, input CreateIssue) (Issue, error) {
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	if input.Title == "" {
		return Issue{}, ErrInvalidTitle
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Issue{}, err
	}
	defer tx.Rollback()

	now := s.now().UTC().Format(time.RFC3339Nano)
	result, err := tx.ExecContext(ctx, `
		INSERT INTO issues(title, description, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)`, input.Title, input.Description, StatusTriage, now, now)
	if err != nil {
		return Issue{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Issue{}, err
	}
	identifier := fmt.Sprintf("AL-%d", id)
	if _, err := tx.ExecContext(ctx, `UPDATE issues SET identifier = ? WHERE id = ?`, identifier, id); err != nil {
		return Issue{}, err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO events(issue_id, kind, detail, created_at) VALUES (?, 'created', ?, ?)`, id, string(StatusTriage), now); err != nil {
		return Issue{}, err
	}
	if err := tx.Commit(); err != nil {
		return Issue{}, err
	}
	return s.Get(ctx, identifier)
}

func (s *Service) Get(ctx context.Context, identifier string) (Issue, error) {
	var issue Issue
	err := s.db.QueryRowContext(ctx, `
		SELECT identifier, title, description, status, created_at, updated_at
		FROM issues WHERE identifier = ?`, strings.ToUpper(strings.TrimSpace(identifier))).Scan(
		&issue.Identifier, &issue.Title, &issue.Description, &issue.Status, &issue.CreatedAt, &issue.UpdatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return Issue{}, ErrNotFound
	}
	return issue, err
}

func (s *Service) Move(ctx context.Context, identifier string, status Status) (Issue, error) {
	if !ValidStatus(status) {
		return Issue{}, ErrInvalidStatus
	}
	identifier = strings.ToUpper(strings.TrimSpace(identifier))

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Issue{}, err
	}
	defer tx.Rollback()

	var id int64
	var previous Status
	if err := tx.QueryRowContext(ctx, `SELECT id, status FROM issues WHERE identifier = ?`, identifier).Scan(&id, &previous); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Issue{}, ErrNotFound
		}
		return Issue{}, err
	}
	if previous == status {
		_ = tx.Rollback()
		return s.Get(ctx, identifier)
	}

	now := s.now().UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx, `UPDATE issues SET status = ?, updated_at = ? WHERE id = ?`, status, now, id); err != nil {
		return Issue{}, err
	}
	detail := fmt.Sprintf("%s -> %s", previous, status)
	if _, err := tx.ExecContext(ctx, `INSERT INTO events(issue_id, kind, detail, created_at) VALUES (?, 'status_changed', ?, ?)`, id, detail, now); err != nil {
		return Issue{}, err
	}
	if err := tx.Commit(); err != nil {
		return Issue{}, err
	}
	return s.Get(ctx, identifier)
}
