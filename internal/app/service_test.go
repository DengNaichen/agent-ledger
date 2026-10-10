package app

import (
	"context"
	"errors"
	"path/filepath"
	"slices"
	"testing"
)

// alTeam creates team AL, the team most tests number their issues in.
func alTeam(t *testing.T, s *Service) {
	t.Helper()
	if _, err := s.CreateTeam(context.Background(), CreateTeam{Key: "AL", Name: "InfTask"}); err != nil {
		t.Fatal(err)
	}
}

func TestIssueLifecycle(t *testing.T) {
	service, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	alTeam(t, service)

	ctx := context.Background()
	issue, err := service.Create(ctx, CreateIssue{Team: "AL", Title: "Ship the first board", Description: "One vertical slice."})
	if err != nil {
		t.Fatal(err)
	}
	if issue.Identifier != "AL-1" || issue.Status != StatusTriage {
		t.Fatalf("unexpected issue: %#v", issue)
	}

	issue, err = service.Update(ctx, UpdateIssue{Identifier: issue.Identifier, Status: new(StatusAgentReady)})
	if err != nil {
		t.Fatal(err)
	}
	if issue.Status != StatusAgentReady {
		t.Fatalf("status = %q, want %q", issue.Status, StatusAgentReady)
	}

	issues, err := service.List(ctx, ListIssues{Status: new(StatusAgentReady)})
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || issues[0].Identifier != issue.Identifier {
		t.Fatalf("unexpected list: %#v", issues)
	}

	// Repeating a status and editing other fields must not add status events.
	for _, update := range []UpdateIssue{
		{Identifier: issue.Identifier, Status: new(StatusAgentReady)},
		{Identifier: issue.Identifier, Description: new("Updated context")},
	} {
		if _, err := service.Update(ctx, update); err != nil {
			t.Fatal(err)
		}
	}
	var history string
	if err := service.db.QueryRowContext(ctx, `SELECT group_concat(kind || ':' || detail, ',') FROM (SELECT kind, detail FROM events ORDER BY id)`).Scan(&history); err != nil {
		t.Fatal(err)
	}
	if history != "created:triage,status_changed:triage -> agent_ready" {
		t.Fatalf("unexpected event history: %s", history)
	}
}

func TestIssueLifecycleNumbersPerTeam(t *testing.T) {
	service, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	ctx := context.Background()
	for _, input := range []CreateTeam{{Key: "AL", Name: "InfTask"}, {Key: "TOK", Name: " HarnessKit "}} {
		if team, err := service.CreateTeam(ctx, input); err != nil || team.NextNumber != 1 {
			t.Fatalf("create team %#v: %#v, %v", input, team, err)
		}
	}

	var identifiers []string
	for _, team := range []string{"AL", "TOK", "AL", "TOK", "TOK"} {
		issue, err := service.Create(ctx, CreateIssue{Team: team, Title: "Work"})
		if err != nil {
			t.Fatal(err)
		}
		identifiers = append(identifiers, issue.Identifier)
	}
	if want := []string{"AL-1", "TOK-1", "AL-2", "TOK-2", "TOK-3"}; !slices.Equal(identifiers, want) {
		t.Fatalf("identifiers = %q, want %q", identifiers, want)
	}

	// Rejected issues use up no number.
	for _, tc := range []struct {
		input CreateIssue
		want  error
	}{
		{CreateIssue{Title: "No team"}, ErrTeamRequired},
		{CreateIssue{Team: "NOPE", Title: "Unknown team"}, ErrTeamNotFound},
		{CreateIssue{Team: "al", Title: "Keys match exactly"}, ErrTeamNotFound},
		{CreateIssue{Team: "AL", Title: " "}, ErrInvalidTitle},
		{CreateIssue{Team: "AL", Title: "Missing Project", Project: "Missing"}, ErrProjectNotFound},
	} {
		if _, err := service.Create(ctx, tc.input); !errors.Is(err, tc.want) {
			t.Fatalf("create %#v: err = %v, want %v", tc.input, err, tc.want)
		}
	}
	if issue, err := service.Create(ctx, CreateIssue{Team: "AL", Title: "Next"}); err != nil || issue.Identifier != "AL-3" {
		t.Fatalf("issue = %#v, err = %v; want AL-3", issue, err)
	}

	for _, input := range []CreateTeam{
		{Key: "tok", Name: "Lowercase"}, {Key: "T", Name: "Too short"}, {Key: "TOOLONG1", Name: "Too long"},
		{Key: "1AB", Name: "Starts with a digit"}, {Key: "A-B", Name: "Punctuation"}, {Key: "OPS", Name: " "},
	} {
		if _, err := service.CreateTeam(ctx, input); !errors.Is(err, ErrInvalidTeam) {
			t.Fatalf("create team %#v: err = %v, want %v", input, err, ErrInvalidTeam)
		}
	}
	if _, err := service.CreateTeam(ctx, CreateTeam{Key: "AL", Name: "Again"}); err == nil {
		t.Fatal("created a second team with key AL")
	}
	teams, err := service.ListTeams(ctx)
	if want := []Team{{Key: "AL", Name: "InfTask", NextNumber: 4}, {Key: "TOK", Name: "HarnessKit", NextNumber: 4}}; err != nil || !slices.Equal(teams, want) {
		t.Fatalf("teams = %#v, err = %v; want %#v", teams, err, want)
	}

	issues, err := service.List(ctx, ListIssues{Team: "TOK"})
	if err != nil {
		t.Fatal(err)
	}
	identifiers = nil
	for _, issue := range issues {
		identifiers = append(identifiers, issue.Identifier)
	}
	if want := []string{"TOK-3", "TOK-2", "TOK-1"}; !slices.Equal(identifiers, want) {
		t.Fatalf("TOK issues = %q, want %q", identifiers, want)
	}
	if _, err := service.List(ctx, ListIssues{Team: "NOPE"}); !errors.Is(err, ErrTeamNotFound) {
		t.Fatalf("list unknown team: err = %v", err)
	}
}

func TestCreateTeamStartNumber(t *testing.T) {
	service, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	ctx := context.Background()
	start := 1227
	if team, err := service.CreateTeam(ctx, CreateTeam{Key: "TOK", Name: "TokfinityFde", StartNumber: &start}); err != nil || team.NextNumber != 1227 {
		t.Fatalf("team = %#v, err = %v; want next number 1227", team, err)
	}
	if issue, err := service.Create(ctx, CreateIssue{Team: "TOK", Title: "First after import"}); err != nil || issue.Identifier != "TOK-1227" {
		t.Fatalf("issue = %#v, err = %v; want TOK-1227", issue, err)
	}

	zero := 0
	if _, err := service.CreateTeam(ctx, CreateTeam{Key: "OPS", Name: "Ops", StartNumber: &zero}); !errors.Is(err, ErrInvalidTeam) {
		t.Fatalf("start number 0: err = %v, want %v", err, ErrInvalidTeam)
	}
	if _, err := service.CreateTeam(ctx, CreateTeam{Key: "TOK", Name: "Again"}); !errors.Is(err, ErrTeamExists) {
		t.Fatalf("duplicate key: err = %v, want %v", err, ErrTeamExists)
	}
	teams, err := service.ListTeams(ctx)
	if want := []Team{{Key: "TOK", Name: "TokfinityFde", NextNumber: 1228}}; err != nil || !slices.Equal(teams, want) {
		t.Fatalf("teams = %#v, err = %v; want %#v", teams, err, want)
	}
}
