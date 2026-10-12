package app

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// Only public teams or explicit members can read team content. Workspace
// administrators must join a private team before accessing its issues.
const visibleTeamIDs = `SELECT t.id FROM teams t WHERE ($viewer='' OR NOT t.private OR EXISTS(SELECT 1 FROM team_members tm WHERE tm.team_id=t.id AND tm.subject=$viewer))`
const visibleProject = `(NOT EXISTS(SELECT 1 FROM project_teams WHERE project_id=p.id) OR EXISTS(SELECT 1 FROM project_teams WHERE project_id=p.id AND team_id IN (` + visibleTeamIDs + `)))`

var sqlParameters = regexp.MustCompile(`\?([0-9]*)`)

// viewerSQL binds the trusted subject alongside existing positional parameters.
// Number the authored SQL placeholders first, so a viewer predicate inserted in
// the SELECT list cannot shift their indexes (including repeated ?1 references).
func viewerSQL(ctx context.Context, query string, args ...any) (string, []any) {
	index := 0
	query = sqlParameters.ReplaceAllStringFunc(query, func(value string) string {
		if len(value) > 1 {
			n, _ := strconv.Atoi(value[1:])
			if n > index {
				index = n
			}
			return value
		}
		index++
		return "?" + strconv.Itoa(index)
	})
	query = strings.ReplaceAll(query, "$viewer", "?"+strconv.Itoa(len(args)+1))
	return query, append(args, requestIdentity(ctx).Subject)
}

var ErrTeamOwnerRequired = newError(KindForbidden, "team owner or workspace administrator is required")
var ErrPrivateJoinConfirmation = newError(KindInvalid, "confirm private team access before joining")
var ErrInvalidTeamMember = newError(KindInvalid, "team role must be member or owner")

type TeamMember struct {
	WorkspaceAdmin bool `json:"workspace_admin"`
	Identity
	Role   string `json:"role"`
	Status string `json:"status"`
}
type ChangeTeamMember struct {
	Key     string `json:"key" jsonschema:"team key"`
	Subject string `json:"subject" jsonschema:"workspace member subject"`
	Role    string `json:"role,omitempty" jsonschema:"member or owner; required unless removing"`
	Remove  bool   `json:"remove,omitempty" jsonschema:"remove this member"`
}
type JoinTeam struct {
	Key            string `json:"key" jsonschema:"team key"`
	Leave          bool   `json:"leave,omitempty" jsonschema:"leave instead of joining"`
	ConfirmPrivate bool   `json:"confirm_private,omitempty" jsonschema:"administrator explicitly confirms access to a private team"`
}

