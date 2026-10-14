package app

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

type Initiative struct {
	ID          int64   `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Archived    bool    `json:"archived"`
	ProjectIDs  []int64 `json:"project_ids" jsonschema:"only projects visible to the caller"`
}
type CreateInitiative struct {
	Name        string  `json:"name"`
	Description string  `json:"description,omitempty"`
	ProjectIDs  []int64 `json:"project_ids,omitempty"`
}
type UpdateInitiative struct {
	ID          int64    `json:"id"`
	Name        *string  `json:"name,omitempty"`
	Description *string  `json:"description,omitempty"`
	Archived    *bool    `json:"archived,omitempty"`
	ProjectIDs  *[]int64 `json:"project_ids,omitempty" jsonschema:"replace visible associations; hidden associations are preserved"`
}

func (s *Service) ListInitiatives(ctx context.Context, includeArchived bool) ([]Initiative, error) {
	return s.listInitiatives(ctx, includeArchived, 0)
}

func (s *Service) listInitiatives(ctx context.Context, includeArchived bool, filterID int64) ([]Initiative, error) {
	if err := s.checkAccess(ctx); err != nil {
		return nil, err
	}
	query, args := viewerSQL(ctx, `SELECT i.id,i.name,i.description,i.archived,p.id
 FROM initiatives i LEFT JOIN initiative_projects ip ON ip.initiative_id=i.id
 LEFT JOIN projects p ON p.id=ip.project_id AND `+visibleProject+`
 WHERE i.workspace_id=? AND (? OR NOT i.archived) AND (?=0 OR i.id=?) ORDER BY i.name,p.id`, s.workspace, includeArchived, filterID, filterID)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []Initiative{}
	for rows.Next() {
		var item Initiative
		var id sql.NullInt64
		if err := rows.Scan(&item.ID, &item.Name, &item.Description, &item.Archived, &id); err != nil {
			return nil, err
		}
		if len(result) == 0 || result[len(result)-1].ID != item.ID {
			item.ProjectIDs = []int64{}
			result = append(result, item)
		}
		if id.Valid {
			result[len(result)-1].ProjectIDs = append(result[len(result)-1].ProjectIDs, id.Int64)
		}
	}
	return result, rows.Err()
}

func (s *Service) GetInitiative(ctx context.Context, id int64) (Initiative, error) {
	items, err := s.listInitiatives(ctx, true, id)
	if err != nil {
		return Initiative{}, err
	}
	for _, item := range items {
		if item.ID == id {
			return item, nil
		}
	}
	return Initiative{}, ErrInitiativeNotFound
}

func initiativeWriteError(err error) error {
	if errors.Is(projectWriteError(err), ErrProjectExists) {
		return ErrInitiativeExists
	}
	return err
}

func (s *Service) CreateInitiative(ctx context.Context, input CreateInitiative) (Initiative, error) {
	if err := s.checkAccess(ctx); err != nil {
		return Initiative{}, err
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" {
		return Initiative{}, ErrInvalidName
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Initiative{}, err
	}
	defer tx.Rollback()
	res, err := tx.ExecContext(ctx, `INSERT INTO initiatives(workspace_id,name,description) VALUES(?,?,?)`, s.workspace, input.Name, input.Description)
	if err != nil {
		return Initiative{}, initiativeWriteError(err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return Initiative{}, err
	}
	if err = s.replaceInitiativeProjects(ctx, tx, id, false, input.ProjectIDs); err != nil {
		return Initiative{}, err
	}
	if err = tx.Commit(); err != nil {
		return Initiative{}, err
	}
	return s.GetInitiative(ctx, id)
}

func (s *Service) UpdateInitiative(ctx context.Context, input UpdateInitiative) (Initiative, error) {
	current, err := s.GetInitiative(ctx, input.ID)
	if err != nil {
		return Initiative{}, err
	}
	if input.Name != nil {
		name := strings.TrimSpace(*input.Name)
		if name == "" {
			return Initiative{}, ErrInvalidName
		}
		input.Name = &name
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Initiative{}, err
	}
	defer tx.Rollback()
	if input.ProjectIDs != nil {
		var archived bool
		if err = tx.QueryRowContext(ctx, `SELECT COALESCE(?,archived) FROM initiatives WHERE id=? AND workspace_id=?`, input.Archived, current.ID, s.workspace).Scan(&archived); err != nil {
			return Initiative{}, err
		}
		if err = s.replaceInitiativeProjects(ctx, tx, current.ID, archived, *input.ProjectIDs); err != nil {
			return Initiative{}, err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE initiatives SET name=COALESCE(?,name),description=COALESCE(?,description),archived=COALESCE(?,archived) WHERE id=? AND workspace_id=?`, input.Name, input.Description, input.Archived, current.ID, s.workspace); err != nil {
		return Initiative{}, initiativeWriteError(err)
	}
	if err = tx.Commit(); err != nil {
		return Initiative{}, err
	}
	return s.GetInitiative(ctx, current.ID)
}

func (s *Service) replaceInitiativeProjects(ctx context.Context, tx *sql.Tx, id int64, archived bool, projects []int64) error {
	for _, project := range projects {
		var projectArchived, linked bool
		query, args := viewerSQL(ctx, `SELECT p.archived,EXISTS(SELECT 1 FROM initiative_projects WHERE initiative_id=? AND project_id=p.id) FROM projects p WHERE p.id=? AND p.workspace_id=? AND `+visibleProject, id, project, s.workspace)
		err := tx.QueryRowContext(ctx, query, args...).Scan(&projectArchived, &linked)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrProjectNotFound
		}
		if err != nil {
			return err
		}
		if !linked && (archived || projectArchived) {
			return ErrInitiativeArchived
		}
	}
	// Only replace visible associations; a collaborator cannot remove hidden projects.
	query, args := viewerSQL(ctx, `DELETE FROM initiative_projects WHERE initiative_id=? AND project_id IN (SELECT p.id FROM projects p WHERE p.workspace_id=? AND `+visibleProject+`)`, id, s.workspace)
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		return err
	}
	for _, project := range projects {
		if _, err := tx.ExecContext(ctx, `INSERT INTO initiative_projects(initiative_id,project_id) VALUES(?,?) ON CONFLICT DO NOTHING`, id, project); err != nil {
			return err
		}
	}
	return nil
}
