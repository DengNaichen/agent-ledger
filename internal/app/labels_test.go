package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func labelService(t *testing.T) *Service {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	s = inWorkspace(t, s, "default")
	alTeam(t, s)
	return s
}
func mustLabel(t *testing.T, s *Service, name string, group int64) Label {
	t.Helper()
	l, err := s.CreateLabel(context.Background(), CreateLabel{Name: name, Color: "#5e6ad2", GroupID: group})
	if err != nil {
		t.Fatal(err)
	}
	return l
}
func mustIssue(t *testing.T, s *Service, title string) Issue {
	t.Helper()
	i, err := s.Create(context.Background(), CreateIssue{Team: "AL", Title: title})
	if err != nil {
		t.Fatal(err)
	}
	return i
}
func checkLabels(t *testing.T, s *Service, id string, want ...int64) {
	t.Helper()
	i, err := s.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	got := []int64{}
	for _, l := range i.Labels {
		got = append(got, l.ID)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("%s labels = %v, want %v", id, got, want)
	}
}
func TestLabelsLifecycle(t *testing.T) {
	s := labelService(t)
	ctx := context.Background()
	var count int
	if err := s.db.QueryRow(`SELECT count(*) FROM labels`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	group, err := s.SaveLabelGroup(ctx, SaveLabelGroup{Name: " Risk "})
	if err != nil || group.Name != "Risk" {
		t.Fatalf("group: %#v %v", group, err)
	}
	low := mustLabel(t, s, " Low ", group.ID)
	high := mustLabel(t, s, "High", group.ID)
	front := mustLabel(t, s, "Frontend", 0)
	if low.Name != "Low" {
		t.Fatal(low)
	}
	a := mustIssue(t, s, "A")
	b := mustIssue(t, s, "B")
	change := func(input ChangeIssueLabels) {
		t.Helper()
		if err := s.ChangeIssueLabels(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	change(ChangeIssueLabels{Identifiers: []string{a.Identifier, b.Identifier}, Add: []int64{low.ID, front.ID, front.ID}})
	checkLabels(t, s, a.Identifier, low.ID, front.ID)
	checkLabels(t, s, b.Identifier, low.ID, front.ID)
	before, err := s.Get(ctx, a.Identifier)
	if err != nil {
		t.Fatal(err)
	}
	change(ChangeIssueLabels{Identifiers: []string{a.Identifier}, Add: []int64{low.ID}})
	after, err := s.Get(ctx, a.Identifier)
	if err != nil || before.UpdatedAt != after.UpdatedAt {
		t.Fatalf("idempotent add changed timestamp: %v", err)
	}
	change(ChangeIssueLabels{Identifiers: []string{a.Identifier}, Add: []int64{high.ID}})
	checkLabels(t, s, a.Identifier, high.ID, front.ID)
	if _, err := s.db.Exec(`INSERT INTO issue_labels(issue_id,label_id) SELECT id, ? FROM issues WHERE identifier = ?`, low.ID, a.Identifier); err == nil {
		t.Fatal("database allowed two labels from one group")
	}
	renamed, err := s.UpdateLabel(ctx, UpdateLabel{ID: front.ID, Name: new("UI"), Description: new("User interface changes"), Color: new("#abcdef")})
	if err != nil || renamed.ID != front.ID || renamed.Description != "User interface changes" {
		t.Fatalf("rename: %#v %v", renamed, err)
	}
	if _, err := s.SaveLabelGroup(ctx, SaveLabelGroup{ID: group.ID, Name: "Impact"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateLabel(ctx, UpdateLabel{ID: high.ID, Archived: new(true)}); err != nil {
		t.Fatal(err)
	}
	change(ChangeIssueLabels{Identifiers: []string{a.Identifier}, Add: []int64{high.ID}})
	if err := s.ChangeIssueLabels(ctx, ChangeIssueLabels{Identifiers: []string{b.Identifier}, Add: []int64{high.ID}}); !errors.Is(err, ErrInvalidLabel) {
		t.Fatal(err)
	}
	checkLabels(t, s, b.Identifier, low.ID, front.ID)
	active, err := s.ListLabels(ctx, false)
	if err != nil || len(active) != 2 {
		t.Fatalf("active: %v %v", active, err)
	}
	all, err := s.ListLabels(ctx, true)
	if err != nil || len(all) != 3 {
		t.Fatalf("all: %v %v", all, err)
	}
	detail, err := s.Details(ctx, a.Identifier)
	if err != nil || len(detail.Labels) != 2 {
		t.Fatalf("detail: %#v %v", detail, err)
	}
	if _, err := s.UpdateLabel(ctx, UpdateLabel{ID: high.ID, Archived: new(false), Description: new("")}); err != nil {
		t.Fatal(err)
	}
	change(ChangeIssueLabels{Identifiers: []string{b.Identifier}, Add: []int64{high.ID}})
	change(ChangeIssueLabels{Identifiers: []string{a.Identifier, b.Identifier}, Remove: []int64{front.ID}})
	change(ChangeIssueLabels{Identifiers: []string{a.Identifier}, Remove: []int64{front.ID}})
	checkLabels(t, s, a.Identifier, high.ID)
	for _, input := range []CreateLabel{{Name: "low", Color: "#123456", GroupID: group.ID}, {Name: " ", Color: "#123456"}, {Name: "bad color", Color: "red"}, {Name: "bad group", Color: "#123456", GroupID: 999}} {
		if _, err := s.CreateLabel(ctx, input); !errors.Is(err, ErrInvalidLabel) {
			t.Fatalf("invalid create %#v: %v", input, err)
		}
	}
	mustLabel(t, s, "Low", 0)
	if _, err := s.SaveLabelGroup(ctx, SaveLabelGroup{Name: " impact "}); !errors.Is(err, ErrInvalidLabel) {
		t.Fatal(err)
	}
	if _, err := s.UpdateLabel(ctx, UpdateLabel{ID: 999, Name: new("missing")}); !errors.Is(err, ErrLabelNotFound) {
		t.Fatal(err)
	}
	if _, err := s.UpdateLabel(ctx, UpdateLabel{ID: low.ID, Name: new("HIGH")}); !errors.Is(err, ErrInvalidLabel) {
		t.Fatal(err)
	}
}
func TestLabelsAtomicityAndMerge(t *testing.T) {
	s := labelService(t)
	ctx := context.Background()
	a := mustIssue(t, s, "A")
	b := mustIssue(t, s, "B")
	g, err := s.SaveLabelGroup(ctx, SaveLabelGroup{Name: "Risk"})
	if err != nil {
		t.Fatal(err)
	}
	low := mustLabel(t, s, "low", g.ID)
	high := mustLabel(t, s, "high", g.ID)
	x := mustLabel(t, s, "X", 0)
	y := mustLabel(t, s, "Y", 0)
	if err := s.ChangeIssueLabels(ctx, ChangeIssueLabels{Identifiers: []string{a.Identifier, b.Identifier}, Add: []int64{low.ID, x.ID}}); err != nil {
		t.Fatal(err)
	}
	for _, input := range []ChangeIssueLabels{
		{Identifiers: []string{a.Identifier, "missing"}, Add: []int64{high.ID}, Remove: []int64{x.ID}},
		{Identifiers: []string{a.Identifier}, Add: []int64{low.ID, high.ID}},
		{Identifiers: []string{a.Identifier}, Add: []int64{x.ID}, Remove: []int64{x.ID}},
		{Identifiers: []string{a.Identifier}, Add: []int64{999}, Remove: []int64{x.ID}},
		{Identifiers: []string{a.Identifier}, Remove: []int64{999}},
		{},
	} {
		if err := s.ChangeIssueLabels(ctx, input); err == nil {
			t.Fatalf("accepted %#v", input)
		}
		checkLabels(t, s, a.Identifier, low.ID, x.ID)
	}
	if _, err := s.UpdateLabel(ctx, UpdateLabel{ID: high.ID, Archived: new(true)}); err != nil {
		t.Fatal(err)
	}
	if err := s.ChangeIssueLabels(ctx, ChangeIssueLabels{Identifiers: []string{a.Identifier, b.Identifier}, Add: []int64{y.ID, high.ID}, Remove: []int64{x.ID}}); err == nil {
		t.Fatal("applied archived label")
	}
	checkLabels(t, s, a.Identifier, low.ID, x.ID)
	checkLabels(t, s, b.Identifier, low.ID, x.ID)
	for _, input := range []MergeLabels{{SourceID: x.ID, TargetID: low.ID}, {SourceID: low.ID, TargetID: high.ID}, {SourceID: x.ID, TargetID: x.ID}, {SourceID: 999, TargetID: x.ID}} {
		if err := s.MergeLabels(ctx, input); err == nil {
			t.Fatalf("accepted merge %#v", input)
		}
	}
	if err := s.ChangeIssueLabels(ctx, ChangeIssueLabels{Identifiers: []string{a.Identifier}, Add: []int64{y.ID}}); err != nil {
		t.Fatal(err)
	}
	if err := s.MergeLabels(ctx, MergeLabels{SourceID: x.ID, TargetID: y.ID}); err != nil {
		t.Fatal(err)
	}
	checkLabels(t, s, a.Identifier, low.ID, y.ID)
	checkLabels(t, s, b.Identifier, low.ID, y.ID)
	if _, err := s.UpdateLabel(ctx, UpdateLabel{ID: high.ID, Archived: new(false)}); err != nil {
		t.Fatal(err)
	}
	if err := s.MergeLabels(ctx, MergeLabels{SourceID: low.ID, TargetID: high.ID}); err != nil {
		t.Fatal(err)
	}
	checkLabels(t, s, a.Identifier, high.ID, y.ID)
	if _, err := s.UpdateLabel(ctx, UpdateLabel{ID: low.ID, Name: new("gone")}); !errors.Is(err, ErrLabelNotFound) {
		t.Fatal(err)
	}
}
func TestLabelFilters(t *testing.T) {
	s := labelService(t)
	ctx := context.Background()
	g, err := s.SaveLabelGroup(ctx, SaveLabelGroup{Name: "Risk"})
	if err != nil {
		t.Fatal(err)
	}
	low := mustLabel(t, s, "low", g.ID)
	high := mustLabel(t, s, "high", g.ID)
	x := mustLabel(t, s, "X", 0)
	a := mustIssue(t, s, "A")
	b := mustIssue(t, s, "B")
	c := mustIssue(t, s, "C")
	for _, input := range []ChangeIssueLabels{{Identifiers: []string{a.Identifier}, Add: []int64{low.ID, x.ID}}, {Identifiers: []string{b.Identifier}, Add: []int64{high.ID}}} {
		if err := s.ChangeIssueLabels(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.UpdateLabel(ctx, UpdateLabel{ID: low.ID, Archived: new(true)}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		input ListIssues
		want  []string
	}{
		{ListIssues{LabelIDs: []int64{low.ID, high.ID}}, []string{a.Identifier, b.Identifier}},
		{ListIssues{LabelIDs: []int64{low.ID, x.ID}, LabelMode: "all"}, []string{a.Identifier}},
		{ListIssues{LabelIDs: []int64{low.ID, high.ID}, LabelMode: "all"}, []string{}},
		{ListIssues{LabelIDs: []int64{x.ID}, LabelMode: "none"}, []string{b.Identifier, c.Identifier}},
		{ListIssues{LabelMode: "unlabeled"}, []string{c.Identifier}},
		{ListIssues{LabelGroupIDs: []int64{g.ID}}, []string{a.Identifier, b.Identifier}},
		{ListIssues{LabelGroupIDs: []int64{g.ID}, LabelIDs: []int64{x.ID}, LabelMode: "all"}, []string{a.Identifier}},
		{ListIssues{LabelGroupIDs: []int64{g.ID}, Query: "B"}, []string{b.Identifier}},
		{ListIssues{LabelGroupIDs: []int64{g.ID}, LabelMode: "none"}, []string{c.Identifier}},
	} {
		issues, err := s.List(ctx, tc.input)
		if err != nil {
			t.Fatal(err)
		}
		got := []string{}
		for _, i := range issues {
			got = append(got, i.Identifier)
		}
		slices.Sort(got)
		slices.Sort(tc.want)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("%#v: %v want %v", tc.input, got, tc.want)
		}
		for _, summary := range Summaries(issues) {
			if summary.Labels == nil {
				t.Fatal("nil labels in summary")
			}
		}
	}
	for _, input := range []ListIssues{{LabelMode: "bogus"}, {LabelMode: "unlabeled", LabelIDs: []int64{x.ID}}, {LabelIDs: []int64{999}}, {LabelGroupIDs: []int64{999}}} {
		if _, err := s.List(ctx, input); err == nil {
			t.Fatalf("accepted %#v", input)
		}
	}
}
func TestLabelUpgrade(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	// Label tables come last in the schema. A database at the current
	// migration version without them gets them on open, with no migration.
	old := strings.Split(schema, "CREATE TABLE IF NOT EXISTS label_groups")[0] + "CREATE TABLE users(subject TEXT PRIMARY KEY NOT NULL,name TEXT NOT NULL,email TEXT NOT NULL);"
	if _, err = db.Exec(old + fmt.Sprintf(`PRAGMA user_version = %d;`, len(migrations))); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO workspaces(id,slug,name) VALUES (1,'default','Default'); INSERT INTO teams(id,workspace_id,key,name,next_number) VALUES (1,1,'AL','InfTask',2); INSERT INTO issues(id,workspace_id,identifier,team_id,number,title,description,status,created_at,updated_at) VALUES (1,1,'AL-1',1,1,'Keep me','Existing body','todo','before','before'); INSERT INTO comments(issue_id,body,created_at) VALUES (1,'Keep comment','before')`); err != nil {
		t.Fatal(err)
	}
	db.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	def, err := s.InWorkspace(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	l := mustLabel(t, def, "New label", 0)
	if err := def.ChangeIssueLabels(ctx, ChangeIssueLabels{Identifiers: []string{"AL-1"}, Add: []int64{l.ID}}); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if def, err = s.InWorkspace(ctx, "default"); err != nil {
		t.Fatal(err)
	}
	detail, err := def.Details(ctx, "AL-1")
	if err != nil || detail.Title != "Keep me" || detail.Description != "Existing body" || len(detail.Comments) != 1 || len(detail.Labels) != 1 {
		t.Fatalf("lost existing data: %#v %v", detail, err)
	}
	// Additive schema installation does not invoke historical migrations or require a backup.
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "backups")); !os.IsNotExist(err) {
		t.Fatalf("unexpected migration backup: %v", err)
	}
}
