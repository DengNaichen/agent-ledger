package app

import (
	"context"
	"database/sql"
)

var ErrInvalidAssignee = newError(KindInvalid, "assignee must be an active workspace member with team access")
var ErrInvalidAssigneeFilter = newError(KindInvalid, "assignee and assigned_to_me cannot be combined")

type AssignmentChange struct {
	Actor     *Identity `json:"actor"`
	From      *Identity `json:"from"`
	To        *Identity `json:"to"`
	CreatedAt string    `json:"created_at"`
}

type ListAssignees struct {
	Team string `json:"team" jsonschema:"required visible team key"`
}

// Candidates reveal membership only after the caller passes the team boundary.
func (s *Service) ListAssignees(ctx context.Context, input ListAssignees) ([]Identity, error) {
	if err := s.checkAccess(ctx); err != nil {
		return nil, err
	}
	if input.Team == "" {
		return nil, ErrTeamRequired
	}
	team, err := s.teamID(ctx, input.Team)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT u.subject,u.name,u.email FROM users u
 JOIN workspace_members m ON m.subject=u.subject JOIN teams t ON t.workspace_id=m.workspace_id
 WHERE t.id=? AND m.status='active' AND (NOT t.private OR EXISTS(SELECT 1 FROM team_members tm WHERE tm.team_id=t.id AND tm.subject=u.subject)) ORDER BY u.name,u.email,u.subject`, team)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	candidates := []Identity{}
	for rows.Next() {
		var user Identity
		if err = rows.Scan(&user.Subject, &user.Name, &user.Email); err != nil {
			return nil, err
		}
		candidates = append(candidates, user)
	}
	return candidates, rows.Err()
}

func (s *Service) validateAssignee(ctx context.Context, tx *sql.Tx, team int64, subject string) error {
	if subject == "" {
		return nil
	}
	var eligible bool
	err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM workspace_members m JOIN teams t ON t.workspace_id=m.workspace_id
 WHERE t.id=? AND m.subject=? AND m.status='active' AND (NOT t.private OR EXISTS(SELECT 1 FROM team_members tm WHERE tm.team_id=t.id AND tm.subject=m.subject)))`, team, subject).Scan(&eligible)
	if err != nil {
		return err
	}
	if !eligible {
		return ErrInvalidAssignee
	}
	return nil
}

func recordAssignment(ctx context.Context, tx *sql.Tx, id int64, from, to, now string) error {
	if from == to {
		return nil
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO issue_assignments(issue_id,actor_subject,from_subject,to_subject,created_at) VALUES(?,NULLIF(?,''),NULLIF(?,''),NULLIF(?,''),?)`, id, requestIdentity(ctx).Subject, from, to, now)
	return err
}

func (s *Service) assignmentHistory(ctx context.Context, id int64) ([]AssignmentChange, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT COALESCE(a.subject,''),COALESCE(a.name,''),COALESCE(a.email,''),
 COALESCE(f.subject,''),COALESCE(f.name,''),COALESCE(f.email,''),COALESCE(t.subject,''),COALESCE(t.name,''),COALESCE(t.email,''),h.created_at
 FROM issue_assignments h LEFT JOIN users a ON a.subject=h.actor_subject LEFT JOIN users f ON f.subject=h.from_subject LEFT JOIN users t ON t.subject=h.to_subject WHERE h.issue_id=? ORDER BY h.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	history := []AssignmentChange{}
	for rows.Next() {
		var a, f, t Identity
		var change AssignmentChange
		if err = rows.Scan(&a.Subject, &a.Name, &a.Email, &f.Subject, &f.Name, &f.Email, &t.Subject, &t.Name, &t.Email, &change.CreatedAt); err != nil {
			return nil, err
		}
		if a.Subject != "" {
			change.Actor = &a
		}
		if f.Subject != "" {
			change.From = &f
		}
		if t.Subject != "" {
			change.To = &t
		}
		history = append(history, change)
	}
	return history, rows.Err()
}
