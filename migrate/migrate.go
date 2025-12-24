package migrate

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"sort"
	"strings"
)

type Migrator struct {
	DB         *sql.DB
	FolderPath string
}

func NewMigrator(db *sql.DB, folder string) *Migrator {
	return &Migrator{
		DB:         db,
		FolderPath: folder,
	}
}

func (m *Migrator) EnsureSchemaTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(255) PRIMARY KEY,
		applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`
	_, err := m.DB.Exec(query)
	return err
}

func (m *Migrator) AppliedVersions() (map[string]bool, error) {
	rows, err := m.DB.Query("SELECT version FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	applied := map[string]bool{}
	for rows.Next() {
		var v string
		rows.Scan(&v)
		applied[v] = true
	}
	return applied, nil
}

func (m *Migrator) RunMigrations() error {
	// Ensure schema table
	if err := m.EnsureSchemaTable(); err != nil {
		return fmt.Errorf("ensure schema table: %w", err)
	}

	applied, err := m.AppliedVersions()
	if err != nil {
		return fmt.Errorf("get applied versions: %w", err)
	}

	// Scan migration folder
	files, err := filepath.Glob(filepath.Join(m.FolderPath, "*.sql"))
	if err != nil {
		return err
	}

	// Sort lexicographically (001, 002, 003...)
	sort.Strings(files)

	for _, file := range files {
		version := filepath.Base(file)

		if applied[version] {
			continue
		}

		sqlBytes, err := ioutil.ReadFile(file)
		if err != nil {
			return fmt.Errorf("read file %s: %w", file, err)
		}

		statements := strings.Split(string(sqlBytes), ";")

		tx, err := m.DB.Begin()
		if err != nil {
			return err
		}

		for _, stmt := range statements {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}

			if _, err := tx.Exec(stmt); err != nil {
				tx.Rollback()
				return fmt.Errorf("error executing %s: %w", version, err)
			}
		}

		if _, err := tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", version); err != nil {
			tx.Rollback()
			return err
		}

		if err := tx.Commit(); err != nil {
			return err
		}

		log.Printf("Migration applied: %s", version)
	}

	return nil
}
