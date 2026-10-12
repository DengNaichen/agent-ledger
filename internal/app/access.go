package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"
)

// Identity is supplied by trusted authentication middleware, never request JSON.
type Identity struct {
	Subject string `json:"subject"`
	Name    string `json:"name"`
	Email   string `json:"email"`
}
type identityKey struct{}

func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, identity)
}
func requestIdentity(ctx context.Context) Identity {
	identity, _ := ctx.Value(identityKey{}).(Identity)
	return identity
}

// ForRequests prevents missing middleware identity from becoming system access.
// Open's original service is reserved for local administration and background jobs.
func (s *Service) ForRequests(requireIdentity bool) *Service {
	scoped := *s
	scoped.requireIdentity = requireIdentity
	return &scoped
}
func (s *Service) authenticated(ctx context.Context) (Identity, error) {
	identity := requestIdentity(ctx)
	if identity.Subject == "" && s.requireIdentity {
		return identity, ErrIdentityRequired
	}
	return identity, nil
}

var (
	ErrIdentityRequired    = newError(KindForbidden, "authenticated identity is required")
	ErrAccessDenied        = newError(KindForbidden, "workspace access denied")
	ErrAdminRequired       = newError(KindForbidden, "workspace administrator is required")
	ErrCloudRequired       = newError(KindForbidden, "member management requires authenticated cloud access")
	ErrLastAdmin           = newError(KindConflict, "cannot remove the last active administrator")
	ErrInvalidMember       = newError(KindInvalid, "role must be admin or member; status must be active or suspended")
	ErrMemberNotFound      = newError(KindNotFound, "member not found")
	ErrInvalidInvite       = newError(KindInvalid, "invitation is invalid or has been reset")
	ErrAlreadyBootstrapped = newError(KindConflict, "workspace already has members")
)

type WorkspaceAccess struct {
	Enabled bool   `json:"enabled"`
	Role    string `json:"role"`
	Subject string `json:"subject"`
}
type Member struct {
	Identity
	Role   string `json:"role"`
	Status string `json:"status"`
}
type UpdateMember struct {
	Subject string `json:"subject"`
	Role    string `json:"role"`
	Status  string `json:"status"`
}

