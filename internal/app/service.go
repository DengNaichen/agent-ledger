package app

import (
	"database/sql"
	_ "embed"
	"errors"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// Service reads and writes the database. A Service returned by InWorkspace
// reads and writes one workspace; Open returns one that manages workspaces,
// backups, and GitHub sync.
type Service struct {
	db        *sql.DB
	backupDir string
	now       func() time.Time
	workspace int64
}

func Open(path string) (*Service, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("database path is required")
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)

	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, err
		}
	}
	backupDir := filepath.Join(filepath.Dir(path), "backups")
	if err := migrate(db, backupDir, migrations); err != nil {
		db.Close()
		return nil, err
	}

	return &Service{db: db, backupDir: backupDir, now: time.Now}, nil
}

func (s *Service) Close() error { return s.db.Close() }
