package app

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// migrations upgrade an existing database one version at a time, and PRAGMA
// user_version counts how many have run. schema.sql always describes the
// current shape, so a new database starts at len(migrations) without running
// any. Append only; never edit a migration that has shipped.
var migrations = []string{}

func migrate(db *sql.DB, backupDir string, migrations []string) error {
	var tables int
	if err := db.QueryRow(`SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'issues'`).Scan(&tables); err != nil {
		return err
	}
	if tables == 0 {
		if _, err := db.Exec(schema); err != nil {
			return err
		}
		_, err := db.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, len(migrations)))
		return err
	}

	var version int
	if err := db.QueryRow(`PRAGMA user_version`).Scan(&version); err != nil {
		return err
	}
	if version < len(migrations) {
		if err := os.MkdirAll(backupDir, 0o755); err != nil {
			return err
		}
		if _, err := db.Exec(`VACUUM INTO ?`, filepath.Join(backupDir, fmt.Sprintf("before-migration-%d.db", version+1))); err != nil {
			return err
		}
		// Rebuilding a table, which changing a CHECK constraint requires, only
		// works with foreign keys off outside the transaction.
		if _, err := db.Exec(`PRAGMA foreign_keys = OFF`); err != nil {
			return err
		}
		for ; version < len(migrations); version++ {
			if err := runMigration(db, migrations[version], version+1); err != nil {
				return fmt.Errorf("migration %d: %w", version+1, err)
			}
		}
		if _, err := db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
			return err
		}
	}
	_, err := db.Exec(schema)
	return err
}

func runMigration(db *sql.DB, statements string, version int) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(statements); err != nil {
		return err
	}
	rows, err := tx.Query(`PRAGMA foreign_key_check`)
	if err != nil {
		return err
	}
	violated := rows.Next()
	rows.Close()
	if violated {
		return errors.New("leaves foreign key violations")
	}
	if _, err := tx.Exec(fmt.Sprintf(`PRAGMA user_version = %d`, version)); err != nil {
		return err
	}
	return tx.Commit()
}
