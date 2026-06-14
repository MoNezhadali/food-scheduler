package database

import (
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strings"
	"time"
)

// RunMigrations applies all pending *.up.sql files from migrFS under the
// given dialect subdirectory (e.g. "sqlite"). Files are sorted lexicographically
// so naming them 000001_name.up.sql guarantees order.
// Applied versions are tracked in the _schema_migrations table.
func RunMigrations(db *sql.DB, migrFS fs.FS, dialect string) error {
	if _, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS _schema_migrations (
			version    INTEGER NOT NULL PRIMARY KEY,
			filename   TEXT    NOT NULL,
			applied_at TEXT    NOT NULL
		)
	`); err != nil {
		return fmt.Errorf("create migrations table: %w", err)
	}

	applied, err := appliedVersions(db)
	if err != nil {
		return err
	}

	files, err := fs.Glob(migrFS, dialect+"/*.up.sql")
	if err != nil {
		return fmt.Errorf("glob migrations: %w", err)
	}
	sort.Strings(files)

	for _, f := range files {
		ver, err := versionFromFilename(f)
		if err != nil {
			return err
		}
		if applied[ver] {
			continue
		}
		if err := applyMigration(db, migrFS, f, ver, dialect); err != nil {
			return err
		}
	}
	return nil
}

func appliedVersions(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query(`SELECT version FROM _schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	applied := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		applied[v] = true
	}
	return applied, rows.Err()
}

func applyMigration(db *sql.DB, migrFS fs.FS, filename string, version int, dialect string) error {
	content, err := fs.ReadFile(migrFS, filename)
	if err != nil {
		return fmt.Errorf("read migration %s: %w", filename, err)
	}
	if _, err := db.Exec(string(content)); err != nil {
		return fmt.Errorf("apply migration %s: %w", filename, err)
	}
	_, err = db.Exec(
		trackingInsertSQL(dialect),
		version, filename, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

func trackingInsertSQL(dialect string) string {
	if dialect == "postgres" {
		return `INSERT INTO _schema_migrations (version, filename, applied_at) VALUES ($1, $2, $3)`
	}
	return `INSERT INTO _schema_migrations (version, filename, applied_at) VALUES (?, ?, ?)`
}

func versionFromFilename(path string) (int, error) {
	base := path
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	var version int
	if _, err := fmt.Sscanf(base, "%d", &version); err != nil {
		return 0, fmt.Errorf("cannot parse version from %q", path)
	}
	return version, nil
}
