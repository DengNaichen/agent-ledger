package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"modernc.org/sqlite"
)

var labelColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

type LabelGroup struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}
type SaveLabelGroup struct {
	ID   int64  `json:"id,omitempty" jsonschema:"omit to create; existing ID to rename"`
	Name string `json:"name"`
}
type Label struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Color       string `json:"color"`
	Description string `json:"description"`
	GroupID     int64  `json:"group_id"`
	Archived    bool   `json:"archived"`
}
type CreateLabel struct {
	Name        string `json:"name"`
	Color       string `json:"color" jsonschema:"six-digit hex color including #"`
	Description string `json:"description,omitempty"`
	GroupID     int64  `json:"group_id,omitempty" jsonschema:"existing group ID; zero means ungrouped; group membership is fixed after creation"`
}
type UpdateLabel struct {
	ID          int64   `json:"id"`
	Name        *string `json:"name,omitempty"`
	Color       *string `json:"color,omitempty"`
	Description *string `json:"description,omitempty"`
	Archived    *bool   `json:"archived,omitempty"`
}
type ChangeIssueLabels struct {
	Identifiers []string `json:"identifiers" jsonschema:"issues to change atomically; at least one"`
	Add         []int64  `json:"add,omitempty" jsonschema:"label IDs to add; at most one per group; replaces existing value in that group"`
	Remove      []int64  `json:"remove,omitempty" jsonschema:"label IDs to remove; absent associations are ignored"`
}
type MergeLabels struct {
	SourceID int64 `json:"source_id"`
	TargetID int64 `json:"target_id" jsonschema:"active label in the same group; source is deleted and its associations transferred"`
}

