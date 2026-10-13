package app

import (
	"reflect"
	"testing"
	"time"
)

func TestIssueDelegationStartsProgress(t *testing.T) {
	for _, status := range Statuses {
		t.Run(string(status), func(t *testing.T) {
			_, s, admin, _ := accessFixture(t)
			if _, err := s.CreateTeam(admin, CreateTeam{Key: "AL", Name: "Alpha"}); err != nil {
				t.Fatal(err)
			}
			issue, err := s.Create(admin, CreateIssue{Team: "AL", Title: "Work", Status: &status, Assignee: "admin"})
			if err != nil {
				t.Fatal(err)
			}
			now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
			s.now = func() time.Time { return now }
			input := DelegateIssue{Identifier: issue.Identifier, ClientContext: delegationContext()}
			first, err := s.Delegate(admin, input)
			if err != nil {
				t.Fatal(err)
			}
			after, err := s.Details(admin, issue.Identifier)
			if err != nil || after.Status != StatusInProgress {
				t.Fatal(after, err)
			}
			wantTime := now.Format(time.RFC3339Nano)
			wantEvents := 1
			if status == StatusInProgress {
				wantTime = issue.UpdatedAt
				wantEvents = 0
			}
			if after.UpdatedAt != wantTime {
				t.Fatal(after.UpdatedAt, wantTime)
			}
			var events int
			if err := s.db.QueryRow(`SELECT count(*) FROM events WHERE kind='status_changed'`).Scan(&events); err != nil || events != wantEvents {
				t.Fatal(events, err)
			}
			if wantEvents == 1 {
				var detail, timestamp string
				if err := s.db.QueryRow(`SELECT detail,created_at FROM events WHERE kind='status_changed'`).Scan(&detail, &timestamp); err != nil || detail != string(status)+" -> in_progress" || timestamp != wantTime {
					t.Fatal(detail, timestamp, err)
				}
			}
			now = now.Add(time.Hour)
			duplicate, err := s.Delegate(admin, input)
			if err != nil || !reflect.DeepEqual(first, duplicate) {
				t.Fatal(duplicate, err)
			}
			input.ClientContext.AgentID = "child"
			if _, err := s.Delegate(admin, input); err != nil {
				t.Fatal(err)
			}
			repeated, err := s.Details(admin, issue.Identifier)
			if err != nil || repeated.UpdatedAt != wantTime || len(repeated.Delegations) != 2 {
				t.Fatal(repeated, err)
			}
			if err := s.db.QueryRow(`SELECT count(*) FROM events WHERE kind='status_changed'`).Scan(&events); err != nil || events != wantEvents {
				t.Fatal(events, err)
			}
			// A repeated execution identity also restarts work after a later transition.
			done := StatusDone
			if _, err := s.Update(admin, UpdateIssue{Identifier: issue.Identifier, Status: &done}); err != nil {
				t.Fatal(err)
			}
			input.ClientContext.AgentID = ""
			duplicate, err = s.Delegate(admin, input)
			if err != nil || !reflect.DeepEqual(first, duplicate) {
				t.Fatal(duplicate, err)
			}
			restarted, err := s.Get(admin, issue.Identifier)
			if err != nil || restarted.Status != StatusInProgress {
				t.Fatal(restarted, err)
			}
		})
	}
}

func TestIssueDelegationProgressRollback(t *testing.T) {
	for _, target := range []string{"UPDATE OF status ON issues", "INSERT ON events", "INSERT ON issue_delegations"} {
		t.Run(target, func(t *testing.T) {
			_, s, admin, _ := accessFixture(t)
			if _, err := s.CreateTeam(admin, CreateTeam{Key: "AL", Name: "Alpha"}); err != nil {
				t.Fatal(err)
			}
			issue, err := s.Create(admin, CreateIssue{Team: "AL", Title: "Work", Assignee: "admin"})
			if err != nil {
				t.Fatal(err)
			}
			before, err := s.Details(admin, issue.Identifier)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.db.Exec(`CREATE TRIGGER fail_delegation BEFORE ` + target + ` BEGIN SELECT RAISE(ABORT,'injected failure'); END`); err != nil {
				t.Fatal(err)
			}
			if _, err := s.Delegate(admin, DelegateIssue{Identifier: issue.Identifier, ClientContext: delegationContext()}); err == nil {
				t.Fatal("expected failure")
			}
			after, err := s.Details(admin, issue.Identifier)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatal("partial write", after, err)
			}
			var events int
			if err := s.db.QueryRow(`SELECT count(*) FROM events WHERE kind='status_changed'`).Scan(&events); err != nil || events != 0 {
				t.Fatal(events, err)
			}
		})
	}
}
