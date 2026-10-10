package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestBackupKeepsOneCopyPerDay(t *testing.T) {
	dir := t.TempDir()
	service, err := Open(filepath.Join(dir, "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	alTeam(t, service)
	day := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	service.now = func() time.Time { return day }
	ctx := context.Background()
	if _, err := service.Create(ctx, CreateIssue{Team: "AL", Title: "Keep me"}); err != nil {
		t.Fatal(err)
	}

	first, err := service.Backup(ctx)
	if err != nil {
		t.Fatal(err)
	}
	again, err := service.Backup(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || again != "" {
		t.Fatalf("first = %q, again = %q; want one copy per day", first, again)
	}
	for range 20 {
		day = day.AddDate(0, 0, 1)
		if _, err := service.Backup(ctx); err != nil {
			t.Fatal(err)
		}
	}

	daily, err := filepath.Glob(filepath.Join(dir, "backups", "daily-*.db"))
	if err != nil {
		t.Fatal(err)
	}
	if len(daily) != backupsKept || filepath.Base(daily[0]) != "daily-2026-10-08.db" {
		t.Fatalf("daily backups = %v", daily)
	}
	restored, err := Open(daily[len(daily)-1])
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if issues, err := restored.List(ctx, ListIssues{}); err != nil || len(issues) != 1 {
		t.Fatalf("issues in backup = %#v, err = %v", issues, err)
	}
}
