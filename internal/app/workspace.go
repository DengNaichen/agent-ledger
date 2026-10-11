package app

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"slices"
	"strings"
)

// workspaceSlug matches a workspace slug. Pages live under /<slug>/, so the
// reserved slugs are the server's own top-level paths.
var (
	workspaceSlug = regexp.MustCompile(`^[a-z][a-z0-9-]{1,31}$`)
	reservedSlugs = []string{"api", "assets", "auth", "mcp", "w", "webhooks"}
)

// Workspace groups teams, Projects, and labels. Its slug names it in page,
// REST, and MCP paths and cannot change.
type Workspace struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

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