func saveIdentity(ctx context.Context, tx *sql.Tx, identity Identity) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO users(subject,name,email) VALUES(?,?,?) ON CONFLICT(subject) DO UPDATE SET name=excluded.name,email=excluded.email`, identity.Subject, identity.Name, identity.Email)
	return err
}
func (s *Service) checkAccess(ctx context.Context) error {
	_, err := s.Access(ctx)
	return err
}
func (s *Service) Access(ctx context.Context) (WorkspaceAccess, error) {
	identity, err := s.authenticated(ctx)
	if err != nil {
		return WorkspaceAccess{}, err
	}
	if identity.Subject == "" {
		return WorkspaceAccess{Enabled: false}, nil
	}
	access := WorkspaceAccess{Enabled: true, Subject: identity.Subject}
	err = s.db.QueryRowContext(ctx, `SELECT role FROM workspace_members WHERE workspace_id=? AND subject=? AND status='active'`, s.workspace, identity.Subject).Scan(&access.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return WorkspaceAccess{}, ErrAccessDenied
	}
	return access, err
}
func (s *Service) requireAdmin(ctx context.Context) error {
	access, err := s.Access(ctx)
	if err != nil {
		return err
	}
	if !access.Enabled {
		return ErrCloudRequired
	}
	if access.Role != "admin" {
		return ErrAdminRequired
	}
	return nil
}
func (s *Service) ListMembers(ctx context.Context) ([]Member, error) {
	access, err := s.Access(ctx)
	if err != nil {
		return nil, err
	}
	if !access.Enabled {
		return nil, ErrCloudRequired
	}
	rows, err := s.db.QueryContext(ctx, `SELECT u.subject,u.name,u.email,m.role,m.status FROM workspace_members m JOIN users u ON u.subject=m.subject WHERE m.workspace_id=? ORDER BY u.name,u.subject`, s.workspace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	members := []Member{}
	for rows.Next() {
		var m Member
		if err := rows.Scan(&m.Subject, &m.Name, &m.Email, &m.Role, &m.Status); err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}
func (s *Service) UpdateMember(ctx context.Context, input UpdateMember) error {
	if err := s.requireAdmin(ctx); err != nil {
		return err
	}
	if (input.Role != "admin" && input.Role != "member") || (input.Status != "active" && input.Status != "suspended") {
		return ErrInvalidMember
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Recheck inside the transaction: another administrator may have changed us.
	var actorRole, actorStatus string
	if err := tx.QueryRowContext(ctx, `SELECT role,status FROM workspace_members WHERE workspace_id=? AND subject=?`, s.workspace, requestIdentity(ctx).Subject).Scan(&actorRole, &actorStatus); err != nil {
		return err
	}
	if actorRole != "admin" || actorStatus != "active" {
		return ErrAdminRequired
	}
	var role, status string
	err = tx.QueryRowContext(ctx, `SELECT role,status FROM workspace_members WHERE workspace_id=? AND subject=?`, s.workspace, input.Subject).Scan(&role, &status)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrMemberNotFound
	}
	if err != nil {
		return err
	}
	if role == "admin" && status == "active" && (input.Role != "admin" || input.Status != "active") {
		var count int
		if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM workspace_members WHERE workspace_id=? AND role='admin' AND status='active'`, s.workspace).Scan(&count); err != nil {
			return err
		}
		if count == 1 {
			return ErrLastAdmin
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE workspace_members SET role=?,status=? WHERE workspace_id=? AND subject=?`, input.Role, input.Status, s.workspace, input.Subject); err != nil {
		return err
	}
	return tx.Commit()
}

// ResetInvite rotates a reusable Member invitation. Only its digest is stored.
func (s *Service) ResetInvite(ctx context.Context) (string, error) {
	if err := s.requireAdmin(ctx); err != nil {
		return "", err
	}
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(bytes)
	digest := sha256.Sum256([]byte(token))
	_, err := s.db.ExecContext(ctx, `INSERT INTO workspace_invites(workspace_id,digest) VALUES(?,?) ON CONFLICT(workspace_id) DO UPDATE SET digest=excluded.digest`, s.workspace, digest[:])
	return token, err
}
func (s *Service) JoinWorkspace(ctx context.Context, token string) (Workspace, error) {
	identity, err := s.authenticated(ctx)
	if err != nil {
		return Workspace{}, err
	}
	if identity.Subject == "" {
		return Workspace{}, ErrCloudRequired
	}
	digest := sha256.Sum256([]byte(token))
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Workspace{}, err
	}
	defer tx.Rollback()
	var id int64
	var workspace Workspace
	err = tx.QueryRowContext(ctx, `SELECT w.id,w.slug,w.name FROM workspace_invites i JOIN workspaces w ON w.id=i.workspace_id WHERE i.digest=?`, digest[:]).Scan(&id, &workspace.Slug, &workspace.Name)
	if errors.Is(err, sql.ErrNoRows) {
		return Workspace{}, ErrInvalidInvite
	}
	if err != nil {
		return Workspace{}, err
	}
	var status string
	err = tx.QueryRowContext(ctx, `SELECT status FROM workspace_members WHERE workspace_id=? AND subject=?`, id, identity.Subject).Scan(&status)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Workspace{}, err
	}
	if status == "suspended" {
		return Workspace{}, ErrAccessDenied
	}
	if err = saveIdentity(ctx, tx, identity); err != nil {
		return Workspace{}, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO workspace_members(workspace_id,subject,role,status) VALUES(?,?,'member','active') ON CONFLICT(workspace_id,subject) DO NOTHING`, id, identity.Subject); err != nil {
		return Workspace{}, err
	}
	return workspace, tx.Commit()
}

// BootstrapAccess explicitly initializes an existing workspace from a local CLI.
// It cannot overwrite established membership or be used by a request service.
func (s *Service) BootstrapAccess(ctx context.Context, slug, subject string) error {
	if s.requireIdentity || requestIdentity(ctx).Subject != "" {
		return ErrAccessDenied
	}
	if strings.TrimSpace(subject) == "" {
		return ErrIdentityRequired
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM workspaces WHERE slug=?`, slug).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrWorkspaceNotFound
	}
	if err != nil {
		return err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT count(*) FROM workspace_members WHERE workspace_id=?`, id).Scan(&count); err != nil {
		return err
	}
	if count != 0 {
		return ErrAlreadyBootstrapped
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO users(subject,name,email) VALUES(?,'','') ON CONFLICT(subject) DO NOTHING`, subject); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO workspace_members(workspace_id,subject,role,status) VALUES(?,?,'admin','active')`, id, subject); err != nil {
		return err
	}
	return tx.Commit()
}

// RememberIdentity refreshes the display profile only for a verified subject.
func (s *Service) RememberIdentity(ctx context.Context) error {
	identity, err := s.authenticated(ctx)
	if err != nil {
		return err
	}
	if identity.Subject == "" {
		return nil
	}
	_, err = s.db.ExecContext(ctx, `INSERT INTO users(subject,name,email) VALUES(?,?,?) ON CONFLICT(subject) DO UPDATE SET name=excluded.name,email=excluded.email WHERE users.name<>excluded.name OR users.email<>excluded.email`, identity.Subject, identity.Name, identity.Email)
	return err
}
