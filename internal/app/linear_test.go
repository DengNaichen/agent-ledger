package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// importedRows describes every row an import can write, so a failed import
// can be checked to have written nothing.
func importedRows(t *testing.T, s *Service) string {
	t.Helper()
	var rows string
	if err := s.db.QueryRow(`SELECT (SELECT group_concat(key || ' ' || next_number, ', ') FROM teams) || ' | ' ||
		(SELECT group_concat(name || ' ' || description || ' ' || archived, ', ') FROM projects) || ' | ' ||
		(SELECT count(*) FROM issues) || ' issues, ' || (SELECT count(*) FROM comments) || ' comments, ' ||
		(SELECT count(*) FROM relations) || ' relations, ' || (SELECT count(*) FROM links) || ' links, ' ||
		(SELECT count(*) FROM events) || ' events, ' || (SELECT count(*) FROM project_teams) || ' project teams'`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

func TestImportLinear(t *testing.T) {
	root, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	ctx := context.Background()
	s := inWorkspace(t, root, "default")
	if _, err := s.CreateTeam(ctx, CreateTeam{Key: "LIN", Name: "Linear", StartNumber: new(20)}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateProject(ctx, "Delta", "Local Delta", "LIN"); err != nil {
		t.Fatal(err)
	}
	const fixture = "testdata/linear"

	small := inWorkspace(t, root, "small")
	if _, err := small.CreateTeam(ctx, CreateTeam{Key: "LIN", Name: "Linear", StartNumber: new(9)}); err != nil {
		t.Fatal(err)
	}

	// Checks before writing: the team must exist, and every number must be
	// below next_number.
	before := importedRows(t, s)
	if _, err := s.ImportLinear(ctx, "NOPE", fixture); !errors.Is(err, ErrTeamNotFound) {
		t.Fatalf("missing team: %v", err)
	}
	if _, err := small.ImportLinear(ctx, "LIN", fixture); err == nil || !strings.Contains(err.Error(), "LIN-9") {
		t.Fatalf("number not below next_number: %v", err)
	}

	// Input the mapping does not cover aborts the whole import.
	var issues []map[string]any
	data, err := os.ReadFile(filepath.Join(fixture, "issues.json"))
	if err != nil {
		t.Fatal(err)
	}
	projects, err := os.ReadFile(filepath.Join(fixture, "projects.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		change func(issue map[string]any)
		want   string
	}{
		{"unknown status", func(issue map[string]any) { issue["state"] = map[string]any{"name": "Duplicate"} }, `"Duplicate"`},
		{"unknown label", func(issue map[string]any) { issue["labels"] = []any{map[string]any{"name": "Frontend"}} }, `"Frontend"`},
		{"two kinds", func(issue map[string]any) {
			issue["labels"] = []any{map[string]any{"name": "Bug"}, map[string]any{"name": "Chore"}}
		}, "Bug"},
		{"unknown relation", func(issue map[string]any) {
			issue["relations"] = []any{map[string]any{"type": "duplicate", "relatedIssue": map[string]any{"identifier": "LIN-1"}}}
		}, `"duplicate"`},
		{"missing relation target", func(issue map[string]any) {
			issue["relations"] = []any{map[string]any{"type": "blocks", "relatedIssue": map[string]any{"identifier": "LIN-8"}}}
		}, "LIN-8"},
		{"comment without author", func(issue map[string]any) {
			issue["comments"] = []any{map[string]any{"body": "Who?", "createdAt": "2026-01-09T10:00:00.000Z", "user": nil, "botActor": nil}}
		}, "author"},
		{"identifier of another team", func(issue map[string]any) { issue["identifier"] = "OTHER-9" }, "OTHER-9"},
	} {
		if err := json.Unmarshal(data, &issues); err != nil {
			t.Fatal(err)
		}
		tc.change(issues[len(issues)-1])
		dir := t.TempDir()
		changed, err := json.Marshal(issues)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "issues.json"), changed, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "projects.json"), projects, 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := s.ImportLinear(ctx, "LIN", dir); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: err = %v, want it to name %s", tc.name, err, tc.want)
		}
	}
	if after := importedRows(t, s); after != before {
		t.Fatalf("failed imports wrote rows:\nbefore %s\nafter  %s", before, after)
	}

	result, err := s.ImportLinear(ctx, "LIN", fixture)
	if err != nil {
		t.Fatal(err)
	}
	if want := (LinearImport{Issues: 8, Comments: 4, Relations: 3, Links: 2, Projects: 4}); result != want {
		t.Fatalf("result = %#v, want %#v", result, want)
	}

	// Fields are kept as they are; the trashed LIN-8 is skipped. Statuses,
	// priority, type from labels, Project, and parent are mapped.
	var stored string
	if err := s.db.QueryRow(`SELECT group_concat(row, char(10)) FROM (SELECT identifier || ' ' || number || ' ' || title || ' [' || description || '] ' ||
		status || ' ' || priority || ' ' || ifnull(type, '-') || ' ' || ifnull((SELECT name FROM projects WHERE id = project_id), '-') || ' ' ||
		ifnull((SELECT p.identifier FROM issues p WHERE p.id = issues.parent_id), '-') || ' ' || created_at || ' ' || updated_at AS row
		FROM issues WHERE workspace_id = ? ORDER BY id)`, s.workspace).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"LIN-1 1 Plan the import [] triage 0 spec Alpha - 2026-01-01T09:00:00.000Z 2026-01-11T09:00:00.000Z",
		"LIN-2 2 Parse the snapshot [Read **issues** first.] backlog 1 spec Alpha LIN-1 2026-01-02T09:00:00.000Z 2026-01-12T09:00:00.000Z",
		"LIN-3 3 Link the documents [See the design.] todo 2 feature - LIN-1 2026-01-03T09:00:00.000Z 2026-01-13T09:00:00.000Z",
		"LIN-4 4 Fix the parser [] agent_ready 3 bug - - 2026-01-04T09:00:00.000Z 2026-01-14T09:00:00.000Z",
		"LIN-5 5 Speed up the import [] in_progress 4 improvement Beta - 2026-01-05T09:00:00.000Z 2026-01-15T09:00:00.000Z",
		"LIN-6 6 Tidy the scripts [] in_review 0 chore Gamma - 2026-01-06T09:00:00.000Z 2026-01-16T09:00:00.000Z",
		"LIN-7 7 Ship it [] done 0 - Delta - 2026-01-07T09:00:00.000Z 2026-01-17T09:00:00.000Z",
		"LIN-9 9 Drop the old plan [] canceled 0 - - - 2026-01-09T09:00:00.000Z 2026-01-19T09:00:00.000Z",
	}, "\n")
	if stored != want {
		t.Fatalf("issues:\n%s\nwant:\n%s", stored, want)
	}

	// Every Project is imported; one already in the workspace is matched by
	// name and left alone. Completed and canceled ones are archived, and
	// content follows the description after a blank line.
	listed, err := s.ListProjects(ctx, true)
	if wantProjects := []Project{
		{ID: 2, Teams: []string{"LIN"}, Name: "Alpha", Description: "Import the tracker.\n\n## Plan\n\nMove everything."},
		{ID: 3, Teams: []string{"LIN"}, Name: "Beta", Description: "Make it fast.", Archived: true},
		{ID: 1, Teams: []string{"LIN"}, Name: "Delta", Description: "Local Delta"},
		{ID: 6, Teams: []string{}, Name: "Epsilon", Description: "Nothing in it yet."},
		{ID: 4, Teams: []string{"LIN"}, Name: "Gamma", Description: "Only notes.", Archived: true},
	}; err != nil || !reflect.DeepEqual(listed, wantProjects) {
		t.Fatalf("projects = %#v, err = %v", listed, err)
	}

	// Comments are in time order with their authors, replies included;
	// non-GitHub attachments become links; relations use identifiers.
	detail, err := s.Details(ctx, "LIN-1")
	if err != nil {
		t.Fatal(err)
	}
	if wantComments := []Comment{
		{Body: "First, from a person.", CreatedAt: "2026-01-02T09:00:00.000Z", Author: "Ada Example"},
		{Body: "A reply from a bot.", CreatedAt: "2026-01-03T09:00:00.000Z", Author: "Robot"},
		{Body: "Last, edited later.", CreatedAt: "2026-01-04T09:00:00.000Z", Author: "Bo Example"},
	}; !reflect.DeepEqual(detail.Comments, wantComments) {
		t.Fatalf("LIN-1 comments = %#v", detail.Comments)
	}
	identifiers := func(summaries []IssueSummary) []string {
		var result []string
		for _, summary := range summaries {
			result = append(result, summary.Identifier)
		}
		return result
	}
	if len(detail.Links) != 0 || !slices.Equal(identifiers(detail.Children), []string{"LIN-3", "LIN-2"}) || !slices.Equal(identifiers(detail.Related), []string{"LIN-4"}) {
		t.Fatalf("LIN-1 = %#v", detail)
	}
	detail, err = s.Details(ctx, "LIN-3")
	if wantLinks := []Link{{Title: "Design notes", URL: "https://example.com/design"}, {Title: "Chat thread", URL: "https://chat.example.com/thread/1"}}; err != nil ||
		!reflect.DeepEqual(detail.Links, wantLinks) || !slices.Equal(identifiers(detail.BlockedBy), []string{"LIN-2"}) ||
		!reflect.DeepEqual(detail.Comments, []Comment{{Body: "Only comment.", CreatedAt: "2026-01-06T09:00:00.000Z", Author: "Bo Example"}}) {
		t.Fatalf("LIN-3 = %#v, err = %v", detail, err)
	}
	detail, err = s.Details(ctx, "LIN-2")
	if err != nil || !slices.Equal(identifiers(detail.Blocks), []string{"LIN-3"}) || !slices.Equal(identifiers(detail.Related), []string{"LIN-4"}) {
		t.Fatalf("LIN-2 = %#v, err = %v", detail, err)
	}
	var relations string
	if err := s.db.QueryRow(`SELECT group_concat(a.identifier || ' ' || kind || ' ' || b.identifier, ', ') FROM relations
		JOIN issues a ON a.id = issue_id JOIN issues b ON b.id = other_id`).Scan(&relations); err != nil || relations != "LIN-1 related LIN-4, LIN-2 blocks LIN-3, LIN-2 related LIN-4" {
		t.Fatalf("relations = %q, err = %v", relations, err)
	}

	// Each issue has one created event at its createdAt; numbering is unchanged.
	var events string
	if err := s.db.QueryRow(`SELECT group_concat(identifier || ' ' || kind || ' ' || detail || ' ' || events.created_at, ', ') FROM events
		JOIN issues ON issues.id = issue_id WHERE identifier IN ('LIN-1', 'LIN-9')`).Scan(&events); err != nil ||
		events != "LIN-1 created triage 2026-01-01T09:00:00.000Z, LIN-9 created canceled 2026-01-09T09:00:00.000Z" {
		t.Fatalf("events = %q, err = %v", events, err)
	}
	var count int
	if err := s.db.QueryRow(`SELECT count(*) FROM events JOIN issues ON issues.id = issue_id WHERE workspace_id = ? AND kind = 'created'`, s.workspace).Scan(&count); err != nil || count != 8 {
		t.Fatalf("created events = %d, err = %v", count, err)
	}
	teams, err := s.ListTeams(ctx)
	if err != nil || !slices.Equal(teams, []Team{{Key: "LIN", Name: "Linear", NextNumber: 20}}) {
		t.Fatalf("teams = %#v, err = %v", teams, err)
	}

	// Importing again finds every number taken and writes nothing.
	before = importedRows(t, s)
	_, err = s.ImportLinear(ctx, "LIN", fixture)
	if err == nil || !strings.Contains(err.Error(), "LIN-1, LIN-2, LIN-3, LIN-4, LIN-5, LIN-6, LIN-7, LIN-9") {
		t.Fatalf("second import: %v", err)
	}
	if after := importedRows(t, s); after != before {
		t.Fatalf("second import wrote rows:\nbefore %s\nafter  %s", before, after)
	}
	if created, err := s.Create(ctx, CreateIssue{Team: "LIN", Title: "After import"}); err != nil || created.Identifier != "LIN-20" {
		t.Fatalf("after import: %#v, %v", created, err)
	}
}
