package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// linearIssue holds the fields of an issues.json entry, written by
// scripts/linear/export.py, that ImportLinear reads.
type linearIssue struct {
	Identifier  string `json:"identifier"`
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
	Trashed     bool   `json:"trashed"`
	State       struct {
		Name string `json:"name"`
	} `json:"state"`
	Project *struct {
		Name string `json:"name"`
	} `json:"project"`
	Parent *struct {
		Identifier string `json:"identifier"`
	} `json:"parent"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
	Relations []struct {
		Type         string `json:"type"`
		RelatedIssue struct {
			Identifier string `json:"identifier"`
		} `json:"relatedIssue"`
	} `json:"relations"`
	Attachments []struct {
		Title      string `json:"title"`
		URL        string `json:"url"`
		SourceType string `json:"sourceType"`
	} `json:"attachments"`
	Comments []linearComment `json:"comments"`
}

// linearComment is a comment or a reply; a person writes it as user, an
// integration as botActor.
type linearComment struct {
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
	User      *struct {
		Name string `json:"name"`
	} `json:"user"`
	BotActor *struct {
		Name string `json:"name"`
	} `json:"botActor"`
}

type linearProject struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Content     string `json:"content"`
	State       string `json:"state"`
}

// LinearImport counts the rows ImportLinear wrote. Projects counts only the
// Projects it created, not those it matched by name.
type LinearImport struct {
	Issues, Comments, Relations, Links, Projects int
}

var linearStatuses = map[string]Status{
	"Triage": StatusTriage, "Backlog": StatusBacklog, "Todo": StatusTodo, "Agent Ready": StatusAgentReady,
	"In Progress": StatusInProgress, "In Review": StatusInReview, "Done": StatusDone, "Canceled": StatusCanceled,
}

var linearTypes = map[string]string{"Spec": "spec", "Feature": "feature", "Bug": "bug", "Improvement": "improvement", "Chore": "chore"}

// linearType is spec when the labels include Spec, and otherwise the one
// Feature, Bug, Improvement, or Chore label.
func linearType(issue linearIssue) (string, error) {
	spec, kinds := false, []string{}
	for _, label := range issue.Labels {
		issueType, ok := linearTypes[label.Name]
		if !ok {
			return "", fmt.Errorf("%s has label %q, which has no type", issue.Identifier, label.Name)
		}
		if issueType == "spec" {
			spec = true
		} else {
			kinds = append(kinds, label.Name)
		}
	}
	switch {
	case spec:
		return "spec", nil
	case len(kinds) > 1:
		return "", fmt.Errorf("%s has more than one type label: %s", issue.Identifier, strings.Join(kinds, ", "))
	case len(kinds) == 1:
		return linearTypes[kinds[0]], nil
	}
	return "", nil
}

// ImportLinear copies the issues of a Linear export in dir (issues.json and
// projects.json) into the team with this key, keeping their identifiers and
// numbers. It writes everything in one transaction, or nothing: every number
// must be free in the team and below its next_number, and any status, label,
// relation, or comment the mapping does not cover aborts the import.
func (s *Service) ImportLinear(ctx context.Context, team, dir string) (LinearImport, error) {
	var issues []linearIssue
	var projects []linearProject
	for name, target := range map[string]any{"issues.json": &issues, "projects.json": &projects} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return LinearImport{}, err
		}
		if err := json.Unmarshal(data, target); err != nil {
			return LinearImport{}, fmt.Errorf("%s: %w", name, err)
		}
	}
	// Issues in the trash are not imported.
	issues = slices.DeleteFunc(issues, func(issue linearIssue) bool { return issue.Trashed })
	slices.SortFunc(issues, func(a, b linearIssue) int { return a.Number - b.Number })

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return LinearImport{}, err
	}
	defer tx.Rollback()
	var teamID int64
	var next int
	err = tx.QueryRowContext(ctx, `SELECT id, next_number FROM teams WHERE workspace_id = ? AND key = ?`, s.workspace, team).Scan(&teamID, &next)
	if errors.Is(err, sql.ErrNoRows) {
		return LinearImport{}, ErrTeamNotFound
	}
	if err != nil {
		return LinearImport{}, err
	}
	rows, err := tx.QueryContext(ctx, `SELECT number FROM issues WHERE team_id = ?`, teamID)
	if err != nil {
		return LinearImport{}, err
	}
	used := map[int]bool{}
	for rows.Next() {
		var number int
		if err := rows.Scan(&number); err != nil {
			rows.Close()
			return LinearImport{}, err
		}
		used[number] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return LinearImport{}, err
	}
	var taken []string
	for _, issue := range issues {
		if issue.Identifier != fmt.Sprintf("%s-%d", team, issue.Number) {
			return LinearImport{}, fmt.Errorf("%s is not %s-%d: its identifier must be the team key and its number", issue.Identifier, team, issue.Number)
		}
		if issue.Number >= next {
			return LinearImport{}, fmt.Errorf("%s is not below team %s's next number %d", issue.Identifier, team, next)
		}
		if used[issue.Number] {
			taken = append(taken, issue.Identifier)
		}
	}
	if len(taken) > 0 {
		return LinearImport{}, fmt.Errorf("team %s already has %d of these numbers: %s", team, len(taken), strings.Join(taken, ", "))
	}

	var result LinearImport
	for _, project := range projects {
		// Content follows the description after a blank line.
		description := project.Description
		if description != "" && project.Content != "" {
			description += "\n\n"
		}
		description += project.Content
		archived := project.State == "completed" || project.State == "canceled"
		inserted, err := tx.ExecContext(ctx, `INSERT INTO projects(workspace_id, name, description, archived) VALUES (?, ?, ?, ?)
			ON CONFLICT(workspace_id, name) DO NOTHING`, s.workspace, project.Name, description, archived)
		if err != nil {
			return LinearImport{}, err
		}
		created, err := inserted.RowsAffected()
		if err != nil {
			return LinearImport{}, err
		}
		result.Projects += int(created)
	}

	// lookup returns the id of an issue or Project in the workspace.
	lookup := func(table, column, value string) (int64, error) {
		var id int64
		err := tx.QueryRowContext(ctx, `SELECT id FROM `+table+` WHERE workspace_id = ? AND `+column+` = ?`, s.workspace, value).Scan(&id)
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("%s %q not found in the workspace", table, value)
		}
		return id, err
	}
	ids := make([]int64, len(issues))
	for i, issue := range issues {
		status, ok := linearStatuses[issue.State.Name]
		if !ok {
			return LinearImport{}, fmt.Errorf("%s has status %q, which has no mapping", issue.Identifier, issue.State.Name)
		}
		issueType, err := linearType(issue)
		if err != nil {
			return LinearImport{}, err
		}
		var projectID sql.NullInt64
		if issue.Project != nil {
			if projectID.Int64, err = lookup("projects", "name", issue.Project.Name); err != nil {
				return LinearImport{}, err
			}
			projectID.Valid = true
		}
		inserted, err := tx.ExecContext(ctx, `INSERT INTO issues(workspace_id, identifier, team_id, number, title, description, status, type, project_id, created_at, updated_at, priority)
			VALUES (?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), ?, ?, ?, ?)`,
			s.workspace, issue.Identifier, teamID, issue.Number, issue.Title, issue.Description, status, issueType, projectID, issue.CreatedAt, issue.UpdatedAt, issue.Priority)
		if err != nil {
			return LinearImport{}, fmt.Errorf("%s: %w", issue.Identifier, err)
		}
		if ids[i], err = inserted.LastInsertId(); err != nil {
			return LinearImport{}, err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO events(issue_id, kind, detail, created_at) VALUES (?, 'created', ?, ?)`, ids[i], string(status), issue.CreatedAt); err != nil {
			return LinearImport{}, err
		}
		result.Issues++
	}

	// Parents and relations name issues by identifier, so they are written
	// once every issue exists.
	for i, issue := range issues {
		if issue.Parent != nil {
			parentID, err := lookup("issues", "identifier", issue.Parent.Identifier)
			if err != nil {
				return LinearImport{}, fmt.Errorf("parent of %s: %w", issue.Identifier, err)
			}
			if _, err := tx.ExecContext(ctx, `UPDATE issues SET parent_id = ? WHERE id = ?`, parentID, ids[i]); err != nil {
				return LinearImport{}, err
			}
		}
		for _, relation := range issue.Relations {
			if relation.Type != "blocks" && relation.Type != "related" {
				return LinearImport{}, fmt.Errorf("%s has relation %q, which has no mapping", issue.Identifier, relation.Type)
			}
			otherID, err := lookup("issues", "identifier", relation.RelatedIssue.Identifier)
			if err != nil {
				return LinearImport{}, fmt.Errorf("%s %s: %w", issue.Identifier, relation.Type, err)
			}
			from, to := ids[i], otherID
			if relation.Type == "related" && from > to {
				from, to = to, from
			}
			inserted, err := tx.ExecContext(ctx, `INSERT INTO relations(issue_id, kind, other_id) VALUES (?, ?, ?) ON CONFLICT DO NOTHING`, from, relation.Type, to)
			if err != nil {
				return LinearImport{}, err
			}
			created, err := inserted.RowsAffected()
			if err != nil {
				return LinearImport{}, err
			}
			result.Relations += int(created)
		}
		// Replies become ordinary comments in time order.
		slices.SortStableFunc(issue.Comments, func(a, b linearComment) int { return strings.Compare(a.CreatedAt, b.CreatedAt) })
		for _, comment := range issue.Comments {
			var author string
			switch {
			case comment.User != nil:
				author = comment.User.Name
			case comment.BotActor != nil:
				author = comment.BotActor.Name
			default:
				return LinearImport{}, fmt.Errorf("a comment on %s has neither a user nor a bot as its author", issue.Identifier)
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO comments(issue_id, body, created_at, author) VALUES (?, ?, ?, ?)`, ids[i], comment.Body, comment.CreatedAt, author); err != nil {
				return LinearImport{}, err
			}
			result.Comments++
		}
		// GitHub attachments are pull requests, which GitHub sync records.
		for _, attachment := range issue.Attachments {
			if attachment.SourceType == "github" {
				continue
			}
			if _, err := tx.ExecContext(ctx, `INSERT INTO links(issue_id, title, url) VALUES (?, ?, ?)`, ids[i], attachment.Title, attachment.URL); err != nil {
				return LinearImport{}, err
			}
			result.Links++
		}
	}
	if err := tx.Commit(); err != nil {
		return LinearImport{}, err
	}
	return result, nil
}