// TeamDirectory includes private metadata for administrators, never issue data.
func (s *Service) TeamDirectory(ctx context.Context) ([]Team, error) { return s.teams(ctx, true) }
func (s *Service) teams(ctx context.Context, directory bool) ([]Team, error) {
	access, err := s.Access(ctx)
	if err != nil {
		return nil, err
	}
	query := `SELECT t.key,t.name,t.next_number,t.emoji,t.private,COALESCE(tm.role,'') FROM teams t LEFT JOIN team_members tm ON tm.team_id=t.id AND tm.subject=$viewer WHERE t.workspace_id=?`
	if !directory || access.Role != "admin" {
		query += ` AND t.id IN (` + visibleTeamIDs + `)`
	}
	query, args := viewerSQL(ctx, query+` ORDER BY t.key`, s.workspace)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Team{}
	for rows.Next() {
		var team Team
		var role string
		if err := rows.Scan(&team.Key, &team.Name, &team.NextNumber, &team.Emoji, &team.Private, &role); err != nil {
			return nil, err
		}
		if access.Enabled {
			team.MembershipEnabled = true
			team.Joined = role != ""
			team.Accessible = !team.Private || team.Joined
			team.CanManage = access.Role == "admin" || role == "owner"
			// A directory entry must not expose private issue counts through numbering.
			if !team.Accessible {
				team.NextNumber = 0
			}
		}
		result = append(result, team)
	}
	return result, rows.Err()
}
func (s *Service) teamSettings(ctx context.Context, key string) (Team, error) {
	teams, err := s.TeamDirectory(ctx)
	if err != nil {
		return Team{}, err
	}
	for _, team := range teams {
		if team.Key == key {
			return team, nil
		}
	}
	return Team{}, ErrTeamNotFound
}
func (s *Service) ListTeamMembers(ctx context.Context, key string) ([]TeamMember, error) {
	team, err := s.teamSettings(ctx, key)
	if err != nil {
		return nil, err
	}
	if !team.MembershipEnabled {
		return nil, ErrCloudRequired
	}
	query := `SELECT u.subject,u.name,u.email,CASE WHEN wm.role='admin' THEN 'owner' ELSE tm.role END,wm.status,wm.role='admin' FROM team_members tm JOIN teams t ON t.id=tm.team_id JOIN users u ON u.subject=tm.subject JOIN workspace_members wm ON wm.workspace_id=t.workspace_id AND wm.subject=tm.subject WHERE t.workspace_id=? AND t.key=? ORDER BY u.name,u.subject`
	rows, err := s.db.QueryContext(ctx, query, s.workspace, key)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []TeamMember{}
	for rows.Next() {
		var m TeamMember
		if err := rows.Scan(&m.Subject, &m.Name, &m.Email, &m.Role, &m.Status, &m.WorkspaceAdmin); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
func (s *Service) ChangeTeamMember(ctx context.Context, input ChangeTeamMember) error {
	team, err := s.teamSettings(ctx, input.Key)
	if err != nil {
		return err
	}
	if !team.MembershipEnabled {
		return ErrCloudRequired
	}
	if !team.CanManage {
		return ErrTeamOwnerRequired
	}
	if !input.Remove && input.Role != "member" && input.Role != "owner" {
		return ErrInvalidTeamMember
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	if err := tx.QueryRowContext(ctx, `SELECT id FROM teams WHERE workspace_id=? AND key=?`, s.workspace, input.Key).Scan(&id); err != nil {
		return err
	}
	if input.Remove {
		_, err = tx.ExecContext(ctx, `DELETE FROM team_members WHERE team_id=? AND subject=?`, id, input.Subject)
	} else {
		var active bool
		err = tx.QueryRowContext(ctx, `SELECT status='active' FROM workspace_members WHERE workspace_id=? AND subject=?`, s.workspace, input.Subject).Scan(&active)
		if errors.Is(err, sql.ErrNoRows) || err == nil && !active {
			return ErrMemberNotFound
		}
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO team_members(team_id,subject,role) VALUES(?,?,?) ON CONFLICT(team_id,subject) DO UPDATE SET role=excluded.role`, id, input.Subject, input.Role)
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}
func (s *Service) JoinTeam(ctx context.Context, input JoinTeam) (Team, error) {
	team, err := s.teamSettings(ctx, input.Key)
	if err != nil {
		return Team{}, err
	}
	if !team.MembershipEnabled {
		return Team{}, ErrCloudRequired
	}
	if input.Leave {
		_, err = s.db.ExecContext(ctx, `DELETE FROM team_members WHERE team_id=(SELECT id FROM teams WHERE workspace_id=? AND key=?) AND subject=?`, s.workspace, input.Key, requestIdentity(ctx).Subject)
		if err != nil {
			return Team{}, err
		}
		team.Joined = false
		team.Accessible = !team.Private
		access, err := s.Access(ctx)
		if err != nil {
			return Team{}, err
		}
		team.CanManage = access.Role == "admin"
		if !team.Accessible {
			team.NextNumber = 0
		}
		return team, nil
	}
	if team.Private && !team.Joined {
		access, err := s.Access(ctx)
		if err != nil {
			return Team{}, err
		}
		if access.Role != "admin" {
			return Team{}, ErrTeamOwnerRequired
		}
		if !input.ConfirmPrivate {
			return Team{}, ErrPrivateJoinConfirmation
		}
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO team_members(team_id,subject,role) SELECT id,?,'member' FROM teams WHERE workspace_id=? AND key=? ON CONFLICT(team_id,subject) DO NOTHING`, requestIdentity(ctx).Subject, s.workspace, input.Key)
	if err != nil {
		return Team{}, err
	}
	return s.teamSettings(ctx, input.Key)
}
