package app

import (
	"context"
	"database/sql"
	"errors"
	"modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
	"slices"
	"strings"
)

type Project struct {
	ID          int64    `json:"id" jsonschema:"stable Project id"`
	Teams       []string `json:"teams" jsonschema:"associated Team keys; empty only for historical unassigned Projects"`
	Emoji       string   `json:"emoji" jsonschema:"emoji icon; empty uses the default icon"`
	Name        string   `json:"name" jsonschema:"unique Project name"`
	Description string   `json:"description" jsonschema:"the Project outcome"`
	Archived    bool     `json:"archived"`
}

type ProjectRef struct {
	ID   int64  `json:"id,omitempty" jsonschema:"stable Project id; provide id or name; when both are supplied they must match"`
	Name string `json:"name,omitempty" jsonschema:"current Project name"`
}

type UpdateProject struct {
	NewName     *string   `json:"new_name,omitempty" jsonschema:"new Project name; trimmed, nonempty, and unique in this workspace; omitted leaves unchanged"`
	ID          int64     `json:"id,omitempty" jsonschema:"stable Project id; provide id or name; both must match"`
	Name        string    `json:"name,omitempty" jsonschema:"current Project name used for lookup; use new_name to rename"`
	Teams       *[]string `json:"teams,omitempty" jsonschema:"replace associated Team keys; at least one; omitted leaves unchanged"`
	Emoji       *string   `json:"emoji,omitempty" jsonschema:"Unicode emoji icon; omitted leaves unchanged, empty clears"`
	Description *string   `json:"description,omitempty" jsonschema:"new Project outcome; omitted leaves unchanged"`
	Archived    *bool     `json:"archived,omitempty" jsonschema:"archive or unarchive; omitted leaves unchanged"`
}

func (s *Service) ListProjects(ctx context.Context, includeArchived bool, teamFilter ...string) ([]Project, error) {
	if err := s.checkAccess(ctx); err != nil {
		return nil, err
	}
	team := ""
	if len(teamFilter) > 0 {
		team = teamFilter[0]
	}
	if _, err := s.teamID(ctx, team); err != nil {
		return nil, err
	}
	query, args := viewerSQL(ctx, `SELECT p.id,p.name,p.description,p.archived,p.emoji,t.key FROM projects p
 LEFT JOIN project_teams pt ON pt.project_id=p.id LEFT JOIN teams t ON t.id=pt.team_id AND t.id IN (`+visibleTeamIDs+`)
 WHERE p.workspace_id=? AND `+visibleProject+` AND (? OR NOT p.archived) AND (?='' OR EXISTS(SELECT 1 FROM project_teams f JOIN teams ft ON ft.id=f.team_id WHERE f.project_id=p.id AND ft.key=?))
 ORDER BY p.name,t.key`, s.workspace, includeArchived, team, team)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	projects := []Project{}
	for rows.Next() {
		var p Project
		var key sql.NullString
		if err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Archived, &p.Emoji, &key); err != nil {
			return nil, err
		}
		if len(projects) == 0 || projects[len(projects)-1].ID != p.ID {
			p.Teams = []string{}
			projects = append(projects, p)
		}
		if key.Valid {
			projects[len(projects)-1].Teams = append(projects[len(projects)-1].Teams, key.String)
		}
	}
	return projects, rows.Err()
}

func (s *Service) GetProject(ctx context.Context, ref ProjectRef) (Project, error) {
	if err := s.checkAccess(ctx); err != nil {
		return Project{}, err
	}
	if ref.ID < 0 || (ref.ID == 0 && ref.Name == "") {
		return Project{}, ErrProjectReference
	}
	var p Project
	query, args := viewerSQL(ctx, `SELECT p.id,p.name,p.description,p.archived,p.emoji FROM projects p WHERE p.workspace_id=? AND ((?<>0 AND p.id=?) OR (?=0 AND p.name=?)) AND `+visibleProject, s.workspace, ref.ID, ref.ID, ref.ID, ref.Name)
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&p.ID, &p.Name, &p.Description, &p.Archived, &p.Emoji)
	if errors.Is(err, sql.ErrNoRows) {
		return Project{}, ErrProjectNotFound
	}
	if err != nil {
		return Project{}, err
	}
	if ref.Name != "" && ref.Name != p.Name {
		return Project{}, ErrProjectReference
	}
	p.Teams = []string{}
	query, args = viewerSQL(ctx, `SELECT t.key FROM project_teams pt JOIN teams t ON t.id=pt.team_id WHERE pt.project_id=? AND t.id IN (`+visibleTeamIDs+`) ORDER BY t.key`, p.ID)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return Project{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return Project{}, err
		}
		p.Teams = append(p.Teams, key)
	}
	return p, rows.Err()
}

