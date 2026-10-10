package app

import (
	"context"
	"os"
	"path/filepath"
	"sort"
)

const backupsKept = 14

// Backup writes today's copy of the database unless it already exists, then
// keeps only the newest daily copies. Copies named before-migration-* are left
// alone.
func (s *Service) Backup(ctx context.Context) (string, error) {
	if err := os.MkdirAll(s.backupDir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(s.backupDir, "daily-"+s.now().UTC().Format("2006-01-02")+".db")
	if _, err := os.Stat(path); err == nil {
		return "", nil
	}
	if _, err := s.db.ExecContext(ctx, `VACUUM INTO ?`, path); err != nil {
		return "", err
	}
	daily, err := filepath.Glob(filepath.Join(s.backupDir, "daily-*.db"))
	if err != nil {
		return "", err
	}
	sort.Strings(daily)
	for len(daily) > backupsKept {
		if err := os.Remove(daily[0]); err != nil {
			return "", err
		}
		daily = daily[1:]
	}
	return path, nil
}
