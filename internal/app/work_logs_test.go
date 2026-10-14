package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestWorkLogLifecycle(t *testing.T) {
	root, s, admin, member := accessFixture(t)
	if _, err := s.CreateTeam(admin, CreateTeam{Key: "AL", Name: "Alpha"}); err != nil {
		t.Fatal(err)
	}
	issue, err := s.Create(admin, CreateIssue{Team: "AL", Title: "Records"})
	if err != nil {
		t.Fatal(err)
	}
	comment, err := s.AddComment(admin, issue.Identifier, "Human feedback")
	if err != nil {
		t.Fatal(err)
	}
	input := AddWorkLog{Identifier: issue.Identifier, Summary: "Investigated", Body: "Detailed findings"}
	progress, err := s.AddWorkLog(member, input)
	if err != nil || progress.Kind != "progress" || progress.Actor.Subject != "member" {
		t.Fatal(progress, err)
	}
	input.Kind, input.Summary, input.Body = "decision", "Keep historical comments", "Do not guess their authors."
	decision, err := s.AddWorkLog(member, input)
	if err != nil {
		t.Fatal(err)
	}
	detail, err := s.Details(admin, issue.Identifier)
	if err != nil || detail.WorkLogCount != 2 || !reflect.DeepEqual(detail.Comments, []Comment{comment}) || !reflect.DeepEqual(detail.Decisions, []WorkLogSummary{decision.WorkLogSummary}) || !reflect.DeepEqual(detail.Issue, issue) {
		t.Fatal(detail, err)
	}
	page, err := s.ListWorkLogs(admin, ListWorkLogs{Identifier: issue.Identifier, Limit: 1})
	if err != nil || len(page.WorkLogs) != 1 || page.WorkLogs[0].ID != decision.ID || page.NextBefore != decision.ID {
		t.Fatal(page, err)
	}
	// Inserting between pages must not duplicate or skip older records.
	if _, err = s.AddWorkLog(member, input); err != nil {
		t.Fatal(err)
	}
	page, err = s.ListWorkLogs(admin, ListWorkLogs{Identifier: issue.Identifier, Limit: 1, Before: page.NextBefore})
	if err != nil || len(page.WorkLogs) != 1 || page.WorkLogs[0].ID != progress.ID || page.NextBefore != 0 {
		t.Fatal(page, err)
	}
	page, err = s.ListWorkLogs(admin, ListWorkLogs{Identifier: issue.Identifier, Kind: "decision"})
	if err != nil || len(page.WorkLogs) != 2 || page.NextBefore != 0 {
		t.Fatal(page, err)
	}
	got, err := s.GetWorkLog(admin, GetWorkLog{Identifier: issue.Identifier, ID: decision.ID})
	if err != nil || !reflect.DeepEqual(got, decision) {
		t.Fatal(got, err)
	}
	for _, mutate := range []func(*AddWorkLog){
		func(i *AddWorkLog) { i.Kind = "unknown" }, func(i *AddWorkLog) { i.Summary = " " },
		func(i *AddWorkLog) { i.Summary = "two\nlines" }, func(i *AddWorkLog) { i.Summary = strings.Repeat("字", 241) },
		func(i *AddWorkLog) { i.Body = "\n " }, func(i *AddWorkLog) { i.Body = strings.Repeat("x", 65537) },
	} {
		invalid := input
		mutate(&invalid)
		if _, err := s.AddWorkLog(member, invalid); !errors.Is(err, ErrInvalidWorkLog) {
			t.Fatal(invalid.Kind, err)
		}
	}
	for _, query := range []ListWorkLogs{
		{Identifier: issue.Identifier, Kind: "unknown"}, {Identifier: issue.Identifier, Limit: -1},
		{Identifier: issue.Identifier, Limit: 101}, {Identifier: issue.Identifier, Before: -1},
	} {
		if _, err := s.ListWorkLogs(member, query); !errors.Is(err, ErrInvalidWorkLogQuery) {
			t.Fatal(query, err)
		}
	}
	if _, err = root.db.Exec(`CREATE TRIGGER fail_work_log BEFORE INSERT ON work_logs BEGIN SELECT RAISE(ABORT,'storage unavailable'); END;`); err != nil {
		t.Fatal(err)
	}
	if _, err = s.AddWorkLog(member, input); err == nil {
		t.Fatal("accepted failed insert")
	}
	detail, err = s.Details(admin, issue.Identifier)
	if err != nil || detail.WorkLogCount != 3 || len(detail.Comments) != 1 {
		t.Fatal(detail, err)
	}
}