func labelError(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrLabelNotFound
	}
	var se *sqlite.Error
	if errors.As(err, &se) && se.Code()&255 == 19 {
		return fmt.Errorf("%w: duplicate name or invalid reference", ErrInvalidLabel)
	}
	return err
}
func labelName(name string) (string, string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", "", fmt.Errorf("%w: name is required", ErrInvalidLabel)
	}
	return name, strings.ToLower(name), nil
}
func (s *Service) SaveLabelGroup(ctx context.Context, input SaveLabelGroup) (LabelGroup, error) {
	name, key, err := labelName(input.Name)
	if err != nil {
		return LabelGroup{}, err
	}
	var group LabelGroup
	if input.ID == 0 {
		err = s.db.QueryRowContext(ctx, `INSERT INTO label_groups(workspace_id, name, name_key) VALUES (?, ?, ?) RETURNING id, name`, s.workspace, name, key).Scan(&group.ID, &group.Name)
	} else {
		err = s.db.QueryRowContext(ctx, `UPDATE label_groups SET name = ?, name_key = ? WHERE id = ? AND workspace_id = ? RETURNING id, name`, name, key, input.ID, s.workspace).Scan(&group.ID, &group.Name)
	}
	return group, labelError(err)
}
func (s *Service) ListLabelGroups(ctx context.Context) ([]LabelGroup, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name FROM label_groups WHERE workspace_id = ? ORDER BY name_key, id`, s.workspace)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := []LabelGroup{}
	for rows.Next() {
		var g LabelGroup
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

const labelColumns = `id, name, color, description, COALESCE(group_id, 0), archived`

func scanLabel(row interface{ Scan(...any) error }) (Label, error) {
	var l Label
	err := row.Scan(&l.ID, &l.Name, &l.Color, &l.Description, &l.GroupID, &l.Archived)
	return l, labelError(err)
}
func (s *Service) CreateLabel(ctx context.Context, input CreateLabel) (Label, error) {
	name, key, err := labelName(input.Name)
	if err != nil {
		return Label{}, err
	}
	if !labelColor.MatchString(input.Color) {
		return Label{}, fmt.Errorf("%w: color must be #RRGGBB", ErrInvalidLabel)
	}
	return scanLabel(s.db.QueryRowContext(ctx, `INSERT INTO labels(workspace_id, name, name_key, color, description, group_id) VALUES (?, ?, ?, ?, ?, NULLIF(?, 0)) RETURNING `+labelColumns,
		s.workspace, name, key, input.Color, input.Description, input.GroupID))
}
func (s *Service) UpdateLabel(ctx context.Context, input UpdateLabel) (Label, error) {
	var key *string
	if input.Name != nil {
		name, k, err := labelName(*input.Name)
		if err != nil {
			return Label{}, err
		}
		input.Name = &name
		key = &k
	}
	if input.Color != nil && !labelColor.MatchString(*input.Color) {
		return Label{}, fmt.Errorf("%w: color must be #RRGGBB", ErrInvalidLabel)
	}
	return scanLabel(s.db.QueryRowContext(ctx, `UPDATE labels SET name = COALESCE(?, name), name_key = COALESCE(?, name_key), color = COALESCE(?, color), description = COALESCE(?, description), archived = COALESCE(?, archived) WHERE id = ? AND workspace_id = ? RETURNING `+labelColumns, input.Name, key, input.Color, input.Description, input.Archived, input.ID, s.workspace))
}
func (s *Service) labels(ctx context.Context, where string, args ...any) ([]Label, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+labelColumns+` FROM labels WHERE workspace_id = ? AND `+where+` ORDER BY name_key, id`, append([]any{s.workspace}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	labels := []Label{}
	for rows.Next() {
		l, err := scanLabel(rows)
		if err != nil {
			return nil, err
		}
		labels = append(labels, l)
	}
	return labels, rows.Err()
}
func (s *Service) ListLabels(ctx context.Context, includeArchived bool) ([]Label, error) {
	return s.labels(ctx, `(? OR NOT archived)`, includeArchived)
}
func (s *Service) issueLabels(ctx context.Context, issueID int64) ([]Label, error) {
	return s.labels(ctx, `id IN (SELECT label_id FROM issue_labels WHERE issue_id = ?)`, issueID)
}
func (s *Service) ChangeIssueLabels(ctx context.Context, input ChangeIssueLabels) error {
	if len(input.Identifiers) == 0 {
		return fmt.Errorf("%w: at least one issue is required", ErrInvalidLabel)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	added := []Label{}
	groups := map[int64]int64{}
	ids := map[int64]bool{}
	for _, id := range input.Add {
		if ids[id] {
			continue
		}
		ids[id] = true
		l, err := scanLabel(tx.QueryRowContext(ctx, `SELECT `+labelColumns+` FROM labels WHERE id = ? AND workspace_id = ?`, id, s.workspace))
		if err != nil {
			return err
		}
		if l.GroupID != 0 {
			if _, exists := groups[l.GroupID]; exists {
				return fmt.Errorf("%w: multiple additions in one group", ErrInvalidLabel)
			}
			groups[l.GroupID] = id
		}
		added = append(added, l)
	}
	for _, id := range input.Remove {
		if ids[id] {
			return fmt.Errorf("%w: cannot add and remove the same label", ErrInvalidLabel)
		}
		var found int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM labels WHERE id = ? AND workspace_id = ?`, id, s.workspace).Scan(&found); err != nil {
			return labelError(err)
		}
	}
	for _, identifier := range input.Identifiers {
		var issueID int64
		if err := tx.QueryRowContext(ctx, `SELECT id FROM issues WHERE workspace_id = ? AND identifier = ?`, s.workspace, identifier).Scan(&issueID); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		changed := int64(0)
		for _, id := range input.Remove {
			result, err := tx.ExecContext(ctx, `DELETE FROM issue_labels WHERE issue_id = ? AND label_id = ?`, issueID, id)
			if err != nil {
				return err
			}
			n, _ := result.RowsAffected()
			changed += n
		}
		for _, l := range added {
			var exists int
			if err := tx.QueryRowContext(ctx, `SELECT count(*) FROM issue_labels WHERE issue_id = ? AND label_id = ?`, issueID, l.ID).Scan(&exists); err != nil {
				return err
			}
			if exists > 0 {
				continue
			}
			if l.Archived {
				return fmt.Errorf("%w: archived labels cannot be applied", ErrInvalidLabel)
			}
			if l.GroupID != 0 {
				if _, err := tx.ExecContext(ctx, `DELETE FROM issue_labels WHERE issue_id = ? AND label_id IN (SELECT id FROM labels WHERE group_id = ?)`, issueID, l.GroupID); err != nil {
					return err
				}
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO issue_labels(issue_id, label_id) VALUES (?, ?)`, issueID, l.ID); err != nil {
				return err
			}
			changed++
		}
		if changed > 0 {
			if _, err := tx.ExecContext(ctx, `UPDATE issues SET updated_at = ? WHERE id = ?`, s.now().UTC().Format(time.RFC3339Nano), issueID); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}
func (s *Service) MergeLabels(ctx context.Context, input MergeLabels) error {
	if input.SourceID == input.TargetID {
		return fmt.Errorf("%w: merge requires distinct labels", ErrInvalidLabel)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	source, err := scanLabel(tx.QueryRowContext(ctx, `SELECT `+labelColumns+` FROM labels WHERE id = ? AND workspace_id = ?`, input.SourceID, s.workspace))
	if err != nil {
		return err
	}
	target, err := scanLabel(tx.QueryRowContext(ctx, `SELECT `+labelColumns+` FROM labels WHERE id = ? AND workspace_id = ?`, input.TargetID, s.workspace))
	if err != nil {
		return err
	}
	if source.GroupID != target.GroupID || target.Archived {
		return fmt.Errorf("%w: target must be active and in the same group", ErrInvalidLabel)
	}
	// Remove the source associations before inserting the replacement to respect exclusivity.
	rows, err := tx.QueryContext(ctx, `SELECT issue_id FROM issue_labels WHERE label_id = ?`, source.ID)
	if err != nil {
		return err
	}
	issueIDs := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		issueIDs = append(issueIDs, id)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM issue_labels WHERE label_id = ?`, source.ID); err != nil {
		return err
	}
	for _, id := range issueIDs {
		if _, err = tx.ExecContext(ctx, `INSERT INTO issue_labels(issue_id,label_id) VALUES (?,?) ON CONFLICT DO NOTHING`, id, target.ID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE issues SET updated_at = ? WHERE id = ?`, s.now().UTC().Format(time.RFC3339Nano), id); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM labels WHERE id = ?`, source.ID); err != nil {
		return err
	}
	return tx.Commit()
}

// Each selected label or group is a predicate. Modes combine those predicates,
// while the resulting label filter combines with other issue filters using AND.
func (s *Service) labelFilter(ctx context.Context, input ListIssues) (string, []any, error) {
	mode := input.LabelMode
	if mode == "" {
		mode = "any"
	}
	if mode != "any" && mode != "all" && mode != "none" && mode != "unlabeled" {
		return "", nil, fmt.Errorf("%w: unknown label filter mode", ErrInvalidLabel)
	}
	if mode == "unlabeled" {
		if len(input.LabelIDs)+len(input.LabelGroupIDs) > 0 {
			return "", nil, fmt.Errorf("%w: unlabeled cannot include label selections", ErrInvalidLabel)
		}
		return ` AND NOT EXISTS (SELECT 1 FROM issue_labels WHERE issue_id = issues.id)`, nil, nil
	}
	predicates := []string{}
	args := []any{}
	for _, selection := range []struct {
		ids              []int64
		table, condition string
	}{{input.LabelIDs, "labels", "il.label_id = ?"}, {input.LabelGroupIDs, "label_groups", "l.group_id = ?"}} {
		for _, id := range selection.ids {
			var found int
			if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM `+selection.table+` WHERE id = ? AND workspace_id = ?`, id, s.workspace).Scan(&found); err != nil {
				return "", nil, labelError(err)
			}
			predicates = append(predicates, `EXISTS (SELECT 1 FROM issue_labels il JOIN labels l ON l.id = il.label_id WHERE il.issue_id = issues.id AND `+selection.condition+`)`)
			args = append(args, id)
		}
	}
	if len(predicates) == 0 {
		return "", nil, nil
	}
	join := " OR "
	prefix := ""
	if mode == "all" {
		join = " AND "
	}
	if mode == "none" {
		prefix = "NOT "
	}
	return ` AND ` + prefix + `(` + strings.Join(predicates, join) + `)`, args, nil
}
