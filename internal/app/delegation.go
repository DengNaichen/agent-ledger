package app

import (
	"context"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

var ErrInvalidDelegation = newError(KindInvalid, "valid agent session context is required; use the repository delegate hook")
var ErrDelegationRequiresAssignee = newError(KindInvalid, "assign a human assignee before delegating this issue")

// ExecutionContext is client-reported provenance, never an authorization identity.
type ExecutionContext struct {
	Agent     string `json:"agent" jsonschema:"codex or claude; supplied by the hook host adapter"`
	SessionID string `json:"session_id" jsonschema:"host hook session ID; subagents may share their parent's ID"`
	AgentID   string `json:"agent_id,omitempty" jsonschema:"host-provided subagent ID; omitted when unavailable"`
	AgentType string `json:"agent_type,omitempty" jsonschema:"host-provided agent profile; not an identity"`
	Hostname  string `json:"hostname" jsonschema:"execution machine hostname; not an address or credential"`
}

type DelegateIssue struct {
	Identifier    string            `json:"identifier" jsonschema:"issue identifier to work on"`
	ClientContext *ExecutionContext `json:"client_context,omitempty" jsonschema:"injected by the repository PreToolUse hook; do not supply or guess manually"`
}

type Delegation struct {
	ID int64 `json:"id"`
	ExecutionContext
	Actor     *Identity `json:"actor"`
	CreatedAt string    `json:"created_at"`
}

func validateExecutionContext(input *ExecutionContext) error {
	if input == nil || (input.Agent != "codex" && input.Agent != "claude") {
		return ErrInvalidDelegation
	}
	for _, field := range []struct {
		value    string
		limit    int
		required bool
	}{
		{input.SessionID, 256, true}, {input.AgentID, 256, false},
		{input.AgentType, 128, false}, {input.Hostname, 253, true},
	} {
		if (field.required && field.value == "") || len(field.value) > field.limit ||
			field.value != strings.TrimSpace(field.value) || !utf8.ValidString(field.value) ||
			strings.ContainsFunc(field.value, unicode.IsControl) {
			return ErrInvalidDelegation
		}
	}
	return nil
}

// Delegate registers a participant and starts progress atomically, without claiming a lock.
func (s *Service) Delegate(ctx context.Context, input DelegateIssue) (Delegation, error) {
	if err := s.checkAccess(ctx); err != nil {
		return Delegation{}, err
	}
	if err := validateExecutionContext(input.ClientContext); err != nil {
		return Delegation{}, err
	}
	id, err := s.issueID(ctx, input.Identifier)
	if err != nil {
		return Delegation{}, err
	}
	if !id.Valid {
		return Delegation{}, ErrNotFound
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Delegation{}, err
	}
	defer tx.Rollback()
	var assigned bool
	var status Status
	if err = tx.QueryRowContext(ctx, `SELECT assignee_subject IS NOT NULL, status FROM issues WHERE id=?`, id).Scan(&assigned, &status); err != nil {
		return Delegation{}, err
	}
	if !assigned {
		return Delegation{}, ErrDelegationRequiresAssignee
	}
	now := s.now().UTC().Format(time.RFC3339Nano)
	c := input.ClientContext
	_, err = tx.ExecContext(ctx, `INSERT INTO issue_delegations(issue_id,agent,session_id,agent_id,agent_type,hostname,actor_subject,created_at)
 VALUES(?,?,?,?,?,?,NULLIF(?,''),?) ON CONFLICT(issue_id,agent,session_id,agent_id,hostname) DO NOTHING`,
		id, c.Agent, c.SessionID, c.AgentID, c.AgentType, c.Hostname, requestIdentity(ctx).Subject, now)
	if err != nil {
		return Delegation{}, err
	}
	if status != StatusInProgress {
		if err := s.changeStatus(ctx, tx, input.Identifier, StatusInProgress, "", now); err != nil {
			return Delegation{}, err
		}
	}

	d, err := scanDelegation(tx.QueryRowContext(ctx, delegationColumns+` WHERE d.issue_id=? AND d.agent=? AND d.session_id=? AND d.agent_id=? AND d.hostname=?`,
		id, c.Agent, c.SessionID, c.AgentID, c.Hostname))
	if err != nil {
		return Delegation{}, err
	}
	return d, tx.Commit()
}

const delegationColumns = `SELECT d.id,d.agent,d.session_id,d.agent_id,d.agent_type,d.hostname,d.created_at,
 COALESCE(u.subject,''),COALESCE(u.name,''),COALESCE(u.email,'')
 FROM issue_delegations d LEFT JOIN users u ON u.subject=d.actor_subject`

func scanDelegation(row interface{ Scan(...any) error }) (Delegation, error) {
	var d Delegation
	var actor Identity
	err := row.Scan(&d.ID, &d.Agent, &d.SessionID, &d.AgentID, &d.AgentType, &d.Hostname, &d.CreatedAt, &actor.Subject, &actor.Name, &actor.Email)
	if actor.Subject != "" {
		d.Actor = &actor
	}
	return d, err
}

// The caller has already passed the issue's workspace and team boundary.
func (s *Service) delegations(ctx context.Context, id int64) ([]Delegation, error) {
	rows, err := s.db.QueryContext(ctx, delegationColumns+` WHERE d.issue_id=? ORDER BY d.id`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	history := []Delegation{}
	for rows.Next() {
		d, err := scanDelegation(rows)
		if err != nil {
			return nil, err
		}
		history = append(history, d)
	}
	return history, rows.Err()
}
