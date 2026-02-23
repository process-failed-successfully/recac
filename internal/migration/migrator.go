package migration

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Migrator handles database migrations.
type Migrator struct {
	DB     *sql.DB
	Dir    string
	Driver string // "sqlite" or "postgres"
}

// NewMigrator creates a new Migrator instance.
func NewMigrator(db *sql.DB, dir, driver string) *Migrator {
	return &Migrator{
		DB:     db,
		Dir:    dir,
		Driver: driver,
	}
}

// EnsureTable ensures the schema_migrations table exists.
func (m *Migrator) EnsureTable() error {
	// Works for SQLite and Postgres
	query := `CREATE TABLE IF NOT EXISTS schema_migrations (
        version VARCHAR(14) PRIMARY KEY,
        name VARCHAR(255),
        applied_at TIMESTAMP
    );`
	_, err := m.DB.Exec(query)
	return err
}

// GetStatus returns the applied and pending migrations.
func (m *Migrator) GetStatus() (*Status, error) {
	if err := m.EnsureTable(); err != nil {
		return nil, err
	}

	// 1. Get Applied from DB
	rows, err := m.DB.Query("SELECT version, name, applied_at FROM schema_migrations ORDER BY version ASC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var applied []Migration
	appliedMap := make(map[string]bool)

	for rows.Next() {
		var mig Migration
		// Try scanning directly. modernc.org/sqlite and lib/pq both support scanning TIMESTAMP to time.Time
		if err := rows.Scan(&mig.Version, &mig.Name, &mig.AppliedAt); err != nil {
			return nil, err
		}
		applied = append(applied, mig)
		appliedMap[mig.Version] = true
	}

	// 2. Get Files from Dir
	files, err := os.ReadDir(m.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			// No dir = no pending
			return &Status{Applied: applied, Pending: []Migration{}}, nil
		}
		return nil, err
	}

	// Collect all available migrations from files
	available := make(map[string]Migration)

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		name := f.Name()
		if strings.HasSuffix(name, ".up.sql") {
			// version is first 14 chars (YYYYMMDDHHMMSS)
			if len(name) < 15 {
				continue
			}
			version := name[:14]
			migName := strings.TrimSuffix(name[15:], ".up.sql")

			// Store or update
			mig := available[version]
			mig.Version = version
			mig.Name = migName
			mig.UpFile = filepath.Join(m.Dir, name)
			available[version] = mig
		} else if strings.HasSuffix(name, ".down.sql") {
			if len(name) < 15 {
				continue
			}
			version := name[:14]
			mig := available[version]
			mig.DownFile = filepath.Join(m.Dir, name)
			available[version] = mig
		}
	}

	// Identify Pending
	var pending []Migration
	var versions []string
	for k := range available {
		versions = append(versions, k)
	}
	sort.Strings(versions)

	for _, v := range versions {
		if !appliedMap[v] {
			pending = append(pending, available[v])
		}
	}

	return &Status{
		Applied: applied,
		Pending: pending,
	}, nil
}

// Up applies pending migrations.
func (m *Migrator) Up(steps int) error {
	status, err := m.GetStatus()
	if err != nil {
		return err
	}

	if len(status.Pending) == 0 {
		return nil // Nothing to do
	}

	limit := len(status.Pending)
	if steps > 0 && steps < limit {
		limit = steps
	}

	for i := 0; i < limit; i++ {
		mig := status.Pending[i]
		if err := m.applyUp(mig); err != nil {
			return fmt.Errorf("failed to apply migration %s: %w", mig.Version, err)
		}
		fmt.Printf("Applied %s_%s\n", mig.Version, mig.Name)
	}
	return nil
}

// Down reverts applied migrations.
func (m *Migrator) Down(steps int) error {
	status, err := m.GetStatus()
	if err != nil {
		return err
	}

	if len(status.Applied) == 0 {
		return nil
	}

	limit := len(status.Applied)
	if steps > 0 && steps < limit {
		limit = steps
	}

	// Reverse iterate
	for i := 0; i < limit; i++ {
		idx := len(status.Applied) - 1 - i
		mig := status.Applied[idx]

		downFile, err := m.findDownFile(mig.Version)
		if err != nil {
			return fmt.Errorf("failed to find down file for %s: %w", mig.Version, err)
		}
		mig.DownFile = downFile

		if err := m.applyDown(mig); err != nil {
			return fmt.Errorf("failed to revert migration %s: %w", mig.Version, err)
		}
		fmt.Printf("Reverted %s_%s\n", mig.Version, mig.Name)
	}
	return nil
}

func (m *Migrator) findDownFile(version string) (string, error) {
	files, err := os.ReadDir(m.Dir)
	if err != nil {
		return "", err
	}
	for _, f := range files {
		if strings.HasPrefix(f.Name(), version) && strings.HasSuffix(f.Name(), ".down.sql") {
			return filepath.Join(m.Dir, f.Name()), nil
		}
	}
	return "", fmt.Errorf("down migration file not found")
}

func (m *Migrator) bindVar(i int) string {
	if m.Driver == "postgres" {
		return fmt.Sprintf("$%d", i)
	}
	return "?"
}

func (m *Migrator) applyUp(mig Migration) error {
	content, err := os.ReadFile(mig.UpFile)
	if err != nil {
		return err
	}
	sqlContent := string(content)

	tx, err := m.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(sqlContent); err != nil {
		return err
	}

	query := fmt.Sprintf("INSERT INTO schema_migrations (version, name, applied_at) VALUES (%s, %s, %s)",
		m.bindVar(1), m.bindVar(2), m.bindVar(3))
	if _, err := tx.Exec(query, mig.Version, mig.Name, time.Now()); err != nil {
		return err
	}

	return tx.Commit()
}

func (m *Migrator) applyDown(mig Migration) error {
	content, err := os.ReadFile(mig.DownFile)
	if err != nil {
		return err
	}
	sqlContent := string(content)

	tx, err := m.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(sqlContent); err != nil {
		return err
	}

	query := fmt.Sprintf("DELETE FROM schema_migrations WHERE version = %s", m.bindVar(1))
	if _, err := tx.Exec(query, mig.Version); err != nil {
		return err
	}

	return tx.Commit()
}