func TestWorkLogAuthorization(t *testing.T) {
	root, s, admin, member := accessFixture(t)
	if _, err := s.CreateTeam(admin, CreateTeam{Key: "SEC", Name: "Secret", Private: true}); err != nil {
		t.Fatal(err)
	}
	issue, err := s.Create(admin, CreateIssue{Team: "SEC", Title: "Secret"})
	if err != nil {
		t.Fatal(err)
	}
	input := AddWorkLog{Identifier: issue.Identifier, Kind: "decision", Summary: "Private choice", Body: "Private reasons"}
	record, err := s.AddWorkLog(admin, input)
	if err != nil {
		t.Fatal(err)
	}
	reject := func(service *Service, ctx context.Context, want error) {
		t.Helper()
		if _, err := service.AddWorkLog(ctx, input); !errors.Is(err, want) {
			t.Fatal("add", err)
		}
		if _, err := service.ListWorkLogs(ctx, ListWorkLogs{Identifier: issue.Identifier}); !errors.Is(err, want) {
			t.Fatal("list", err)
		}
		if _, err := service.GetWorkLog(ctx, GetWorkLog{Identifier: issue.Identifier, ID: record.ID}); !errors.Is(err, want) {
			t.Fatal("get", err)
		}
		if _, err := service.Details(ctx, issue.Identifier); !errors.Is(err, want) {
			t.Fatal("detail", err)
		}
	}
	reject(s, member, ErrNotFound)
	reject(s, context.Background(), ErrIdentityRequired)
	if _, err = root.CreateWorkspace(admin, Workspace{Slug: "other", Name: "Other"}); err != nil {
		t.Fatal(err)
	}
	other, err := root.InWorkspace(admin, "other")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = other.CreateTeam(admin, CreateTeam{Key: "SEC", Name: "Other"}); err != nil {
		t.Fatal(err)
	}
	if _, err = other.Create(admin, CreateIssue{Team: "SEC", Title: "Same identifier"}); err != nil {
		t.Fatal(err)
	}
	page, err := other.ListWorkLogs(admin, ListWorkLogs{Identifier: issue.Identifier})
	if err != nil || len(page.WorkLogs) != 0 {
		t.Fatal(page, err)
	}
	if _, err = other.GetWorkLog(admin, GetWorkLog{Identifier: issue.Identifier, ID: record.ID}); !errors.Is(err, ErrNotFound) {
		t.Fatal(err)
	}
	if err = s.ChangeTeamMember(admin, ChangeTeamMember{Key: "SEC", Subject: "member", Role: "member"}); err != nil {
		t.Fatal(err)
	}
	if _, err = s.GetWorkLog(member, GetWorkLog{Identifier: issue.Identifier, ID: record.ID}); err != nil {
		t.Fatal(err)
	}
	if err = s.UpdateMember(admin, UpdateMember{Subject: "member", Role: "member", Status: "suspended"}); err != nil {
		t.Fatal(err)
	}
	reject(s, member, ErrAccessDenied)
}

func TestWorkLogsUpgrade(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "upgrade.db")
	root, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s := inWorkspace(t, root, "default")
	alTeam(t, s)
	ctx := context.Background()
	issue, err := s.Create(ctx, CreateIssue{Team: "AL", Title: "Preserve"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.AddComment(ctx, issue.Identifier, "Legacy agent or human"); err != nil {
		t.Fatal(err)
	}
	removeCycleSchema(t, root.db)
	if _, err = root.db.Exec(`UPDATE comments SET author='Imported author'; DROP TABLE work_logs; PRAGMA user_version=12;`); err != nil {
		t.Fatal(err)
	}
	root.Close()
	root, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	fresh, err := Open(filepath.Join(t.TempDir(), "fresh.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	if !reflect.DeepEqual(structure(t, root.db), structure(t, fresh.db)) {
		t.Fatal("fresh/upgrade mismatch")
	}
	if _, err = os.Stat(filepath.Join(dir, "backups", "before-migration-13.db")); err != nil {
		t.Fatal(err)
	}
	s, err = root.InWorkspace(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}
	detail, err := s.Details(ctx, issue.Identifier)
	if err != nil || !reflect.DeepEqual(detail.Issue, issue) || len(detail.Comments) != 1 || detail.Comments[0].Author != "Imported author" || detail.Comments[0].Body != "Legacy agent or human" || detail.WorkLogCount != 0 || len(detail.Decisions) != 0 {
		t.Fatal(detail, err)
	}
	if _, err = s.AddWorkLog(ctx, AddWorkLog{Identifier: issue.Identifier, Summary: "New log", Body: "Works after upgrade"}); err != nil {
		t.Fatal(err)
	}
	var integrity string
	if err = root.db.QueryRow(`PRAGMA integrity_check`).Scan(&integrity); err != nil || integrity != "ok" {
		t.Fatal(integrity, err)
	}
	rows, err := root.db.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() || rows.Err() != nil {
		t.Fatal("foreign key violation", rows.Err())
	}
}
