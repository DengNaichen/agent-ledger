package app

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var ErrInvalidWorkLog = newError(KindInvalid, "work log requires kind progress or decision, a single-line summary (1–240 characters), and a body (1–65536 bytes)")
var ErrInvalidWorkLogQuery = newError(KindInvalid, "work log query requires kind progress or decision (or empty for all), limit 1–100, and nonnegative before")

type WorkLogSummary struct {
	ID        int64     `json:"id"`
	Kind      string    `json:"kind"`
	Summary   string    `json:"summary"`
	Actor     *Identity `json:"actor"`
	CreatedAt string    `json:"created_at"`
}

type WorkLog struct {
	WorkLogSummary
	Body string `json:"body"`
}

type AddWorkLog struct {
	Identifier string `json:"identifier" jsonschema:"issue identifier"`
	Kind       string `json:"kind,omitempty" jsonschema:"progress (default) or decision; use decision only for a settled choice with its rationale"`
	Summary    string `json:"summary" jsonschema:"required single-line summary, at most 240 characters"`
	Body       string `json:"body" jsonschema:"required Markdown record, at most 65536 bytes; for decisions explain context, choice and reasons"`
}

type ListWorkLogs struct {
	Identifier string `json:"identifier" jsonschema:"issue identifier"`
	Kind       string `json:"kind,omitempty" jsonschema:"progress or decision; omitted lists both"`
	Limit      int    `json:"limit,omitempty" jsonschema:"page size 1–100; omitted or zero defaults to 20"`
	Before     int64  `json:"before,omitempty" jsonschema:"next_before from the previous page; omitted or zero starts with the newest record"`
}

type GetWorkLog struct {
	Identifier string `json:"identifier" jsonschema:"issue identifier"`
	ID         int64  `json:"id" jsonschema:"work log id from decisions or list_work_logs"`
}

type WorkLogPage struct {
	WorkLogs   []WorkLogSummary `json:"work_logs"`
	NextBefore int64            `json:"next_before" jsonschema:"cursor for older records; zero means no more records"`
}

func (s *Service) AddWorkLog(ctx context.Context, input AddWorkLog) (WorkLog, error) {
	if err := s.checkAccess(ctx); err != nil {
		return WorkLog{}, err
	}
	if input.Kind == "" {
		input.Kind = "progress"
	}
	input.Summary = strings.TrimSpace(input.Summary)
	if (input.Kind != "progress" && input.Kind != "decision") || input.Summary == "" ||
		utf8.RuneCountInString(input.Summary) > 240 || !utf8.ValidString(input.Summary) || strings.ContainsFunc(input.Summary, unicode.IsControl) ||
		strings.TrimSpace(input.Body) == "" || len(input.Body) > 65536 || !utf8.ValidString(input.Body) {
		return WorkLog{}, ErrInvalidWorkLog
	}
	_, issueID, err := s.get(ctx, input.Identifier)
	if err != nil {
		return WorkLog{}, err
	}
	createdAt := s.now().UTC().Format(time.RFC3339Nano)
	result, err := s.db.ExecContext(ctx, `INSERT INTO work_logs(issue_id,kind,summary,body,actor_subject,created_at) VALUES(?,?,?,?,NULLIF(?,''),?)`,
		issueID, input.Kind, input.Summary, input.Body, requestIdentity(ctx).Subject, createdAt)
	if err != nil {
		return WorkLog{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return WorkLog{}, err
	}
	return s.GetWorkLog(ctx, GetWorkLog{Identifier: input.Identifier, ID: id})
}

const workLogColumns = `SELECT w.id,w.kind,w.summary,w.created_at,COALESCE(u.subject,''),COALESCE(u.name,''),COALESCE(u.email,'')`
const workLogFrom = ` FROM work_logs w LEFT JOIN users u ON u.subject=w.actor_subject`

func scanWorkLog(row interface{ Scan(...any) error }, body *string) (WorkLogSummary, error) {
	var log WorkLogSummary
	var actor Identity
	fields := []any{&log.ID, &log.Kind, &log.Summary, &log.CreatedAt, &actor.Subject, &actor.Name, &actor.Email}
	if body != nil {
		fields = append(fields, body)
	}
	err := row.Scan(fields...)
	if actor.Subject != "" {
		log.Actor = &actor
	}
	return log, err
}

func (s *Service) GetWorkLog(ctx context.Context, input GetWorkLog) (WorkLog, error) {
	if err := s.checkAccess(ctx); err != nil {
		return WorkLog{}, err
	}
	_, issueID, err := s.get(ctx, input.Identifier)
	if err != nil {
		return WorkLog{}, err
	}
	var log WorkLog
	log.WorkLogSummary, err = scanWorkLog(s.db.QueryRowContext(ctx, workLogColumns+`,w.body`+workLogFrom+` WHERE w.issue_id=? AND w.id=?`, issueID, input.ID), &log.Body)
	if errors.Is(err, sql.ErrNoRows) {
		return WorkLog{}, ErrNotFound
	}
	return log, err
}

func (s *Service) ListWorkLogs(ctx context.Context, input ListWorkLogs) (WorkLogPage, error) {
	if err := s.checkAccess(ctx); err != nil {
		return WorkLogPage{}, err
	}
	if input.Limit == 0 {
		input.Limit = 20
	}
	if (input.Kind != "" && input.Kind != "progress" && input.Kind != "decision") || input.Limit < 1 || input.Limit > 100 || input.Before < 0 {
		return WorkLogPage{}, ErrInvalidWorkLogQuery
	}
	_, issueID, err := s.get(ctx, input.Identifier)
	if err != nil {
		return WorkLogPage{}, err
	}
	where := ` WHERE w.issue_id=?`
	args := []any{issueID}
	if input.Kind != "" {
		where += ` AND w.kind=?`
		args = append(args, input.Kind)
	}
	if input.Before > 0 {
		where += ` AND w.id<?`
		args = append(args, input.Before)
	}
	args = append(args, input.Limit+1)
	logs, err := s.workLogSummaries(ctx, where+` ORDER BY w.id DESC LIMIT ?`, args...)
	if err != nil {
		return WorkLogPage{}, err
	}
	page := WorkLogPage{WorkLogs: logs}
	if len(logs) > input.Limit {
		page.WorkLogs = logs[:input.Limit]
		page.NextBefore = page.WorkLogs[input.Limit-1].ID
	}
	return page, nil
}

// The caller must first check the issue's workspace and team access.
func (s *Service) workLogSummaries(ctx context.Context, where string, args ...any) ([]WorkLogSummary, error) {
	rows, err := s.db.QueryContext(ctx, workLogColumns+workLogFrom+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	logs := []WorkLogSummary{}
	for rows.Next() {
		log, err := scanWorkLog(rows, nil)
		if err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}
	return logs, rows.Err()
}
