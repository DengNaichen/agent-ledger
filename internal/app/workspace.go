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
	if err := scoped.checkAccess(ctx); err != nil {
		return nil, err
	}
	return &scoped, nil
}

func (s *Service) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	identity, err := s.authenticated(ctx)
	if err != nil {
		return nil, err
	}
	query := `SELECT slug, name FROM workspaces`
	args := []any{}
	if identity.Subject != "" {
		query += ` WHERE id IN (SELECT workspace_id FROM workspace_members WHERE subject=? AND status='active')`
		args = append(args, identity.Subject)
	}
	rows, err := s.db.QueryContext(ctx, query+` ORDER BY slug`, args...)
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
	identity, err := s.authenticated(ctx)
	if err != nil {
		return Workspace{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Workspace{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO workspaces(slug, name) VALUES (?, ?) ON CONFLICT(slug) DO NOTHING`, workspace.Slug, workspace.Name)
	if err != nil {
		return Workspace{}, err
	}
	if inserted, _ := result.RowsAffected(); inserted == 0 {
		return Workspace{}, ErrWorkspaceExists
	}
	if identity.Subject != "" {
		if err := saveIdentity(ctx, tx, identity); err != nil {
			return Workspace{}, err
		}
		id, err := result.LastInsertId()
		if err != nil {
			return Workspace{}, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO workspace_members(workspace_id,subject,role,status) VALUES(?,?,'admin','active')`, id, identity.Subject); err != nil {
			return Workspace{}, err
		}
	}
	return workspace, tx.Commit()
}

// RenameWorkspace changes the name of the workspace with this slug.
func (s *Service) RenameWorkspace(ctx context.Context, slug, name string) (Workspace, error) {
	scoped, err := s.InWorkspace(ctx, slug)
	if err != nil {
		return Workspace{}, err
	}
	if requestIdentity(ctx).Subject != "" {
		if err := scoped.requireAdmin(ctx); err != nil {
			return Workspace{}, err
		}
	}
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
