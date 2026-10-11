package app

import (
	"context"
	"database/sql"
	"errors"
)

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
