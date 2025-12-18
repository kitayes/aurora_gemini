package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func RunMigrations(db *sql.DB, dir string) error {
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
  name TEXT PRIMARY KEY,
  applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)

	for _, fname := range files {
		var exists int
		if err := db.QueryRow(`SELECT 1 FROM schema_migrations WHERE name=? LIMIT 1`, fname).Scan(&exists); err != nil && err != sql.ErrNoRows {
			return fmt.Errorf("check migration %s: %w", fname, err)
		}
		if exists == 1 {
			continue
		}

		path := filepath.Join(dir, fname)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		tx, err := db.Begin()
		if err != nil {
			return err
		}

		if _, err := tx.Exec(string(data)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migration %s failed: %w", fname, err)
		}

		if _, err := tx.Exec(`INSERT INTO schema_migrations(name) VALUES(?)`, fname); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s failed: %w", fname, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s failed: %w", fname, err)
		}
	}

	return nil
}