func (s *Service) replaceProjectTeams(ctx context.Context, tx *sql.Tx, id int64, keys []string) error {
	if len(keys) == 0 {
		return ErrProjectTeamsRequired
	}
	ids := []int64{}
	for _, key := range keys {
		var team int64
		query, args := viewerSQL(ctx, `SELECT id FROM teams WHERE workspace_id=? AND key=? AND id IN (`+visibleTeamIDs+`)`, s.workspace, key)
		err := tx.QueryRowContext(ctx, query, args...).Scan(&team)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrTeamNotFound
		}
		if err != nil {
			return err
		}
		if !slices.Contains(ids, team) {
			ids = append(ids, team)
		}
	}
	// A public collaborator only replaces the associations visible to them.
	query, args := viewerSQL(ctx, `SELECT team_id FROM project_teams WHERE project_id=? AND team_id NOT IN (`+visibleTeamIDs+`)`, id)
	hidden, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	for hidden.Next() {
		var team int64
		if err := hidden.Scan(&team); err != nil {
			hidden.Close()
			return err
		}
		ids = append(ids, team)
	}
	err = hidden.Err()
	hidden.Close()
	if err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT team_id FROM issues WHERE project_id=?`, id)
	if err != nil {
		return err
	}
	for rows.Next() {
		var used int64
		if err := rows.Scan(&used); err != nil {
			rows.Close()
			return err
		}
		if !slices.Contains(ids, used) {
			rows.Close()
			return ErrProjectTeamInUse
		}
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM project_teams WHERE project_id=?`, id); err != nil {
		return err
	}
	for _, team := range ids {
		if _, err = tx.ExecContext(ctx, `INSERT INTO project_teams(project_id,team_id) VALUES (?,?)`, id, team); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) CreateProject(ctx context.Context, name, description string, teams ...string) (Project, error) {
	if err := s.checkAccess(ctx); err != nil {
		return Project{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Project{}, ErrInvalidName
	}
	if len(teams) == 0 {
		return Project{}, ErrProjectTeamsRequired
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Project{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO projects(workspace_id,name,description) VALUES (?,?,?)`, s.workspace, name, description)
	if err != nil {
		return Project{}, projectWriteError(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return Project{}, err
	}
	if err = s.replaceProjectTeams(ctx, tx, id, teams); err != nil {
		return Project{}, err
	}
	if err = tx.Commit(); err != nil {
		return Project{}, err
	}
	return s.GetProject(ctx, ProjectRef{ID: id})
}

func (s *Service) UpdateProject(ctx context.Context, input UpdateProject) (Project, error) {
	if err := s.checkAccess(ctx); err != nil {
		return Project{}, err
	}
	if input.NewName != nil {
		name := strings.TrimSpace(*input.NewName)
		if name == "" {
			return Project{}, ErrInvalidName
		}
		input.NewName = &name
	}
	if input.Emoji != nil && !validEmoji(*input.Emoji) {
		return Project{}, ErrInvalidEmoji
	}
	p, err := s.GetProject(ctx, ProjectRef{ID: input.ID, Name: input.Name})
	if err != nil {
		return Project{}, err
	}
	query := `UPDATE projects SET name=COALESCE(?,name),description=COALESCE(?,description),archived=COALESCE(?,archived),emoji=COALESCE(?,emoji) WHERE id=? AND workspace_id=?`
	args := []any{input.NewName, input.Description, input.Archived, input.Emoji, p.ID, s.workspace}
	if input.Teams == nil {
		if _, err := s.db.ExecContext(ctx, query, args...); err != nil {
			return Project{}, projectWriteError(err)
		}
		return s.GetProject(ctx, ProjectRef{ID: p.ID})
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Project{}, err
	}
	defer tx.Rollback()
	if err = s.replaceProjectTeams(ctx, tx, p.ID, *input.Teams); err != nil {
		return Project{}, err
	}
	_, err = tx.ExecContext(ctx, query, args...)
	if err != nil {
		return Project{}, projectWriteError(err)
	}
	if err = tx.Commit(); err != nil {
		return Project{}, err
	}
	return s.GetProject(ctx, ProjectRef{ID: p.ID})
}

// resolveProject preserves omitted and explicitly cleared references. A pair
// of supplied references must identify the same Project, including clearing.
func (s *Service) resolveProject(ctx context.Context, id *int64, name *string) (sql.NullInt64, error) {
	if id == nil && (name == nil || *name == "") {
		return sql.NullInt64{}, nil
	}
	if id != nil && *id == 0 {
		if name != nil && *name != "" {
			return sql.NullInt64{}, ErrProjectReference
		}
		return sql.NullInt64{}, nil
	}
	ref := ProjectRef{}
	if id != nil {
		ref.ID = *id
	}
	if name != nil {
		ref.Name = *name
		if *name == "" && id != nil {
			return sql.NullInt64{}, ErrProjectReference
		}
	}
	p, err := s.GetProject(ctx, ref)
	return sql.NullInt64{Int64: p.ID, Valid: err == nil}, err
}

func (s *Service) validateIssueProject(ctx context.Context, tx *sql.Tx, project sql.NullInt64, team int64, current sql.NullInt64) error {
	if !project.Valid {
		return nil
	}
	var archived, compatible bool
	err := tx.QueryRowContext(ctx, `SELECT archived,EXISTS(SELECT 1 FROM project_teams WHERE project_id=p.id AND team_id=?) FROM projects p WHERE p.id=? AND p.workspace_id=?`, team, project.Int64, s.workspace).Scan(&archived, &compatible)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrProjectNotFound
	}
	if err != nil {
		return err
	}
	if !compatible {
		return ErrProjectTeam
	}
	if archived && project != current {
		return ErrProjectArchived
	}
	return nil
}

func projectWriteError(err error) error {
	var constraint *sqlite.Error
	if errors.As(err, &constraint) && constraint.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
		return ErrProjectExists
	}
	return err
}
